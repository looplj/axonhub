package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MoeForge/moejs"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	entplugin "github.com/looplj/axonhub/internal/ent/plugin"
	"github.com/looplj/axonhub/internal/ent/predicate"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/watcher"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/pkg/xcache/live"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/looplj/axonhub/internal/pkg/xredis"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/events"
)

const (
	pluginBypassReason = "plugin-manager"
	pluginCacheName    = "axonhub:plugins:scoped-hooks"
	pluginRedisChannel = "axonhub:cache:plugins"

	// errDatabasePacketTooLarge is returned when the database rejects the package blob.
	errDatabasePacketTooLarge = "插件包超过数据库允许的单次写入大小（max_allowed_packet）"

	defaultRefreshInterval = 30 * time.Second
	defaultDebounce        = 500 * time.Millisecond
)

var (
	// ErrGenerationClosed is returned when a retired generation is acquired.
	ErrGenerationClosed = errors.New("plugin generation is closed")
	// ErrResourceUnavailable means the UI tree is missing or incomplete.
	// The caller can retry after the manager rebuilds it.
	ErrResourceUnavailable = errors.New("plugin resource unavailable")

	errUpgradePreview = errors.New("plugin upgrade preview")
)

// pluginMetaColumns is every plugins column except package.
// Package bytes are read only by readPackage.
var pluginMetaColumns = []string{
	entplugin.FieldID,
	entplugin.FieldCreatedAt,
	entplugin.FieldUpdatedAt,
	entplugin.FieldDeletedAt,
	entplugin.FieldPluginID,
	entplugin.FieldName,
	entplugin.FieldVersion,
	entplugin.FieldDescription,
	entplugin.FieldManifest,
	entplugin.FieldPackageSha256,
	entplugin.FieldEnabled,
	entplugin.FieldPriority,
	entplugin.FieldGrantedPermissions,
	entplugin.FieldGrantedNetwork,
	entplugin.FieldConfig,
}

// RuntimeStatus is this process's view of one plugin. It is not stored.
type RuntimeStatus string

const (
	RuntimeStatusActive   RuntimeStatus = "active"
	RuntimeStatusDisabled RuntimeStatus = "disabled"
	RuntimeStatusError    RuntimeStatus = "error"
)

// RuntimeStatusEntry is one in-memory status row.
type RuntimeStatusEntry struct {
	ID       int
	PluginID string
	Status   RuntimeStatus
	Reason   string
}

// UpgradePreview is returned when an enabled plugin gains permissions, network
// rules, events, hooks, pages, or actions.
// Nothing is written until a later Install submits the full new grant.
type UpgradePreview struct {
	CurrentVersion string
	NewVersion     string
	Permissions    []string
	Network        []string
	Events         []string
	Hooks          []string
	Pages          []string
	Actions        []string
}

// InstallResult is a new or upgraded row, or a preview that left the current row running.
// Plugin.Package is always empty; the ZIP stays in the database column only.
type InstallResult struct {
	Plugin  *ent.Plugin
	Preview *UpgradePreview
}

// Builtin is a package-less plugin registered by a later task.
// Apply and OnDispose are no-ops while the registry is empty.
type Builtin interface {
	PluginID() string
	Apply(ctx context.Context) error
	OnDispose(ctx context.Context) error
}

// Manager loads installed plugins into an immutable Generation and extracts UI
// files into a temp directory private to this process.
type Manager struct {
	db       *ent.Client
	cache    *live.Cache[*Generation]
	notifier watcher.Notifier[live.CacheEvent[struct{}]]
	tempRoot string

	refreshInterval time.Duration
	extract         func(data []byte, dest string) ([]string, error)
	// compileModule, when set, replaces compileContents for a package that
	// contains plugin.js. Tests count calls and inject failures.
	compileModule func(data []byte) (*moejs.Module, error)
	forceNext     atomic.Bool
	packageReads  atomic.Int64

	refreshMu   sync.Mutex
	resourcesMu sync.Mutex
	resources   map[resourceKey]*pluginResource

	callsMu   sync.Mutex
	callsCond *sync.Cond
	calls     int
	stopping  bool

	builtinMu sync.Mutex
	builtins  map[string]Builtin

	// cacheMode is ManagerParams.Cache.Mode. Empty is stored as memory.
	// Counters are per instance unless this is redis or two-level.
	cacheMode string

	counterMu      sync.Mutex
	countersClosed bool
	redis          redis.UniversalClient
	memCounters    *memoryCounters
	writes         *pluginWriteLimiter
	channels       *biz.ChannelService
	webhooks       *biz.WebhookNotifier
	dispatcher     *eventDispatcher

	// kvNow is the clock for KV expiry. Nil means time.Now.
	// Tests in this package may set it.
	kvNow func() time.Time

	// outboundTransport, when set, handles ctx.fetch and the page fetch route.
	// Nil uses http.DefaultTransport. Redirects are never followed.
	outboundTransport http.RoundTripper

	// callNow is the clock for the circuit breaker. Nil means time.Now.
	// Tests in this package may set it. It is not the KV clock.
	callNow func() time.Time

	// marketHTTP, when set, fetches the market index and packages.
	// Nil uses http.DefaultTransport. The index URL stays MarketIndexURL.
	marketHTTP *http.Client
	// marketNow is the clock for the market index cache. Nil means time.Now.
	marketNow   func() time.Time
	marketMu    sync.Mutex
	marketCache *marketIndexCache

	execMu sync.Mutex
	execs  map[int]*pluginExec

	hookMu       sync.Mutex
	hookCounters map[int]hookCounter
	// hookTimeout overrides the 10s request-hook budget. Zero uses the default.
	// Tests set it so a timeout case does not wait the full budget.
	hookTimeout time.Duration

	stopOnce sync.Once
	stopErr  error
}

// hookCounter holds process-local request-hook counters for one installation.
type hookCounter struct {
	overloaded     int64
	protectedDrops int64
	overridden     int64
}

type managerConfig struct {
	refreshInterval time.Duration
	debounce        time.Duration
	extract         func(data []byte, dest string) ([]string, error)
}

// ManagerParams is the FX dependency set for the process manager.
type ManagerParams struct {
	fx.In

	Client   *ent.Client
	Cache    xcache.Config
	Bus      *events.Bus
	Channels *biz.ChannelService
	Webhooks *biz.WebhookNotifier
}

