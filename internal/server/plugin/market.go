package plugin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/build"
)

const (
	// MarketIndexURL is the only plugin market index. It is not configurable.
	// The file lives in this repository, so publishing an entry does not require an AxonHub release.
	MarketIndexURL = "https://raw.githubusercontent.com/looplj/axonhub/unstable/plugins/registry.json"

	marketIndexMaxBytes = 1 << 20
	marketIndexTTL      = 5 * time.Minute
	marketFetchTimeout  = 15 * time.Second
)

// MarketEntry is one compatible plugin from the official index.
// Only the highest compatible version is offered. UpdateFrom is the installed
// version when that version is older than the offer, or when it is not valid
// semver and differs from the offer. Equal versions and newer installs leave
// UpdateFrom empty. InstalledVersion is empty when the plugin is not installed.
type MarketEntry struct {
	ID               string
	Name             string
	Description      string
	Version          string
	AxonhubVersion   string
	InstalledVersion *string
	UpdateFrom       *string
}

// marketIndexCache is a successful index response. Failures are not stored.
type marketIndexCache struct {
	fetchedAt time.Time
	body      []byte
}

type marketVersion struct {
	Version        string
	URL            string
	SHA256         string
	AxonhubVersion string
}

type marketOffer struct {
	ID          string
	Name        string
	Description string
	Best        marketVersion
	sem         *semver.Version
}

type marketIndexData struct {
	offers   []*marketOffer
	versions map[string][]marketVersion
}

// SetMarketClient injects the HTTP client used for the index and packages.
// A nil client uses the default transport. Callers cannot change MarketIndexURL.
func (m *Manager) SetMarketClient(client *http.Client) {
	m.marketMu.Lock()
	defer m.marketMu.Unlock()
	m.marketHTTP = client
}

// SetMarketClock injects the clock used for the index cache. A nil clock uses time.Now.
func (m *Manager) SetMarketClock(now func() time.Time) {
	m.marketMu.Lock()
	defer m.marketMu.Unlock()
	m.marketNow = now
}

// MarketCatalog returns compatible plugins in index order.
func (m *Manager) MarketCatalog(ctx context.Context) ([]*MarketEntry, error) {
	index, err := m.readMarketIndex(ctx)
	if err != nil {
		return nil, err
	}
	installed, err := m.installedMarketVersions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*MarketEntry, 0, len(index.offers))
	for _, offer := range index.offers {
		entry := &MarketEntry{
			ID:             offer.ID,
			Name:           offer.Name,
			Description:    offer.Description,
			Version:        offer.Best.Version,
			AxonhubVersion: offer.Best.AxonhubVersion,
		}
		if current, ok := installed[offer.ID]; ok {
			entry.InstalledVersion = lo.ToPtr(current)
			entry.UpdateFrom = marketUpdateFrom(current, offer.Best.Version)
		}
		out = append(out, entry)
	}
	return out, nil
}

// InstallFromMarket downloads one compatible index version and installs it.
// version must be present in the index, not only the highest offer.
// The returned bytes are the downloaded archive, including when Install returns a preview.
func (m *Manager) InstallFromMarket(ctx context.Context, pluginID, version string, submitted *[]string, submittedNetwork *[]string) (*InstallResult, []byte, error) {
	index, err := m.readMarketIndex(ctx)
	if err != nil {
		return nil, nil, err
	}
	match, ok := index.find(pluginID, version)
	if !ok {
		return nil, nil, fmt.Errorf("plugin %q version %q is not available in the market index", pluginID, version)
	}
	body, err := m.fetchMarket(ctx, match.URL, MaxPackageBytes)
	if err != nil {
		return nil, nil, err
	}
	sum := sha256.Sum256(body)
	if hex.EncodeToString(sum[:]) != match.SHA256 {
		return nil, nil, fmt.Errorf("market package sha256 does not match the index for plugin %q version %q", pluginID, version)
	}
	pkg, err := ParsePackage(body)
	if err != nil {
		return nil, nil, err
	}
	if pkg.Manifest.ID != pluginID || pkg.Manifest.Version != match.Version {
		return nil, nil, fmt.Errorf("market package manifest does not match plugin %q version %q", pluginID, version)
	}
	result, err := m.Install(ctx, body, submitted, submittedNetwork)
	if err != nil {
		return nil, nil, err
	}
	return result, body, nil
}

