package biz

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/samber/lo"
	"github.com/shopspring/decimal"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channelmodelprice"
	"github.com/looplj/axonhub/internal/ent/channelmodelpriceversion"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xtime"
	"github.com/looplj/axonhub/internal/scopes"
)

type SaveChannelModelPriceInput struct {
	ModelID string             `json:"modelId"`
	Price   objects.ModelPrice `json:"price"`
}

type ActionType string

const (
	ActionTypeCreate ActionType = "create"
	ActionTypeUpdate ActionType = "update"
	ActionTypeDelete ActionType = "delete"
	ActionTypeSkip   ActionType = "skip"
)

type PriceChangeAction struct {
	Type          ActionType
	ModelID       string
	Price         objects.ModelPrice
	ExistingPrice *ent.ChannelModelPrice // nil if create
}

const channelModelPriceQueryBatchSize = 500

type channelModelPriceTemplate struct {
	ModelID string
	Price   objects.ModelPrice
}

func uniqueNonEmptyModelIDs(modelIDs []string) []string {
	return lo.Uniq(lo.Filter(modelIDs, func(modelID string, _ int) bool {
		return modelID != ""
	}))
}

func modelCardToChannelModelPrice(card *objects.ModelCard) (objects.ModelPrice, bool) {
	if card == nil {
		return objects.ModelPrice{}, false
	}

	items := make([]objects.ModelPriceItem, 0, 4)
	addItem := func(code objects.PriceItemCode, cost float64) {
		if cost <= 0 || math.IsNaN(cost) || math.IsInf(cost, 0) {
			return
		}

		items = append(items, objects.ModelPriceItem{
			ItemCode: code,
			Pricing: objects.Pricing{
				Mode:         objects.PricingModeUsagePerUnit,
				UsagePerUnit: lo.ToPtr(decimal.NewFromFloat(cost)),
			},
		})
	}

	addItem(objects.PriceItemCodeUsage, card.Cost.Input)
	addItem(objects.PriceItemCodeCompletion, card.Cost.Output)
	addItem(objects.PriceItemCodePromptCachedToken, card.Cost.CacheRead)
	addItem(objects.PriceItemCodeWriteCachedTokens, card.Cost.CacheWrite)

	if len(items) == 0 {
		return objects.ModelPrice{}, false
	}

	return objects.ModelPrice{Items: items}, true
}

// modelLibraryPriceReferencePrefix marks usage-log cost that fell back to the
// model library price because the channel had no explicit price for the model.
const modelLibraryPriceReferencePrefix = "model_library:"

// modelLibraryPriceReferenceID builds the synthetic reference id recorded on
// usage logs whose cost was resolved from the model library fallback.
func modelLibraryPriceReferenceID(modelID string) string {
	return modelLibraryPriceReferencePrefix + modelID
}

// modelLibraryPriceModelIDCandidates returns the model ids to try, in order,
// when resolving a model library price. The exact id is tried first, then the
// base model with a known effort/tier suffix stripped (e.g.
// "gemini-3.8-flash-high" -> "gemini-3.8-flash"). Providers expose the same
// underlying model under several suffixed names, and the model library usually
// only carries the base id.
func modelLibraryPriceModelIDCandidates(modelID string) []string {
	candidates := []string{modelID}

	for _, suffix := range modelLibraryPriceSuffixes {
		if base, ok := strings.CutSuffix(modelID, suffix); ok && base != "" {
			candidates = append(candidates, base)
		}
	}

	return lo.Uniq(candidates)
}

// modelLibraryPriceSuffixes are the effort/tier suffixes stripped when the exact
// model library id has no price. Keep this list in sync with the provider model
// naming conventions (e.g. Antigravity "gemini-3-flash" / "-low" / "-medium" /
// "-high").
var modelLibraryPriceSuffixes = []string{"-high", "-medium", "-low"}

// modelLibraryPriceMemo caches model-library price resolutions for the duration
// of a single request. Cost injection and usage-log creation both resolve the
// price for the same channel and model; without the memo they repeat the
// catalog lookup, adding avoidable synchronous database reads on the response
// path (and once per usage-bearing chunk for streams).
type modelLibraryPriceMemo struct {
	mu      sync.RWMutex
	entries map[string]modelLibraryPriceMemoEntry
}

type modelLibraryPriceMemoEntry struct {
	price *objects.ModelPrice
	found bool
}

type modelLibraryPriceMemoKey struct{}

