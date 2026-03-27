package biz

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/eko/gocache/lib/v4/store"
	"go.uber.org/fx"
	"golang.org/x/oauth2"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/oidcidentity"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"go.uber.org/zap"
)

type ProviderInfo struct {
	Name             string `json:"name"`
	DisplayName      string `json:"display_name"`
	JITEnabled       bool   `json:"jit_enabled"`
	IconURL          string `json:"icon_url"`
	ButtonColor      string `json:"button_color"`
	IsLinked         bool   `json:"is_linked"`
	LinkedIdentityID string `json:"linked_identity_id,omitempty"`
	LinkedEmail      string `json:"linked_email,omitempty"`
}

type OIDCProvider struct {
	Name                  string            `conf:"name" yaml:"name" json:"name"`
	DisplayName           string            `conf:"display_name" yaml:"display_name" json:"display_name"`
	IssuerURL             string            `conf:"issuer_url" yaml:"issuer_url" json:"issuer_url"`
	ClientID              string            `conf:"client_id" yaml:"client_id" json:"client_id"`
	ClientSecret          string            `conf:"client_secret" yaml:"client_secret" json:"client_secret"`
	ExtraScopes           []string          `conf:"extra_scopes" yaml:"extra_scopes" json:"extra_scopes"`
	JITEnabled            bool              `conf:"jit_enabled" yaml:"jit_enabled" json:"jit_enabled"`
	AutoLinkByEmail       bool              `conf:"auto_link_by_email" yaml:"auto_link_by_email" json:"auto_link_by_email"`
	RequireEmailVerified  bool              `conf:"require_email_verified" yaml:"require_email_verified" json:"require_email_verified"`
	RoleMappings          map[string]string `conf:"role_mappings" yaml:"role_mappings" json:"role_mappings"`
	RoleMappingPrecedence string            `conf:"role_mapping_precedence" yaml:"role_mapping_precedence" json:"role_mapping_precedence"`
	EnablePKCE            bool              `conf:"enable_pkce" yaml:"enable_pkce" json:"enable_pkce"`
	// UI customization
	IconURL      string `conf:"icon_url" yaml:"icon_url" json:"icon_url"`
	ButtonColor  string `conf:"button_color" yaml:"button_color" json:"button_color"`
	SyncUserInfo bool   `conf:"sync_user_info" yaml:"sync_user_info" json:"sync_user_info"`
}

type OIDCConfig struct {
	Providers []OIDCProvider `conf:"providers" yaml:"providers" json:"providers"`
}

type OIDCService struct {
	cfg OIDCConfig

	cache     xcache.Cache[[]byte]
	db        *ent.Client
	providers map[string]*oidcProvider
}

