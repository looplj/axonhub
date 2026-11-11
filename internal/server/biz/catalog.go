package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/tidwall/gjson"
	"golang.org/x/sync/singleflight"

	_ "embed"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
)

const (
	catalogSourceUpstream = "upstream"
	catalogSourceFallback = "fallback"
	catalogFetchTimeout   = 20 * time.Second

	// catalogHoldTTL bounds how long a fetched catalog stays in memory. It
	// matches the admin UI's query staleTime so one page visit reuses a single
	// fetch, and nothing stays resident between admin sessions.
	catalogHoldTTL = 5 * time.Minute
)

// Bundled snapshots. scripts/sync/sync-model-developers.js copies providers.json here.
//
//go:embed catalogdata/providers.json
var catalogFallbackJSON []byte

//go:embed catalogdata/models.json
var catalogExtraModelsJSON []byte

// CatalogSnapshot is the catalog payload served to the admin UI.
type CatalogSnapshot struct {
	Data      objects.JSONRawMessage `json:"data"`
	FetchedAt *time.Time             `json:"fetched_at,omitempty"`
	Source    string                 `json:"source"`
	Filtered  bool                   `json:"filtered"`
}

// catalogPayload is a finished JSON payload. Only encoded bytes are retained:
// the upstream catalog decoded into maps costs ~4x its JSON size and is all
// pointers the GC has to scan.
type catalogPayload struct {
	data      []byte
	fetchedAt time.Time
	source    string
}

type heldCatalog struct {
	payload *catalogPayload
	release *time.Timer
}

// CatalogService fetches the public model catalog on demand, applies AxonHub
// developer filters for the filtered variant, and serves it through GraphQL.
// Each variant is held for holdTTL after a fetch and then released.
type CatalogService struct {
	system     *SystemService
	httpClient *httpclient.HttpClient

	mu         sync.Mutex
	filtered   heldCatalog
	unfiltered heldCatalog
	holdTTL    time.Duration
	sf         singleflight.Group
}

func NewCatalogService(system *SystemService, httpClient *httpclient.HttpClient) *CatalogService {
	return &CatalogService{
		system:     system,
		httpClient: httpClient,
		holdTTL:    catalogHoldTTL,
	}
}

func (s *CatalogService) Snapshot(ctx context.Context, filtered bool) (CatalogSnapshot, error) {
	payload, err := s.ensure(ctx, filtered)
	if err != nil {
		return CatalogSnapshot{}, err
	}

	return snapshotFromPayload(payload, filtered), nil
}

// Invalidate releases both held variants so the next request refetches, e.g.
// after the upstream URL changes.
func (s *CatalogService) Invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.filtered.clear()
	s.unfiltered.clear()
}

func (s *CatalogService) ensure(ctx context.Context, filtered bool) (*catalogPayload, error) {
	if held := s.held(filtered); held != nil {
		return held, nil
	}

	key := "catalog-unfiltered"
	if filtered {
		key = "catalog-filtered"
	}

	value, err, _ := s.sf.Do(key, func() (any, error) {
		if held := s.held(filtered); held != nil {
			return held, nil
		}

		data, source, err := s.load(ctx, filtered)
		if err != nil {
			return nil, err
		}

		return s.hold(filtered, data, source), nil
	})
	if err != nil {
		return nil, err
	}

	payload, ok := value.(*catalogPayload)
	if !ok || payload == nil {
		return nil, fmt.Errorf("catalog load returned no payload")
	}

	return payload, nil
}

