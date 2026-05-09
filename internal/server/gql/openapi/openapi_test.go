package openapi

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/scopes"
	"github.com/looplj/axonhub/internal/server/biz"
)

// fixtures bundles pre-created entities used across the OpenAPI E2E tests.
type fixtures struct {
	project        *ent.Project
	user           *ent.User
	serviceAccount *ent.APIKey
	targetKey      *ent.APIKey
	template       *ent.APIKeyProfileTemplate

	otherProject  *ent.Project
	otherTemplate *ent.APIKeyProfileTemplate
	otherKey      *ent.APIKey
}

// setupOpenAPI wires real biz services around an in-memory ent client and
// produces a context carrying a service account API key principal — exactly
// what `WithOpenAPIAuth` would inject in a real request, so the privacy layer
// runs for real (no test bypass).
func setupOpenAPI(t *testing.T, serviceAccountScopes []string) (*mutationResolver, fixtures, context.Context, *ent.Client) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	t.Cleanup(func() { _ = client.Close() })

	// Setup ctx for fixture construction with privacy bypass.
	setupCtx := ent.NewContext(context.Background(), client)
	setupCtx = authz.WithTestBypass(setupCtx)

	hashed, err := biz.HashPassword("test-password")
	require.NoError(t, err)

	owner, err := client.User.Create().
		SetEmail(fmt.Sprintf("owner-%d@example.com", time.Now().UnixNano())).
		SetPassword(hashed).
		SetFirstName("Owner").
		SetLastName("User").
		SetStatus(user.StatusActivated).
		Save(setupCtx)
	require.NoError(t, err)

	proj, err := client.Project.Create().
		SetName(fmt.Sprintf("project-%d", time.Now().UnixNano())).
		SetDescription("primary").
		SetStatus(project.StatusActive).
		Save(setupCtx)
	require.NoError(t, err)

	saKey, err := biz.GenerateAPIKey("ah")
	require.NoError(t, err)

	sa, err := client.APIKey.Create().
		SetName("service-account").
		SetKey(saKey).
		SetUserID(owner.ID).
		SetProjectID(proj.ID).
		SetType(apikey.TypeServiceAccount).
		SetScopes(serviceAccountScopes).
		Save(setupCtx)
	require.NoError(t, err)
	// Resolve project edge so withAPIKeyPrincipal-equivalent doesn't trip up.
	sa.Edges.Project = proj

	targetKeyValue, err := biz.GenerateAPIKey("ah")
	require.NoError(t, err)

	target, err := client.APIKey.Create().
		SetName("target-llm-key").
		SetKey(targetKeyValue).
		SetUserID(owner.ID).
		SetProjectID(proj.ID).
		SetType(apikey.TypeUser).
		SetProfiles(&objects.APIKeyProfiles{
			ActiveProfile: "Default",
			Profiles:      []objects.APIKeyProfile{{Name: "Default"}},
		}).
		Save(setupCtx)
	require.NoError(t, err)

	tmpl, err := client.APIKeyProfileTemplate.Create().
		SetName("prod-template").
		SetDescription("Production template").
		SetProject(proj).
		SetProfile(&objects.APIKeyProfile{
			Name: "Production",
			ModelMappings: []objects.ModelMapping{
				{From: "claude-3", To: "claude-3-opus"},
			},
		}).
		Save(setupCtx)
	require.NoError(t, err)

	// Foreign-project resources for cross-project denial tests.
	otherProj, err := client.Project.Create().
		SetName(fmt.Sprintf("other-project-%d", time.Now().UnixNano())).
		SetDescription("foreign").
		SetStatus(project.StatusActive).
		Save(setupCtx)
	require.NoError(t, err)

	otherTmpl, err := client.APIKeyProfileTemplate.Create().
		SetName("other-template").
		SetDescription("foreign template").
		SetProject(otherProj).
		SetProfile(&objects.APIKeyProfile{Name: "ForeignProfile"}).
		Save(setupCtx)
	require.NoError(t, err)

	otherKeyValue, err := biz.GenerateAPIKey("ah")
	require.NoError(t, err)

	otherKey, err := client.APIKey.Create().
		SetName("foreign-key").
		SetKey(otherKeyValue).
		SetUserID(owner.ID).
		SetProjectID(otherProj.ID).
		SetType(apikey.TypeUser).
		Save(setupCtx)
	require.NoError(t, err)

	// Real services (memory cache, no Redis).
	cacheCfg := xcache.Config{Mode: xcache.ModeMemory}

	projectSvc := &biz.ProjectService{
		ProjectCache: xcache.NewFromConfig[xcache.Entry[ent.Project]](cacheCfg),
	}

	apiKeySvc := biz.NewAPIKeyService(biz.APIKeyServiceParams{
		CacheConfig:    cacheCfg,
		Ent:            client,
		ProjectService: projectSvc,
		KeyPrefix:      "ah",
	})
	t.Cleanup(apiKeySvc.Stop)

	tmplSvc := biz.NewAPIKeyProfileTemplateService(biz.APIKeyProfileTemplateServiceParams{
		Ent: client,
	})

	resolver := &Resolver{
		apiKeyService:                apiKeySvc,
		apiKeyProfileTemplateService: tmplSvc,
	}

	// Real call ctx: API key principal, no privacy bypass.
	callCtx := ent.NewContext(context.Background(), client)
	callCtx = contexts.WithAPIKey(callCtx, sa)
	callCtx = contexts.WithProjectID(callCtx, proj.ID)

	return &mutationResolver{resolver}, fixtures{
		project:        proj,
		user:           owner,
		serviceAccount: sa,
		targetKey:      target,
		template:       tmpl,
		otherProject:   otherProj,
		otherTemplate:  otherTmpl,
		otherKey:       otherKey,
	}, callCtx, client
}

