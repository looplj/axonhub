// 职责: 比较全局模型与服务商目录，生成手动导入的候选清单。
// 关联文件:
//
//	internal/server/biz/catalog.go
//	internal/server/biz/model_catalog_apply.go
//
// 注意事项:
//
//	同名后缀只用于查找候选；冲突版本须由管理员选择合并。

package biz

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
)

var (
	catalogPriceFields = []string{"input", "output", "cacheRead", "cacheWrite"}
	catalogCostKeys    = map[string]string{"input": "input", "output": "output", "cacheRead": "cache_read", "cacheWrite": "cache_write"}
)

type CatalogImportScope struct {
	Channels     bool
	Providers    []string
	IgnorePrefix bool
}

type CatalogImportTier struct {
	Label string
	Value float64
}

type CatalogImportRate struct {
	Field ReferencePriceField
	Base  *float64
	Tiers []CatalogImportTier
}

type CatalogImportOffer struct {
	Provider, ModelID, Name, Developer, Canonical, Type string
	Rates                                               []CatalogImportRate
	Identity                                            string
	entry                                               map[string]any
}

type CatalogImportLocalPrice struct {
	Field                        ReferencePriceField
	Value                        *float64
	Follow                       bool
	Provider, ModelID, Canonical string
}

type CatalogImportMember struct{ ID, Identity string }

type CatalogImportRoute struct {
	ChannelID           int
	ChannelName, Actual string
	Requests            []string
}

type CatalogRouteChoice struct {
	ChannelID       int
	Actual, Request string
}

type CatalogImportRow struct {
	ModelID, Name, Version string
	MergeTarget            string
	MergeTargetOccupied    bool
	Members                []CatalogImportMember
	Routes                 []CatalogImportRoute
	Archived, Changed      bool
	Offers                 []*CatalogImportOffer
	Preferred              *CatalogImportOffer
	Prices                 []CatalogImportLocalPrice
}

type CatalogImportPreview struct {
	Source, UpstreamURL, Revision string
	Providers                     []string
	Rows                          []*CatalogImportRow
}

type CatalogPriceAction string

const (
	CatalogPriceFollow  CatalogPriceAction = "FOLLOW"
	CatalogPriceRebind  CatalogPriceAction = "REBIND"
	CatalogPriceFixBase CatalogPriceAction = "FIX_BASE"
	CatalogPriceFix     CatalogPriceAction = "FIX"
	CatalogPriceTier    CatalogPriceAction = "TIER"
)

type CatalogImportPriceInput struct {
	Field             ReferencePriceField
	Expected          *float64
	Identity          string
	Action            CatalogPriceAction
	Provider, ModelID string
	Tier              *string
}

type CatalogImportSelection struct {
	ManualMerge                                                      bool
	ModelID, Version, Name, Developer, Provider, SourceModelID, Type string
	Identity                                                         string
	Members                                                          []CatalogImportMember
	Routes                                                           []CatalogRouteChoice
	Prices                                                           []CatalogImportPriceInput
}

type CatalogImportStatus string

const (
	CatalogApplied  CatalogImportStatus = "applied"
	CatalogFailed   CatalogImportStatus = "failed"
	CatalogInvalid  CatalogImportStatus = "invalid"
	CatalogConflict CatalogImportStatus = "conflict"
	CatalogChanged  CatalogImportStatus = "catalogChanged"
)

type CatalogImportOutcome struct {
	ModelID string
	Status  CatalogImportStatus
}

type catalogOfferKey struct{ provider, id string }

type catalogIndex struct {
	offers                          map[catalogOfferKey]*CatalogImportOffer
	exact, canonical, byLastSegment map[string][]*CatalogImportOffer
	providers                       []string
}

func catalogLastIDSegment(id string) string {
	if slash := strings.LastIndexByte(id, '/'); slash >= 0 && slash+1 < len(id) {
		return id[slash+1:]
	}
	return id
}

