// 职责: 分页读取站点设置所指向的上游模型报价。
// 关联文件:
//   internal/server/biz/catalog.go
//   internal/server/biz/model_catalog_preview.go
//   internal/server/gql/model.graphql
// 注意事项:
//   游标绑定目录来源和内容，抓取时间与实例不影响分页。

package biz

import (
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/tidwall/gjson"
)

type ModelCatalogOffersPage struct {
	Source     string
	FetchedAt  time.Time
	Offers     []*CatalogImportOffer
	NextCursor *string
}

type catalogOfferCursor struct {
	Version  string `json:"version"`
	Provider string `json:"provider"`
	ModelID  string `json:"modelID"`
}

func (s *CatalogService) ModelCatalogOffers(ctx context.Context, providers []string, first int, after string, refresh bool) (*ModelCatalogOffersPage, error) {
	if first < 1 || first > 250 {
		return nil, errors.New("first must be between 1 and 250")
	}
	var cursor catalogOfferCursor
	if after != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(after)
		if refresh || err != nil || json.Unmarshal(decoded, &cursor) != nil || cursor.Version == "" || cursor.Provider == "" || cursor.ModelID == "" {
			return nil, errors.New("invalid model catalog cursor")
		}
	}
	snapshot, err := s.snapshot(ctx, false, refresh)
	if err != nil {
		return nil, err
	}
	version := snapshot.Source + ":" + snapshot.contentHash
	if after != "" && version != cursor.Version {
		return nil, errors.New("model catalog snapshot changed, restart pagination")
	}
	entries, err := scanCatalogOfferEntries(snapshot.Data, providers, cursor)
	if err != nil {
		return nil, err
	}
	end := min(first, len(entries))
	page := &ModelCatalogOffersPage{Source: snapshot.Source, FetchedAt: *snapshot.FetchedAt, Offers: make([]*CatalogImportOffer, 0, end)}
	for _, entry := range entries[:end] {
		var data map[string]any
		if err := json.Unmarshal([]byte(entry.raw), &data); err != nil {
			return nil, fmt.Errorf("decode model catalog offer: %w", err)
		}
		page.Offers = append(page.Offers, catalogOfferFromEntry(entry.provider, entry.id, data))
	}
	if end < len(entries) {
		last := entries[end-1]
		raw, err := json.Marshal(catalogOfferCursor{Version: version, Provider: last.provider, ModelID: last.id})
		if err != nil {
			return nil, err
		}
		encoded := base64.RawURLEncoding.EncodeToString(raw)
		page.NextCursor = &encoded
	}
	return page, nil
}

type catalogOfferEntry struct {
	catalogOfferKey

	raw string
}

// Decode prices and other metadata only for the requested page.
func scanCatalogOfferEntries(data []byte, providers []string, cursor catalogOfferCursor) ([]catalogOfferEntry, error) {
	entries := make([]catalogOfferEntry, 0)
	seen := make(map[catalogOfferKey]bool)
	var err error
	gjson.GetBytes(data, "providers").ForEach(func(provider, value gjson.Result) bool {
		if provider.Str < cursor.Provider || (len(providers) != 0 && !slices.Contains(providers, provider.Str)) {
			return true
		}
		models := value.Get("models")
		if models.Type == gjson.Null {
			return true
		}
		if !models.IsArray() {
			err = fmt.Errorf("model catalog provider %q has invalid models", provider.Str)
			return false
		}
		models.ForEach(func(_, model gjson.Result) bool {
			id := model.Get("id").Str
			key := catalogOfferKey{provider: provider.Str, id: id}
			if id == "" || seen[key] || (key.provider == cursor.Provider && id <= cursor.ModelID) {
				return true
			}
			seen[key] = true
			entries = append(entries, catalogOfferEntry{catalogOfferKey: key, raw: model.Raw})
			return true
		})
		return true
	})
	slices.SortFunc(entries, func(a, b catalogOfferEntry) int {
		return cmp.Or(cmp.Compare(a.provider, b.provider), cmp.Compare(a.id, b.id))
	})
	return entries, err
}
