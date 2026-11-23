package biz

import (
	"context"
	"encoding/json"
	"errors"
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

var errCatalogInvalidated = errors.New("catalog invalidated during load")

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
	Data        objects.JSONRawMessage `json:"data"`
	FetchedAt   *time.Time             `json:"fetched_at,omitempty"`
	Source      string                 `json:"source"`
	Filtered    bool                   `json:"filtered"`
	upstreamURL string
}

// catalogPayload is a finished JSON payload. Only encoded bytes are retained:
// the upstream catalog decoded into maps costs ~4x its JSON size and is all
// pointers the GC has to scan.
type catalogPayload struct {
	data        []byte
	fetchedAt   time.Time
	source      string
	upstreamURL string
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

	mu          sync.Mutex
	filtered    heldCatalog
	unfiltered  heldCatalog
	generation  uint64
	upstreamURL string // Effective mirror for this cache generation.
	holdTTL     time.Duration
	sf          singleflight.Group
}

func NewCatalogService(system *SystemService, httpClient *httpclient.HttpClient) *CatalogService {
	return &CatalogService{
		system:     system,
		httpClient: httpClient,
		holdTTL:    catalogHoldTTL,
	}
}

func (s *CatalogService) Snapshot(ctx context.Context, filtered bool) (CatalogSnapshot, error) {
	// The public catalog query finishes warming the cache even if its caller leaves.
	return s.snapshot(context.WithoutCancel(ctx), filtered, false)
}

// A fresh snapshot joins an in-flight fetch but never returns a previously held result.
func (s *CatalogService) snapshot(ctx context.Context, filtered, fresh bool) (CatalogSnapshot, error) {
	for {
		payload, err := s.ensure(ctx, filtered, fresh)
		if ctx.Err() != nil {
			return CatalogSnapshot{}, ctx.Err()
		}
		if errors.Is(err, errCatalogInvalidated) {
			continue
		}
		if err != nil {
			return CatalogSnapshot{}, err
		}
		return snapshotFromPayload(payload, filtered), nil
	}
}

// InvalidateAfterCommit keeps a concurrent reader from refilling the cache
// from the previous URL while a settings update is still uncommitted.
func (s *CatalogService) InvalidateAfterCommit(ctx context.Context) {
	runAfterCommit(ctx, func(context.Context) { s.Invalidate() })
}

// Invalidate releases both held variants so the next request refetches, e.g.
// after the upstream URL changes.
func (s *CatalogService) Invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.invalidateLocked()
}

func (s *CatalogService) invalidateLocked() {
	s.generation++
	s.filtered.clear()
	s.unfiltered.clear()
}

// A transaction reads the effective URL on the primary and bypasses the system settings caches.
func (s *CatalogService) currentUpstreamURL(ctx context.Context) (string, error) {
	var url string
	err := s.system.RunInTransaction(ctx, func(ctx context.Context) error {
		settings, err := s.system.CatalogSettings(ctx)
		if err != nil {
			return err
		}
		url = settings.EffectiveUpstreamURL()
		return nil
	})
	return url, err
}

func (s *CatalogService) ensure(ctx context.Context, filtered, fresh bool) (*catalogPayload, error) {
	s.mu.Lock()
	generation := s.generation
	s.mu.Unlock()

	settingsCtx, cancel := context.WithTimeout(ctx, catalogFetchTimeout)
	upstreamURL, err := s.currentUpstreamURL(settingsCtx)
	cancel()
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	if generation != s.generation {
		s.mu.Unlock()
		return nil, errCatalogInvalidated
	}
	if s.upstreamURL != upstreamURL {
		s.invalidateLocked()
		s.upstreamURL = upstreamURL
		generation = s.generation
	}
	held := s.slot(filtered).payload
	s.mu.Unlock()
	if held != nil && !fresh {
		return held, nil
	}
	key := fmt.Sprintf("%t/%d", filtered, generation)
	result := s.sf.DoChan(key, func() (any, error) {
		data, source, err := s.load(context.WithoutCancel(ctx), filtered, upstreamURL)
		if err != nil {
			return nil, err
		}

		payload := s.hold(filtered, data, source, upstreamURL, generation)
		if payload == nil {
			return nil, errCatalogInvalidated
		}
		return payload, nil
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-result:
		if res.Err != nil {
			return nil, res.Err
		}
		payload, ok := res.Val.(*catalogPayload)
		if !ok || payload == nil {
			return nil, fmt.Errorf("catalog load returned no payload")
		}
		return payload, nil
	}
}

// load fetches the upstream catalog and falls back to the bundled one when the
// fetch or filtering fails. The fallback is held for the same TTL so an
// unreachable upstream is not retried on every request.
func (s *CatalogService) load(ctx context.Context, filtered bool, upstreamURL string) ([]byte, string, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, catalogFetchTimeout)
	defer cancel()

	body, err := s.fetchUpstream(fetchCtx, upstreamURL)
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
		Data:        objects.JSONRawMessage(payload.data),
		FetchedAt:   &fetchedAt,
		Source:      source,
		Filtered:    filtered,
		upstreamURL: payload.upstreamURL,
	}
}

func (s *CatalogService) slot(filtered bool) *heldCatalog {
	if filtered {
		return &s.filtered
	}

	return &s.unfiltered
}

// hold keeps the payload for holdTTL, then releases it.
func (s *CatalogService) hold(filtered bool, data []byte, source, upstreamURL string, generation uint64) *catalogPayload {
	payload := &catalogPayload{
		data:        data,
		fetchedAt:   time.Now().UTC(),
		source:      source,
		upstreamURL: upstreamURL,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// A fetch started before a mirror change may finish after its replacement.
	if s.generation != generation {
		return nil
	}
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