// WithModelLibraryPriceMemo installs a request-scoped model-library price memo.
// Call it once per inbound request so repeated cost calculations reuse a single
// lookup. It is idempotent: the memo already present (if any) is kept.
func WithModelLibraryPriceMemo(ctx context.Context) context.Context {
	if _, ok := ctx.Value(modelLibraryPriceMemoKey{}).(*modelLibraryPriceMemo); ok {
		return ctx
	}

	return context.WithValue(ctx, modelLibraryPriceMemoKey{}, &modelLibraryPriceMemo{})
}

func modelLibraryPriceMemoFromContext(ctx context.Context) *modelLibraryPriceMemo {
	memo, _ := ctx.Value(modelLibraryPriceMemoKey{}).(*modelLibraryPriceMemo)

	return memo
}

func (m *modelLibraryPriceMemo) get(modelID string) (*objects.ModelPrice, bool, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.entries[modelID]
	if !ok {
		return nil, false, false
	}

	return entry.price, entry.found, true
}

func (m *modelLibraryPriceMemo) set(modelID string, price *objects.ModelPrice, found bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.entries == nil {
		m.entries = make(map[string]modelLibraryPriceMemoEntry)
	}

	m.entries[modelID] = modelLibraryPriceMemoEntry{price: price, found: found}
}

// resolveModelLibraryPrice resolves a price from the model library by model id.
// It is used as a billing-time fallback when a channel has no explicit price
// for the model. The exact model id is matched first; when it has no priceable
// entry, a known effort/tier suffix is stripped and the base model is tried.
// Only enabled and disabled models are considered, mirroring the
// auto-configuration eligibility rules. It returns false when no priceable
// model library entry exists.
func (svc *ChannelService) resolveModelLibraryPrice(ctx context.Context, modelID string) (*objects.ModelPrice, bool) {
	if modelID == "" {
		return nil, false
	}

	memo := modelLibraryPriceMemoFromContext(ctx)
	if memo != nil {
		if price, found, cached := memo.get(modelID); cached {
			return price, found
		}
	}

	price, found, err := svc.lookupModelLibraryPrice(ctx, modelID)
	if err != nil {
		// The lookup failed (e.g. canceled context or a transient database
		// error). Do not cache the result: a later calculation with a healthy
		// context (the usage-log write runs with cancellation removed) must be
		// able to resolve the price instead of reusing this failed attempt.
		return nil, false
	}

	if memo != nil {
		memo.set(modelID, price, found)
	}

	return price, found
}

// lookupModelLibraryPrice performs the model-library query used by
// resolveModelLibraryPrice, without any request-scoped caching. The returned
// error separates a genuine miss (nil error, found=false) from a failed lookup
// (non-nil error), so callers can avoid caching results affected by an error.
func (svc *ChannelService) lookupModelLibraryPrice(ctx context.Context, modelID string) (*objects.ModelPrice, bool, error) {
	db := svc.entFromContext(ctx)

	var lookupErr error

	for _, candidate := range modelLibraryPriceModelIDCandidates(modelID) {
		catalogModel, err := db.Model.Query().
			Where(
				model.ModelID(candidate),
				model.StatusIn(model.StatusEnabled, model.StatusDisabled),
			).
			First(ctx)
		if err != nil {
			if !ent.IsNotFound(err) {
				log.Warn(ctx, "failed to resolve model library price",
					log.String("model_id", modelID),
					log.String("candidate_model_id", candidate),
					log.Cause(err))

				lookupErr = fmt.Errorf("failed to query model library price for %q: %w", candidate, err)
			}

			continue
		}

		price, ok := modelCardToChannelModelPrice(catalogModel.ModelCard)
		if !ok {
			continue
		}

		return &price, true, nil
	}

	return nil, false, lookupErr
}

