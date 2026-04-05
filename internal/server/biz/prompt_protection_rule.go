package biz

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/dlclark/regexp2"
	"github.com/samber/lo"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/promptprotectionrule"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/watcher"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/pkg/xcache/live"
	"github.com/looplj/axonhub/internal/pkg/xmap"
)

type promptProtectionPatternCache struct {
	regex      *regexp2.Regexp
	compileErr bool
	err        error
}

var promptProtectionRegexCache = xmap.New[string, *promptProtectionPatternCache]()

type PromptProtectionRuleServiceParams struct {
	fx.In

	CacheConfig xcache.Config
	Ent         *ent.Client
}

func NewPromptProtectionRuleService(params PromptProtectionRuleServiceParams) *PromptProtectionRuleService {
	svc := &PromptProtectionRuleService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
	}

	cacheMode := params.CacheConfig.Mode
	if cacheMode == "" {
		cacheMode = xcache.ModeMemory
	}

	watcherMode := cacheMode
	if watcherMode == xcache.ModeTwoLevel {
		watcherMode = watcher.ModeRedis
	}

	notifier, err := watcher.NewWatcherFromConfig[live.CacheEvent[struct{}]](watcher.Config{
		Mode:  watcherMode,
		Redis: params.CacheConfig.Redis,
	}, watcher.WatcherFromConfigOptions{
		RedisChannel: "axonhub:cache:prompt_protection_rules",
		Buffer:       32,
	})
	if err != nil {
		panic(fmt.Errorf("prompt protection rule watcher init failed: %w", err))
	}

	svc.promptProtectionRuleNotifier = notifier
	svc.enabledRulesCache = live.NewCache(live.Options[[]*ent.PromptProtectionRule]{
		Name:            "axonhub:enabled_prompt_protection_rules",
		InitialValue:    []*ent.PromptProtectionRule{},
		RefreshInterval: 30 * time.Second,
		DebounceDelay:   500 * time.Millisecond,
		RefreshFunc:     svc.onEnabledRulesRefreshed,
		Watcher:         notifier,
	})

	if err := svc.enabledRulesCache.Load(context.Background(), true); err != nil {
		panic(fmt.Errorf("prompt protection rule cache initial load failed: %w", err))
	}

	return svc
}

type PromptProtectionRuleService struct {
	*AbstractService

	enabledRulesCache            *live.Cache[[]*ent.PromptProtectionRule]
	promptProtectionRuleNotifier watcher.Notifier[live.CacheEvent[struct{}]]
}

func (svc *PromptProtectionRuleService) ValidateSettings(pattern string, settings *objects.PromptProtectionSettings) error {
	if _, err := getOrCompilePromptProtectionPattern(pattern); err != nil {
		return fmt.Errorf("invalid regex pattern: %w", err)
	}

	if settings == nil {
		return fmt.Errorf("settings are required")
	}

	if settings.Action != objects.PromptProtectionActionMask && settings.Action != objects.PromptProtectionActionReject {
		return fmt.Errorf("invalid action: %s", settings.Action)
	}

	if settings.Action == objects.PromptProtectionActionMask && settings.Replacement == "" {
		return fmt.Errorf("replacement is required for mask action")
	}

	validScopes := []objects.PromptProtectionScope{
		objects.PromptProtectionScopeSystem,
		objects.PromptProtectionScopeDeveloper,
		objects.PromptProtectionScopeUser,
		objects.PromptProtectionScopeAssistant,
		objects.PromptProtectionScopeTool,
	}
	for _, scope := range settings.Scopes {
		if !slices.Contains(validScopes, scope) {
			return fmt.Errorf("invalid scope: %s", scope)
		}
	}

	return nil
}

func MatchPromptProtectionRule(pattern string, content string) bool {
	re, err := getOrCompilePromptProtectionPattern(pattern)
	if err != nil {
		return false
	}

	match, err := re.MatchString(content)
	if err != nil {
		return false
	}

	return match
}

func ReplacePromptProtectionRule(pattern string, content string, replacement string) string {
	re, err := getOrCompilePromptProtectionPattern(pattern)
	if err != nil {
		return content
	}

	replaced, err := re.Replace(content, replacement, -1, -1)
	if err != nil {
		return content
	}

	return replaced
}

func getOrCompilePromptProtectionPattern(pattern string) (*regexp2.Regexp, error) {
	if cached, ok := promptProtectionRegexCache.Load(pattern); ok {
		if cached.compileErr {
			return nil, cached.err
		}

		return cached.regex, nil
	}

	compiled, err := regexp2.Compile(pattern, regexp2.None)

	cached := &promptProtectionPatternCache{}
	if err != nil {
		cached.compileErr = true
		cached.err = err
		promptProtectionRegexCache.Store(pattern, cached)

		return nil, err
	}

	cached.regex = compiled
	promptProtectionRegexCache.Store(pattern, cached)

	return compiled, nil
}

func (svc *PromptProtectionRuleService) Stop() {
	if svc.enabledRulesCache != nil {
		svc.enabledRulesCache.Stop()
	}
}

func (svc *PromptProtectionRuleService) onEnabledRulesRefreshed(ctx context.Context, _ []*ent.PromptProtectionRule, lastUpdate time.Time) ([]*ent.PromptProtectionRule, time.Time, bool, error) {
	ctx = authz.WithSystemBypass(ctx, "prompt-protection-rule-cache")
	client := svc.entFromContext(ctx)

	q := client.PromptProtectionRule.Query().
		Where(promptprotectionrule.StatusEQ(promptprotectionrule.StatusEnabled)).
		Order(ent.Asc(promptprotectionrule.FieldID))

	rules, err := q.All(ctx)
	if err != nil {
		return nil, lastUpdate, false, err
	}

	newUpdateTime := lastUpdate
	if len(rules) > 0 {
		newUpdateTime = lo.MaxBy(rules, func(a, b *ent.PromptProtectionRule) bool {
			return a.UpdatedAt.After(b.UpdatedAt)
		}).UpdatedAt
	}

	// We always mark as changed because:
	// - list membership changes (enable/disable/delete) are not safely detectable from time alone
	// - cache refresh cost is low (rules list is expected to be small)
	return rules, newUpdateTime, true, nil
}