// NewManager creates the process temp root, watcher, and cache, then loads metadata.
func NewManager(params ManagerParams) (*Manager, error) {
	return newManager(params, managerConfig{})
}

func newManager(params ManagerParams, cfg managerConfig) (*Manager, error) {
	if params.Client == nil {
		return nil, errors.New("plugin manager requires a database client")
	}
	interval := cfg.refreshInterval
	if interval <= 0 {
		interval = defaultRefreshInterval
	}
	debounce := cfg.debounce
	if debounce <= 0 {
		debounce = defaultDebounce
	}

	tempRoot, err := os.MkdirTemp("", "axonhub-plugins-*")
	if err != nil {
		return nil, fmt.Errorf("create plugin temp directory: %w", err)
	}
	if err := os.Chmod(tempRoot, 0o700); err != nil {
		_ = os.RemoveAll(tempRoot)
		return nil, fmt.Errorf("create plugin temp directory: %w", err)
	}

	cacheMode := params.Cache.Mode
	if cacheMode == "" {
		cacheMode = xcache.ModeMemory
	}
	manager := &Manager{
		db:              params.Client,
		tempRoot:        tempRoot,
		refreshInterval: interval,
		extract:         cfg.extract,
		resources:       map[resourceKey]*pluginResource{},
		builtins:        map[string]Builtin{},
		cacheMode:       cacheMode,
		execs:           map[int]*pluginExec{},
		writes:          newPluginWriteLimiter(),
		channels:        params.Channels,
		webhooks:        params.Webhooks,
	}
	manager.callsCond = sync.NewCond(&manager.callsMu)
	if manager.countersUseRedis() {
		redisClient, err := xredis.NewClient(params.Cache.Redis)
		if err != nil {
			_ = os.RemoveAll(tempRoot)
			return nil, fmt.Errorf("plugin counter redis: %w", err)
		}
		manager.redis = redisClient
	} else {
		manager.memCounters = newMemoryCounters()
	}

	watcherMode := cacheMode
	if watcherMode == xcache.ModeTwoLevel {
		watcherMode = watcher.ModeRedis
	}
	notifier, err := watcher.NewWatcherFromConfig[live.CacheEvent[struct{}]](watcher.Config{
		Mode:  watcherMode,
		Redis: params.Cache.Redis,
	}, watcher.WatcherFromConfigOptions{
		RedisChannel: pluginRedisChannel,
		Buffer:       32,
	})
	if err != nil {
		manager.closeRedis()
		_ = os.RemoveAll(tempRoot)
		return nil, fmt.Errorf("plugin watcher init failed: %w", err)
	}
	manager.notifier = notifier
	if params.Bus != nil {
		manager.dispatcher = newEventDispatcher(manager, params.Bus)
	}

	initial := newGeneration(manager, nil)
	manager.cache = live.NewCache(live.Options[*Generation]{
		Name:            pluginCacheName,
		InitialValue:    initial,
		RefreshInterval: interval,
		DebounceDelay:   debounce,
		RefreshFunc:     manager.refresh,
		OnSwap:          manager.onSwap,
		Watcher:         notifier,
	})
	if err := manager.cache.Load(context.Background(), true); err != nil {
		if manager.dispatcher != nil {
			manager.dispatcher.close()
		}
		manager.cache.Stop()
		manager.closeRedis()
		_ = os.RemoveAll(tempRoot)
		return nil, fmt.Errorf("load plugins: %w", err)
	}
	return manager, nil
}

// Start reloads plugins and applies builtins that are already enabled.
func (m *Manager) Start(ctx context.Context) error {
	if err := m.ForceRefresh(ctx); err != nil {
		return err
	}
	m.syncBuiltins(ctx, nil, m.Current())
	return nil
}

// Stop waits for in-flight acquires, then deletes this process temp root.
// Other processes' directories are left alone.
func (m *Manager) Stop(ctx context.Context) error {
	m.stopOnce.Do(func() {
		m.stopErr = m.stop(ctx)
	})
	return m.stopErr
}

func (m *Manager) stop(ctx context.Context) error {
	m.callsMu.Lock()
	m.stopping = true
	m.callsMu.Unlock()
	if m.dispatcher != nil {
		m.dispatcher.close()
	}
	// Wait for any extraction already in progress before deleting its directories.
	m.refreshMu.Lock()
	defer m.refreshMu.Unlock()
	if m.cache != nil {
		m.cache.Stop()
	}

	m.callsMu.Lock()
	m.stopping = true
	gen := m.current()
	var release *Generation
	if gen != nil {
		gen.mu.Lock()
		gen.closed = true
		if gen.refs == 0 && !gen.released {
			gen.released = true
			release = gen
		}
		gen.mu.Unlock()
	}
	m.callsMu.Unlock()

	if release != nil {
		release.releaseResources()
	}
	if err := m.waitCalls(ctx); err != nil {
		m.closeRedis()
		return err
	}
	m.closeExecs()
	m.syncBuiltins(context.Background(), gen, nil)
	m.closeRedis()
	if m.tempRoot == "" {
		return nil
	}
	return os.RemoveAll(m.tempRoot)
}

func (m *Manager) waitCalls(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error(context.Background(), "plugin manager wait panicked",
					log.Any("panic", recovered),
					log.Stack("stack"))
			}
		}()
		m.callsMu.Lock()
		for m.calls > 0 {
			m.callsCond.Wait()
		}
		m.callsMu.Unlock()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

// IsBuiltin reports whether pluginID is reserved or registered with RegisterBuiltin.
// Uploaded packages are not builtin unless their id is in one of those sets.
func (m *Manager) IsBuiltin(pluginID string) bool {
	if _, reserved := ReservedPluginIDs[pluginID]; reserved {
		return true
	}
	if m == nil {
		return false
	}
	m.builtinMu.Lock()
	defer m.builtinMu.Unlock()
	_, ok := m.builtins[pluginID]
	return ok
}

// CountersArePerInstance is true unless this process shares counters through Redis.
// redis and two-level use the shared counter. An empty mode is stored as memory.
func (m *Manager) CountersArePerInstance() bool {
	if m == nil {
		return true
	}
	return m.cacheMode != xcache.ModeTwoLevel && m.cacheMode != xcache.ModeRedis
}