// createChannelModelPriceIfMissing atomically creates a current price when the
// live (channel, model) pair is absent. The candidate reference ID identifies
// whether this transaction won the upsert race, so only the winner creates the
// initial version.
func (svc *ChannelService) createChannelModelPriceIfMissing(
	ctx context.Context,
	channelID int,
	template channelModelPriceTemplate,
	now time.Time,
) (bool, error) {
	candidateReferenceID := generateReferenceID()
	db := svc.entFromContext(ctx)

	err := db.ChannelModelPrice.Create().
		SetChannelID(channelID).
		SetModelID(template.ModelID).
		SetPrice(template.Price).
		SetReferenceID(candidateReferenceID).
		OnConflictColumns(
			channelmodelprice.FieldChannelID,
			channelmodelprice.FieldModelID,
			channelmodelprice.FieldDeletedAt,
		).
		Ignore().
		Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to create channel model price: %w", err)
	}

	entity, err := authz.RunWithScopeDecision(ctx, scopes.ScopeWriteChannels, func(queryCtx context.Context) (*ent.ChannelModelPrice, error) {
		return db.ChannelModelPrice.Query().
			Where(
				channelmodelprice.ChannelID(channelID),
				channelmodelprice.ModelID(template.ModelID),
			).
			Only(queryCtx)
	})
	if err != nil {
		return false, fmt.Errorf("failed to load channel model price after upsert: %w", err)
	}

	if entity.ReferenceID != candidateReferenceID {
		return false, nil
	}

	if err := svc.createChannelModelPriceVersion(ctx, entity, now); err != nil {
		return false, err
	}

	return true, nil
}

func (svc *ChannelService) resolveChannelModelPriceTemplates(
	ctx context.Context,
	modelIDs []string,
) ([]channelModelPriceTemplate, error) {
	modelIDs = uniqueNonEmptyModelIDs(modelIDs)
	if len(modelIDs) == 0 {
		return nil, nil
	}

	db := svc.entFromContext(ctx)
	modelsByID := make(map[string]*ent.Model, len(modelIDs))
	for start := 0; start < len(modelIDs); start += channelModelPriceQueryBatchSize {
		end := min(start+channelModelPriceQueryBatchSize, len(modelIDs))
		catalogModels, err := authz.RunWithScopeDecision(ctx, scopes.ScopeWriteChannels, func(queryCtx context.Context) ([]*ent.Model, error) {
			return db.Model.Query().
				Where(
					model.ModelIDIn(modelIDs[start:end]...),
					model.StatusIn(model.StatusEnabled, model.StatusDisabled),
				).
				All(queryCtx)
		})
		if err != nil {
			return nil, fmt.Errorf("failed to query model prices from model library: %w", err)
		}

		for _, catalogModel := range catalogModels {
			modelsByID[catalogModel.ModelID] = catalogModel
		}
	}

	templates := make([]channelModelPriceTemplate, 0, len(modelsByID))
	for _, modelID := range modelIDs {
		catalogModel, exists := modelsByID[modelID]
		if !exists {
			continue
		}

		price, ok := modelCardToChannelModelPrice(catalogModel.ModelCard)
		if !ok {
			continue
		}

		templates = append(templates, channelModelPriceTemplate{
			ModelID: modelID,
			Price:   price,
		})
	}

	return templates, nil
}

// applyChannelModelPriceTemplates must run inside the logical channel mutation
// transaction so each current price and initial version commit together.
func (svc *ChannelService) applyChannelModelPriceTemplates(
	ctx context.Context,
	channelID int,
	templates []channelModelPriceTemplate,
) (bool, error) {
	changed := false
	now := xtime.UTCNow()
	for _, template := range templates {
		created, err := svc.createChannelModelPriceIfMissing(ctx, channelID, template, now)
		if err != nil {
			return false, fmt.Errorf("failed to auto-configure channel model price: model_id=%s: %w", template.ModelID, err)
		}

		changed = changed || created
	}

	return changed, nil
}

func (svc *ChannelService) createChannelModelPriceVersion(
	ctx context.Context,
	entity *ent.ChannelModelPrice,
	now time.Time,
) error {
	_, err := svc.entFromContext(ctx).ChannelModelPriceVersion.Create().
		SetChannelID(entity.ChannelID).
		SetModelID(entity.ModelID).
		SetChannelModelPriceID(entity.ID).
		SetPrice(entity.Price).
		SetStatus(channelmodelpriceversion.StatusActive).
		SetEffectiveStartAt(now).
		SetReferenceID(entity.ReferenceID).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to create channel model price version: %w", err)
	}

	return nil
}

// createChannelModelPrice creates the current price and its initial active
// version. The caller must run it inside the logical channel mutation
// transaction so the two records cannot be committed independently.
func (svc *ChannelService) createChannelModelPrice(
	ctx context.Context,
	channelID int,
	modelID string,
	price objects.ModelPrice,
	now time.Time,
) (*ent.ChannelModelPrice, error) {
	entity, err := svc.entFromContext(ctx).ChannelModelPrice.Create().
		SetChannelID(channelID).
		SetModelID(modelID).
		SetPrice(price).
		SetReferenceID(generateReferenceID()).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create channel model price: %w", err)
	}

	if err := svc.createChannelModelPriceVersion(ctx, entity, now); err != nil {
		return nil, err
	}

	return entity, nil
}