func catalogRate(raw map[string]any, key string) *float64 {
	n, ok := raw[key].(float64)
	if !ok || n < 0 || math.IsInf(n, 0) || math.IsNaN(n) {
		return nil
	}
	return &n
}

func offerRates(entry map[string]any) []CatalogImportRate {
	cost, _ := entry["cost"].(map[string]any)
	rawTiers, _ := cost["tiers"].([]any)
	result := make([]CatalogImportRate, 0, len(catalogPriceFields))
	for _, field := range catalogPriceFields {
		key := catalogCostKeys[field]
		rate := CatalogImportRate{Field: ReferencePriceField(field), Base: catalogRate(cost, key), Tiers: []CatalogImportTier{}}
		for _, raw := range rawTiers {
			tier, _ := raw.(map[string]any)
			threshold, _ := tier["tier"].(map[string]any)
			if value := catalogRate(tier, key); value != nil && threshold != nil {
				label := modelString(threshold, "type")
				if size, ok := threshold["size"].(float64); ok {
					label = label + " " + strconv.FormatFloat(size, 'f', -1, 64)
				}
				if label != "" {
					rate.Tiers = append(rate.Tiers, CatalogImportTier{label, *value})
				}
			}
		}
		// Older catalogs expose just this compatibility field.
		if len(rawTiers) == 0 {
			legacy, _ := cost["context_over_200k"].(map[string]any)
			if v := catalogRate(legacy, key); v != nil {
				rate.Tiers = append(rate.Tiers, CatalogImportTier{"context >200k", *v})
			}
		}
		result = append(result, rate)
	}
	return result
}

func catalogModelType(raw string) string {
	switch raw {
	case "image":
		return string(model.TypeImageGeneration)
	case "video":
		return string(model.TypeVideoGeneration)
	case "reranking":
		return string(model.TypeRerank)
	default:
		return strings.ReplaceAll(raw, "-", "_")
	}
}

func indexCatalog(data []byte) (*catalogIndex, error) {
	var catalog catalogFile
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, fmt.Errorf("decode model catalog: %w", err)
	}
	idx := &catalogIndex{offers: map[catalogOfferKey]*CatalogImportOffer{}, exact: map[string][]*CatalogImportOffer{}, canonical: map[string][]*CatalogImportOffer{}, byLastSegment: map[string][]*CatalogImportOffer{}, providers: sortedStringKeys(catalog.Providers)}
	for _, provider := range idx.providers {
		for _, entry := range catalog.Providers[provider].Models {
			id := modelString(entry, "id")
			if id == "" {
				continue
			}
			key := catalogOfferKey{provider, id}
			if _, exists := idx.offers[key]; exists {
				continue
			}
			canonical := modelString(entry, "canonical_model_id")
			developer, _, ok := strings.Cut(canonical, "/")
			if !ok {
				developer = ""
			}
			switch developer {
			case "moonshotai":
				developer = "moonshot"
			case "deepseek-ai":
				developer = "deepseek"
			case "qwen":
				developer = "alibaba"
			}
			if developer == "" && slices.Contains(DefaultDeveloperIDs, provider) {
				developer = provider
			}
			name := modelString(entry, "display_name")
			if name == "" {
				name = modelString(entry, "name")
			}
			if name == "" {
				name = id
			}
			offer := &CatalogImportOffer{Provider: provider, ModelID: id, Name: name, Developer: developer, Canonical: canonical, Rates: offerRates(entry), Type: catalogModelType(modelString(entry, "type")), entry: entry}
			identity, _ := json.Marshal([3]string{canonical, modelString(entry, "version"), modelString(entry, "release_date")})
			offer.Identity = string(identity)
			idx.offers[key] = offer
			idx.exact[id] = append(idx.exact[id], offer)
			if canonical != "" {
				idx.canonical[canonical] = append(idx.canonical[canonical], offer)
			}
			suffix := catalogLastIDSegment(id)
			idx.byLastSegment[suffix] = append(idx.byLastSegment[suffix], offer)
		}
	}
	return idx, nil
}