// RegisterBuiltin adds a package-less plugin. An empty registry is a no-op.
func (m *Manager) RegisterBuiltin(plugin Builtin) {
	if plugin == nil || plugin.PluginID() == "" {
		return
	}
	m.builtinMu.Lock()
	defer m.builtinMu.Unlock()
	m.builtins[plugin.PluginID()] = plugin
}

// Current returns the generation serving new calls.
func (m *Manager) Current() *Generation {
	return m.current()
}

func (m *Manager) current() *Generation {
	if m == nil || m.cache == nil {
		return nil
	}
	return m.cache.GetData()
}

// Refresh runs one non-forced refresh, the same comparison the interval uses.
func (m *Manager) Refresh(ctx context.Context) error {
	return m.cache.Load(ctx, false)
}

// ForceRefresh reloads even when the count and max(updated_at) fingerprint matches.
func (m *Manager) ForceRefresh(ctx context.Context) error {
	m.forceNext.Store(true)
	return m.cache.Load(ctx, true)
}

// PackageReads counts SELECT statements that include the package column.
func (m *Manager) PackageReads() int64 {
	return m.packageReads.Load()
}

// VMCount is the number of JavaScript runtimes this process holds for every
// plugin, idle and borrowed. A compiled module is not a runtime.
func (m *Manager) VMCount() int {
	if m == nil {
		return 0
	}
	m.execMu.Lock()
	execs := make([]*pluginExec, 0, len(m.execs))
	for _, exec := range m.execs {
		execs = append(execs, exec)
	}
	m.execMu.Unlock()
	count := 0
	for _, exec := range execs {
		exec.mu.Lock()
		count += exec.borrowed + len(exec.idle)
		exec.mu.Unlock()
	}
	return count
}

// RuntimeStatuses returns this process's in-memory status for the current generation.
func (m *Manager) RuntimeStatuses() []RuntimeStatusEntry {
	gen := m.current()
	if gen == nil {
		return nil
	}
	plugins := gen.Plugins()
	out := make([]RuntimeStatusEntry, 0, len(plugins))
	for _, item := range plugins {
		out = append(out, RuntimeStatusEntry{
			ID:       item.ID,
			PluginID: item.PluginID,
			Status:   item.Status,
			Reason:   item.Reason,
		})
	}
	return out
}

// Install parses a package and either inserts a disabled row or upgrades the active row.
// submitted == nil means the caller did not send a permission grant. A non-nil slice,
// including an empty one, is the full new permission grant for an upgrade.
// New installs ignore submitted permissions.
// submittedNetwork == nil converges the previous network grant with the new manifest.
// A new install has no previous grant, so that result is empty. A non-nil network
// slice is the full new grant and must be a subset of manifest.network.
func (m *Manager) Install(ctx context.Context, zip []byte, submitted *[]string, submittedNetwork *[]string) (*InstallResult, error) {
	pkg, err := ParsePackage(zip)
	if err != nil {
		return nil, err
	}

	ctx = authz.WithSystemBypass(ctx, pluginBypassReason)
	rows, err := m.queryRecords(ctx, entplugin.PluginIDEQ(pkg.Manifest.ID))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return m.insertPlugin(ctx, zip, pkg, submittedNetwork)
	}
	if len(rows) > 1 {
		return nil, fmt.Errorf("plugin %q has more than one active row", pkg.Manifest.ID)
	}
	return m.upgradePlugin(ctx, rows[0], zip, pkg, submitted, submittedNetwork)
}

func (m *Manager) insertPlugin(ctx context.Context, zip []byte, pkg *Package, submittedNetwork *[]string) (*InstallResult, error) {
	network := []string{}
	if submittedNetwork != nil {
		normalized, err := networkSubset(*submittedNetwork, pkg.Manifest.NetworkGrants())
		if err != nil {
			return nil, err
		}
		network = normalized
	}
	var saved *ent.Plugin
	err := m.runInTx(ctx, func(txCtx context.Context) error {
		row, err := m.client(txCtx).Plugin.Create().
			SetPluginID(pkg.Manifest.ID).
			SetName(pkg.Manifest.Name).
			SetVersion(pkg.Manifest.Version).
			SetDescription(pkg.Manifest.Description).
			SetManifest(pkg.Manifest).
			SetPackage(zip).
			SetPackageSha256(pkg.SHA256).
			SetEnabled(false).
			SetGrantedPermissions([]string{}).
			SetGrantedNetwork(network).
			SetConfig(map[string]any{}).
			Save(txCtx)
		if err != nil {
			return err
		}
		row.Package = nil
		saved = row
		return nil
	})
	if err != nil {
		return nil, packetTooLargeError(err)
	}
	if err := m.finishWrite(ctx); err != nil {
		return nil, err
	}
	return &InstallResult{Plugin: saved}, nil
}

func (m *Manager) upgradePlugin(ctx context.Context, current pluginRecord, zip []byte, pkg *Package, submitted *[]string, submittedNetwork *[]string) (*InstallResult, error) {
	var (
		saved   *ent.Plugin
		preview *UpgradePreview
	)
	err := m.runInTx(ctx, func(txCtx context.Context) error {
		row, err := m.oneRecord(txCtx, current.ID)
		if err != nil {
			return err
		}
		plan, err := PrepareUpgrade(&row.Manifest, &pkg.Manifest, row.Granted, row.GrantedNetwork, row.Config, submitted, submittedNetwork)
		if err != nil {
			return err
		}
		// An enabled plugin must stay runnable, so a new version that needs config it lacks is applied only while disabled.
		if row.Enabled {
			if err := checkConfigForEnable(pkg.Manifest.ConfigSchema, plan.Config); err != nil {
				return fmt.Errorf("disable the plugin before upgrading, then configure it: %w", err)
			}
		}
		if row.Enabled && plan.Diff.NeedsPreview() && submitted == nil && submittedNetwork == nil {
			preview = previewFrom(row, pkg, plan.Diff)
			saved = row.entity()
			return errUpgradePreview
		}
		updated, err := m.client(txCtx).Plugin.UpdateOneID(row.ID).
			SetName(pkg.Manifest.Name).
			SetVersion(pkg.Manifest.Version).
			SetDescription(pkg.Manifest.Description).
			SetManifest(pkg.Manifest).
			SetPackage(zip).
			SetPackageSha256(pkg.SHA256).
			SetGrantedPermissions(plan.Granted).
			SetGrantedNetwork(plan.GrantedNetwork).
			SetConfig(plan.Config).
			Save(txCtx)
		if err != nil {
			return err
		}
		updated.Package = nil
		saved = updated
		return nil
	})
	if xerrors.Is(err, errUpgradePreview) {
		return &InstallResult{Plugin: saved, Preview: preview}, nil
	}
	if err != nil {
		return nil, packetTooLargeError(err)
	}
	if err := m.finishWrite(ctx); err != nil {
		return nil, err
	}
	return &InstallResult{Plugin: saved}, nil
}

