package biz

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
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
	Name       string `json:"name"`
	JITEnabled bool   `json:"jit_enabled"`
}

type OIDCProvider struct {
	Name                  string            `conf:"name" yaml:"name" json:"name"`
	IssuerURL             string            `conf:"issuer_url" yaml:"issuer_url" json:"issuer_url"`
	ClientID              string            `conf:"client_id" yaml:"client_id" json:"client_id"`
	ClientSecret          string            `conf:"client_secret" yaml:"client_secret" json:"client_secret"`
	Scopes                []string          `conf:"scopes" yaml:"scopes" json:"scopes"`
	JITEnabled            bool              `conf:"jit_enabled" yaml:"jit_enabled" json:"jit_enabled"`
	AutoLinkByEmail       bool              `conf:"auto_link_by_email" yaml:"auto_link_by_email" json:"auto_link_by_email"`
	RoleMappings          map[string]string `conf:"role_mappings" yaml:"role_mappings" json:"role_mappings"`
	RoleMappingPrecedence string            `conf:"role_mapping_precedence" yaml:"role_mapping_precedence" json:"role_mapping_precedence"`
	EnablePKCE            bool              `conf:"enable_pkce" yaml:"enable_pkce" json:"enable_pkce"`
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
	for _, p := range params.Config.Providers {
		provider, err := oidc.NewProvider(ctx, p.IssuerURL)
		if err != nil {
			log.Error(ctx, "Failed to initialize OIDC provider", log.String("provider", p.Name), zap.Error(err))
			continue
		}

		// This redirect URI is for IdP -> backend callback handling.
		// The backend will then issue a short-lived exchange code and redirect to
		// the frontend callback route: /oauth/oidc/idp-callback?code=...
		redirectURL := "/oauth/oidc/callback"
		if numProviders > 1 {
			redirectURL = fmt.Sprintf("/oauth/oidc/callback/%s", p.Name)
		}

		scopes := p.Scopes
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

func (s *OIDCService) CountProviders() int {
	return len(s.cfg.Providers)
}

func (s *OIDCService) GetProviders(ctx context.Context) []ProviderInfo {
	var providers []ProviderInfo
	for _, p := range s.cfg.Providers {
		providers = append(providers, ProviderInfo{
			Name:       p.Name,
			JITEnabled: p.JITEnabled,
		})
	}
	return providers
}

func (s *OIDCService) GetAuthorizeURL(ctx context.Context, providerName string, baseURL string) (string, string, error) {
	p, ok := s.providers[providerName]
	if !ok {
		return "", "", fmt.Errorf("OIDC provider not found")
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

func (s *OIDCService) Callback(ctx context.Context, providerName, code, state string) (string, error) {
	p, ok := s.providers[providerName]
	if !ok {
		return "", fmt.Errorf("OIDC provider not found")
	}

	var opts []oauth2.AuthCodeOption
	if p.config.EnablePKCE {
		verifierBytes, err := s.cache.Get(ctx, "oidc_pkce:"+state)
		if err != nil || len(verifierBytes) == 0 {
			return "", fmt.Errorf("invalid state parameter: invalid state or PKCE verifier expired")
		}
		opts = append(opts, oauth2.SetAuthURLParam("code_verifier", string(verifierBytes)))
		_ = s.cache.Delete(ctx, "oidc_pkce:"+state) // Consume once
	}

	oauth2Token, err := p.oauth2.Exchange(ctx, code, opts...)
	if err != nil {
		return "", fmt.Errorf("failed to exchange authorization code: %w", err)
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return "", fmt.Errorf("No id_token field in oauth2 token")
	}

	verifier := p.oidc.Verifier(&oidc.Config{ClientID: p.config.ClientID})
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return "", fmt.Errorf("failed to verify ID token: %w", err)
	}

	var claims struct {
		Email             string `json:"email"`
		EmailVerified     bool   `json:"email_verified"`
		Name              string `json:"name"`
		GivenName         string `json:"given_name"`
		FamilyName        string `json:"family_name"`
		PreferredUsername string `json:"preferred_username"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return "", fmt.Errorf("failed to parse claims: %w", err)
	}

	userEntity, err := s.resolveUser(ctx, p, idToken.Subject, claims.Email, claims.EmailVerified, claims.Name)
	if err != nil {
		return "", err
	}

	if userEntity.Status == "deactivated" {
		return "", fmt.Errorf("User account is deactivated")
	}

	// Generate short-lived exchange code
	exchangeCodeBytes := make([]byte, 32)
	_, _ = rand.Read(exchangeCodeBytes)
	exchangeCode := hex.EncodeToString(exchangeCodeBytes)

	// Cache user ID for exchange (valid for 5 mins)
	err = s.cache.Set(ctx, "oidc_exchange:"+exchangeCode, []byte(fmt.Sprintf("%d", userEntity.ID)), store.WithExpiration(5*time.Minute))
	if err != nil {
		return "", fmt.Errorf("failed to cache exchange code: %w", err)
	}

	return exchangeCode, nil
}

func (s *OIDCService) resolveUser(ctx context.Context, p *oidcProvider, subject, email string, emailVerified bool, name string) (*ent.User, error) {
	// Elevate privileges for OIDC user resolution as this is an unauthenticated flow
	ctx = contexts.WithUser(ctx, &ent.User{IsOwner: true})

	// 1. Try to find existing OIDC identity
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
		return identity.Edges.User, nil
	} else if !ent.IsNotFound(err) {
		return nil, fmt.Errorf("database error querying identity: %w", err)
	}

	// 2. Identity not found. Check auto-link
	if p.config.AutoLinkByEmail && email != "" && emailVerified {
		existingUser, err := s.db.User.Query().Where(user.Email(email)).Only(ctx)
		if err == nil {
			// Found user, link identity
			err = s.createIdentity(ctx, existingUser.ID, p.config.IssuerURL, subject, email, p.config.Name)
			if err != nil {
				return nil, fmt.Errorf("failed to link identity: %w", err)
			}
			return existingUser, nil
		} else if !ent.IsNotFound(err) {
			return nil, fmt.Errorf("database error querying user by email: %w", err)
		}
	}

	// 3. JIT Provisioning
	if !p.config.JITEnabled {
		return nil, fmt.Errorf("User not found and JIT provisioning is disabled")
	}

	if email == "" {
		// Generate placeholder email if none provided
		email = fmt.Sprintf("%s@%s.oidc", subject, p.config.Name)
	}

	// Generate a cryptographically secure random password for JIT users.
	// These users are expected to authenticate via OIDC only.
	randPasswordBytes := make([]byte, 32)
	if _, err := rand.Read(randPasswordBytes); err != nil {
		return nil, fmt.Errorf("failed to generate secure password: %w", err)
	}
	randPassword := hex.EncodeToString(randPasswordBytes)

	newUser, err := s.db.User.Create().
		SetEmail(email).
		SetFirstName(name).
		SetPassword(randPassword).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to provision user: %w", err)
	}

	err = s.createIdentity(ctx, newUser.ID, p.config.IssuerURL, subject, email, p.config.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create identity for new user: %w", err)
	}

	return newUser, nil
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
