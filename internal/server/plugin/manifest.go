// Package plugin parses .axplugin archives and validates manifest.json.
// Parsing never writes the database. Installation is a later task.
package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/build"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/events"
)

const (
	// MaxConfigBytes is the stored config JSON limit from the plugin runtime table.
	MaxConfigBytes = 64 << 10

	errEventsNotEnabled        = "events are not enabled"
	errRequestHooksNotEnabled  = "request hooks are not enabled"
	errResponseHooksNotEnabled = "response hooks are not enabled"
	errUIPanelsNotEnabled      = "ui panels are not enabled"
	errProvidersNotEnabled     = "custom channel providers are not enabled"
	errFetchNotEnabled         = "fetch is not enabled"
)

// fieldGate is one row of the manifest field-open table.
// Opening a field is a one-line change: set open and enforce that field's rule.
// schemaVersion and pluginApiVersion stay the same when a field opens.
type fieldGate struct {
	name    string
	open    bool
	message string
}

var fieldGates = []fieldGate{
	{name: "events", open: true, message: errEventsNotEnabled},
	{name: "hooks.request", open: true, message: errRequestHooksNotEnabled},
	{name: "hooks.attempt", open: true, message: errRequestHooksNotEnabled},
	{name: "hooks.response", message: errResponseHooksNotEnabled},
	{name: "hooks.streamChunk", message: errResponseHooksNotEnabled},
	{name: "ui.panels", message: errUIPanelsNotEnabled},
	{name: "providers", message: errProvidersNotEnabled},
}

type permissionGate struct {
	name    string
	open    bool
	message string
}

// pluginPermissionGates is the plugin permission vocabulary.
// read_plugins and write_plugins are user scopes and are intentionally absent.
var pluginPermissionGates = []permissionGate{
	{name: "read_channels", open: true},
	{name: "read_requests", open: true},
	{name: "read_dashboard", open: true},
	{name: "read_api_keys", open: true},
	{name: "read_projects", open: true},
	{name: "read_settings", open: true},
	{name: "write_channels", open: true},
	{name: "webhook", open: true},
	{name: "fetch", message: errFetchNotEnabled},
}

// supportedPluginAPIVersions is the plugin API generations this process accepts.
var supportedPluginAPIVersions = map[int]struct{}{
	1: {},
}

// ReservedPluginIDs are builtin plugin ids. Uploaded packages cannot use them.
// A later builtin registry may add ids during init.
var ReservedPluginIDs = map[string]struct{}{
	"axonhub.webhook": {},
}

// ManifestDiff is the set of permissions, network rules, events, hooks, and
// UI contributions the new manifest declares and the old one did not. Name, version,
// description, axonhubVersion, entry, and title changes are not included.
type ManifestDiff struct {
	Permissions []string
	Network     []string
	Events      []string
	Hooks       []string
	Pages       []string
	Actions     []string
}

// NeedsPreview reports whether an enabled plugin must ask the owner before upgrade.
// The enabled check itself belongs to the install transaction.
func (d ManifestDiff) NeedsPreview() bool {
	return len(d.Permissions) > 0 || len(d.Network) > 0 || len(d.Events) > 0 || len(d.Hooks) > 0 || len(d.Pages) > 0 || len(d.Actions) > 0
}

// UpgradePlan is the pure result of comparing two manifests.
// Callers write it in their own transaction.
type UpgradePlan struct {
	Diff           ManifestDiff
	Granted        []string
	GrantedNetwork []string
	Config         map[string]any
}

// ParseManifest decodes manifest JSON and validates it against axonhubVersion.
func ParseManifest(data []byte, axonhubVersion string) (*objects.PluginManifest, error) {
	var manifest objects.PluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	if err := ValidateManifest(&manifest, axonhubVersion); err != nil {
		return nil, err
	}
	return &manifest, nil
}

