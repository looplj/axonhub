// 职责: 验证保存 Vertex 凭证不会覆盖并发禁用记录。
// 关联文件:
//
//	internal/server/biz/channel.go
//	internal/server/biz/channel_apikey.go
//	internal/server/biz/channel_provider_quota_hook.go
//
// 注意事项:
//
//	使用共享 Ent 事务隔离 SQLite 写锁差异，通过保存钩子和 synctest 固定并发顺序。
package biz

import (
	"context"
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
)

func TestChannelService_UpdateVertexCredentialsPreservesConcurrentDisable(t *testing.T) {
	account := syntheticVertexAccount(t, "concurrent")
	for _, mode := range []string{"enabled", "manual", "scheduled"} {
		t.Run(mode, func(t *testing.T) {
			svc, client := setupTestChannelService(t)
			defer client.Close()
			ctx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
			credentials := objects.ChannelCredentials{
				VertexAPIKeys:         []objects.VertexAPIKey{{APIKey: "synthetic-key", ProjectID: "target", Region: "global"}},
				VertexServiceAccounts: []objects.GCPCredential{account},
			}
			create := client.Channel.Create().SetName(mode).SetType(channel.TypeGeminiVertex).
				SetBaseURL("https://example.invalid/v1").SetCredentials(credentials).
				SetSupportedModels([]string{"test-model"}).SetDefaultTestModel("test-model").
				SetStatus(channel.StatusEnabled)
			if mode != "enabled" {
				create.SetStatus(channel.StatusDisabled).SetErrorMessage("operator disabled")
			}
			if mode == "scheduled" {
				create.SetErrorMessage(scheduledDisableReason).SetAutoDisabledAt(time.Now().UTC().Round(0)).
					SetAutoDisableExpiresAt(time.Now().Add(time.Hour).UTC().Round(0))
			}
			stored, err := create.Save(ctx)
			require.NoError(t, err)
			credentials.VertexServiceAccounts = []objects.GCPCredential{account}
			credentials.VertexServiceAccounts[0].Region = "us-central1"

			var prepared, releaseSave chan struct{}
			client.Channel.Use(func(next ent.Mutator) ent.Mutator {
				return ent.MutateFunc(func(ctx context.Context, mutation ent.Mutation) (ent.Value, error) {
					if _, savingCredentials := mutation.Field(channel.FieldCredentials); savingCredentials {
						close(prepared)
						<-releaseSave
					}
					return next.Mutate(ctx, mutation)
				})
			})
			// RunInTransaction explicitly supports caller-owned transactions. Sharing
			// one lets both real SQL mutations finish in a controlled order without
			// SQLite's cross-transaction SQLITE_BUSY masking the lost-update bug.
			tx, err := client.Tx(ctx)
			require.NoError(t, err)
			txCtx := ent.NewContext(ent.NewTxContext(ctx, tx), tx.Client())
			synctest.Test(t, func(t *testing.T) {
				prepared = make(chan struct{})
				releaseSave = make(chan struct{})
				updateDone := make(chan error, 1)
				disableDone := make(chan error, 1)
				run := func(action func() error, done chan<- error) {
					defer func() {
						if recovered := recover(); recovered != nil {
							done <- fmt.Errorf("concurrent operation panicked: %v", recovered)
						}
					}()
					done <- action()
				}
				go run(func() error {
					_, err := svc.UpdateChannel(txCtx, stored.ID, &ent.UpdateChannelInput{Credentials: &credentials})
					return err
				}, updateDone)
				<-prepared
				serialized := !svc.apiKeyOpsLock.TryLock()
				if !serialized {
					svc.apiKeyOpsLock.Unlock()
				}
				go run(func() error {
					return svc.DisableAPIKey(txCtx, stored.ID, account.CredentialRef(), 401, "concurrent failure")
				}, disableDone)
				// Mutex waits are not durable synctest blocking. Drain the disable
				// only when the paused update does not already own serialization.
				if !serialized {
					synctest.Wait()
				}
				close(releaseSave)
				require.NoError(t, <-updateDone)
				require.NoError(t, <-disableDone)
			})
			require.NoError(t, tx.Commit())

			persisted, err := client.Channel.Get(ctx, stored.ID)
			require.NoError(t, err)
			require.Len(t, persisted.DisabledAPIKeys, 1, "credential save lost the concurrent disable")
			require.Equal(t, account.CredentialRef(), persisted.DisabledAPIKeys[0].Key)
			require.Equal(t, "concurrent failure", persisted.DisabledAPIKeys[0].Reason)
			require.Equal(t, 401, persisted.DisabledAPIKeys[0].ErrorCode)
			require.Equal(t, credentials, persisted.Credentials)
			require.Equal(t, stored.Status, persisted.Status)
			require.Equal(t, stored.ErrorMessage, persisted.ErrorMessage)
			require.Equal(t, stored.AutoDisabledAt, persisted.AutoDisabledAt)
			require.Equal(t, stored.AutoDisableExpiresAt, persisted.AutoDisableExpiresAt)
		})
	}
}

func TestChannelService_CredentialUpdateReleasesPreviousQuotaDisables(t *testing.T) {
	env := setupQuotaKeyTestEnv(t)
	ch := env.createOpenCodeChannel(t, quotaTestKeyA, quotaTestKeyB)
	env.server.usage(quotaTestKeyA, 100, 10, 10)
	env.server.usage(quotaTestKeyB, 10, 10, 10)
	env.poll(t)
	require.Equal(t, []string{quotaTestKeyA}, quotaOwnedKeys(env.reload(t, ch.ID)))

	baseURL := "https://example.invalid/v1"
	_, err := env.channels.UpdateChannel(env.ctx, ch.ID, &ent.UpdateChannelInput{
		BaseURL:     &baseURL,
		Credentials: &ch.Credentials,
	})
	require.NoError(t, err)

	persisted := env.reload(t, ch.ID)
	require.Equal(t, baseURL, persisted.BaseURL)
	require.Empty(t, quotaOwnedKeys(persisted))
	require.ElementsMatch(t, []string{quotaTestKeyA, quotaTestKeyB}, enabledKeysOf(persisted))
}