func preferredOffer(offers []*CatalogImportOffer) *CatalogImportOffer {
	if len(offers) == 1 {
		return offers[0]
	}
	var official *CatalogImportOffer
	var identity string
	for _, o := range offers {
		if o.Canonical == "" {
			continue
		}
		if identity != "" && identity != o.Canonical {
			return nil
		}
		identity = o.Canonical
		owner, _, _ := strings.Cut(o.Canonical, "/")
		if o.Provider == owner || (owner == "moonshotai" && o.Provider == "moonshot-ai") {
			if official != nil {
				return nil
			}
			official = o
		}
	}
	return official
}

// A canonical name can refer to several separately priced variants. Only use
// it as an exact alias when all matching providers list the same raw model ID.
func (idx *catalogIndex) matchingOffers(id string, allowCanonical bool) []*CatalogImportOffer {
	if offers := idx.exact[id]; len(offers) > 0 || !allowCanonical {
		return offers
	}
	offers := idx.canonical[id]
	for _, offer := range offers {
		if offer.ModelID != offers[0].ModelID || catalogLastIDSegment(offer.ModelID) != catalogLastIDSegment(id) {
			return nil
		}
	}
	return offers
}

func shareCatalogIdentity(ids []string, idx *catalogIndex) bool {
	identity := map[string]string{}
	for _, id := range ids {
		offers := idx.matchingOffers(id, true)
		if len(offers) == 0 {
			return false
		}
		for _, offer := range offers {
			if offer.Canonical == "" {
				return false
			}
			for _, key := range []string{"canonical_model_id", "version", "release_date"} {
				value := modelString(offer.entry, key)
				if value == "" {
					continue
				}
				if known := identity[key]; known != "" && known != value {
					return false
				}
				identity[key] = value
			}
		}
	}
	return true
}

func catalogLocalPrices(card *objects.ModelCard) []CatalogImportLocalPrice {
	result := make([]CatalogImportLocalPrice, 0, 4)
	for _, field := range catalogPriceFields {
		price := CatalogImportLocalPrice{Field: ReferencePriceField(field)}
		if card != nil {
			binding := card.CatalogPrices[field]
			price.Follow, price.Provider, price.ModelID, price.Canonical = binding.Follow, binding.Provider, binding.ModelID, binding.Canonical
			price.Value = card.ReferencePrice(field)
		}
		result = append(result, price)
	}
	return result
}

// ModelEditVersion records the persisted fields an edit could overwrite, including price ownership.
func ModelEditVersion(m *ent.Model) (string, error) {
	data, err := json.Marshal([]any{m.ID, m.UpdatedAt.UTC(), m.ModelID, m.Type, m.Name, m.Developer, m.Icon, m.Group, m.Status, m.Remark, m.ModelCard, m.Settings})
	return string(data), err
}

func (o *CatalogImportOffer) rate(field ReferencePriceField) *CatalogImportRate {
	if o != nil {
		for i := range o.Rates {
			if o.Rates[i].Field == field {
				return &o.Rates[i]
			}
		}
	}
	return nil
}

func (svc *ModelService) CatalogImportPreview(ctx context.Context, scope CatalogImportScope) (*CatalogImportPreview, error) {
	snapshot, err := svc.catalog.snapshot(ctx, false, false)
	if err != nil {
		return nil, err
	}
	return svc.catalogPreviewFromSnapshot(ctx, scope, snapshot)
}