// ValidateManifest checks closed fields, shape, the reserved id set, plugin API
// version, axonhubVersion against the injected AxonHub version, and permissions.
func ValidateManifest(manifest *objects.PluginManifest, axonhubVersion string) error {
	if manifest == nil {
		return errors.New("manifest is required")
	}
	if err := rejectClosedFields(manifest); err != nil {
		return err
	}
	if err := manifest.Validate(); err != nil {
		return err
	}
	if _, reserved := ReservedPluginIDs[manifest.ID]; reserved {
		return fmt.Errorf("plugin id %q is reserved", manifest.ID)
	}
	if err := checkPluginAPIVersion(manifest.PluginAPIVersion); err != nil {
		return err
	}
	if err := checkAxonhubVersion(manifest.AxonhubVersion, axonhubVersion); err != nil {
		return err
	}
	if err := checkPermissions(manifest.Permissions); err != nil {
		return err
	}
	if err := checkEvents(manifest.Events); err != nil {
		return err
	}
	return nil
}

// ValidateCurrentManifest validates a manifest against the running AxonHub version.
func ValidateCurrentManifest(manifest *objects.PluginManifest) error {
	return ValidateManifest(manifest, build.Version)
}

// DiffManifest returns what next declares that old did not.
func DiffManifest(old, next *objects.PluginManifest) ManifestDiff {
	if old == nil {
		old = &objects.PluginManifest{}
	}
	if next == nil {
		next = &objects.PluginManifest{}
	}
	return ManifestDiff{
		Permissions: addedStrings(next.Permissions, old.Permissions),
		Network:     addedStrings(next.NetworkGrants(), old.NetworkGrants()),
		Events:      addedStrings(next.Events, old.Events),
		Hooks:       addedHooks(old.Hooks, next.Hooks),
		Pages:       addedStrings(pageKeys(next.UI), pageKeys(old.UI)),
		Actions:     addedActions(old.UI, next.UI),
	}
}

// ConvergePermissions applies the upgrade grant rule.
// A nil submitted argument means the caller did not send grantedPermissions:
// the result is the intersection of oldGrant and declared, with nothing new added.
// A non-nil submitted slice is the full new grant and must be a subset of declared.
func ConvergePermissions(oldGrant []string, declared []string, submitted *[]string) ([]string, error) {
	return convergeGrant(oldGrant, declared, submitted, "permission")
}

// ConvergeNetwork applies the same intersection and full-set rules as permissions.
// Tokens are the declared URL string or the regex source, compared exactly.
func ConvergeNetwork(oldGrant []string, declared []string, submitted *[]string) ([]string, error) {
	return convergeGrant(oldGrant, declared, submitted, "network rule")
}

func convergeGrant(oldGrant []string, declared []string, submitted *[]string, kind string) ([]string, error) {
	declaredSet := lo.SliceToMap(declared, func(item string) (string, struct{}) {
		return item, struct{}{}
	})
	if submitted == nil {
		oldSet := lo.SliceToMap(oldGrant, func(item string) (string, struct{}) {
			return item, struct{}{}
		})
		granted := make([]string, 0)
		seen := make(map[string]struct{}, len(declared))
		for _, item := range declared {
			if _, ok := oldSet[item]; !ok {
				continue
			}
			if _, dup := seen[item]; dup {
				continue
			}
			seen[item] = struct{}{}
			granted = append(granted, item)
		}
		return granted, nil
	}
	granted := make([]string, 0, len(*submitted))
	seen := make(map[string]struct{}, len(*submitted))
	for _, item := range *submitted {
		if _, ok := declaredSet[item]; !ok {
			return nil, fmt.Errorf("granted %s %q is not declared by the plugin", kind, item)
		}
		if _, dup := seen[item]; dup {
			continue
		}
		seen[item] = struct{}{}
		granted = append(granted, item)
	}
	return granted, nil
}

// CheckSecretDowngrade rejects a field that was format secret and is still
// declared without that format. Removing the field is allowed.
// The error names the field and the reason, never a config value.
func CheckSecretDowngrade(oldSchema, newSchema *objects.PluginConfigSchema) error {
	if oldSchema == nil {
		return nil
	}
	for _, name := range slices.Sorted(maps.Keys(oldSchema.Properties)) {
		if oldSchema.Properties[name].Format != objects.ConfigFormatSecret {
			continue
		}
		if newSchema == nil {
			continue
		}
		next, ok := newSchema.Properties[name]
		if !ok || next.Format == objects.ConfigFormatSecret {
			continue
		}
		return fmt.Errorf("config field %q must remain secret: secret fields cannot be downgraded", name)
	}
	return nil
}