// ensureChannelModelPrices fills prices only for the supplied model IDs that
// do not already have a current channel price. Repeated calls preserve all
// existing prices and version history.
func (svc *ChannelService) ensureChannelModelPrices(ctx context.Context, channelID int, modelIDs []string) (bool, error) {
	modelIDs = uniqueNonEmptyModelIDs(modelIDs)
	if len(modelIDs) == 0 {
		return false, nil
	}

	changed := false
	err := svc.RunInTransaction(ctx, func(ctx context.Context) error {
		db := svc.entFromContext(ctx)
		existingModelIDs := make(map[string]struct{}, len(modelIDs))
		for start := 0; start < len(modelIDs); start += channelModelPriceQueryBatchSize {
			end := min(start+channelModelPriceQueryBatchSize, len(modelIDs))
			existingPrices, err := authz.RunWithScopeDecision(ctx, scopes.ScopeWriteChannels, func(queryCtx context.Context) ([]*ent.ChannelModelPrice, error) {
				return db.ChannelModelPrice.Query().
					Where(
						channelmodelprice.ChannelID(channelID),
						channelmodelprice.ModelIDIn(modelIDs[start:end]...),
					).
					Select(channelmodelprice.FieldModelID).
					All(queryCtx)
			})
			if err != nil {
				return fmt.Errorf("failed to query existing channel model prices: %w", err)
			}

			for _, existingPrice := range existingPrices {
				existingModelIDs[existingPrice.ModelID] = struct{}{}
			}
		}

		missingModelIDs := lo.Filter(modelIDs, func(modelID string, _ int) bool {
			_, exists := existingModelIDs[modelID]

			return !exists
		})
		if len(missingModelIDs) == 0 {
			return nil
		}

		templates, err := svc.resolveChannelModelPriceTemplates(ctx, missingModelIDs)
		if err != nil {
			return err
		}

		changed, err = svc.applyChannelModelPriceTemplates(ctx, channelID, templates)

		return err
	})

	return changed, err
}

func calculatePriceChanges(prices []*ent.ChannelModelPrice, inputs []SaveChannelModelPriceInput) []PriceChangeAction {
	existingMap := lo.KeyBy(prices, func(p *ent.ChannelModelPrice) string {
		return p.ModelID
	})

	inputSet := make(map[string]struct{}, len(inputs))

	var actions []PriceChangeAction

	// 1. Identify updates and creates
	// Iterate over inputs in order to keep deterministic action ordering.
	for _, input := range inputs {
		modelID := input.ModelID
		inputSet[modelID] = struct{}{}

		existing, ok := existingMap[modelID]
		if !ok {
			actions = append(actions, PriceChangeAction{
				Type:          ActionTypeCreate,
				ModelID:       modelID,
				Price:         input.Price,
				ExistingPrice: nil,
			})
		} else {
			// Only update if price changed
			if existing.Price.Equals(input.Price) {
				actions = append(actions, PriceChangeAction{
					Type:          ActionTypeSkip,
					ModelID:       modelID,
					Price:         input.Price,
					ExistingPrice: existing,
				})
			} else {
				actions = append(actions, PriceChangeAction{
					Type:          ActionTypeUpdate,
					ModelID:       modelID,
					Price:         input.Price,
					ExistingPrice: existing,
				})
			}
		}
	}

	// 2. Identify deletes: present in existing but not in inputs
	for _, existing := range prices {
		if _, ok := inputSet[existing.ModelID]; !ok {
			actions = append(actions, PriceChangeAction{
				Type:          ActionTypeDelete,
				ModelID:       existing.ModelID,
				ExistingPrice: existing,
			})
		}
	}

	return actions
}

