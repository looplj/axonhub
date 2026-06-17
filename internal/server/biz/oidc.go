package biz

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/eko/gocache/lib/v4/store"
	"github.com/go-viper/mapstructure/v2"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"golang.org/x/oauth2"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/oidcidentity"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

type ProviderInfo struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	DisplayName      string `json:"display_name"`
	JITEnabled       bool   `json:"jit_enabled"`
	IconURL          string `json:"icon_url"`
	ButtonColor      string `json:"button_color"`
	Active           bool   `json:"active"`
	OIDCLoginOnly    bool   `json:"oidc_login_only"`
	LastCheck        int64  `json:"last_check,omitempty"`
	IsLinked         bool   `json:"is_linked"`
	LinkedIdentityID string `json:"linked_identity_id,omitempty"`
	LinkedEmail      string `json:"linked_email,omitempty"`
}

type OIDCProvider struct {
	ID                   string   `conf:"id" yaml:"id" json:"id"`
	Name                 string   `conf:"name" yaml:"name" json:"name"`
	DisplayName          string   `conf:"display_name" yaml:"display_name" json:"display_name"`
	IssuerURL            string   `conf:"issuer_url" yaml:"issuer_url" json:"issuer_url"`
	ClientID             string   `conf:"client_id" yaml:"client_id" json:"client_id"`
	ClientSecret         string   `conf:"client_secret" yaml:"client_secret" json:"client_secret"`
	ExtraScopes          []string `conf:"extra_scopes" yaml:"extra_scopes" json:"extra_scopes"`
	JITEnabled           bool     `conf:"jit_enabled" yaml:"jit_enabled" json:"jit_enabled"`
	AutoLinkByEmail      bool     `conf:"auto_link_by_email" yaml:"auto_link_by_email" json:"auto_link_by_email"`
	RequireEmailVerified bool     `conf:"require_email_verified" yaml:"require_email_verified" json:"require_email_verified"`
	EnablePKCE           bool     `conf:"enable_pkce" yaml:"enable_pkce" json:"enable_pkce"`
	// UI customization
	IconURL      string `conf:"icon_url" yaml:"icon_url" json:"icon_url"`
	ButtonColor  string `conf:"button_color" yaml:"button_color" json:"button_color"`
	SyncUserInfo bool   `conf:"sync_user_info" yaml:"sync_user_info" json:"sync_user_info"`
	// Manual configuration (if discovery is not used)
	Issuer      string `conf:"issuer" yaml:"issuer" json:"issuer"`
	AuthURL     string `conf:"auth_url" yaml:"auth_url" json:"auth_url"`
	TokenURL    string `conf:"token_url" yaml:"token_url" json:"token_url"`
	UserInfoURL string `conf:"user_info_url" yaml:"user_info_url" json:"user_info_url"`
	JWKSURL     string `conf:"jwks_url" yaml:"jwks_url" json:"jwks_url"`
	RedirectURL string `conf:"redirect_url" yaml:"redirect_url" json:"redirect_url"`
	// Behavior
	OIDCLoginOnly      bool              `conf:"oidc_login_only" yaml:"oidc_login_only" json:"oidc_login_only"`
	GroupClaims        []string          `conf:"group_claims" yaml:"group_claims" json:"group_claims"`
	GroupParser        GroupParserConfig `conf:"group_parser" yaml:"group_parser" json:"group_parser"`
	RoleMappingRules   []RoleMappingRule `conf:"role_mappings" yaml:"role_mappings" json:"role_mappings"`
	DefaultRoles       []string          `conf:"default_roles" yaml:"default_roles" json:"default_roles"`
	DefaultScopes      []string          `conf:"default_scopes" yaml:"default_scopes" json:"default_scopes"`
	SyncRoleStrategy   string            `conf:"sync_role_strategy" yaml:"sync_role_strategy" json:"sync_role_strategy"`       // "always", "create_only", "merge", "manual_first"
	RolePrecedenceMode string            `conf:"role_precedence_mode" yaml:"role_precedence_mode" json:"role_precedence_mode"` // "merge", "highest"
}

type RoleMappingRule struct {
	MatchGroup string `conf:"match_group" yaml:"match_group" json:"match_group"`
	IsRegex    bool   `conf:"is_regex" yaml:"is_regex" json:"is_regex"`
	DBRole     string `conf:"db_role" yaml:"db_role" json:"db_role"`
	Priority   int    `conf:"priority" yaml:"priority" json:"priority"`
}

type GroupParserConfig struct {
	RegexReplacePattern string `conf:"regex_replace_pattern" yaml:"regex_replace_pattern" json:"regex_replace_pattern"`
	RegexReplaceWith    string `conf:"regex_replace_with" yaml:"regex_replace_with" json:"regex_replace_with"`
	CaseSensitive       bool   `conf:"case_sensitive" yaml:"case_sensitive" json:"case_sensitive"`
}

func (p OIDCProvider) normalize() OIDCProvider {
	normalized := p
	normalized.ID = strings.TrimSpace(normalized.ID)
	normalized.Name = strings.TrimSpace(normalized.Name)
	normalized.DisplayName = strings.TrimSpace(normalized.DisplayName)

	switch {
	case normalized.ID != "":
	case normalized.Name != "":
		normalized.ID = normalized.Name
	case normalized.DisplayName != "":
		normalized.ID = normalized.DisplayName
	}

	if normalized.Name == "" {
		normalized.Name = normalized.ID
	}

	if normalized.DisplayName == "" {
		normalized.DisplayName = normalized.Name
	}

	return normalized
}