// Enable grants subsets of the stored manifest permissions and network rules,
// then marks the row enabled. Both slices are the complete new grants.
// It fails while any configSchema.required field is missing.
func (m *Manager) Enable(ctx context.Context, id int, granted []string, grantedNetwork []string) error {
	ctx = authz.WithSystemBypass(ctx, pluginBypassReason)
	err := m.runInTx(ctx, func(txCtx context.Context) error {
		row, err := m.oneRecord(txCtx, id)
		if err != nil {
			return err
		}
		if err := checkConfigForEnable(row.Manifest.ConfigSchema, row.Config); err != nil {
			return err
		}
		normalized, err := permissionsSubset(granted, row.Manifest.Permissions)
		if err != nil {
			return err
		}
		network, err := networkSubset(grantedNetwork, row.Manifest.NetworkGrants())
		if err != nil {
			return err
		}
		return m.client(txCtx).Plugin.UpdateOneID(id).
			SetEnabled(true).
			SetGrantedPermissions(normalized).
			SetGrantedNetwork(network).
			Exec(txCtx)
	})
	if err != nil {
		return packetTooLargeError(err)
	}
	return m.finishWrite(ctx)
}

// Disable keeps the row and drops this process's UI files and compiled module on the next refresh.
func (m *Manager) Disable(ctx context.Context, id int) error {
	ctx = authz.WithSystemBypass(ctx, pluginBypassReason)
	err := m.runInTx(ctx, func(txCtx context.Context) error {
		if _, err := m.oneRecord(txCtx, id); err != nil {
			return err
		}
		return m.client(txCtx).Plugin.UpdateOneID(id).SetEnabled(false).Exec(txCtx)
	})
	if err != nil {
		return err
	}
	return m.finishWrite(ctx)
}

// UpdateSettings validates config against the stored schema and saves priority and config.
// An enabled plugin must keep every required field set; a disabled one may save partial config.
func (m *Manager) UpdateSettings(ctx context.Context, id int, priority int, config map[string]any) error {
	ctx = authz.WithSystemBypass(ctx, pluginBypassReason)
	err := m.runInTx(ctx, func(txCtx context.Context) error {
		row, err := m.oneRecord(txCtx, id)
		if err != nil {
			return err
		}
		config = keepSecretValues(row.Manifest.ConfigSchema, row.Config, config)
		if err := validateStoredConfig(row.Manifest.ConfigSchema, config); err != nil {
			return err
		}
		if row.Enabled {
			if err := checkConfigForEnable(row.Manifest.ConfigSchema, config); err != nil {
				return err
			}
		}
		return m.client(txCtx).Plugin.UpdateOneID(id).
			SetPriority(priority).
			SetConfig(config).
			Exec(txCtx)
	})
	if err != nil {
		return err
	}
	return m.finishWrite(ctx)
}

// Delete clears the package bytes, keeps package_sha256, deletes KV rows for
// this installation, and soft-deletes the row. All three writes share one transaction.
func (m *Manager) Delete(ctx context.Context, id int) error {
	ctx = authz.WithSystemBypass(ctx, pluginBypassReason)
	err := m.runInTx(ctx, func(txCtx context.Context) error {
		row, err := m.oneRecord(txCtx, id)
		if err != nil {
			return err
		}
		if m.IsBuiltin(row.PluginID) {
			return fmt.Errorf("plugin %q is built in and cannot be deleted", row.PluginID)
		}
		if err := m.client(txCtx).Plugin.UpdateOneID(id).ClearPackage().Exec(txCtx); err != nil {
			return err
		}
		if err := m.KVDeleteInstallation(txCtx, row.PluginID, row.ID); err != nil {
			return err
		}
		return m.client(txCtx).Plugin.DeleteOneID(id).Exec(txCtx)
	})
	if err != nil {
		return err
	}
	return m.finishWrite(ctx)
}

func (m *Manager) finishWrite(ctx context.Context) error {
	if ent.TxFromContext(ctx) != nil {
		m.syncAfterCommit(ctx)
		return nil
	}
	return m.sync(ctx)
}

func (m *Manager) syncAfterCommit(ctx context.Context) {
	tx := ent.TxFromContext(ctx)
	tx.OnCommit(func(next ent.Committer) ent.Committer {
		return ent.CommitFunc(func(commitCtx context.Context, tx *ent.Tx) error {
			if err := next.Commit(commitCtx, tx); err != nil {
				return err
			}
			// Complete local refresh before clients query the updated page catalog.
			refreshCtx, cancel := context.WithTimeout(context.WithoutCancel(commitCtx), time.Minute)
			defer cancel()
			if err := m.sync(refreshCtx); err != nil {
				// The transaction is already committed; do not report a rollback.
				log.Error(refreshCtx, "plugin manager refresh failed", log.Cause(err))
			}
			return nil
		})
	})
}

func (m *Manager) sync(ctx context.Context) error {
	ctx = authz.WithSystemBypass(ctx, pluginBypassReason)
	// A post-commit refresh must use the database, not the completed transaction.
	ctx = ent.NewContext(ctx, m.db)
	ctx = ent.NewTxContext(ctx, nil)
	if err := m.ForceRefresh(ctx); err != nil {
		return err
	}
	if m.notifier == nil {
		return nil
	}
	if err := m.notifier.Notify(ctx, live.NewForceRefreshEvent[struct{}]()); err != nil {
		log.Warn(ctx, "plugin cache notify failed", log.Cause(err))
	}
	return nil
}