func TestOpenAPIResolver_CreateLLMAPIKey_HappyPath(t *testing.T) {
	mr, _, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeWriteAPIKeys),
	})

	got, err := mr.CreateLLMAPIKey(ctx, "  example-key  ")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "example-key", got.Name)
	require.NotEmpty(t, got.Key)
	require.ElementsMatch(t,
		[]string{string(scopes.ScopeReadChannels), string(scopes.ScopeWriteRequests)},
		got.Scopes,
	)
}

func TestOpenAPIResolver_CreateLLMAPIKey_MissingScopeDenied(t *testing.T) {
	mr, _, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys), // 缺 write
	})

	_, err := mr.CreateLLMAPIKey(ctx, "should-fail")
	require.Error(t, err)
}

func TestOpenAPIResolver_UpdateAPIKeyProfiles_HappyPath(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys),
		string(scopes.ScopeWriteAPIKeys),
	})

	input := objects.APIKeyProfiles{
		ActiveProfile: "Production",
		Profiles: []objects.APIKeyProfile{
			{Name: "Default"},
			{
				Name: "Production",
				ModelMappings: []objects.ModelMapping{
					{From: "gpt-4", To: "gpt-4o"},
				},
			},
		},
	}

	got, err := mr.UpdateAPIKeyProfiles(ctx, objects.GUID{ID: fx.targetKey.ID}, input)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.Profiles)
	require.Equal(t, "Production", got.Profiles.ActiveProfile)
	require.Len(t, got.Profiles.Profiles, 2)
	require.Equal(t, "Production", got.Profiles.Profiles[1].Name)
	require.Equal(t, "gpt-4", got.Profiles.Profiles[1].ModelMappings[0].From)
}