func (p OIDCProvider) providerID() string {
	return p.normalize().ID
}

func (p OIDCProvider) providerDisplayName() string {
	return p.normalize().DisplayName
}

func (p OIDCProvider) issuer() string {
	if p.Issuer != "" {
		return p.Issuer
	}

	return p.IssuerURL
}

func normalizeOIDCProviderIdentifier(value string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), " ", ""))
}

func (p OIDCProvider) matchesIdentifier(identifier string) bool {
	normalizedIdentifier := normalizeOIDCProviderIdentifier(identifier)
	if normalizedIdentifier == "" {
		return false
	}

	for _, candidate := range []string{p.providerID(), p.Name} {
		if normalizeOIDCProviderIdentifier(candidate) == normalizedIdentifier {
			return true
		}
	}

	return false
}

type OIDCConfig struct {
	Providers []OIDCProvider `conf:"providers" yaml:"providers" json:"providers"`
}

type OIDCService struct {
	*AbstractService

	cfg OIDCConfig

	cache         xcache.Cache[[]byte]
	mu            sync.Mutex
	providers     map[string]*oidcProvider
	lastCheck     map[string]int64
	exchangeLocks sync.Map // per-code locks to prevent concurrent exchange code reuse
}

type oidcProvider struct {
	config OIDCProvider

	oauth2   oauth2.Config
	oidc     *oidc.Provider // optional, may be nil if manual config is used
	verifier *oidc.IDTokenVerifier
}

type OIDCServiceParams struct {
	fx.In

	Config      OIDCConfig
	CacheConfig xcache.Config
	DB          *ent.Client
}

func NewOIDCService(params OIDCServiceParams) (*OIDCService, error) {
	ctx := context.Background()
	svc := &OIDCService{
		AbstractService: &AbstractService{db: params.DB},
		cfg:             params.Config,
		cache:           xcache.NewFromConfig[[]byte](params.CacheConfig),
		mu:              sync.Mutex{},
		providers:       make(map[string]*oidcProvider),
		lastCheck:       make(map[string]int64),
	}

	numProviders := len(params.Config.Providers)
	seenProviderIDs := make(map[string]string, numProviders)

	for i, p := range params.Config.Providers {
		p = p.normalize()

		providerID := p.providerID()
		if providerID == "" {
			return nil, fmt.Errorf("OIDC provider at index %d requires id or name", i)
		}

		normalizedProviderID := normalizeOIDCProviderIdentifier(providerID)
		if previousProviderID, ok := seenProviderIDs[normalizedProviderID]; ok {
			return nil, fmt.Errorf("duplicate OIDC provider id %q conflicts with %q", providerID, previousProviderID)
		}

		seenProviderIDs[normalizedProviderID] = providerID

		svc.cfg.Providers[i] = p
		svc.lastCheck[providerID] = time.Now().Unix()

		var (
			provider     *oidc.Provider
			oauth2Config oauth2.Config
			verifier     *oidc.IDTokenVerifier
			err          error
		)

		// 1. Try Discovery if IssuerURL is provided
		if p.IssuerURL != "" {
			provider, err = oidc.NewProvider(ctx, p.IssuerURL)
			if err != nil {
				log.Warn(ctx, "OIDC discovery failed", log.String("provider", providerID), log.String("issuer", p.IssuerURL), zap.Error(err))
			} else {
				oauth2Config.Endpoint = provider.Endpoint()
				verifier = provider.Verifier(&oidc.Config{ClientID: p.ClientID})
			}
		}

		// 2. Manual Overrides / Configuration
		if p.AuthURL != "" && p.TokenURL != "" {
			oauth2Config.Endpoint = oauth2.Endpoint{
				AuthURL:  p.AuthURL,
				TokenURL: p.TokenURL,
			}
		}

		if p.JWKSURL != "" {
			issuer := p.Issuer
			if issuer == "" {
				issuer = p.IssuerURL
			}

			keySet := oidc.NewRemoteKeySet(ctx, p.JWKSURL)
			verifier = oidc.NewVerifier(issuer, keySet, &oidc.Config{ClientID: p.ClientID})
		}

		// Validation: Ensure we have enough to proceed
		if oauth2Config.Endpoint.AuthURL == "" || oauth2Config.Endpoint.TokenURL == "" {
			log.Error(ctx, "OIDC provider missing required endpoints (discovery failed and no manual endpoints provided)", log.String("provider", providerID))
			continue
		}

		// Resolve icon_url: supports http(s) URL, data: URI, or local file path.
		if resolved, err := resolveIconURL(p.IconURL); err != nil {
			log.Error(ctx, "Failed to resolve icon for OIDC provider", log.String("provider", providerID), zap.Error(err))
		} else {
			p.IconURL = resolved
			svc.cfg.Providers[i].IconURL = resolved
		}

		// This redirect URI is for IdP -> backend callback handling.
		// The backend will then issue a short-lived exchange code and redirect to
		// the frontend callback route: /oauth/oidc/idp-callback?code=...
		redirectURL := p.RedirectURL
		if redirectURL == "" {
			redirectURL = "/oauth/oidc/callback"
			if numProviders > 1 {
				redirectURL = fmt.Sprintf("/oauth/oidc/callback/%s", providerID)
			}
		}

		scopes := p.ExtraScopes
		if len(scopes) == 0 {
			scopes = []string{oidc.ScopeOpenID, "profile", "email"}
		}

		oauth2Config.ClientID = p.ClientID
		oauth2Config.ClientSecret = p.ClientSecret
		oauth2Config.RedirectURL = redirectURL
		oauth2Config.Scopes = scopes

		svc.providers[providerID] = &oidcProvider{
			config:   p,
			oauth2:   oauth2Config,
			oidc:     provider,
			verifier: verifier,
		}
	}

	return svc, nil
}