func (svc *ChannelService) SaveChannelModelPrices(
	ctx context.Context,
	channelID int,
	inputs []SaveChannelModelPriceInput,
) ([]*ent.ChannelModelPrice, error) {
	seenModelIDs := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		if _, ok := seenModelIDs[input.ModelID]; ok {
			return nil, fmt.Errorf("duplicate model price input: model_id=%s", input.ModelID)
		}

		seenModelIDs[input.ModelID] = struct{}{}

		if err := input.Price.Validate(); err != nil {
			return nil, fmt.Errorf("invalid model price: model_id=%s: %w", input.ModelID, err)
		}
	}

	db := svc.entFromContext(ctx)

	prices, err := db.ChannelModelPrice.Query().
		Where(channelmodelprice.ChannelID(channelID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query existing channel model prices: %w", err)
	}

	actions := calculatePriceChanges(prices, inputs)

	var (
		results []*ent.ChannelModelPrice
		now     = xtime.UTCNow()
	)

	err = svc.RunInTransaction(ctx, func(ctx context.Context) error {
		db := svc.entFromContext(ctx)

		for _, action := range actions {
			var (
				entity *ent.ChannelModelPrice
				refID  string
				err    error
			)

			switch action.Type {
			case ActionTypeSkip:
				results = append(results, action.ExistingPrice)
				continue

			case ActionTypeDelete:
				// Archive old versions
				_, err = db.ChannelModelPriceVersion.Update().
					Where(
						channelmodelpriceversion.ChannelModelPriceIDEQ(action.ExistingPrice.ID),
						channelmodelpriceversion.StatusEQ(channelmodelpriceversion.StatusActive),
					).
					SetStatus(channelmodelpriceversion.StatusArchived).
					SetEffectiveEndAt(now).
					Save(ctx)
				if err != nil {
					return fmt.Errorf("failed to archive channel model price versions for delete: %w", err)
				}

				err = db.ChannelModelPrice.DeleteOne(action.ExistingPrice).Exec(ctx)
				if err != nil {
					return fmt.Errorf("failed to delete channel model price: %w", err)
				}

				continue

			case ActionTypeCreate:
				entity, err = svc.createChannelModelPrice(ctx, channelID, action.ModelID, action.Price, now)
				if err != nil {
					return err
				}

				results = append(results, entity)
				continue

			case ActionTypeUpdate:
				entity = action.ExistingPrice
				// Archive old versions
				_, err = db.ChannelModelPriceVersion.Update().
					Where(
						channelmodelpriceversion.ChannelModelPriceIDEQ(entity.ID),
						channelmodelpriceversion.StatusEQ(channelmodelpriceversion.StatusActive),
					).
					SetStatus(channelmodelpriceversion.StatusArchived).
					SetEffectiveEndAt(now).
					Save(ctx)
				if err != nil {
					return fmt.Errorf("failed to archive old channel model price versions: %w", err)
				}

				refID = generateReferenceID()

				entity, err = db.ChannelModelPrice.UpdateOneID(entity.ID).
					SetPrice(action.Price).
					SetReferenceID(refID).
					Save(ctx)
				if err != nil {
					return fmt.Errorf("failed to update channel model price: %w", err)
				}
			}

			if err = svc.createChannelModelPriceVersion(ctx, entity, now); err != nil {
				return err
			}

			results = append(results, entity)
		}

		// Force update channel updated_at to trigger reload cache.¬
		return db.Channel.UpdateOneID(channelID).
			SetUpdatedAt(now).
			Exec(ctx)
	})
	if err != nil {
		return nil, err
	}

	// Refresh cached model prices for enabled channel
	if ch := svc.GetEnabledChannel(channelID); ch != nil {
		svc.preloadModelPrices(ctx, ch)

		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "refreshed cached model prices after save",
				log.Int("channel_id", channelID),
				log.Int("count", len(ch.cachedModelPrices)),
			)
		}
	}

	return results, nil
}

// preloadModelPrices loads active model prices for a channel and caches them.
func (svc *ChannelService) preloadModelPrices(ctx context.Context, ch *Channel) {
	prices, err := svc.entFromContext(ctx).ChannelModelPrice.Query().
		Where(
			channelmodelprice.ChannelID(ch.ID),
			channelmodelprice.DeletedAtEQ(0),
		).
		All(ctx)
	if err != nil {
		log.Warn(ctx, "failed to preload model prices", log.Int("channel_id", ch.ID), log.Cause(err))
		return
	}

	cache := make(map[string]*ent.ChannelModelPrice, len(prices))
	for _, p := range prices {
		cache[p.ModelID] = p
	}

	ch.cachedModelPrices = cache
	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "preloaded model prices", log.Int("channel_id", ch.ID), log.Int("count", len(cache)))
	}
}

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func generateReferenceID() string {
	b := make([]byte, 8)
	for i := range b {
		//nolint:gosec // not a security issue.
		b[i] = letters[rand.IntN(len(letters))]
	}

	return string(b)
}