func (m *Manager) refresh(ctx context.Context, current *Generation, _ time.Time) (*Generation, time.Time, bool, error) {
	m.refreshMu.Lock()
	defer m.refreshMu.Unlock()
	m.callsMu.Lock()
	stopping := m.stopping
	m.callsMu.Unlock()
	if stopping {
		return current, time.Time{}, false, ErrGenerationClosed
	}

	ctx = authz.WithSystemBypass(ctx, pluginBypassReason)
	fp, err := m.fingerprint(ctx)
	if err != nil {
		return current, time.Time{}, false, err
	}
	force := m.forceNext.Swap(false)
	if current != nil && current.fingerprint.valid && !force && fp.equal(current.fingerprint) && !current.needsReload() {
		return current, fp.maxUpdated, false, nil
	}
	next, err := m.buildGeneration(ctx, current)
	if err != nil {
		return current, fp.maxUpdated, false, err
	}
	next.fingerprint = fp
	next.fingerprint.valid = true
	return next, fp.maxUpdated, true, nil
}

func (m *Manager) onSwap(old, next *Generation) {
	if old != nil && old != next {
		m.retire(old)
	}
	m.syncBuiltins(context.Background(), old, next)
	if m.dispatcher != nil {
		m.dispatcher.sync(next)
	}
}

func (m *Manager) retire(gen *Generation) {
	if gen == nil {
		return
	}
	m.callsMu.Lock()
	gen.mu.Lock()
	gen.closed = true
	finish := gen.refs == 0 && !gen.released
	if finish {
		gen.released = true
	}
	gen.mu.Unlock()
	m.callsMu.Unlock()
	if finish {
		gen.releaseResources()
	}
}

func (m *Manager) buildGeneration(ctx context.Context, current *Generation) (*Generation, error) {
	rows, err := m.queryRecords(ctx)
	if err != nil {
		return nil, err
	}
	prev := map[int]*LoadedPlugin{}
	if current != nil {
		for _, item := range current.plugins {
			prev[item.ID] = item
		}
	}

	loaded := make([]*LoadedPlugin, 0, len(rows))
	holds := make([]*pluginResource, 0)
	for _, row := range rows {
		item := row.loaded()
		if !row.Enabled {
			item.Status = RuntimeStatusDisabled
			loaded = append(loaded, item)
			continue
		}
		resource, err := m.resourceFor(ctx, row, prev[row.ID])
		if err != nil {
			reason := err.Error()
			item.Status = RuntimeStatusError
			item.Reason = reason
			log.Warn(ctx, "plugin load failed",
				log.Int("id", row.ID),
				log.String("plugin_id", row.PluginID),
				log.String("reason", reason))
			loaded = append(loaded, item)
			continue
		}
		if resource != nil {
			resource.hold()
			holds = append(holds, resource)
			item.resource = resource
		}
		item.Status = RuntimeStatusActive
		loaded = append(loaded, item)
	}

	slices.SortFunc(loaded, func(a, b *LoadedPlugin) int {
		if a.Priority != b.Priority {
			if a.Priority < b.Priority {
				return -1
			}
			return 1
		}
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})

	gen := newGeneration(m, loaded)
	gen.holds = holds
	return gen, nil
}

