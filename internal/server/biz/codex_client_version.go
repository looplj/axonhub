// 职责: 解析 Codex 渠道客户端版本，并保存从 npm 检查到的稳定版本。
// 关联文件:
//   internal/server/biz/channel_llm.go
//   internal/server/biz/channel_model_sync.go
//   internal/server/biz/codex_catalog.go
//   internal/server/biz/model_fetcher.go
//   llm/transformer/openai/codex/models.go
// 遵循的 Rules:
//   .agents/rules/go-general.md
// 注意事项:
//   持久化状态只包含自动检查结果，目录和手动版本仍按请求选取。

package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Masterminds/semver/v3"
	"golang.org/x/sync/singleflight"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/system"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/transformer/openai/codex"
)

const (
	codexReleaseKey            = "codex_client_release"
	codexRegistryURL           = "https://registry.npmjs.org/@openai%2Fcodex/latest"
	codexVersionRefreshTimeout = 8 * time.Second
)

type CodexCatalogVersion struct {
	Version string
	Source  string
}

type codexRelease struct {
	Version     string    `json:"version"`
	LastChecked time.Time `json:"last_checked"`
}

type codexVersionCache struct {
	current atomic.Pointer[CodexCatalogVersion]
	refresh singleflight.Group
	// Tests can use an isolated registry without changing the OAuth client.
	registryURL string
}

func normalizeCodexClientVersion(value string) string {
	version, err := semver.StrictNewVersion(strings.TrimSpace(value))
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d.%d.%d", version.Major(), version.Minor(), version.Patch())
}

func parseStableCodexVersion(value string) *semver.Version {
	version, err := semver.StrictNewVersion(value)
	if err != nil || version.Prerelease() != "" {
		return nil
	}
	return version
}

func codexVersionFromUserAgent(value string) string {
	for product := range strings.FieldsSeq(value) {
		version, ok := strings.CutPrefix(product, "codex_cli_rs/")
		if ok {
			return normalizeCodexClientVersion(version)
		}
		break
	}
	return ""
}

func (svc *ChannelService) resolveCodexClientVersion(ops []objects.OverrideOperation) CodexCatalogVersion {
	if len(ops) > 0 {
		headers := make(http.Header)
		ApplyModelFetchHeaderOverrides(headers, ops)
		if version := normalizeCodexClientVersion(headers.Get("Version")); version != "" {
			return CodexCatalogVersion{Version: version, Source: "channel_version"}
		}
		if version := codexVersionFromUserAgent(headers.Get("User-Agent")); version != "" {
			return CodexCatalogVersion{Version: version, Source: "channel_user_agent"}
		}
	}
	if svc != nil {
		if current := svc.codexVersions.current.Load(); current != nil {
			return *current
		}
	}
	return CodexCatalogVersion{Version: codex.DefaultClientVersion, Source: "builtin"}
}

func (svc *ChannelService) loadCodexRelease(ctx context.Context) {
	release, err := authz.RunAsHost(ctx, svc.db, "codex-release-load", func(ctx context.Context) (codexRelease, error) {
		row, err := ent.FromContext(ctx).System.Query().Where(system.KeyEQ(codexReleaseKey)).Only(ctx)
		if ent.IsNotFound(err) {
			return codexRelease{}, nil
		}
		if err != nil {
			return codexRelease{}, err
		}
		var release codexRelease
		err = json.Unmarshal([]byte(row.Value), &release)
		return release, err
	})
	if err != nil {
		log.Warn(ctx, "failed to read cached Codex release", log.Cause(err))
		return
	}
	svc.publishCodexRelease(release.Version, "cache")
}

func (svc *ChannelService) publishCodexRelease(value, source string) {
	version := parseStableCodexVersion(value)
	builtin := parseStableCodexVersion(codex.DefaultClientVersion)
	if version == nil || version.LessThan(builtin) {
		return
	}
	svc.codexVersions.current.Store(&CodexCatalogVersion{Version: normalizeCodexClientVersion(value), Source: source})
}