func catalogChannelRoutes(ctx context.Context, query *ent.ChannelQuery) (map[string][]CatalogImportRoute, error) {
	channels, err := query.Where(channel.StatusIn(channel.StatusEnabled, channel.StatusDisabled)).
		Order(ent.Asc(channel.FieldID)).
		Select(channel.FieldID, channel.FieldName, channel.FieldSettings, channel.FieldSupportedModels).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("load configured channel models: %w", err)
	}
	routes := map[string][]CatalogImportRoute{}
	for _, ch := range channels {
		requests := map[string][]string{}
		for name, entry := range (&Channel{Channel: ch}).GetModelEntries() {
			requests[entry.ActualModel] = append(requests[entry.ActualModel], name)
		}
		for _, id := range lo.Uniq(ch.SupportedModels) {
			if id != "" {
				slices.Sort(requests[id])
				routes[id] = append(routes[id], CatalogImportRoute{ChannelID: ch.ID, ChannelName: ch.Name, Actual: id, Requests: requests[id]})
			}
		}
	}
	return routes, nil
}

func (svc *ModelService) catalogPreviewFromSnapshot(ctx context.Context, scope CatalogImportScope, snapshot CatalogSnapshot) (*CatalogImportPreview, error) {
	idx, err := indexCatalog(snapshot.Data)
	if err != nil {
		return nil, err
	}
	revision := sha256.Sum256([]byte(snapshot.upstreamURL + "\x00" + snapshot.Source))
	publicURL, err := url.Parse(snapshot.upstreamURL)
	if err != nil {
		return nil, fmt.Errorf("parse catalog address: %w", err)
	}
	publicURL.User, publicURL.RawQuery, publicURL.Fragment, publicURL.ForceQuery = nil, "", "", false
	preview := &CatalogImportPreview{UpstreamURL: publicURL.String(), Source: snapshot.Source, Revision: hex.EncodeToString(revision[:]), Providers: idx.providers, Rows: []*CatalogImportRow{}}
	if !scope.Channels && len(scope.Providers) == 0 {
		return preview, nil
	}
	db := svc.entFromContext(ctx)
	local, err := db.Model.Query().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("load global models: %w", err)
	}
	existing := map[string]*ent.Model{}
	importedIDs := map[string]string{}
	channelRoutes := map[string][]CatalogImportRoute{}
	providerOffers := map[string][]*CatalogImportOffer{}
	for _, m := range local {
		existing[m.ModelID] = m
		if m.ModelCard != nil {
			for _, alias := range m.ModelCard.CatalogAliases {
				importedIDs[alias] = m.ModelID
			}
		}
	}
	// An exact local ID takes precedence over an alias recorded on another card.
	for id := range existing {
		importedIDs[id] = id
	}
	allIDs := map[string]bool{}
	if scope.Channels {
		channelRoutes, err = catalogChannelRoutes(ctx, db.Channel.Query())
		if err != nil {
			return nil, err
		}
		for id := range channelRoutes {
			allIDs[id] = true
		}
	}
	for key, offer := range idx.offers {
		if slices.Contains(scope.Providers, key.provider) {
			allIDs[key.id] = true
			providerOffers[key.id] = append(providerOffers[key.id], offer)
		}
	}
	aliases, mergeTargets := idx.groupModelIDs(allIDs, importedIDs, channelRoutes, scope.IgnorePrefix)
	// Keep missing followed offers reachable under their existing local model ID.
	selectedProviders := lo.Intersect(scope.Providers, idx.providers)
	if snapshot.Source == catalogSourceUpstream && len(selectedProviders) > 0 {
		for _, m := range local {
			if aliases[m.ModelID] != nil || m.Status == model.StatusArchived || m.ModelCard == nil {
				continue
			}
			for _, binding := range m.ModelCard.CatalogPrices {
				if binding.Follow && slices.Contains(selectedProviders, binding.Provider) && idx.offers[catalogOfferKey{binding.Provider, binding.ModelID}] == nil {
					aliases[m.ModelID] = []string{m.ModelID}
					break
				}
			}
		}
	}
	for _, id := range sortedStringKeys(aliases) {
		slices.Sort(aliases[id])
		target := mergeTargets[id]
		row := &CatalogImportRow{ModelID: id, Name: id, MergeTarget: target, MergeTargetOccupied: target != "" && importedIDs[target] != ""}
		for _, alias := range slices.Compact(aliases[id]) {
			exact := idx.matchingOffers(alias, scope.IgnorePrefix)
			var identity string
			if len(aliases[id]) > 1 || target != "" {
				identities := lo.Uniq(lo.Map(exact, func(o *CatalogImportOffer, _ int) string { return o.Identity }))
				slices.Sort(identities)
				// Individual identities are JSON arrays and contain no literal newlines.
				identity = strings.Join(identities, "\n")
			}
			row.Members = append(row.Members, CatalogImportMember{alias, identity})
			if routes, configured := channelRoutes[alias]; configured {
				row.Routes = append(row.Routes, routes...)
				row.Offers = append(row.Offers, exact...)
			} else {
				row.Offers = append(row.Offers, providerOffers[alias]...)
			}
		}
		row.Offers = lo.Uniq(row.Offers)
		row.Preferred = preferredOffer(row.Offers)
		if scope.IgnorePrefix && len(row.Offers) == 0 {
			row.Offers = idx.canonical[id]
			if len(row.Offers) == 0 {
				row.Offers = idx.byLastSegment[catalogLastIDSegment(id)]
			}
		}
		if m := existing[id]; m != nil {
			row.Name, row.Archived = m.Name, m.Status == model.StatusArchived
			row.Version, err = ModelEditVersion(m)
			if err != nil {
				return nil, err
			}
			row.Prices = catalogLocalPrices(m.ModelCard)
			idx.addBoundOffers(row, len(providerOffers[id]) == 0)
		} else if row.Preferred != nil {
			row.Name = row.Preferred.Name
		}
		row.Offers = lo.Uniq(row.Offers)
		sort.Slice(row.Offers, func(i, j int) bool {
			a, b := row.Offers[i], row.Offers[j]
			if a.Provider != b.Provider {
				return a.Provider < b.Provider
			}
			return a.ModelID < b.ModelID
		})
		preview.Rows = append(preview.Rows, row)
	}
	return preview, nil
}