func (m *Manager) resourceFor(ctx context.Context, row pluginRecord, prev *LoadedPlugin) (*pluginResource, error) {
	if !validSHA(row.PackageSHA256) {
		return nil, errors.New("package sha256 is not lowercase hex")
	}
	key := resourceKey{id: row.ID, sha: row.PackageSHA256}
	retry := prev != nil && prev.Status == RuntimeStatusError

	m.resourcesMu.Lock()
	existing := m.resources[key]
	m.resourcesMu.Unlock()
	if existing != nil && existing.complete() && !retry {
		return existing, nil
	}

	data, sha, err := m.readPackage(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	if sha != row.PackageSHA256 || sha256Hex(data) != row.PackageSHA256 {
		return nil, errors.New("package sha256 mismatch")
	}
	resource := existing
	if resource == nil {
		resource = &pluginResource{key: key, manager: m}
	}
	if err := m.publishResource(resource, data); err != nil {
		return nil, err
	}

	m.resourcesMu.Lock()
	if current := m.resources[key]; current != nil && current != resource {
		resource = current
	} else {
		m.resources[key] = resource
	}
	m.resourcesMu.Unlock()
	if !resource.complete() {
		return nil, errors.New("plugin resource is incomplete")
	}
	return resource, nil
}

func (m *Manager) publishResource(resource *pluginResource, data []byte) error {
	resource.mu.Lock()
	resource.ready = false
	resource.module = nil
	resource.hasPluginJS = false
	resource.mu.Unlock()

	contents, err := readPackage(data)
	if err != nil {
		return err
	}
	var module *moejs.Module
	if contents.hasPluginJS() {
		if m.compileModule != nil {
			module, err = m.compileModule(data)
		} else {
			module, err = compileContents(contents)
		}
		if err != nil {
			return err
		}
		if module == nil {
			return errors.New("plugin.js did not compile")
		}
	} else if err := rejectDisallowed(contents, nil); err != nil {
		return err
	}

	finalDir := filepath.Join(m.tempRoot, strconv.Itoa(resource.key.id), resource.key.sha)
	staging := finalDir + ".staging"
	cleanupStaging := func() {
		if err := os.RemoveAll(staging); err != nil {
			log.Warn(context.Background(), "delete plugin staging directory failed",
				log.String("path", staging),
				log.Cause(err))
		}
	}
	if err := os.RemoveAll(staging); err != nil {
		return fmt.Errorf("reset plugin staging directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(finalDir), 0o700); err != nil {
		return fmt.Errorf("create plugin directory: %w", err)
	}
	if err := os.Chmod(filepath.Dir(finalDir), 0o700); err != nil {
		return fmt.Errorf("create plugin directory: %w", err)
	}

	extract := m.extract
	if extract == nil {
		extract = func(_ []byte, dest string) ([]string, error) {
			return writePreparedUI(data, dest, contents.uiFiles, contents.dirs)
		}
	}
	files, err := extract(data, staging)
	if err != nil {
		cleanupStaging()
		return err
	}
	if err := os.RemoveAll(finalDir); err != nil {
		cleanupStaging()
		return fmt.Errorf("replace plugin directory: %w", err)
	}
	if err := os.Rename(staging, finalDir); err != nil {
		cleanupStaging()
		return fmt.Errorf("publish plugin directory: %w", err)
	}

	resource.mu.Lock()
	resource.dir = finalDir
	resource.files = files
	resource.module = module
	resource.hasPluginJS = module != nil
	resource.ready = true
	resource.mu.Unlock()
	return nil
}

func (m *Manager) requestRebuild() {
	if m == nil || m.cache == nil {
		return
	}
	m.cache.TriggerForceAsyncReload()
}

// readPackage is the only function that selects the package column.
func (m *Manager) readPackage(ctx context.Context, id int) ([]byte, string, error) {
	m.packageReads.Add(1)
	ctx = authz.WithSystemBypass(ctx, pluginBypassReason)
	row, err := m.client(ctx).Plugin.Query().
		Where(entplugin.IDEQ(id)).
		Select(entplugin.FieldPackage, entplugin.FieldPackageSha256).
		Only(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("read plugin package: %w", err)
	}
	return row.Package, row.PackageSha256, nil
}

func (m *Manager) fingerprint(ctx context.Context) (dbFingerprint, error) {
	var rows []fingerprintRow
	err := m.client(ctx).Plugin.Query().
		Aggregate(
			ent.As(ent.Count(), "count"),
			ent.As(ent.Max(entplugin.FieldUpdatedAt), "max"),
		).
		Scan(ctx, &rows)
	if err != nil {
		return dbFingerprint{}, fmt.Errorf("plugin fingerprint: %w", err)
	}
	fp := dbFingerprint{valid: true}
	if len(rows) > 0 {
		fp.count = rows[0].Count
		fp.maxUpdated = rows[0].Max.Time
	}
	return fp, nil
}

func (m *Manager) queryRecords(ctx context.Context, preds ...predicate.Plugin) ([]pluginRecord, error) {
	ctx = authz.WithSystemBypass(ctx, pluginBypassReason)
	rows, err := m.client(ctx).Plugin.Query().
		Where(preds...).Select(pluginMetaColumns...).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query plugins: %w", err)
	}
	return lo.Map(rows, func(row *ent.Plugin, _ int) pluginRecord { return recordFromEntity(row) }), nil
}

func (m *Manager) oneRecord(ctx context.Context, id int) (pluginRecord, error) {
	rows, err := m.queryRecords(ctx, entplugin.IDEQ(id))
	if err != nil {
		return pluginRecord{}, err
	}
	if len(rows) == 0 {
		return pluginRecord{}, fmt.Errorf("plugin %d not found", id)
	}
	return rows[0], nil
}

func (m *Manager) client(ctx context.Context) *ent.Client {
	if client := ent.FromContext(ctx); client != nil {
		return client
	}
	return m.db
}

func (m *Manager) runInTx(ctx context.Context, fn func(context.Context) error) error {
	ctx = authz.WithSystemBypass(ctx, pluginBypassReason)
	if tx := ent.TxFromContext(ctx); tx != nil {
		return fn(ent.NewContext(ctx, tx.Client()))
	}

	tx, err := m.db.Tx(ctx)
	if err != nil {
		if errors.Is(err, ent.ErrTxStarted) {
			return fn(ctx)
		}
		return err
	}

	committed := false
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = tx.Rollback()
			panic(recovered)
		}
		if !committed {
			_ = tx.Rollback()
		}
	}()

	txCtx := ent.NewTxContext(ctx, tx)
	txCtx = ent.NewContext(txCtx, tx.Client())
	if err := fn(txCtx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func (m *Manager) syncBuiltins(ctx context.Context, old, next *Generation) {
	m.builtinMu.Lock()
	if len(m.builtins) == 0 {
		m.builtinMu.Unlock()
		return
	}
	items := make(map[string]Builtin, len(m.builtins))
	maps.Copy(items, m.builtins)
	m.builtinMu.Unlock()

	previous := enabledIDs(old)
	current := enabledIDs(next)
	for _, id := range lo.Keys(items) {
		builtin := items[id]
		if previous[id] && !current[id] {
			if err := builtin.OnDispose(ctx); err != nil {
				log.Warn(ctx, "plugin builtin dispose failed",
					log.String("plugin_id", id),
					log.Cause(err))
			}
		}
	}
	for _, id := range lo.Keys(items) {
		builtin := items[id]
		if current[id] && !previous[id] {
			if err := builtin.Apply(ctx); err != nil {
				log.Warn(ctx, "plugin builtin apply failed",
					log.String("plugin_id", id),
					log.Cause(err))
			}
		}
	}
}

func enabledIDs(gen *Generation) map[string]bool {
	out := map[string]bool{}
	if gen == nil {
		return out
	}
	for _, item := range gen.plugins {
		if item.Enabled {
			out[item.PluginID] = true
		}
	}
	return out
}

// Generation is an immutable snapshot of installed plugins, sorted by priority.
// A retired generation accepts no new calls. An acquired call runs to completion.
type Generation struct {
	manager     *Manager
	plugins     []*LoadedPlugin
	byID        map[int]*LoadedPlugin
	byPluginID  map[string]*LoadedPlugin
	fingerprint dbFingerprint
	holds       []*pluginResource

	mu       sync.Mutex
	refs     int
	closed   bool
	released bool
}

func newGeneration(manager *Manager, plugins []*LoadedPlugin) *Generation {
	gen := &Generation{
		manager:    manager,
		plugins:    plugins,
		byID:       make(map[int]*LoadedPlugin, len(plugins)),
		byPluginID: make(map[string]*LoadedPlugin, len(plugins)),
	}
	for _, item := range plugins {
		item.generation = gen
		gen.byID[item.ID] = item
		gen.byPluginID[item.PluginID] = item
	}
	return gen
}

// VMCount is always zero. Runtimes belong to the process manager, not to a
// generation. Compiled modules are stored on plugin resources.
func (g *Generation) VMCount() int {
	return 0
}

// Plugins returns the plugins in priority order. The slice is a copy; the entries are not.
func (g *Generation) Plugins() []*LoadedPlugin {
	if g == nil {
		return nil
	}
	out := make([]*LoadedPlugin, len(g.plugins))
	copy(out, g.plugins)
	return out
}

// ByID returns the plugin installed at plugins.id.
func (g *Generation) ByID(id int) *LoadedPlugin {
	if g == nil {
		return nil
	}
	return g.byID[id]
}

// ByPluginID returns the active row for a manifest id.
func (g *Generation) ByPluginID(pluginID string) *LoadedPlugin {
	if g == nil {
		return nil
	}
	return g.byPluginID[pluginID]
}

// Acquire pins this generation until Release. A closed generation returns ErrGenerationClosed.
func (g *Generation) Acquire() error {
	return g.acquire(false)
}

// acquirePinned pins a generation a request already captured, including after a
// refresh has closed it. A stopping manager still rejects the pin.
func (g *Generation) acquirePinned() error {
	return g.acquire(true)
}

func (g *Generation) acquire(allowClosed bool) error {
	if g == nil || g.manager == nil {
		return ErrGenerationClosed
	}
	g.manager.callsMu.Lock()
	if g.manager.stopping {
		g.manager.callsMu.Unlock()
		return ErrGenerationClosed
	}
	g.mu.Lock()
	if g.closed && !allowClosed {
		g.mu.Unlock()
		g.manager.callsMu.Unlock()
		return ErrGenerationClosed
	}
	g.refs++
	g.manager.calls++
	g.mu.Unlock()
	g.manager.callsMu.Unlock()
	return nil
}

// Release drops one acquire. The last release of a retired generation deletes unshared temp dirs.
func (g *Generation) Release() {
	if g == nil || g.manager == nil {
		return
	}
	g.manager.callsMu.Lock()
	g.mu.Lock()
	if g.refs == 0 {
		g.mu.Unlock()
		g.manager.callsMu.Unlock()
		return
	}
	g.refs--
	finish := g.closed && g.refs == 0 && !g.released
	if finish {
		g.released = true
	}
	g.manager.calls--
	if g.manager.calls < 0 {
		g.manager.calls = 0
	}
	if g.manager.calls == 0 {
		g.manager.callsCond.Broadcast()
	}
	g.mu.Unlock()
	g.manager.callsMu.Unlock()
	if finish {
		g.releaseResources()
	}
}

func (g *Generation) releaseResources() {
	for _, resource := range g.holds {
		resource.release(g.manager)
	}
}

func (g *Generation) needsReload() bool {
	if g == nil {
		return true
	}
	for _, item := range g.plugins {
		if !item.Enabled {
			continue
		}
		if item.Status == RuntimeStatusError {
			return true
		}
		if item.resource == nil || !item.resource.complete() {
			return true
		}
	}
	return false
}

// LoadedPlugin is one installed row inside a generation.
// It keeps metadata and, when active, a pointer to the on-disk UI tree.
type LoadedPlugin struct {
	ID                 int
	PluginID           string
	Name               string
	Version            string
	Description        string
	Manifest           objects.PluginManifest
	Enabled            bool
	Priority           int
	GrantedPermissions []string
	GrantedNetwork     []string
	Config             map[string]any
	PackageSHA256      string
	Status             RuntimeStatus
	Reason             string

	generation *Generation
	resource   *pluginResource
}

// ResourceDir is the published UI directory, or empty when this row has no files loaded.
func (p *LoadedPlugin) ResourceDir() string {
	if p == nil || p.resource == nil {
		return ""
	}
	p.resource.mu.Lock()
	defer p.resource.mu.Unlock()
	return p.resource.dir
}

// Open opens one UI file from the published tree.
// A missing or partial tree returns ErrResourceUnavailable and asks the manager to rebuild.
func (p *LoadedPlugin) Open(name string) (io.ReadCloser, error) {
	if p == nil || p.resource == nil {
		return nil, ErrResourceUnavailable
	}
	return p.resource.Open(name)
}

// ReadFile reads one UI file. The bytes are returned to the caller and are not stored.
func (p *LoadedPlugin) ReadFile(name string) ([]byte, error) {
	file, err := p.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

// AcquireResource pins the UI directory independently of the generation.
func (p *LoadedPlugin) AcquireResource() error {
	if p == nil || p.resource == nil || p.generation == nil || p.generation.manager == nil {
		return ErrResourceUnavailable
	}
	manager := p.generation.manager
	manager.callsMu.Lock()
	if manager.stopping {
		manager.callsMu.Unlock()
		return ErrGenerationClosed
	}
	p.generation.mu.Lock()
	if p.generation.closed {
		p.generation.mu.Unlock()
		manager.callsMu.Unlock()
		return ErrGenerationClosed
	}
	p.resource.hold()
	manager.calls++
	p.generation.mu.Unlock()
	manager.callsMu.Unlock()
	return nil
}

// ReleaseResource drops one AcquireResource pin.
func (p *LoadedPlugin) ReleaseResource() {
	if p == nil || p.resource == nil || p.generation == nil || p.generation.manager == nil {
		return
	}
	p.resource.release(p.generation.manager)
	manager := p.generation.manager
	manager.callsMu.Lock()
	if manager.calls > 0 {
		manager.calls--
	}
	if manager.calls == 0 {
		manager.callsCond.Broadcast()
	}
	manager.callsMu.Unlock()
}

type resourceKey struct {
	id  int
	sha string
}

type pluginResource struct {
	key     resourceKey
	manager *Manager

	mu          sync.Mutex
	dir         string
	files       []string
	ready       bool
	refs        int
	hasPluginJS bool
	module      *moejs.Module
}

func (r *pluginResource) compiledModule() *moejs.Module {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.module
}

func (r *pluginResource) hold() {
	r.mu.Lock()
	r.refs++
	r.mu.Unlock()
}

func (r *pluginResource) release(manager *Manager) {
	r.mu.Lock()
	if r.refs == 0 {
		r.mu.Unlock()
		return
	}
	r.refs--
	last := r.refs == 0
	dir := r.dir
	module := r.module
	if last {
		r.module = nil
		r.hasPluginJS = false
	}
	r.mu.Unlock()
	if !last {
		return
	}
	if manager != nil {
		manager.releaseExecModule(r.key.id, module)
		manager.resourcesMu.Lock()
		if manager.resources[r.key] == r {
			delete(manager.resources, r.key)
		}
		manager.resourcesMu.Unlock()
	}
	if dir == "" {
		return
	}
	if err := os.RemoveAll(dir); err != nil {
		log.Warn(context.Background(), "delete plugin directory failed",
			log.String("path", dir),
			log.Cause(err))
	}
}

func (r *pluginResource) complete() bool {
	dir, files, ready := r.snapshot()
	if !ready || dir == "" {
		return false
	}
	r.mu.Lock()
	hasJS := r.hasPluginJS
	module := r.module
	r.mu.Unlock()
	if hasJS && module == nil {
		return false
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	for _, name := range files {
		target, err := safeJoin(dir, name)
		if err != nil {
			return false
		}
		info, err := os.Stat(target)
		if err != nil || info.IsDir() {
			return false
		}
	}
	return true
}

func (r *pluginResource) snapshot() (string, []string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.dir, slices.Clone(r.files), r.ready
}

func (r *pluginResource) Open(name string) (io.ReadCloser, error) {
	if !r.complete() {
		if r.manager != nil {
			r.manager.requestRebuild()
		}
		return nil, ErrResourceUnavailable
	}
	dir, files, _ := r.snapshot()
	cleaned := path.Clean(name)
	if !slices.Contains(files, cleaned) {
		return nil, fmt.Errorf("file %q is not in the plugin package", name)
	}
	target, err := safeJoin(dir, cleaned)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(target)
	if err != nil {
		if os.IsNotExist(err) {
			if r.manager != nil {
				r.manager.requestRebuild()
			}
			return nil, ErrResourceUnavailable
		}
		return nil, err
	}
	return file, nil
}

type dbFingerprint struct {
	count      int
	maxUpdated time.Time
	valid      bool
}

func (f dbFingerprint) equal(other dbFingerprint) bool {
	return f.valid && other.valid && f.count == other.count && f.maxUpdated.Equal(other.maxUpdated)
}

type fingerprintRow struct {
	Count int     `json:"count"`
	Max   sqlTime `json:"max"`
}

type pluginRecord struct {
	ID             int
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      int
	PluginID       string
	Name           string
	Version        string
	Description    string
	Manifest       objects.PluginManifest
	PackageSHA256  string
	Enabled        bool
	Priority       int
	Granted        []string
	GrantedNetwork []string
	Config         map[string]any
}

func (row pluginRecord) loaded() *LoadedPlugin {
	granted := slices.Clone(row.Granted)
	if granted == nil {
		granted = []string{}
	}
	network := slices.Clone(row.GrantedNetwork)
	if network == nil {
		network = []string{}
	}
	config := map[string]any{}
	maps.Copy(config, row.Config)
	return &LoadedPlugin{
		ID:                 row.ID,
		PluginID:           row.PluginID,
		Name:               row.Name,
		Version:            row.Version,
		Description:        row.Description,
		Manifest:           row.Manifest,
		Enabled:            row.Enabled,
		Priority:           row.Priority,
		GrantedPermissions: granted,
		GrantedNetwork:     network,
		Config:             config,
		PackageSHA256:      row.PackageSHA256,
	}
}

func (row pluginRecord) entity() *ent.Plugin {
	item := row.loaded()
	return &ent.Plugin{
		ID:                 item.ID,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
		DeletedAt:          row.DeletedAt,
		PluginID:           item.PluginID,
		Name:               item.Name,
		Version:            item.Version,
		Description:        item.Description,
		Manifest:           item.Manifest,
		PackageSha256:      item.PackageSHA256,
		Enabled:            item.Enabled,
		Priority:           item.Priority,
		GrantedPermissions: item.GrantedPermissions,
		GrantedNetwork:     item.GrantedNetwork,
		Config:             item.Config,
	}
}

func previewFrom(row pluginRecord, pkg *Package, diff ManifestDiff) *UpgradePreview {
	return &UpgradePreview{
		CurrentVersion: row.Version,
		NewVersion:     pkg.Manifest.Version,
		Permissions:    slices.Clone(diff.Permissions),
		Network:        slices.Clone(diff.Network),
		Events:         slices.Clone(diff.Events),
		Hooks:          slices.Clone(diff.Hooks),
		Pages:          slices.Clone(diff.Pages),
		Actions:        slices.Clone(diff.Actions),
	}
}

func validateStoredConfig(schema *objects.PluginConfigSchema, config map[string]any) error {
	if config == nil {
		return errors.New("config must be an object")
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("config cannot be encoded: %w", err)
	}
	if len(encoded) > MaxConfigBytes {
		return errors.New("config exceeds 64 KiB")
	}
	if schema == nil || len(schema.Properties) == 0 {
		if len(config) != 0 {
			return errors.New("config must be an empty object")
		}
		return nil
	}
	if err := schema.Validate(); err != nil {
		return err
	}
	return validateConfigValues(schema, config, false)
}

func permissionsSubset(granted, declared []string) ([]string, error) {
	return grantSubset(granted, declared, "permission")
}

func networkSubset(granted, declared []string) ([]string, error) {
	return grantSubset(granted, declared, "network rule")
}

func grantSubset(granted, declared []string, kind string) ([]string, error) {
	declaredSet := lo.SliceToMap(declared, func(item string) (string, struct{}) {
		return item, struct{}{}
	})
	out := make([]string, 0, len(granted))
	seen := make(map[string]struct{}, len(granted))
	for _, item := range granted {
		if _, ok := declaredSet[item]; !ok {
			return nil, fmt.Errorf("granted %s %q is not declared by the plugin", kind, item)
		}
		if _, dup := seen[item]; dup {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out, nil
}

func packetTooLargeError(err error) error {
	if err == nil || xerrors.Is(err, errUpgradePreview) {
		return err
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "max_allowed_packet") || strings.Contains(message, "packet too large") {
		return fmt.Errorf("%s: %w", errDatabasePacketTooLarge, err)
	}
	return err
}

func validSHA(sha string) bool {
	if len(sha) != 64 {
		return false
	}
	for _, r := range sha {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// sqlTime scans the timestamp shapes SQLite and Ent drivers return.
type sqlTime struct {
	Time time.Time
}

func (t *sqlTime) Scan(src any) error {
	if src == nil {
		t.Time = time.Time{}
		return nil
	}
	switch value := src.(type) {
	case time.Time:
		t.Time = value
		return nil
	case string:
		parsed, err := parseDBTime(value)
		if err != nil {
			return err
		}
		t.Time = parsed
		return nil
	case []byte:
		parsed, err := parseDBTime(string(value))
		if err != nil {
			return err
		}
		t.Time = parsed
		return nil
	case int64:
		t.Time = time.Unix(value, 0).UTC()
		return nil
	default:
		return fmt.Errorf("cannot scan %T into time", src)
	}
}

func parseDBTime(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05.999999999 -0700",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time %q", value)
}