// MigrateConfig keeps only keys declared by schema and checks each kept value.
// A nil schema declares no keys. The caller's map is not modified.
// On failure the returned config is nil.
func MigrateConfig(old map[string]any, schema *objects.PluginConfigSchema) (map[string]any, error) {
	if schema == nil || len(schema.Properties) == 0 {
		return map[string]any{}, nil
	}
	if err := schema.Validate(); err != nil {
		return nil, err
	}
	migrated := make(map[string]any, len(schema.Properties))
	for name := range schema.Properties {
		if value, ok := old[name]; ok {
			migrated[name] = value
		}
	}
	if err := validateConfigValues(schema, migrated, false); err != nil {
		return nil, fmt.Errorf("config is not compatible with the new schema: %w", err)
	}
	encoded, err := encodeConfig(migrated)
	if err != nil {
		return nil, err
	}
	if len(encoded) > MaxConfigBytes {
		return nil, errors.New("config exceeds 64 KiB")
	}
	return migrated, nil
}

// PrepareUpgrade runs the shared upgrade checks. It does not write the database.
// A failure returns no plan, so the previous config is left untouched.
// A nil submitted or submittedNetwork pointer converges that grant with the new declaration.
func PrepareUpgrade(old, next *objects.PluginManifest, oldGrant []string, oldNetwork []string, oldConfig map[string]any, submitted *[]string, submittedNetwork *[]string) (*UpgradePlan, error) {
	if old == nil || next == nil {
		return nil, errors.New("manifest is required")
	}
	diff := DiffManifest(old, next)
	granted, err := ConvergePermissions(oldGrant, next.Permissions, submitted)
	if err != nil {
		return nil, err
	}
	network, err := ConvergeNetwork(oldNetwork, next.NetworkGrants(), submittedNetwork)
	if err != nil {
		return nil, err
	}
	if err := CheckSecretDowngrade(old.ConfigSchema, next.ConfigSchema); err != nil {
		return nil, err
	}
	migrated, err := MigrateConfig(oldConfig, next.ConfigSchema)
	if err != nil {
		return nil, err
	}
	return &UpgradePlan{
		Diff:           diff,
		Granted:        granted,
		GrantedNetwork: network,
		Config:         migrated,
	}, nil
}

func rejectClosedFields(manifest *objects.PluginManifest) error {
	for _, gate := range fieldGates {
		if gate.open || !closedFieldPresent(manifest, gate.name) {
			continue
		}
		return errors.New(gate.message)
	}
	return nil
}

func closedFieldPresent(manifest *objects.PluginManifest, name string) bool {
	switch name {
	case "events":
		return manifest.EventsPresent
	case "hooks.request":
		return manifest.Hooks != nil && manifest.Hooks.Request.IsEnabled()
	case "hooks.attempt":
		return manifest.Hooks != nil && manifest.Hooks.Attempt.IsEnabled()
	case "hooks.response":
		return manifest.Hooks != nil && manifest.Hooks.ResponsePresent
	case "hooks.streamChunk":
		return manifest.Hooks != nil && manifest.Hooks.StreamChunkPresent
	case "ui.panels":
		return manifest.UI != nil && manifest.UI.PanelsPresent
	case "providers":
		return manifest.ProvidersPresent
	default:
		return false
	}
}

func checkPluginAPIVersion(version int) error {
	if _, ok := supportedPluginAPIVersions[version]; ok {
		return nil
	}
	return fmt.Errorf("plugin API version %d is not supported", version)
}

func checkAxonhubVersion(constraint, runtime string) error {
	parsed, err := semver.NewConstraint(constraint)
	if err != nil {
		return fmt.Errorf("axonhubVersion %q is not a valid semver range: %w", constraint, err)
	}
	runtimeVersion, err := normalizeAxonHubVersion(runtime)
	if err != nil {
		return err
	}
	if !parsed.Check(runtimeVersion) {
		return fmt.Errorf("axonhub version %s does not satisfy %s", runtime, constraint)
	}
	return nil
}

func normalizeAxonHubVersion(raw string) (*semver.Version, error) {
	parsed, err := semver.NewVersion(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid AxonHub version %q", raw)
	}
	// Prerelease builds must still match ranges such as ^1.0.0.
	normalized, err := semver.NewVersion(fmt.Sprintf("%d.%d.%d", parsed.Major(), parsed.Minor(), parsed.Patch()))
	if err != nil {
		return nil, fmt.Errorf("invalid AxonHub version %q", raw)
	}
	return normalized, nil
}