type oidcProvider struct {
	config OIDCProvider

	oauth2 oauth2.Config
	oidc   *oidc.Provider
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
		cfg:       params.Config,
		cache:     xcache.NewFromConfig[[]byte](params.CacheConfig),
		db:        params.DB,
		providers: make(map[string]*oidcProvider),
	}

	numProviders := len(params.Config.Providers)
	for i, p := range params.Config.Providers {
		provider, err := oidc.NewProvider(ctx, p.IssuerURL)
		if err != nil {
			log.Error(ctx, "Failed to initialize OIDC provider", log.String("provider", p.Name), zap.Error(err))
			continue
		}

		// Resolve icon_url: supports http(s) URL, data: URI, or local file path.
		if resolved, err := resolveIconURL(p.IconURL); err != nil {
			log.Error(ctx, "Failed to resolve icon for OIDC provider", log.String("provider", p.Name), zap.Error(err))
		} else {
			p.IconURL = resolved
			params.Config.Providers[i].IconURL = resolved
		}

		// This redirect URI is for IdP -> backend callback handling.
		// The backend will then issue a short-lived exchange code and redirect to
		// the frontend callback route: /oauth/oidc/idp-callback?code=...
		redirectURL := "/oauth/oidc/callback"
		if numProviders > 1 {
			redirectURL = fmt.Sprintf("/oauth/oidc/callback/%s", p.Name)
		}

		scopes := p.ExtraScopes
		if len(scopes) == 0 {
			scopes = []string{oidc.ScopeOpenID, "profile", "email"}
		}

		oauth2Config := oauth2.Config{
			ClientID:     p.ClientID,
			ClientSecret: p.ClientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  redirectURL,
			Scopes:       scopes,
		}

		svc.providers[p.Name] = &oidcProvider{
			config: p,
			oauth2: oauth2Config,
			oidc:   provider,
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

func (s *OIDCService) GetProviders(ctx context.Context) []ProviderInfo {
	var providers []ProviderInfo

	// Get linked issuers for the current user
	linkedIdentities := make(map[string]*ent.OIDCIdentity)
	if u, ok := contexts.GetUser(ctx); ok {
		identities, err := s.db.OIDCIdentity.Query().
			Where(oidcidentity.UserID(u.ID)).
			All(ctx)
		if err == nil {
			for _, id := range identities {
				linkedIdentities[id.Issuer] = id
			}
		}
	}

	for _, p := range s.cfg.Providers {
		displayName := p.DisplayName
		if displayName == "" {
			displayName = p.Name
		}
		info := ProviderInfo{
			Name:        p.Name,
			DisplayName: displayName,
			JITEnabled:  p.JITEnabled,
			IconURL:     p.IconURL,
			ButtonColor: p.ButtonColor,
		}
		if id, ok := linkedIdentities[p.IssuerURL]; ok {
			info.IsLinked = true
			info.LinkedIdentityID = fmt.Sprintf("gid://axonhub/OIDCIdentity/%d", id.ID)
			info.LinkedEmail = id.Email
		}
		providers = append(providers, info)
	}
	return providers
}

func (s *OIDCService) GetAuthorizeURL(ctx context.Context, providerName string, baseURL string) (string, string, error) {
	p, ok := s.providers[providerName]
	if !ok {
		log.Error(ctx, "OIDC provider not found in map", log.String("provider", providerName), log.Int("map_size", len(s.providers)))
		// Try case-insensitive and space-flexible matching
		for k, val := range s.providers {
			log.Debug(ctx, "Available provider in map", log.String("key", k))
			if strings.EqualFold(strings.ReplaceAll(k, " ", ""), strings.ReplaceAll(providerName, " ", "")) {
				log.Info(ctx, "Found OIDC provider using flexible matching", log.String("original", providerName), log.String("matched", k))
				p = val
				ok = true
				break
			}
		}
		if !ok {
			return "", "", fmt.Errorf("OIDC provider not found")
		}
	}

	// Make redirect URL absolute
	oauth2Config := p.oauth2
	if baseURL != "" {
		oauth2Config.RedirectURL = baseURL + p.oauth2.RedirectURL
	}

	stateBytes := make([]byte, 32)
	_, _ = rand.Read(stateBytes)
	state := base64.URLEncoding.EncodeToString(stateBytes)

	var opts []oauth2.AuthCodeOption
	var pkceVerifier string

	if p.config.EnablePKCE {
		pkceVerifierBytes := make([]byte, 32)
		_, _ = rand.Read(pkceVerifierBytes)
		pkceVerifier = base64.URLEncoding.EncodeToString(pkceVerifierBytes)

		// Store PKCE verifier in cache mapped to state
		err := s.cache.Set(ctx, "oidc_pkce:"+state, []byte(pkceVerifier))
		if err != nil {
			return "", "", fmt.Errorf("failed to cache PKCE verifier: %w", err)
		}

		opts = append(opts, oauth2.SetAuthURLParam("code_challenge", oauth2.S256ChallengeFromVerifier(pkceVerifier)))
		opts = append(opts, oauth2.SetAuthURLParam("code_challenge_method", "S256"))
	}

	authURL := oauth2Config.AuthCodeURL(state, opts...)
	return authURL, state, nil
}

func (s *OIDCService) GetLinkAuthorizeURL(ctx context.Context, providerName string, baseURL string, userID int) (string, string, error) {
	authURL, state, err := s.GetAuthorizeURL(ctx, providerName, baseURL)
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

func (s *OIDCService) Callback(ctx context.Context, providerName, code, state string) (string, string, error) {
	// Elevate privileges for database operations as this is an unauthenticated flow
	ctx = contexts.WithUser(ctx, &ent.User{IsOwner: true})

	p, ok := s.providers[providerName]
	if !ok {
		// Try case-insensitive and space-flexible matching
		for k, val := range s.providers {
			if strings.EqualFold(strings.ReplaceAll(k, " ", ""), strings.ReplaceAll(providerName, " ", "")) {
				p = val
				ok = true
				break
			}
		}
		if !ok {
			return "", "", fmt.Errorf("OIDC provider not found: %s", providerName)
		}
	}

	var opts []oauth2.AuthCodeOption
	if p.config.EnablePKCE {
		verifierBytes, err := s.cache.Get(ctx, "oidc_pkce:"+state)
		if err != nil || len(verifierBytes) == 0 {
			return "", "", fmt.Errorf("invalid state parameter: invalid state or PKCE verifier expired")
		}
		opts = append(opts, oauth2.SetAuthURLParam("code_verifier", string(verifierBytes)))
		_ = s.cache.Delete(ctx, "oidc_pkce:"+state) // Consume once
	}

	oauth2Token, err := p.oauth2.Exchange(ctx, code, opts...)
	if err != nil {
		return "", "", fmt.Errorf("failed to exchange authorization code: %w", err)
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return "", "", fmt.Errorf("No id_token field in oauth2 token")
	}

	verifier := p.oidc.Verifier(&oidc.Config{ClientID: p.config.ClientID})
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return "", "", fmt.Errorf("failed to verify ID token: %w", err)
	}

	var claims struct {
		Email             string `json:"email"`
		EmailVerified     bool   `json:"email_verified"`
		Name              string `json:"name"`
		GivenName         string `json:"given_name"`
		FamilyName        string `json:"family_name"`
		PreferredUsername string `json:"preferred_username"`
		Picture           string `json:"picture"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return "", "", fmt.Errorf("failed to parse claims: %w", err)
	}

	// Check if this is a linking flow
	linkUserIDBytes, err := s.cache.Get(ctx, "oidc_link_state:"+state)
	if err == nil && len(linkUserIDBytes) > 0 {
		// Consume link state
		_ = s.cache.Delete(ctx, "oidc_link_state:"+state)
		userID, err := strconv.Atoi(string(linkUserIDBytes))
		if err == nil {
			err = s.createIdentity(ctx, userID, p.config.IssuerURL, idToken.Subject, claims.Email, p.config.Name)
			if err != nil {
				return "", "", fmt.Errorf("failed to link identity: %w", err)
			}
			// Let the API caller know this was a link operation
			return "", "link", nil
		}
	}

	userEntity, err := s.resolveUser(ctx, p, idToken.Subject, claims.Email, claims.EmailVerified, claims.Name, claims.GivenName, claims.FamilyName, claims.Picture)
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
	err = s.cache.Set(ctx, "oidc_exchange:"+exchangeCode, []byte(fmt.Sprintf("%d", userEntity.ID)), store.WithExpiration(5*time.Minute))
	if err != nil {
		return "", "", fmt.Errorf("failed to cache exchange code: %w", err)
	}

	return exchangeCode, "login", nil
}

func (s *OIDCService) resolveUser(ctx context.Context, p *oidcProvider, subject, email string, emailVerified bool, name, givenName, familyName, picture string) (*ent.User, error) {
	// 1. Try to find existing OIDC identity by issuer and subject
	identity, err := s.db.OIDCIdentity.Query().
		Where(
			oidcidentity.Issuer(p.config.IssuerURL),
			oidcidentity.Subject(subject),
		).
		WithUser().
		Only(ctx)

	if err == nil {
		// Update last login
		_, _ = identity.Update().SetLastLoginAt(time.Now()).Save(ctx)

		// Sync user info if enabled
		if p.config.SyncUserInfo && identity.Edges.User != nil {
			updatedUser, err := s.syncUserInfo(ctx, identity.Edges.User, name, givenName, familyName, picture)
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
	if email != "" && emailVerified {
		existingUser, err := s.db.User.Query().Where(user.Email(email)).Only(ctx)
		if err == nil {
			// Found user by email, link this OIDC identity to them.
			err = s.createIdentity(ctx, existingUser.ID, p.config.IssuerURL, subject, email, p.config.Name)
			if err != nil {
				return nil, fmt.Errorf("failed to link OIDC identity: %w", err)
			}

			// Sync user info if enabled
			if p.config.SyncUserInfo {
				updatedUser, err := s.syncUserInfo(ctx, existingUser, name, givenName, familyName, picture)
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

	// Set a magic password indicating this user must login via OIDC only.
	randPassword := OIDC_ONLY_PLACEHOLDER

	firstName := givenName
	lastName := familyName
	if firstName == "" && lastName == "" {
		firstName = name
	}

	// Create the User record FIRST
	userCreate := s.db.User.Create().
		SetEmail(email).
		SetFirstName(firstName).
		SetLastName(lastName).
		SetPassword(randPassword)

	if picture != "" {
		userCreate.SetAvatar(picture)
	}

	newUser, err := userCreate.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Create the Identity record SECOND
	err = s.createIdentity(ctx, newUser.ID, p.config.IssuerURL, subject, email, p.config.Name)
	if err != nil {
		// Note: Since this is a newly created user, failing here leaves an orphaned user.
		// However, with SoftDelete and unique email, re-trying will either hit step 2 or fail.
		return nil, fmt.Errorf("failed to create identity for new user: %w", err)
	}

	return newUser, nil
}

func (s *OIDCService) syncUserInfo(ctx context.Context, u *ent.User, name, givenName, familyName, picture string) (*ent.User, error) {
	firstName := givenName
	lastName := familyName
	if firstName == "" && lastName == "" {
		firstName = name
	}

	update := u.Update()
	if firstName != "" || lastName != "" {
		update.SetFirstName(firstName).SetLastName(lastName)
	}
	if picture != "" {
		update.SetAvatar(picture)
	}

	return update.Save(ctx)
}

func (s *OIDCService) createIdentity(ctx context.Context, userID int, issuer, subject, email, idpName string) error {
	_, err := s.db.OIDCIdentity.Create().
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
	userIDBytes, err := s.cache.
		Get(ctx, cacheKey)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired exchange code")
	}

	userID, err := strconv.Atoi(string(userIDBytes))
	if err != nil {
		return nil, fmt.Errorf("invalid user ID format in cache: %w", err)
	}

	// Delete the code so it can only be used once
	_ = s.cache.Delete(ctx, cacheKey)

	user, err := s.db.User.Get(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return user, nil
}
