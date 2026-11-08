package main

import (
	"context"
	"errors"
	"net/http"

	"github.com/samber/lo"
)

const (
	desktopKeyName    = "AxonHub Desktop"
	desktopKeyLimit   = 20
	statusKeyEnabled  = "enabled"
	statusKeyDisabled = "disabled"
	statusKeyArchived = "archived"
)

// desktopKeyQuery loads keys named Desktop for one project. It asks for
// the secret and does not ask for profiles, scopes, or allowed IPs.
const desktopKeyQuery = `query DesktopPanelKey($first: Int!, $where: APIKeyWhereInput) {
  apiKeys(first: $first, where: $where) {
    edges {
      node {
        id
        name
        status
        key
      }
    }
  }
}`

// createDesktopKeyMutation creates the Desktop key. The input is only the
// name and project id.
const createDesktopKeyMutation = `mutation CreateAPIKey($input: CreateAPIKeyInput!) {
  createAPIKey(input: $input) {
    id
    name
    status
    key
  }
}`

// restoreDesktopKeyMutation enables an archived Desktop key again. Creating
// another key of the same name is rejected while the archived one remains.
const restoreDesktopKeyMutation = `mutation UpdateAPIKeyStatus($id: ID!, $status: APIKeyStatus!) {
  updateAPIKeyStatus(id: $id, status: $status) {
    id
    name
    status
    key
  }
}`

type desktopKeyNode struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Secret string `json:"key"`
}

type desktopKeyData struct {
	APIKeys panelConnection[desktopKeyNode] `json:"apiKeys"`
}

type createdDesktopKey struct {
	CreateAPIKey desktopKeyNode `json:"createAPIKey"`
}

type restoredDesktopKey struct {
	UpdateAPIKeyStatus desktopKeyNode `json:"updateAPIKeyStatus"`
}

// desktopKeyAction is what the Desktop row shows and what Copy writes.
// Label is the name. Copy is the stored secret and is not drawn.
type desktopKeyAction struct {
	Label string
	Copy  string
}

func presentDesktopKey(secret string) desktopKeyAction {
	return desktopKeyAction{Label: desktopKeyName, Copy: secret}
}

func desktopKeyVariables(projectID string) map[string]any {
	return map[string]any{
		"first": desktopKeyLimit,
		"where": map[string]any{
			"projectID": projectID,
			"name":      desktopKeyName,
			"statusIn":  []string{statusKeyEnabled, statusKeyDisabled, statusKeyArchived},
		},
	}
}

func createDesktopKeyVariables(projectID string) map[string]any {
	return map[string]any{
		"input": map[string]string{
			"name":      desktopKeyName,
			"projectID": projectID,
		},
	}
}

func restoreDesktopKeyVariables(id string) map[string]any {
	return map[string]any{
		"id":     id,
		"status": statusKeyEnabled,
	}
}

// selectDesktopKey prefers an enabled Desktop key, then a disabled one, then
// an archived one. A disabled or archived key keeps its name, so another key
// is not created.
func selectDesktopKey(nodes []desktopKeyNode) (desktopKeyNode, bool) {
	matches := lo.Filter(nodes, func(node desktopKeyNode, _ int) bool {
		return node.Name == desktopKeyName && lo.Contains([]string{statusKeyEnabled, statusKeyDisabled, statusKeyArchived}, node.Status)
	})
	for _, status := range []string{statusKeyEnabled, statusKeyDisabled, statusKeyArchived} {
		if node, ok := lo.Find(matches, func(node desktopKeyNode) bool {
			return node.Status == status
		}); ok {
			return node, true
		}
	}
	return desktopKeyNode{}, false
}

// ensureDesktopKey returns the stored secret for the project's Desktop
// key. Server errors are replaced so a response cannot carry the secret
// into a log line.
func ensureDesktopKey(ctx context.Context, client *http.Client, origin string, session desktopSession) (string, error) {
	data, err := postGraphQL[desktopKeyData](ctx, client, origin, session, "DesktopPanelKey", desktopKeyQuery, desktopKeyVariables(session.ProjectID))
	if err != nil {
		return "", errors.New("load desktop api key")
	}
	if node, ok := selectDesktopKey(connectionNodes(data.APIKeys)); ok {
		if node.Status != statusKeyArchived {
			return node.Secret, nil
		}
		restored, err := postGraphQL[restoredDesktopKey](ctx, client, origin, session, "UpdateAPIKeyStatus", restoreDesktopKeyMutation, restoreDesktopKeyVariables(node.ID))
		if err != nil || restored.UpdateAPIKeyStatus.Secret == "" {
			return "", errors.New("restore desktop api key")
		}
		return restored.UpdateAPIKeyStatus.Secret, nil
	}
	created, err := postGraphQL[createdDesktopKey](ctx, client, origin, session, "CreateAPIKey", createDesktopKeyMutation, createDesktopKeyVariables(session.ProjectID))
	if err != nil || created.CreateAPIKey.Secret == "" {
		return "", errors.New("create desktop api key")
	}
	return created.CreateAPIKey.Secret, nil
}