// load fetches the upstream catalog and falls back to the bundled one when the
// fetch or filtering fails. The fallback is held for the same TTL so an
// unreachable upstream is not retried on every request.
func (s *CatalogService) load(ctx context.Context, filtered bool) ([]byte, string, error) {
	// Callers share this load through singleflight, so one caller going away
	// must not turn the shared result into a held fallback.
	ctx = context.WithoutCancel(ctx)
	settings := s.system.CatalogSettingsOrDefault(ctx)

	fetchCtx, cancel := context.WithTimeout(ctx, catalogFetchTimeout)
	defer cancel()

	body, err := s.fetchUpstream(fetchCtx, settings.EffectiveUpstreamURL())
	if err == nil {
		if !filtered {
			return body, catalogSourceUpstream, nil
		}

		var built []byte
		if built, err = buildFilteredCatalog(body); err == nil {
			return built, catalogSourceUpstream, nil
		}
	}

	log.Warn(ctx, "catalog fetch failed, using bundled fallback", log.Cause(err), log.Any("filtered", filtered))

	if !filtered {
		return fallbackCatalogJSON(), catalogSourceFallback, nil
	}

	built, err := buildFilteredCatalog(catalogFallbackJSON)
	if err != nil {
		return nil, "", fmt.Errorf("build fallback catalog: %w", err)
	}

	return built, catalogSourceFallback, nil
}

// fetchUpstream returns the upstream catalog body after checking it is a JSON
// document with a providers object. The body is served verbatim to the admin
// UI, so it must be valid JSON.
func (s *CatalogService) fetchUpstream(ctx context.Context, upstreamURL string) ([]byte, error) {
	if s.httpClient == nil {
		return nil, fmt.Errorf("catalog http client is not configured")
	}

	resp, err := s.httpClient.Do(ctx, &httpclient.Request{
		Method: http.MethodGet,
		URL:    upstreamURL,
		Headers: http.Header{
			"Accept": []string{"application/json"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("fetch catalog: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch catalog: unexpected status %d", resp.StatusCode)
	}

	if err := validateCatalogJSON(resp.Body); err != nil {
		return nil, err
	}

	return resp.Body, nil
}

func validateCatalogJSON(body []byte) error {
	if !json.Valid(body) {
		return fmt.Errorf("parse catalog: invalid JSON")
	}

	if !gjson.GetBytes(body, "providers").IsObject() {
		return fmt.Errorf("catalog is missing providers")
	}

	return nil
}

// buildFilteredCatalog decodes a full catalog, applies the developer filters
// and extra models, and returns the encoded result. The decoded catalog is
// discarded on return.
func buildFilteredCatalog(body []byte) ([]byte, error) {
	var data catalogFile
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}

	if data.Providers == nil {
		return nil, fmt.Errorf("catalog is missing providers")
	}

	prepared := filterCatalogProviders(data, DefaultDeveloperIDs)
	if err := mergeExtraModels(&prepared, catalogExtraModelsJSON); err != nil {
		return nil, fmt.Errorf("merge extra catalog models: %w", err)
	}

	sortCatalogModels(&prepared)

	return json.Marshal(prepared)
}

func fallbackCatalogJSON() []byte {
	if validateCatalogJSON(catalogFallbackJSON) != nil {
		return []byte(`{"providers":{}}`)
	}

	return catalogFallbackJSON
}

func snapshotFromPayload(payload *catalogPayload, filtered bool) CatalogSnapshot {
	fetchedAt := payload.fetchedAt
	source := payload.source
	if source == "" {
		source = catalogSourceFallback
	}

	return CatalogSnapshot{
		Data:      objects.JSONRawMessage(payload.data),
		FetchedAt: &fetchedAt,
		Source:    source,
		Filtered:  filtered,
	}
}

func (s *CatalogService) slot(filtered bool) *heldCatalog {
	if filtered {
		return &s.filtered
	}

	return &s.unfiltered
}

func (s *CatalogService) held(filtered bool) *catalogPayload {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.slot(filtered).payload
}

// hold keeps the payload for holdTTL, then releases it.
func (s *CatalogService) hold(filtered bool, data []byte, source string) *catalogPayload {
	payload := &catalogPayload{
		data:      data,
		fetchedAt: time.Now().UTC(),
		source:    source,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	slot := s.slot(filtered)
	slot.clear()
	slot.payload = payload
	slot.release = time.AfterFunc(s.holdTTL, func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error(context.Background(), "catalog release panicked", log.Any("panic", rec))
			}
		}()

		s.mu.Lock()
		defer s.mu.Unlock()

		if slot.payload == payload {
			slot.payload = nil
			slot.release = nil
		}
	})

	return payload
}

func (h *heldCatalog) clear() {
	if h.release != nil {
		h.release.Stop()
		h.release = nil
	}

	h.payload = nil
}
