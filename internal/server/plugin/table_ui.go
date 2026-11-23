// 职责: 为插件页面提供经过授权的托管表读写入口。
// 关联文件:
//
//	internal/server/plugin/callctx.go
//	internal/server/plugin/hostapi_tables.go
//	internal/server/gql/plugin_call.go
//	internal/objects/plugin_tables.go
//
// 注意事项:
//
//	仅接收插件端点绑定的安装身份。主库会重新核对当前的 UI 入口、授权和表声明。
package plugin

import (
	"context"
	"errors"
	"strings"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
)

func (m *Manager) RunUITableOperation(ctx context.Context, table, operation string, args []objects.JSONRawMessage) (objects.JSONRawMessage, error) {
	call, ok := CallFromContext(ctx)
	if !ok || !call.KVWritable || call.InstallationID <= 0 {
		return nil, xerrors.ForbiddenError("plugin management permission is required")
	}
	if !tableIdentifier.MatchString(table) || len(args) > 2 {
		return nil, xerrors.ValidationError("invalid table name or arguments")
	}

	method := strings.ToLower(operation)
	var write bool
	switch method {
	case "get", "list":
	case "insert", "update", "upsert", "delete":
		write = true
	default:
		return nil, xerrors.ValidationError("unknown table operation")
	}
	input := make([][]byte, len(args))
	for i, arg := range args {
		if len(arg) > maxPluginTableRowBytes {
			return nil, xerrors.ValidationError("table arguments exceed 64 KiB")
		}
		input[i] = arg
	}
	if write && !m.allowWrite(call.ManifestID) {
		return nil, xerrors.NewCodedError(xerrors.ErrorCode(hostRateLimited), "plugin write limit reached")
	}
	caller := pluginTableCaller{
		installationID: call.InstallationID,
		pluginID:       call.ManifestID,
		fromUI:         true,
	}
	result, err := m.runPluginTableMethod(ctx, caller, table, method, write, input)
	if err != nil {
		return nil, pluginUITableError(err)
	}
	return result, nil
}

func pluginUITableError(err error) error {
	if detail, ok := errors.AsType[*pluginTableError](err); ok {
		switch detail.code {
		case hostPermissionDenied:
			return xerrors.ForbiddenError(detail.message)
		case hostNotFound:
			return xerrors.NotFoundError("plugin table")
		case hostInvalidArgument, hostLimitExceeded:
			return xerrors.ValidationError(detail.message)
		}
	}
	if isPluginTableConflict(err) {
		return xerrors.AlreadyExistsError("plugin table row")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return xerrors.NewCodedError("TIMEOUT", "plugin table operation timed out").WithCause(err)
	}
	if errors.Is(err, context.Canceled) {
		return err
	}
	return errors.New("plugin table operation failed")
}