// resolveIconURL normalises the icon_url field.
// - http/https URLs are returned unchanged.
// - data: URIs are returned unchanged.
// - Anything else is treated as a local file path and converted to a base64 data URL.
// An empty string is returned unchanged.
func resolveIconURL(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}

	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "data:") {
		return raw, nil
	}
	// Treat as local file path.
	data, err := os.ReadFile(raw)
	if err != nil {
		return "", fmt.Errorf("reading icon file %q: %w", raw, err)
	}

	ext := strings.ToLower(filepath.Ext(raw))

	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		// Fall back to sniffing the first 512 bytes.
		mimeType = http.DetectContentType(data)
	}

	dataURL := "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)

	return dataURL, nil
}

func (s *OIDCService) CountProviders() int {
	return len(s.cfg.Providers)
}

func findOIDCProviderConfig(providers []OIDCProvider, identifier string) (*OIDCProvider, string, bool) {
	for i := range providers {
		provider := providers[i].normalize()
		if provider.matchesIdentifier(identifier) {
			return &providers[i], provider.providerID(), true
		}
	}

	return nil, "", false
}

func (s *OIDCService) getProviderInfo(identifier string) (bool, int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, providerID, ok := s.getProviderByIdentifierLocked(identifier); ok {
		return true, s.lastCheck[providerID]
	}

	if _, providerID, ok := findOIDCProviderConfig(s.cfg.Providers, identifier); ok {
		return false, s.lastCheck[providerID]
	}

	return false, 0
}

func (s *OIDCService) getProviderByIdentifierLocked(identifier string) (*oidcProvider, string, bool) {
	normalizedIdentifier := normalizeOIDCProviderIdentifier(identifier)
	for providerID, provider := range s.providers {
		if normalizeOIDCProviderIdentifier(providerID) == normalizedIdentifier || provider.config.matchesIdentifier(identifier) {
			return provider, providerID, true
		}
	}

	return nil, "", false
}

func (s *OIDCService) getProviderByIdentifier(identifier string) (*oidcProvider, string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.getProviderByIdentifierLocked(identifier)
}

func (s *OIDCService) markProviderCheck(providerID string, now int64) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	lastCheck := s.lastCheck[providerID]
	if now-lastCheck < 60 {
		return lastCheck
	}

	s.lastCheck[providerID] = now

	return lastCheck
}

func (s *OIDCService) setProvider(providerID string, provider *oidcProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.providers[providerID] = provider
}

func (s *OIDCService) GetProviders(ctx context.Context) []ProviderInfo {
	var providers []ProviderInfo

	// Get linked issuers for the current user
	linkedIdentities := make(map[string]*ent.OIDCIdentity)

	if u, ok := contexts.GetUser(ctx); ok {
		identities, err := s.entFromContext(ctx).OIDCIdentity.Query().
			Where(oidcidentity.UserID(u.ID)).
			All(ctx)
		if err == nil {
			for _, id := range identities {
				linkedIdentities[id.Issuer] = id
			}
		}
	}

	for _, rawProvider := range s.cfg.Providers {
		p := rawProvider.normalize()
		providerID := p.providerID()

		displayName := p.providerDisplayName()
		if displayName == "" {
			displayName = p.Name
		}

		info := ProviderInfo{
			ID:            providerID,
			Name:          p.Name,
			DisplayName:   displayName,
			JITEnabled:    p.JITEnabled,
			IconURL:       p.IconURL,
			ButtonColor:   p.ButtonColor,
			OIDCLoginOnly: p.OIDCLoginOnly,
		}

		ok, lastCheck := s.getProviderInfo(providerID)
		if ok {
			info.Active = true
		} else {
			info.LastCheck = lastCheck
		}

		if id, ok := linkedIdentities[p.issuer()]; ok {
			info.IsLinked = true
			info.LinkedIdentityID = fmt.Sprintf("gid://axonhub/OIDCIdentity/%d", id.ID)
			info.LinkedEmail = id.Email
		}

		providers = append(providers, info)
	}

	return providers
}

