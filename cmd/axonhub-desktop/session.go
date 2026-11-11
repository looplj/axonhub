package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/api"
	"github.com/looplj/axonhub/internal/server/biz"
)

const (
	// ownerEmail has a dotted domain because the initialize and sign-in
	// handlers reject addresses that fail binding:"email". desktop@localhost
	// does not pass that check.
	ownerEmail         = "desktop@axonhub.local"
	ownerFirstName     = "Desktop"
	ownerLastName      = "User"
	brandName          = "AxonHub"
	desktopProjectName = "AxonHub Desktop"
	myProjectsQuery    = `query DesktopProjects { myProjects { id name } }`
)

// desktopSession is the UI session injected before the desktop page runs.
// ProjectID is the GraphQL id string the project store persists, such as
// gid://axonhub/Project/1.
type desktopSession struct {
	Token     string
	ProjectID string
}

type projectRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ensureAccount makes the desktop owner and project ready in the database,
// with secret as the owner's password. fresh reports that this launch
// initialized the system.
type ensureAccount func(ctx context.Context, secret string, fresh bool) error

// openDesktopSession initializes a new system, lets ensure fix up the
// owner and project (a nil ensure skips it), then signs in and picks the
// AxonHub Desktop project.
func openDesktopSession(ctx context.Context, baseURL, secretPath string, client *http.Client, ensure ensureAccount) (desktopSession, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	baseURL = strings.TrimRight(baseURL, "/")

	status, err := waitForStatus(ctx, client, baseURL+"/admin/system/status")
	if err != nil {
		return desktopSession{}, err
	}

	// An initialized database from another install has no secret here
	// yet: ensure sets the owner's password to the one created now.
	secret, err := ownerSecret(secretPath, true)
	if err != nil {
		return desktopSession{}, err
	}
	if !status.IsInitialized {
		if err := initializeSystem(ctx, client, baseURL+"/admin/system/initialize", secret); err != nil {
			return desktopSession{}, err
		}
	}
	if ensure != nil {
		if err := ensure(ctx, secret, !status.IsInitialized); err != nil {
			return desktopSession{}, err
		}
	}

	token, err := signIn(ctx, client, baseURL+"/admin/auth/signin", secret)
	if err != nil {
		return desktopSession{}, err
	}
	projectID, err := defaultProject(ctx, client, baseURL+"/admin/graphql", token)
	if err != nil {
		return desktopSession{}, err
	}
	return desktopSession{Token: token, ProjectID: projectID}, nil
}