// Regression: when the OpenAPI client omits modelMappings, the resolver must
// coerce nil → [] so admin UI's Zod schema (which strictly requires a non-null
// array for modelMappings) doesn't break on read.
func TestOpenAPIResolver_UpdateAPIKeyProfiles_NormalizesNilModelMappings(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys),
		string(scopes.ScopeWriteAPIKeys),
	})

	// ModelMappings intentionally left nil to simulate a client that omits
	// the field in its GraphQL input.
	input := objects.APIKeyProfiles{
		ActiveProfile: "test",
		Profiles: []objects.APIKeyProfile{
			{Name: "test"},
		},
	}

	got, err := mr.UpdateAPIKeyProfiles(ctx, objects.GUID{ID: fx.targetKey.ID}, input)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.Profiles)
	require.Len(t, got.Profiles.Profiles, 1)
	// Critical assertion: ModelMappings must be a non-nil empty slice, not nil.
	require.NotNil(t, got.Profiles.Profiles[0].ModelMappings, "ModelMappings must be normalized to non-nil")
	require.Empty(t, got.Profiles.Profiles[0].ModelMappings)
}

func TestOpenAPIResolver_UpdateAPIKeyProfiles_CrossProjectDenied(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys),
		string(scopes.ScopeWriteAPIKeys),
	})

	// 用其他项目的 key id：privacy 层的 read filter 应让 Get 找不到。
	_, err := mr.UpdateAPIKeyProfiles(ctx, objects.GUID{ID: fx.otherKey.ID}, objects.APIKeyProfiles{
		ActiveProfile: "X",
		Profiles:      []objects.APIKeyProfile{{Name: "X"}},
	})
	require.Error(t, err)
}

func TestOpenAPIResolver_UpdateAPIKeyProfiles_MissingWriteScopeDenied(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys), // 缺 write
	})

	_, err := mr.UpdateAPIKeyProfiles(ctx, objects.GUID{ID: fx.targetKey.ID}, objects.APIKeyProfiles{
		ActiveProfile: "Default",
		Profiles:      []objects.APIKeyProfile{{Name: "Default"}},
	})
	require.Error(t, err)
}

func TestOpenAPIResolver_LoadAPIKeyProfileTemplate_HappyPath(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys),
		string(scopes.ScopeWriteAPIKeys),
	})

	got, err := mr.LoadAPIKeyProfileTemplate(ctx, LoadAPIKeyProfileTemplateInput{
		TemplateID: objects.GUID{ID: fx.template.ID},
		APIKeyID:   objects.GUID{ID: fx.targetKey.ID},
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.Profiles)

	// Append-only semantics: original Default kept, template appended.
	require.Equal(t, "Default", got.Profiles.ActiveProfile, "active profile must not change")
	require.Len(t, got.Profiles.Profiles, 2)
	require.Equal(t, "Default", got.Profiles.Profiles[0].Name)
	require.Equal(t, "Production", got.Profiles.Profiles[1].Name)
}

// 关键：跨项目模板必须被 ent privacy (新增的 APIKeyProjectScopeReadRule) 拦下。
// 如果新规则没生效，LoadTemplate 会读到外项目模板，再因 biz 内 same-project
// 校验报错——错误类型不一样，故同时断言 cross-project 路径报错即可。
func TestOpenAPIResolver_LoadAPIKeyProfileTemplate_CrossProjectDenied(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys),
		string(scopes.ScopeWriteAPIKeys),
	})

	_, err := mr.LoadAPIKeyProfileTemplate(ctx, LoadAPIKeyProfileTemplateInput{
		TemplateID: objects.GUID{ID: fx.otherTemplate.ID},
		APIKeyID:   objects.GUID{ID: fx.targetKey.ID},
	})
	require.Error(t, err)
}

func TestOpenAPIResolver_LoadAPIKeyProfileTemplate_MissingReadScopeDenied(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeWriteAPIKeys), // 缺 read
	})

	_, err := mr.LoadAPIKeyProfileTemplate(ctx, LoadAPIKeyProfileTemplateInput{
		TemplateID: objects.GUID{ID: fx.template.ID},
		APIKeyID:   objects.GUID{ID: fx.targetKey.ID},
	})
	require.Error(t, err)
}