func (m *Manager) readMarketIndex(ctx context.Context) (*marketIndexData, error) {
	body, err := m.loadMarketIndex(ctx)
	if err != nil {
		return nil, err
	}
	return parseMarketIndex(body, build.Version)
}

func (m *Manager) loadMarketIndex(ctx context.Context) ([]byte, error) {
	if body, ok := m.cachedMarketIndex(m.marketTime()); ok {
		return body, nil
	}
	body, err := m.fetchMarket(ctx, MarketIndexURL, marketIndexMaxBytes)
	if err != nil {
		return nil, err
	}
	if _, err := parseMarketIndex(body, build.Version); err != nil {
		return nil, err
	}
	m.storeMarketIndex(m.marketTime(), body)
	return body, nil
}

func (m *Manager) cachedMarketIndex(now time.Time) ([]byte, bool) {
	m.marketMu.Lock()
	defer m.marketMu.Unlock()
	if m.marketCache == nil || now.Before(m.marketCache.fetchedAt) {
		return nil, false
	}
	if now.Sub(m.marketCache.fetchedAt) >= marketIndexTTL {
		return nil, false
	}
	return append([]byte(nil), m.marketCache.body...), true
}

func (m *Manager) storeMarketIndex(now time.Time, body []byte) {
	m.marketMu.Lock()
	defer m.marketMu.Unlock()
	m.marketCache = &marketIndexCache{
		fetchedAt: now,
		body:      append([]byte(nil), body...),
	}
}

func (m *Manager) marketTime() time.Time {
	m.marketMu.Lock()
	now := m.marketNow
	m.marketMu.Unlock()
	if now == nil {
		return time.Now()
	}
	return now()
}

func (m *Manager) marketClient() *http.Client {
	m.marketMu.Lock()
	base := m.marketHTTP
	m.marketMu.Unlock()
	transport := http.DefaultTransport
	if base != nil && base.Transport != nil {
		transport = base.Transport
	}
	return &http.Client{
		Timeout:       marketFetchTimeout,
		Transport:     transport,
		CheckRedirect: allowGitHubMarketRedirect,
	}
}

// allowGitHubMarketRedirect follows the Release asset hop from github.com to
// release-assets.githubusercontent.com. Every other target is refused so a
// registry URL cannot send the hub to an arbitrary host.
func allowGitHubMarketRedirect(req *http.Request, via []*http.Request) error {
	if len(via) == 0 || len(via) >= 10 {
		return errors.New("market fetch stopped after 10 redirects")
	}
	if !githubMarketURL(via[len(via)-1].URL) || !githubMarketURL(req.URL) {
		return errors.New("market fetch does not follow redirects")
	}
	return nil
}

func githubMarketURL(raw *url.URL) bool {
	if raw == nil || raw.Scheme != "https" {
		return false
	}
	return githubMarketHost(raw.Hostname())
}

func githubMarketHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "github.com" || host == "githubusercontent.com" {
		return true
	}
	return strings.HasSuffix(host, ".github.com") || strings.HasSuffix(host, ".githubusercontent.com")
}

func (m *Manager) fetchMarket(ctx context.Context, rawURL string, limit int) ([]byte, error) {
	if err := validateMarketURL(rawURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("market request: %w", err)
	}
	resp, err := m.marketClient().Do(req)
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		return nil, fmt.Errorf("fetch market url: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("market fetch returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(limit)+1))
	if err != nil {
		return nil, fmt.Errorf("read market response: %w", err)
	}
	if len(body) > limit {
		return nil, fmt.Errorf("market response exceeds %d bytes", limit)
	}
	return body, nil
}

func validateMarketURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return errors.New("market url must use https")
	}
	return nil
}

func (m *Manager) installedMarketVersions(ctx context.Context) (map[string]string, error) {
	rows, err := m.queryRecords(ctx)
	if err != nil {
		return nil, err
	}
	installed := make(map[string]string, len(rows))
	for _, row := range rows {
		if _, exists := installed[row.PluginID]; exists {
			continue
		}
		installed[row.PluginID] = row.Version
	}
	return installed, nil
}

func marketUpdateFrom(installed, offered string) *string {
	installedVer, installedErr := semver.NewVersion(installed)
	offeredVer, offeredErr := semver.NewVersion(offered)
	if installedErr != nil || offeredErr != nil {
		if installed != offered {
			return lo.ToPtr(installed)
		}
		return nil
	}
	if installedVer.LessThan(offeredVer) {
		return lo.ToPtr(installed)
	}
	return nil
}