func waitForStatus(ctx context.Context, client *http.Client, rawURL string) (api.SystemStatusResponse, error) {
	var last error
	for {
		if err := ctx.Err(); err != nil {
			if last != nil {
				return api.SystemStatusResponse{}, fmt.Errorf("wait for desktop server: %w", last)
			}
			return api.SystemStatusResponse{}, err
		}
		status, err := getStatus(ctx, client, rawURL)
		if err == nil {
			return status, nil
		}
		last = err
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
}

func getStatus(ctx context.Context, client *http.Client, rawURL string) (api.SystemStatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return api.SystemStatusResponse{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return api.SystemStatusResponse{}, err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return api.SystemStatusResponse{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return api.SystemStatusResponse{}, fmt.Errorf("GET %s: %s", rawURL, resp.Status)
	}
	var status api.SystemStatusResponse
	if err := json.Unmarshal(payload, &status); err != nil {
		return api.SystemStatusResponse{}, fmt.Errorf("decode system status: %w", err)
	}
	return status, nil
}

func initializeSystem(ctx context.Context, client *http.Client, rawURL, secret string) error {
	var result api.InitializeSystemResponse
	err := postJSON(ctx, client, rawURL, "", api.InitializeSystemRequest{
		OwnerEmail:     ownerEmail,
		OwnerPassword:  secret,
		OwnerFirstName: ownerFirstName,
		OwnerLastName:  ownerLastName,
		BrandName:      brandName,
	}, &result)
	if err != nil {
		if strings.Contains(err.Error(), "already initialized") {
			return nil
		}
		return err
	}
	if !result.Success {
		return fmt.Errorf("initialize desktop system: %s", redact(result.Message, secret))
	}
	return nil
}

func signIn(ctx context.Context, client *http.Client, rawURL, secret string) (string, error) {
	var result api.SignInResponse
	if err := postJSON(ctx, client, rawURL, secret, api.SignInRequest{
		Email:    ownerEmail,
		Password: secret,
	}, &result); err != nil {
		return "", err
	}
	if result.Token == "" {
		return "", errors.New("sign-in response did not include a token")
	}
	return result.Token, nil
}

// renewDesktopSession keeps the desktop token usable for as long as the
// process runs. The server idles a session out after AdminSessionIdle and
// will not refresh one inside the last quiet period, so a token that is
// near expiry is refreshed and a rejected or already expired one is
// replaced by signing in again. ProjectID is left as it was.
func renewDesktopSession(ctx context.Context, client *http.Client, baseURL, secretPath string, session desktopSession, now time.Time) (desktopSession, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if session.Token != "" && !sessionDueForRenewal(session.Token, now) {
		return session, nil
	}
	if session.Token != "" {
		token, status, err := refreshSession(ctx, client, baseURL+"/admin/auth/refresh", session.Token)
		if err == nil && token != "" {
			session.Token = token
			return session, nil
		}
		if status == http.StatusNoContent || (status != http.StatusUnauthorized && err != nil && sessionStillValid(session.Token, now)) {
			return session, nil
		}
	}
	secret, err := ownerSecret(secretPath, false)
	if err != nil {
		return desktopSession{}, err
	}
	token, err := signIn(ctx, client, baseURL+"/admin/auth/signin", secret)
	if err != nil {
		return desktopSession{}, err
	}
	session.Token = token
	return session, nil
}

func refreshSession(ctx context.Context, client *http.Client, rawURL, token string) (string, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", resp.StatusCode, err
	}
	if resp.StatusCode == http.StatusNoContent {
		return "", resp.StatusCode, nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", resp.StatusCode, fmt.Errorf("refresh desktop session: %s", resp.Status)
	}
	var result api.RefreshResponse
	if err := json.Unmarshal(payload, &result); err != nil {
		return "", resp.StatusCode, fmt.Errorf("decode refresh: %w", err)
	}
	if result.Token == "" {
		return "", resp.StatusCode, errors.New("refresh response did not include a token")
	}
	return result.Token, resp.StatusCode, nil
}

// sessionDueForRenewal reports that the token is missing an expiry or is
// inside the server's renewal window, including already expired.
func sessionDueForRenewal(token string, now time.Time) bool {
	exp, ok := jwtExpiry(token)
	if !ok {
		return true
	}
	return exp.Sub(now) < biz.AdminSessionRenewal
}

func sessionStillValid(token string, now time.Time) bool {
	exp, ok := jwtExpiry(token)
	return ok && now.Before(exp)
}

func jwtExpiry(token string) (time.Time, bool) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return time.Time{}, false
		}
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp <= 0 {
		return time.Time{}, false
	}
	return time.Unix(claims.Exp, 0), true
}

func defaultProject(ctx context.Context, client *http.Client, rawURL, token string) (string, error) {
	body, err := json.Marshal(map[string]string{"query": myProjectsQuery})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("load projects: %s", resp.Status)
	}

	var decoded struct {
		Data struct {
			MyProjects []projectRef `json:"myProjects"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return "", fmt.Errorf("decode projects: %w", err)
	}
	if len(decoded.Errors) > 0 {
		return "", fmt.Errorf("load projects: %s", decoded.Errors[0].Message)
	}
	return defaultProjectID(decoded.Data.MyProjects)
}

func defaultProjectID(projects []projectRef) (string, error) {
	project, ok := lo.Find(projects, func(item projectRef) bool {
		return item.Name == desktopProjectName && item.ID != ""
	})
	if !ok {
		return "", errors.New("desktop project was not found")
	}
	guid, err := objects.ParseGUID(project.ID)
	if err != nil || guid.Type != ent.TypeProject || guid.ID <= 0 {
		return "", fmt.Errorf("default project id %q is not a project id", project.ID)
	}
	return project.ID, nil
}

func postJSON(ctx context.Context, client *http.Client, rawURL, secret string, request, response any) error {
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("POST %s: %s: %s", rawURL, resp.Status, redact(strings.TrimSpace(string(payload)), secret))
	}
	if response != nil && len(bytes.TrimSpace(payload)) > 0 {
		if err := json.Unmarshal(payload, response); err != nil {
			return fmt.Errorf("decode %s: %w", rawURL, err)
		}
	}
	return nil
}

func redact(message, secret string) string {
	if secret == "" {
		return message
	}
	return strings.ReplaceAll(message, secret, "[redacted]")
}

// sessionPreloadScript runs before the page's own scripts and writes the
// two keys the UI reads from localStorage.
func sessionPreloadScript(session desktopSession) (string, error) {
	tokenKey, err := json.Marshal("axonhub_access_token")
	if err != nil {
		return "", err
	}
	tokenValue, err := json.Marshal(session.Token)
	if err != nil {
		return "", err
	}
	projectKey, err := json.Marshal("axonhub_selected_project_id")
	if err != nil {
		return "", err
	}
	projectValue, err := json.Marshal(session.ProjectID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"localStorage.setItem(%s, %s); localStorage.setItem(%s, %s);",
		tokenKey, tokenValue, projectKey, projectValue,
	), nil
}
