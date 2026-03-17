package biz

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"encoding/json"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/oidcidentity"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/log"
	"go.uber.org/zap"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

type ProviderInfo struct {
	Name       string `json:"name"`
	JITEnabled bool   `json:"jit_enabled"`
}


type OIDCConfig struct {
	Enabled   bool           `json:"enabled" yaml:"enabled"`
	Providers []OIDCProvider `json:"providers" yaml:"providers"`
}

type OIDCProvider struct {
	Name            string `json:"name" yaml:"name"`
	DisplayName     string `json:"display_name" yaml:"display_name"`
	Issuer          string `json:"issuer" yaml:"issuer"`
	ClientID        string `json:"client_id" yaml:"client_id"`
	ClientSecret    string `json:"client_secret" yaml:"client_secret"`
	RedirectURL     string `json:"redirect_url" yaml:"redirect_url"`
	EnablePKCE      bool   `json:"enable_pkce" yaml:"enable_pkce"`
	JITEnabled      bool   `json:"jit_enabled" yaml:"jit_enabled"`
	AutoLinkByEmail bool   `json:"auto_link_by_email" yaml:"auto_link_by_email"`
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

func NewOIDCService(cfg OIDCConfig, cache xcache.Cache[[]byte], db *ent.Client) (*OIDCService, error) {
	ctx := context.Background()
	svc := &OIDCService{
		cfg:       cfg,
		cache:     cache,
		db:        db,
		providers: make(map[string]*oidcProvider),
	}

	for _, p := range cfg.Providers {
		provider, err := oidc.NewProvider(ctx, p.Issuer)
		if err != nil {
			log.Error(ctx, "Failed to initialize OIDC provider", log.String("provider", p.Name), zap.Error(err))
			continue
		}

		redirectURL := fmt.Sprintf("%s/api/admin/auth/oidc/%s/callback", strings.TrimRight("", "/"), p.Name)
		
		oauth2Config := oauth2.Config{
			ClientID:     p.ClientID,
			ClientSecret: p.ClientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  redirectURL,
			Scopes:       append([]string{oidc.ScopeOpenID, "profile", "email"}, []string{oidc.ScopeOpenID, "profile", "email"}...),
		}

		svc.providers[p.Name] = &oidcProvider{
			config: p,
			oauth2: oauth2Config,
			oidc:   provider,
		}
	}

	return svc, nil
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

func (s *OIDCService) GetAuthorizeURL(ctx context.Context, providerName string) (string, string, error) {
	p, ok := s.providers[providerName]
	if !ok {
		return "", "", fmt.Errorf("OIDC provider not found")
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

	authURL := p.oauth2.AuthCodeURL(state, opts...)
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
			return "", fmt.Errorf("Invalid state parameter", "Invalid state or PKCE verifier expired")
		}
		opts = append(opts, oauth2.SetAuthURLParam("code_verifier", string(verifierBytes)))
		_ = s.cache.Delete(ctx, "oidc_pkce:"+state) // Consume once
	}

	oauth2Token, err := p.oauth2.Exchange(ctx, code, opts...)
	if err != nil {
		return "", fmt.Errorf("Failed to exchange authorization code: "+err.Error())
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return "", fmt.Errorf("No id_token field in oauth2 token")
	}

	verifier := p.oidc.Verifier(&oidc.Config{ClientID: p.config.ClientID})
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return "", fmt.Errorf("Failed to verify ID Token: "+err.Error())
	}

	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
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
	err = s.cache.Set(ctx, "oidc_exchange:"+exchangeCode, []byte(fmt.Sprintf("%d", user.ID)))
	if err != nil {
		return "", fmt.Errorf("failed to cache exchange code: %w", err)
	}

	return exchangeCode, nil
}

func (s *OIDCService) resolveUser(ctx context.Context, p *oidcProvider, subject, email string, emailVerified bool, name string) (*ent.User, error) {
	// 1. Try to find existing OIDC identity
	identity, err := s.db.OIDCIdentity.Query().
		Where(
			oidcidentity.Issuer(p.config.Issuer),
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
			err = s.createIdentity(ctx, existingUser.ID, p.config.Issuer, subject, email, p.config.Name)
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

	newUser, err := s.db.User.Create().
		SetEmail(email).
		SetFirstName(name).
		SetPassword("oidc_sso_" + base64.StdEncoding.EncodeToString([]byte(subject))). // Dummy password
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to provision user: %w", err)
	}

	err = s.createIdentity(ctx, newUser.ID, p.config.Issuer, subject, email, p.config.Name)
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
	cacheKey := "oidc_exchange:" + code
	userIDBytes, err := s.cache.
Get(ctx, cacheKey)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired exchange code")
	}

	var userID int
	if err := json.Unmarshal(userIDBytes, &userID); err != nil {
		return nil, fmt.Errorf("invalid user ID format in cache")
	}

	// Delete the code so it can only be used once
	_ = s.cache.Delete(ctx, cacheKey)

	user, err := s.db.User.Get(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return user, nil
}