func parseMarketIndex(body []byte, runtime string) (*marketIndexData, error) {
	var envelope struct {
		SchemaVersion json.RawMessage `json:"schemaVersion"`
		Plugins       json.RawMessage `json:"plugins"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("market index is not valid JSON: %w", err)
	}
	if err := requireMarketSchemaVersion(envelope.SchemaVersion); err != nil {
		return nil, err
	}
	plugins, ok := decodeJSONArray(envelope.Plugins)
	if !ok {
		return nil, errors.New("market index plugins must be an array")
	}
	data := &marketIndexData{
		versions: map[string][]marketVersion{},
	}
	for _, rawPlugin := range plugins {
		id, name, description, versions, keep := decodeMarketPlugin(rawPlugin)
		if !keep {
			continue
		}
		for _, rawVersion := range versions {
			parsed, sem, accept := acceptMarketVersion(rawVersion, runtime)
			if !accept {
				continue
			}
			offer := data.offerFor(id, name, description)
			data.versions[id] = append(data.versions[id], parsed)
			if offer.sem == nil || sem.GreaterThan(offer.sem) {
				offer.Name = name
				offer.Description = description
				offer.Best = parsed
				offer.sem = sem
			}
		}
	}
	return data, nil
}

func (data *marketIndexData) offerFor(id, name, description string) *marketOffer {
	for _, offer := range data.offers {
		if offer.ID == id {
			return offer
		}
	}
	offer := &marketOffer{ID: id, Name: name, Description: description}
	data.offers = append(data.offers, offer)
	return offer
}

func (data *marketIndexData) find(pluginID, version string) (marketVersion, bool) {
	for _, item := range data.versions[pluginID] {
		if item.Version == version {
			return item, true
		}
	}
	return marketVersion{}, false
}

func requireMarketSchemaVersion(raw json.RawMessage) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return errors.New("market index schemaVersion must be 1")
	}
	var version int
	if err := json.Unmarshal(raw, &version); err != nil || version != 1 {
		return errors.New("market index schemaVersion must be 1")
	}
	return nil
}

func decodeJSONArray(raw json.RawMessage) ([]json.RawMessage, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return nil, false
	}
	var items []json.RawMessage
	if err := json.Unmarshal(trimmed, &items); err != nil {
		return nil, false
	}
	return items, true
}

func decodeMarketPlugin(raw json.RawMessage) (string, string, string, []json.RawMessage, bool) {
	var plugin struct {
		ID          string          `json:"id"`
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Versions    json.RawMessage `json:"versions"`
	}
	if err := json.Unmarshal(raw, &plugin); err != nil {
		return "", "", "", nil, false
	}
	id := strings.TrimSpace(plugin.ID)
	name := strings.TrimSpace(plugin.Name)
	if id == "" || name == "" {
		return "", "", "", nil, false
	}
	versions, ok := decodeJSONArray(plugin.Versions)
	if !ok {
		return "", "", "", nil, false
	}
	return id, name, plugin.Description, versions, true
}

func acceptMarketVersion(raw json.RawMessage, runtime string) (marketVersion, *semver.Version, bool) {
	var item struct {
		Version        string `json:"version"`
		URL            string `json:"url"`
		SHA256         string `json:"sha256"`
		AxonhubVersion string `json:"axonhubVersion"`
	}
	if err := json.Unmarshal(raw, &item); err != nil {
		return marketVersion{}, nil, false
	}
	item.Version = strings.TrimSpace(item.Version)
	item.URL = strings.TrimSpace(item.URL)
	item.SHA256 = strings.TrimSpace(item.SHA256)
	item.AxonhubVersion = strings.TrimSpace(item.AxonhubVersion)
	if item.Version == "" || item.URL == "" || item.SHA256 == "" || item.AxonhubVersion == "" {
		return marketVersion{}, nil, false
	}
	if !validSHA(item.SHA256) || validateMarketURL(item.URL) != nil {
		return marketVersion{}, nil, false
	}
	parsed, err := semver.NewVersion(item.Version)
	if err != nil {
		return marketVersion{}, nil, false
	}
	if err := checkAxonhubVersion(item.AxonhubVersion, runtime); err != nil {
		return marketVersion{}, nil, false
	}
	return marketVersion{
		Version:        item.Version,
		URL:            item.URL,
		SHA256:         item.SHA256,
		AxonhubVersion: item.AxonhubVersion,
	}, parsed, true
}