// IsUserRestrictedToOIDC checks if the user is required to login via OIDC only
// because they are linked to an OIDC provider that has OIDCLoginOnly enabled.
func (s *OIDCService) IsUserRestrictedToOIDC(ctx context.Context, u *ent.User) bool {
	if u == nil {
		return false
	}

	// 1. Check if user's password is the magic placeholder
	if u.Password == OIDC_ONLY_PLACEHOLDER {
		return true
	}

	// 2. Check if any of the user's linked OIDC providers have OIDCLoginOnly enabled
	//
	// Use OidcIdentitiesOrErr so we only re-query when the edge was genuinely not
	// eager-loaded (it returns a NotLoadedError in that case), instead of relying on
	// len()==0 which cannot tell "not loaded" from "loaded but the user has none".
	identities, err := u.Edges.OidcIdentitiesOrErr()
	if ent.IsNotLoaded(err) {
		// The edge was not loaded. This runs during pre-auth sign-in, where there is
		// no viewer in the context yet, so the privacy policy on OIDCIdentity would
		// deny a normal query ("no user in context"). Read the user's own OIDC links
		// under a system bypass, mirroring the user lookup in AuthService.AuthenticateUser.
		identities, err = authz.RunWithSystemBypass(ctx, "oidc-restriction-check",
			func(bypassCtx context.Context) ([]*ent.OIDCIdentity, error) {
				return s.entFromContext(bypassCtx).OIDCIdentity.Query().
					Where(oidcidentity.UserID(u.ID)).
					All(bypassCtx)
			})
		if err != nil {
			log.Error(ctx, "failed to query user OIDC identities", zap.Error(err), log.Int("user_id", u.ID))
			return false
		}
	}

	for _, id := range identities {
		for _, p := range s.cfg.Providers {
			if p.issuer() == id.Issuer && p.OIDCLoginOnly {
				return true
			}
		}
	}

	return false
}

func (s *OIDCService) GetAuthorizeURL(ctx context.Context, providerIdentifier string, baseURL string) (string, string, error) {
	p, _, ok := s.getProviderByIdentifier(providerIdentifier)
	if !ok {
		log.Error(ctx, "OIDC provider not found in map", log.String("provider", providerIdentifier))

		cfgProvider, providerID, found := findOIDCProviderConfig(s.cfg.Providers, providerIdentifier)
		if !found {
			return "", "", fmt.Errorf("Provider not found")
		}

		now := time.Now().Unix()

		lastCheck := s.markProviderCheck(providerID, now)
		if remaining := 60 - (now - lastCheck); remaining > 0 {
			return "", "", fmt.Errorf("Please wait %d seconds before retrying this provider", remaining)
		}

		numProviders := len(s.cfg.Providers)

		redirectURL := "/oauth/oidc/callback"
		if numProviders > 1 {
			redirectURL = fmt.Sprintf("/oauth/oidc/callback/%s", providerID)
		}

		scopes := cfgProvider.ExtraScopes
		if len(scopes) == 0 {
			scopes = []string{oidc.ScopeOpenID, "profile", "email"}
		}

		provider, err := oidc.NewProvider(ctx, cfgProvider.IssuerURL)
		if err != nil {
			return "", "", fmt.Errorf("Failed to initialize OIDC provider: %w", err)
		}

		oauth2Config := oauth2.Config{
			ClientID:     cfgProvider.ClientID,
			ClientSecret: cfgProvider.ClientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  redirectURL,
			Scopes:       scopes,
		}

		reinitializedProvider := &oidcProvider{
			config:   *cfgProvider,
			oauth2:   oauth2Config,
			oidc:     provider,
			verifier: provider.Verifier(&oidc.Config{ClientID: cfgProvider.ClientID}),
		}

		s.setProvider(providerID, reinitializedProvider)
		p = reinitializedProvider
	}

	// Make redirect URL absolute
	oauth2Config := p.oauth2
	if baseURL != "" && !strings.HasPrefix(p.oauth2.RedirectURL, "http") {
		oauth2Config.RedirectURL = baseURL + p.oauth2.RedirectURL
	}

	stateBytes := make([]byte, 32)
	_, _ = rand.Read(stateBytes)
	state := base64.RawURLEncoding.EncodeToString(stateBytes)

	// Store state in cache to prevent CSRF (valid for 10 minutes)
	err := s.cache.Set(ctx, "oidc_state:"+state, []byte("1"), store.WithExpiration(10*time.Minute))
	if err != nil {
		return "", "", fmt.Errorf("failed to cache state: %w", err)
	}

	var opts []oauth2.AuthCodeOption

	var pkceVerifier string

	if p.config.EnablePKCE {
		pkceVerifierBytes := make([]byte, 32)
		_, _ = rand.Read(pkceVerifierBytes)
		pkceVerifier = base64.RawURLEncoding.EncodeToString(pkceVerifierBytes)

		// Store PKCE verifier in cache mapped to state (valid for 10 minutes)
		err := s.cache.Set(ctx, "oidc_pkce:"+state, []byte(pkceVerifier), store.WithExpiration(10*time.Minute))
		if err != nil {
			return "", "", fmt.Errorf("failed to cache PKCE verifier: %w", err)
		}

		opts = append(opts, oauth2.SetAuthURLParam("code_challenge", oauth2.S256ChallengeFromVerifier(pkceVerifier)))
		opts = append(opts, oauth2.SetAuthURLParam("code_challenge_method", "S256"))
	}

	authURL := oauth2Config.AuthCodeURL(state, opts...)

	return authURL, state, nil
}

func (s *OIDCService) GetLinkAuthorizeURL(ctx context.Context, providerIdentifier string, baseURL string, userID int) (string, string, error) {
	authURL, state, err := s.GetAuthorizeURL(ctx, providerIdentifier, baseURL)
	if err != nil {
		return "", "", err
	}

	// Cache the intent to link this identity to a specific user
	err = s.cache.Set(ctx, "oidc_link_state:"+state, []byte(strconv.Itoa(userID)), store.WithExpiration(10*time.Minute))
	if err != nil {
		return "", "", fmt.Errorf("failed to cache link state: %w", err)
	}

	return authURL, state, nil
}