func (svc *ChannelService) refreshCodexRelease(ctx context.Context) {
	result := svc.codexVersions.refresh.DoChan("latest", func() (any, error) {
		refreshCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), codexVersionRefreshTimeout)
		defer cancel()
		svc.checkCodexRelease(refreshCtx)
		return nil, nil
	})
	select {
	case <-ctx.Done():
	case <-result:
	}
}

func (svc *ChannelService) checkCodexRelease(ctx context.Context) {
	registryURL := svc.codexVersions.registryURL
	if registryURL == "" {
		registryURL = codexRegistryURL
	}
	latest, err := fetchLatestCodexRelease(ctx, registryURL)
	if err == nil && parseStableCodexVersion(latest) == nil {
		err = fmt.Errorf("npm returned an invalid or prerelease Codex version")
	}
	current := svc.codexVersions.current.Load()
	if err != nil {
		if current != nil {
			svc.publishCodexRelease(current.Version, "cache")
		}
		log.Warn(ctx, "failed to check the latest stable Codex release", log.Cause(err))
		return
	}

	release := codexRelease{Version: normalizeCodexClientVersion(latest), LastChecked: time.Now().UTC()}
	if current != nil && parseStableCodexVersion(latest).LessThan(parseStableCodexVersion(current.Version)) {
		release.Version = current.Version
	}
	if svc.db != nil {
		persisted, err := svc.saveCodexRelease(ctx, release)
		if err != nil {
			log.Warn(ctx, "failed to save the latest stable Codex release", log.Cause(err))
		} else {
			release = persisted
		}
	}
	source := "cache"
	if release.Version == normalizeCodexClientVersion(latest) {
		source = "npm"
	}
	svc.publishCodexRelease(release.Version, source)
}

func (svc *ChannelService) saveCodexRelease(ctx context.Context, candidate codexRelease) (codexRelease, error) {
	return authz.RunAsHost(ctx, svc.db, "codex-release-save", func(ctx context.Context) (codexRelease, error) {
		store := ent.FromContext(ctx).System
		for ctx.Err() == nil {
			row, err := store.Query().Where(system.KeyEQ(codexReleaseKey)).Only(ctx)
			if ent.IsNotFound(err) {
				value, err := json.Marshal(candidate)
				if err != nil {
					return codexRelease{}, err
				}
				if err := store.Create().SetKey(codexReleaseKey).SetValue(string(value)).
					OnConflictColumns(system.FieldKey).DoNothing().Exec(ctx); err != nil {
					return codexRelease{}, err
				}
				// Read again in case a different instance inserted the row first.
				continue
			}
			if err != nil {
				return codexRelease{}, err
			}
			next := candidate
			var previous codexRelease
			if json.Unmarshal([]byte(row.Value), &previous) == nil {
				if version := parseStableCodexVersion(previous.Version); version != nil &&
					parseStableCodexVersion(next.Version).LessThan(version) {
					next.Version = normalizeCodexClientVersion(previous.Version)
				}
				if previous.LastChecked.After(next.LastChecked) {
					next.LastChecked = previous.LastChecked
				}
			}
			if next == previous {
				return next, nil
			}
			value, err := json.Marshal(next)
			if err != nil {
				return codexRelease{}, err
			}
			updated, err := store.Update().
				Where(system.IDEQ(row.ID), system.ValueEQ(row.Value)).
				SetValue(string(value)).
				Save(ctx)
			if err != nil {
				return codexRelease{}, err
			}
			if updated == 1 {
				return next, nil
			}
		}
		return codexRelease{}, ctx.Err()
	})
}

func fetchLatestCodexRelease(ctx context.Context, registryURL string) (string, error) {
	client := &http.Client{
		Timeout: 3 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, registryURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("npm returned HTTP %d", resp.StatusCode)
	}
	const maxMetadataSize = 1 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxMetadataSize+1))
	if err != nil {
		return "", err
	}
	if len(body) > maxMetadataSize {
		return "", fmt.Errorf("npm Codex metadata exceeds %d bytes", maxMetadataSize)
	}
	var metadata struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(body, &metadata); err != nil {
		return "", fmt.Errorf("decode npm Codex metadata: %w", err)
	}
	return metadata.Version, nil
}
