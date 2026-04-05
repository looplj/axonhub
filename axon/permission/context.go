package permission

import "context"

type ctxKey string

const (
	workspaceKey ctxKey = "permission_workspace"
)

func WithWorkspace(ctx context.Context, workspace string) context.Context {
	return context.WithValue(ctx, workspaceKey, workspace)
}

func Workspace(ctx context.Context) string {
	v, _ := ctx.Value(workspaceKey).(string)
	return v
}