func checkEvents(names []string) error {
	if len(names) == 0 {
		return nil
	}
	allowed := lo.SliceToMap(events.OpenEventTypes(), func(name string) (string, struct{}) {
		return name, struct{}{}
	})
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		if _, dup := seen[name]; dup {
			return fmt.Errorf("event %q is duplicated", name)
		}
		seen[name] = struct{}{}
		if _, ok := allowed[name]; !ok {
			return fmt.Errorf("event %q is not enabled", name)
		}
	}
	return nil
}

func checkPermissions(permissions []string) error {
	gates := lo.SliceToMap(pluginPermissionGates, func(gate permissionGate) (string, permissionGate) {
		return gate.name, gate
	})
	seen := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		if _, dup := seen[permission]; dup {
			return fmt.Errorf("permission %q is duplicated", permission)
		}
		seen[permission] = struct{}{}
		gate, ok := gates[permission]
		if !ok {
			return fmt.Errorf("permission %q is not allowed", permission)
		}
		if !gate.open {
			if gate.message != "" {
				return errors.New(gate.message)
			}
			return fmt.Errorf("permission %q is not enabled", permission)
		}
	}
	return nil
}

func addedStrings(next, prev []string) []string {
	prevSet := lo.SliceToMap(prev, func(item string) (string, struct{}) {
		return item, struct{}{}
	})
	seen := make(map[string]struct{}, len(next))
	added := make([]string, 0)
	for _, item := range next {
		if _, ok := prevSet[item]; ok {
			continue
		}
		if _, dup := seen[item]; dup {
			continue
		}
		seen[item] = struct{}{}
		added = append(added, item)
	}
	slices.Sort(added)
	if len(added) == 0 {
		return nil
	}
	return added
}

func addedHooks(old, next *objects.PluginHooksSpec) []string {
	added := make([]string, 0, 2)
	if hookEnabled(next, "request") && (!hookEnabled(old, "request") || len(addedStrings(hookScope(next, "request"), hookScope(old, "request"))) > 0) {
		added = append(added, "request: "+strings.Join(hookScope(next, "request"), ", "))
	}
	if hookEnabled(next, "attempt") && (!hookEnabled(old, "attempt") || len(addedStrings(hookScope(next, "attempt"), hookScope(old, "attempt"))) > 0) {
		added = append(added, "attempt: "+strings.Join(hookScope(next, "attempt"), ", "))
	}
	if len(added) == 0 {
		return nil
	}
	return added
}

func hookEnabled(hooks *objects.PluginHooksSpec, name string) bool {
	if hooks == nil {
		return false
	}
	switch name {
	case "request":
		return hooks.Request.IsEnabled()
	case "attempt":
		return hooks.Attempt.IsEnabled()
	default:
		return false
	}
}

func pageKeys(ui *objects.PluginUI) []string {
	if ui == nil {
		return nil
	}
	keys := make([]string, 0, len(ui.Pages))
	for _, page := range ui.Pages {
		keys = append(keys, page.Key)
	}
	return keys
}

func encodeConfig(config map[string]any) ([]byte, error) {
	encoded, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("config cannot be encoded: %w", err)
	}
	return encoded, nil
}

func hookScope(h *objects.PluginHooksSpec, name string) []string {
	var out []string
	if h == nil {
		return out
	}
	if name == "request" && h.Request != nil {
		for _, v := range h.Request.RequestTypes {
			out = append(out, string(v))
		}
	}
	if name == "attempt" && h.Attempt != nil {
		for _, v := range h.Attempt.APIFormats {
			out = append(out, string(v))
		}
	}
	return out
}

func addedActions(old, next *objects.PluginUI) []string {
	var added []string
	if next == nil {
		return added
	}
	previous := map[string]objects.PluginUIAction{}
	if old != nil {
		for _, action := range old.Actions {
			previous[action.Key] = action
		}
	}
	for _, action := range next.Actions {
		before, ok := previous[action.Key]
		if !ok || before.Slot != action.Slot || before.Effect.Type != action.Effect.Type {
			added = append(added, action.Key)
		}
	}
	return added
}