func (idx *catalogIndex) groupModelIDs(allIDs map[string]bool, importedIDs map[string]string, channelRoutes map[string][]CatalogImportRoute, ignorePrefix bool) (map[string][]string, map[string]string) {
	aliases := map[string][]string{}
	mergeTargets := map[string]string{}
	groups := map[string][]string{}
	for _, id := range sortedStringKeys(allIDs) {
		if parent := importedIDs[id]; parent != "" {
			aliases[parent] = append(aliases[parent], id)
			continue
		}
		target := id
		if ignorePrefix && len(channelRoutes[id]) > 0 {
			target = catalogLastIDSegment(id)
		}
		groups[target] = append(groups[target], id)
	}
	for target, group := range groups {
		occupied := importedIDs[target] != ""
		if len(group) > 1 && !occupied && shareCatalogIdentity(group, idx) {
			aliases[target] = group
			continue
		}
		for _, id := range group {
			aliases[id] = []string{id}
			if ignorePrefix && (len(group) > 1 || (occupied && id != target)) {
				mergeTargets[id] = target
			}
		}
	}
	return aliases, mergeTargets
}

func (idx *catalogIndex) addBoundOffers(row *CatalogImportRow, preferFollowed bool) {
	for _, p := range row.Prices {
		if p.Provider == "" {
			continue
		}
		o := idx.offers[catalogOfferKey{p.Provider, p.ModelID}]
		if o == nil {
			continue
		}
		row.Offers = append(row.Offers, o)
		if preferFollowed && p.Follow {
			row.Preferred = o
		}
		if !p.Follow {
			continue
		}
		rate := o.rate(p.Field)
		if rate != nil && rate.Base != nil && p.Canonical == o.Canonical && (p.Value == nil || *p.Value != *rate.Base) {
			row.Changed = true
		}
	}
}