func (s *OIDCService) Callback(ctx context.Context, providerIdentifier, code, state, baseURL string) (string, string, error) {
	// Elevate privileges for database operations as this is an unauthenticated flow
	ctx = contexts.WithUser(ctx, &ent.User{IsOwner: true})

	p, _, ok := s.getProviderByIdentifier(providerIdentifier)
	if !ok {
		return "", "", fmt.Errorf("OIDC provider not found: %s", providerIdentifier)
	}

	// 1. Validate state parameter (CSRF protection)
	stateExists, err := s.cache.Get(ctx, "oidc_state:"+state)
	if err != nil || len(stateExists) == 0 {
		return "", "", fmt.Errorf("invalid or expired state parameter")
	}

	_ = s.cache.Delete(ctx, "oidc_state:"+state) // Consume state

	var opts []oauth2.AuthCodeOption

	if p.config.EnablePKCE {
		verifierBytes, err := s.cache.Get(ctx, "oidc_pkce:"+state)
		if err != nil || len(verifierBytes) == 0 {
			return "", "", fmt.Errorf("invalid PKCE verifier or verifier expired")
		}

		opts = append(opts, oauth2.SetAuthURLParam("code_verifier", string(verifierBytes)))
		_ = s.cache.Delete(ctx, "oidc_pkce:"+state) // Consume once
	}

	oauth2Config := p.oauth2
	if baseURL != "" && !strings.HasPrefix(p.oauth2.RedirectURL, "http") {
		oauth2Config.RedirectURL = baseURL + p.oauth2.RedirectURL
	}

	oauth2Token, err := oauth2Config.Exchange(ctx, code, opts...)
	if err != nil {
		return "", "", fmt.Errorf("failed to exchange authorization code: %w", err)
	}

	var (
		subject string
		claims  oidcClaims
	)

	rawIDToken, _ := oauth2Token.Extra("id_token").(string)
	if rawIDToken != "" {
		if p.verifier == nil {
			return "", "", fmt.Errorf("OIDC verifier not initialized for provider %s", providerIdentifier)
		}

		idToken, err := p.verifier.Verify(ctx, rawIDToken)
		if err != nil {
			return "", "", fmt.Errorf("failed to verify ID token: %w", err)
		}

		subject = idToken.Subject

		var rawClaims map[string]any
		if err := idToken.Claims(&rawClaims); err != nil {
			return "", "", fmt.Errorf("failed to parse claims: %w", err)
		}

		_ = mapstructure.Decode(rawClaims, &claims)

		claims.Groups = s.extractGroups(rawClaims, p)
	}

	// If id_token is missing OR UserInfoURL is provided, fetch extra claims
	if rawIDToken == "" || p.config.UserInfoURL != "" {
		if rawIDToken == "" && p.config.UserInfoURL == "" {
			return "", "", fmt.Errorf("no id_token and no user_info_url provided for provider %s", providerIdentifier)
		}

		userInfoClaims, err := s.fetchUserInfo(ctx, p, oauth2Token)
		if err != nil {
			if rawIDToken == "" {
				return "", "", fmt.Errorf("failed to fetch user info for OAuth2-only provider: %w", err)
			}

			log.Warn(ctx, "Failed to fetch UserInfo", log.String("provider", providerIdentifier), zap.Error(err))
		} else {
			if subject == "" {
				subject = userInfoClaims.Sub
			}

			// Merge claims (UserInfo usually has more up-to-date data)
			if userInfoClaims.Email != "" {
				claims.Email = userInfoClaims.Email
			}

			if userInfoClaims.EmailVerified {
				claims.EmailVerified = userInfoClaims.EmailVerified
			}

			if userInfoClaims.Name != "" {
				claims.Name = userInfoClaims.Name
			}

			if userInfoClaims.GivenName != "" {
				claims.GivenName = userInfoClaims.GivenName
			}

			if userInfoClaims.FamilyName != "" {
				claims.FamilyName = userInfoClaims.FamilyName
			}

			if userInfoClaims.Picture != "" {
				claims.Picture = userInfoClaims.Picture
			}

			if len(userInfoClaims.Groups) > 0 {
				claims.Groups = userInfoClaims.Groups
			}
		}
	}

	if subject == "" {
		return "", "", fmt.Errorf("failed to resolve subject from id_token or user_info")
	}

	// Check if this is a linking flow
	linkUserIDBytes, err := s.cache.Get(ctx, "oidc_link_state:"+state)
	if err == nil && len(linkUserIDBytes) > 0 {
		// Consume link state
		_ = s.cache.Delete(ctx, "oidc_link_state:"+state)

		userID, err := strconv.Atoi(string(linkUserIDBytes))
		if err != nil {
			return "", "", fmt.Errorf("invalid cached link user ID: %w", err)
		}

		err = s.createIdentity(ctx, userID, p.config.issuer(), subject, claims.Email, p.config.providerDisplayName())
		if err != nil {
			return "", "", fmt.Errorf("failed to link identity: %w", err)
		}
		// Let the API caller know this was a link operation
		return "", "link", nil
	}

	userEntity, err := s.resolveUser(ctx, p, subject, claims.Email, claims.EmailVerified, claims.Name, claims.GivenName, claims.FamilyName, claims.Picture, claims.Groups)
	if err != nil {
		return "", "", err
	}

	if userEntity.Status == "deactivated" {
		return "", "", fmt.Errorf("User account is deactivated")
	}

	// Generate short-lived exchange code
	exchangeCodeBytes := make([]byte, 32)
	_, _ = rand.Read(exchangeCodeBytes)
	exchangeCode := hex.EncodeToString(exchangeCodeBytes)

	// Cache user ID for exchange (valid for 5 mins)
	err = s.cache.Set(ctx, "oidc_exchange:"+exchangeCode, fmt.Appendf(nil, "%d", userEntity.ID), store.WithExpiration(5*time.Minute))
	if err != nil {
		return "", "", fmt.Errorf("failed to cache exchange code: %w", err)
	}

	return exchangeCode, "login", nil
}