func (svc *PromptProtectionRuleService) asyncReloadEnabledRules() {
	if svc.promptProtectionRuleNotifier == nil {
		return
	}

	if err := svc.promptProtectionRuleNotifier.Notify(context.Background(), live.NewForceRefreshEvent[struct{}]()); err != nil {
		log.Warn(context.Background(), "prompt protection rule cache watcher notify failed", log.Cause(err))
	}
}

func (svc *PromptProtectionRuleService) CreateRule(ctx context.Context, input ent.CreatePromptProtectionRuleInput) (*ent.PromptProtectionRule, error) {
	if input.Settings == nil {
		return nil, fmt.Errorf("settings are required")
	}

	if err := svc.ValidateSettings(input.Pattern, input.Settings); err != nil {
		return nil, err
	}

	existing, err := svc.entFromContext(ctx).PromptProtectionRule.Query().
		Where(promptprotectionrule.Name(input.Name)).
		First(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, fmt.Errorf("failed to check existing prompt protection rule: %w", err)
	}

	if existing != nil {
		return nil, fmt.Errorf("prompt protection rule '%s' already exists", input.Name)
	}

	rule, err := svc.entFromContext(ctx).PromptProtectionRule.Create().
		SetInput(input).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create prompt protection rule: %w", err)
	}

	svc.asyncReloadEnabledRules()

	return rule, nil
}

func (svc *PromptProtectionRuleService) UpdateRule(ctx context.Context, id int, input *ent.UpdatePromptProtectionRuleInput) (*ent.PromptProtectionRule, error) {
	current, err := svc.entFromContext(ctx).PromptProtectionRule.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query prompt protection rule: %w", err)
	}

	pattern := lo.FromPtrOr(input.Pattern, current.Pattern)

	settings := current.Settings
	if input.Settings != nil {
		settings = input.Settings
	}

	if err := svc.ValidateSettings(pattern, settings); err != nil {
		return nil, err
	}

	mut := svc.entFromContext(ctx).PromptProtectionRule.UpdateOneID(id).
		SetInput(*input)

	rule, err := mut.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update prompt protection rule: %w", err)
	}

	svc.asyncReloadEnabledRules()

	return rule, nil
}

func (svc *PromptProtectionRuleService) DeleteRule(ctx context.Context, id int) error {
	if err := svc.entFromContext(ctx).PromptProtectionRule.DeleteOneID(id).Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete prompt protection rule: %w", err)
	}

	svc.asyncReloadEnabledRules()

	return nil
}

func (svc *PromptProtectionRuleService) UpdateRuleStatus(ctx context.Context, id int, status promptprotectionrule.Status) (*ent.PromptProtectionRule, error) {
	rule, err := svc.entFromContext(ctx).PromptProtectionRule.UpdateOneID(id).
		SetStatus(status).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update prompt protection rule status: %w", err)
	}

	svc.asyncReloadEnabledRules()

	return rule, nil
}

func (svc *PromptProtectionRuleService) BulkDeleteRules(ctx context.Context, ids []int) error {
	if len(ids) == 0 {
		return nil
	}

	if _, err := svc.entFromContext(ctx).PromptProtectionRule.Delete().
		Where(promptprotectionrule.IDIn(ids...)).
		Exec(ctx); err != nil {
		return fmt.Errorf("failed to bulk delete prompt protection rules: %w", err)
	}

	svc.asyncReloadEnabledRules()

	return nil
}

func (svc *PromptProtectionRuleService) BulkDisableRules(ctx context.Context, ids []int) error {
	if len(ids) == 0 {
		return nil
	}

	if _, err := svc.entFromContext(ctx).PromptProtectionRule.Update().
		Where(promptprotectionrule.IDIn(ids...)).
		SetStatus(promptprotectionrule.StatusDisabled).
		Save(ctx); err != nil {
		return fmt.Errorf("failed to bulk disable prompt protection rules: %w", err)
	}

	svc.asyncReloadEnabledRules()

	return nil
}

func (svc *PromptProtectionRuleService) BulkEnableRules(ctx context.Context, ids []int) error {
	if len(ids) == 0 {
		return nil
	}

	if _, err := svc.entFromContext(ctx).PromptProtectionRule.Update().
		Where(promptprotectionrule.IDIn(ids...)).
		SetStatus(promptprotectionrule.StatusEnabled).
		Save(ctx); err != nil {
		return fmt.Errorf("failed to bulk enable prompt protection rules: %w", err)
	}

	svc.asyncReloadEnabledRules()

	return nil
}

func (svc *PromptProtectionRuleService) ListEnabledRules(ctx context.Context) ([]*ent.PromptProtectionRule, error) {
	if svc.enabledRulesCache != nil {
		return svc.enabledRulesCache.GetData(), nil
	}

	rules, err := svc.entFromContext(ctx).PromptProtectionRule.Query().
		Where(promptprotectionrule.StatusEQ(promptprotectionrule.StatusEnabled)).
		Order(ent.Asc(promptprotectionrule.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list enabled prompt protection rules: %w", err)
	}

	return rules, nil
}