type oidcClaims struct {
	Sub               string   `json:"sub"`
	Email             string   `json:"email"`
	EmailVerified     bool     `json:"email_verified"`
	Name              string   `json:"name"`
	GivenName         string   `json:"given_name"`
	FamilyName        string   `json:"family_name"`
	PreferredUsername string   `json:"preferred_username"`
	Picture           string   `json:"picture"`
	Groups            []string `json:"-"` // Filled manually from GroupClaim
}

func (s *OIDCService) fetchUserInfo(ctx context.Context, p *oidcProvider, token *oauth2.Token) (*oidcClaims, error) {
	if p.oidc != nil {
		userInfo, err := p.oidc.UserInfo(ctx, oauth2.StaticTokenSource(token))
		if err != nil {
			return nil, err
		}

		var claims oidcClaims
		if err := userInfo.Claims(&claims); err != nil {
			return nil, err
		}

		// Fetch groups from claims
		var raw map[string]any
		if err := userInfo.Claims(&raw); err == nil {
			claims.Groups = s.extractGroups(raw, p)
		}

		return &claims, nil
	}

	// Manual fetch via UserInfoURL with a timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.config.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}

	token.SetAuthHeader(req)

	// Use a client with a default timeout as well for safety
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo request failed with status: %d", resp.StatusCode)
	}

	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	var claims oidcClaims

	_ = mapstructure.Decode(raw, &claims)
	claims.Groups = s.extractGroups(raw, p)

	// GitHub fallback: 'sub' might be called 'id' or 'login'
	if claims.Sub == "" {
		if id, ok := raw["id"].(float64); ok {
			claims.Sub = fmt.Sprintf("%.0f", id)
		} else if login, ok := raw["login"].(string); ok {
			claims.Sub = login
		}
	}

	return &claims, nil
}

func (s *OIDCService) extractGroups(raw map[string]any, p *oidcProvider) []string {
	var claims []string
	if len(p.config.GroupClaims) > 0 {
		claims = p.config.GroupClaims
	} else {
		claims = []string{"groups", "roles"} // fallback defaults
	}

	var allGroups []string

	for _, c := range claims {
		if grps := parseGroups(raw[c]); len(grps) > 0 {
			allGroups = append(allGroups, grps...)
		}
	}

	if len(allGroups) == 0 {
		return nil
	}

	var re *regexp.Regexp

	if p.config.GroupParser.RegexReplacePattern != "" {
		compiled, err := regexp.Compile(p.config.GroupParser.RegexReplacePattern)
		if err == nil {
			re = compiled
		} else {
			log.Warn(context.Background(), "Invalid GroupParser regex", zap.Error(err))
		}
	}

	results := make([]string, 0, len(allGroups))
	for _, g := range allGroups {
		if re != nil {
			g = re.ReplaceAllString(g, p.config.GroupParser.RegexReplaceWith)
		}

		if !p.config.GroupParser.CaseSensitive {
			g = strings.ToLower(g)
		}

		results = append(results, g)
	}

	return results
}

func parseGroups(v any) []string {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case string:
		return []string{val}
	case []any:
		res := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				res = append(res, s)
			}
		}

		return res
	case []string:
		return val
	}

	return nil
}

func (s *OIDCService) resolveUser(ctx context.Context, p *oidcProvider, subject, email string, emailVerified bool, name, givenName, familyName, picture string, groups []string) (*ent.User, error) {
	// 1. Try to find existing OIDC identity by issuer and subject
	identity, err := s.entFromContext(ctx).OIDCIdentity.Query().
		Where(
			oidcidentity.Issuer(p.config.issuer()),
			oidcidentity.Subject(subject),
		).
		WithUser().
		Only(ctx)
	if err == nil {
		// Update last login
		_, _ = identity.Update().SetLastLoginAt(time.Now()).Save(ctx)

		// Sync user info if enabled
		if p.config.SyncUserInfo && identity.Edges.User != nil {
			updatedUser, err := s.syncUserInfo(ctx, identity.Edges.User, name, givenName, familyName, picture, groups, p.config)
			if err != nil {
				log.Warn(ctx, "Failed to sync user info during OIDC login", zap.Error(err), log.Int("user_id", identity.UserID))
			} else {
				return updatedUser, nil
			}
		}

		if identity.Edges.User == nil {
			return nil, fmt.Errorf("OIDC identity linked to missing or deleted user")
		}

		return identity.Edges.User, nil
	} else if !ent.IsNotFound(err) {
		return nil, fmt.Errorf("database error querying identity: %w", err)
	}

	// 2. Identity not found. Check if an account with this email already exists (and is verified if required).
	// This follows the "Account First" logic: if a user exists, we link to them.
	if p.config.AutoLinkByEmail && email != "" && emailVerified {
		existingUser, err := s.entFromContext(ctx).User.Query().Where(user.Email(email)).Only(ctx)
		if err == nil {
			// Found user by email, link this OIDC identity to them.
			err = s.createIdentity(ctx, existingUser.ID, p.config.issuer(), subject, email, p.config.Name)
			if err != nil {
				return nil, fmt.Errorf("failed to link OIDC identity: %w", err)
			}

			// Sync user info if enabled
			if p.config.SyncUserInfo {
				updatedUser, err := s.syncUserInfo(ctx, existingUser, name, givenName, familyName, picture, groups, p.config)
				if err != nil {
					log.Warn(ctx, "Failed to sync user info during OIDC link", zap.Error(err), log.Int("user_id", existingUser.ID))
				} else {
					return updatedUser, nil
				}
			}

			return existingUser, nil
		} else if !ent.IsNotFound(err) {
			return nil, fmt.Errorf("database error querying user by email: %w", err)
		}
	}

	// 3. No existing account found. Check if JIT Provisioning is enabled.
	if !p.config.JITEnabled {
		return nil, fmt.Errorf("account not found and JIT provisioning is disabled")
	}

	// Check if email verification is required for new accounts
	if p.config.RequireEmailVerified && !emailVerified {
		return nil, fmt.Errorf("email not verified")
	}

	if email == "" {
		// Generate placeholder email if none provided
		email = fmt.Sprintf("%s@%s.oidc", subject, p.config.Name)
	}

	firstName := givenName
	lastName := familyName

	if firstName == "" && lastName == "" && name != "" {
		parts := strings.SplitN(name, " ", 2)
		firstName = parts[0]

		if len(parts) > 1 {
			lastName = parts[1]
		}
	}

	// Set a magic password indicating this user must login via OIDC only.
	password := OIDC_ONLY_PLACEHOLDER

	// Create the User and Identity record within a transaction to avoid orphaned users
	var createdUser *ent.User

	err = s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)
		userCreate := client.User.Create().
			SetEmail(email).
			SetFirstName(firstName).
			SetLastName(lastName).
			SetPassword(password)

		if picture != "" {
			userCreate.SetAvatar(picture)
		}

		// Apply role mappings to new user
		if err := s.applyRoleMappings(ctx, userCreate.Mutation(), groups, p.config, true); err != nil {
			return fmt.Errorf("failed to apply role mappings: %w", err)
		}

		createdUser, err = userCreate.Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}

		// Create the Identity record
		_, err = client.OIDCIdentity.Create().
			SetUserID(createdUser.ID).
			SetIssuer(p.config.issuer()).
			SetSubject(subject).
			SetEmail(email).
			SetIdpName(p.config.providerDisplayName()).
			SetLastLoginAt(time.Now()).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create identity for new user: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return createdUser, nil
}

func (s *OIDCService) syncUserInfo(ctx context.Context, u *ent.User, name, givenName, familyName, picture string, groups []string, cfg OIDCProvider) (*ent.User, error) {
	firstName := givenName
	lastName := familyName

	if firstName == "" && lastName == "" && name != "" {
		parts := strings.SplitN(name, " ", 2)

		firstName = parts[0]
		if len(parts) > 1 {
			lastName = parts[1]
		}
	}

	update := u.Update()

	if firstName != "" || lastName != "" {
		update.SetFirstName(firstName).SetLastName(lastName)
	}

	if picture != "" {
		update.SetAvatar(picture)
	}

	// Sync roles/scopes
	if cfg.SyncRoleStrategy != "create_only" {
		if err := s.applyRoleMappings(ctx, update.Mutation(), groups, cfg, false); err != nil {
			return nil, fmt.Errorf("failed to apply role mappings: %w", err)
		}
	}

	return update.Save(ctx)
}

func (s *OIDCService) applyRoleMappings(ctx context.Context, m ent.Mutation, groups []string, cfg OIDCProvider, isCreate bool) error {
	um, ok := m.(*ent.UserMutation)
	if !ok {
		return fmt.Errorf("expected UserMutation")
	}

	var (
		isOwner bool
		scopes  []string
	)

	matchedRules := make([]RoleMappingRule, 0)
	matchedAnyGroup := false

	// Flatten matching groups
	for _, group := range groups {
		for _, rule := range cfg.RoleMappingRules {
			matchGroup := rule.MatchGroup
			if !cfg.GroupParser.CaseSensitive {
				matchGroup = strings.ToLower(matchGroup)
			}

			matched := false
			if rule.IsRegex {
				matched, _ = regexp.MatchString(matchGroup, group)
			} else {
				matched, _ = filepath.Match(matchGroup, group)
				if !matched && matchGroup == group {
					// Fallback to strict string match if glob syntax fails
					matched = true
				}
			}

			if matched {
				matchedAnyGroup = true

				matchedRules = append(matchedRules, rule)
			}
		}
	}

	// Calculate roles and scopes
	if !matchedAnyGroup {
		// Apply defaults
		scopes = append(scopes, cfg.DefaultScopes...)
		for _, dr := range cfg.DefaultRoles {
			matchedRules = append(matchedRules, RoleMappingRule{DBRole: dr, Priority: 0})
		}
	} else {
		// Precedence logic on matched rules
		if cfg.RolePrecedenceMode == "highest" && len(matchedRules) > 0 {
			sort.Slice(matchedRules, func(i, j int) bool {
				return matchedRules[i].Priority > matchedRules[j].Priority
			})
			matchedRules = []RoleMappingRule{matchedRules[0]}
		}
	}

	var dbRolesToCompile []string
	// Calculate roles and scopes
	for _, rule := range matchedRules {
		db_role := strings.TrimSpace(rule.DBRole)
		if db_role == "system:owner" {
			isOwner = true
		} else if db_role, ok := strings.CutPrefix(db_role, "scope:"); ok {
			scopes = append(scopes, db_role)
		} else {
			dbRolesToCompile = append(dbRolesToCompile, db_role)
		}
	}

	if isOwner {
		um.SetIsOwner(true)
	} else if len(cfg.RoleMappingRules) > 0 || len(cfg.DefaultRoles) > 0 {
		// Only clear owner flag if role mappings are explicitly configured
		um.SetIsOwner(false)
	}

	if len(scopes) > 0 {
		um.SetScopes(scopes)
	} else if len(cfg.RoleMappingRules) > 0 || len(cfg.DefaultScopes) > 0 {
		// Only clear scopes if role mappings or default scopes are explicitly configured
		um.ClearScopes()
	}

	// Roles logic depending on strategy
	if len(dbRolesToCompile) > 0 {
		client := s.entFromContext(ctx)

		roleEntities, err := client.Role.Query().Where(role.NameIn(dbRolesToCompile...)).All(ctx)
		if err == nil && len(roleEntities) > 0 {
			var roleIDs []int
			for _, r := range roleEntities {
				roleIDs = append(roleIDs, r.ID)
			}

			// Apply strategy logic depending on isCreate or SyncRoleStrategy
			strategy := cfg.SyncRoleStrategy
			if strategy == "" {
				strategy = "always"
			}

			if isCreate {
				um.AddRoleIDs(roleIDs...)
			} else {
				switch strategy {
				case "always":
					um.ClearRoles()
					um.AddRoleIDs(roleIDs...)
				case "merge":
					um.AddRoleIDs(roleIDs...)
				case "manual_first":
					// Manual first approach: we check if user has existing project-based roles or something, but realistically we would need to inspect existing manually set roles.
					// Since we can't easily differentiate manual vs provider here without complex schema, we will treat it as a skip if they have any roles.
					userID, exists := um.ID()
					if exists {
						existingRolesCount, _ := client.Role.Query().Where(role.HasUsersWith(user.IDEQ(userID))).Count(ctx)
						if existingRolesCount == 0 {
							um.AddRoleIDs(roleIDs...)
						}
					}
				}
			}
		}
	} else {
		// No DB roles to map, if strategy is always, clear existing roles
		strategy := cfg.SyncRoleStrategy
		if strategy == "" {
			strategy = "always"
		}

		if !isCreate && strategy == "always" && (len(cfg.RoleMappingRules) > 0 || len(cfg.DefaultRoles) > 0) {
			um.ClearRoles()
		}
	}

	return nil
}

func (s *OIDCService) createIdentity(ctx context.Context, userID int, issuer, subject, email, idpName string) error {
	_, err := s.entFromContext(ctx).OIDCIdentity.Create().
		SetUserID(userID).
		SetIssuer(issuer).
		SetSubject(subject).
		SetEmail(email).
		SetIdpName(idpName).
		SetLastLoginAt(time.Now()).
		Save(ctx)

	return err
}

func (s *OIDCService) ExchangeCode(ctx context.Context, code string) (*ent.User, error) {
	// Elevate privileges for user query as this is an unauthenticated flow
	ctx = contexts.WithUser(ctx, &ent.User{IsOwner: true})

	cacheKey := "oidc_exchange:" + code

	// Acquire a per-code lock to prevent concurrent redemption of the same exchange code.
	// Without this, two concurrent requests could both pass the Get check before either
	// executes Delete, resulting in multiple valid JWTs for a single code.
	lock := &sync.Mutex{}

	actual, loaded := s.exchangeLocks.LoadOrStore(cacheKey, lock)
	if loaded {
		var ok bool
		lock, ok = actual.(*sync.Mutex)
		if !ok {
			return nil, fmt.Errorf("internal error: invalid exchange lock type")
		}
	}

	lock.Lock()
	defer func() {
		lock.Unlock()
		s.exchangeLocks.Delete(cacheKey)
	}()

	userIDBytes, err := s.cache.Get(ctx, cacheKey)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired exchange code")
	}

	// Delete the code immediately so it can only be used once
	_ = s.cache.Delete(ctx, cacheKey)

	userID, err := strconv.Atoi(string(userIDBytes))
	if err != nil {
		return nil, fmt.Errorf("invalid user ID format in cache: %w", err)
	}

	user, err := s.entFromContext(ctx).User.Get(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return user, nil
}
