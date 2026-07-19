package openapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/samber/lo"
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

	systemSvc := biz.NewSystemService(biz.SystemServiceParams{Ent: client})
	quotaSvc := biz.NewQuotaService(client, systemSvc)

	resolver := &Resolver{
		apiKeyService:                apiKeySvc,
		apiKeyProfileTemplateService: tmplSvc,
		quotaService:                 quotaSvc,
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

// Names are identifiers on the OpenAPI surface (apiKey/updateAPIKeyProfiles by
// name), so creating a second key with an existing name in the same project
// must be rejected — mirroring the admin-path CreateAPIKey behavior.
func TestOpenAPIResolver_CreateLLMAPIKey_DuplicateNameRejected(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeWriteAPIKeys),
	})

	_, err := mr.CreateLLMAPIKey(ctx, fx.targetKey.Name)
	require.Error(t, err)
	require.Contains(t, err.Error(), fx.targetKey.Name)
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

	got, err := mr.UpdateAPIKeyProfiles(ctx, &objects.GUID{Type: ent.TypeAPIKey, ID: fx.targetKey.ID}, nil, input)
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

	got, err := mr.UpdateAPIKeyProfiles(ctx, &objects.GUID{Type: ent.TypeAPIKey, ID: fx.targetKey.ID}, nil, input)
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
	_, err := mr.UpdateAPIKeyProfiles(ctx, &objects.GUID{Type: ent.TypeAPIKey, ID: fx.otherKey.ID}, nil, objects.APIKeyProfiles{
		ActiveProfile: "X",
		Profiles:      []objects.APIKeyProfile{{Name: "X"}},
	})
	require.Error(t, err)
}

func TestOpenAPIResolver_UpdateAPIKeyProfiles_MissingWriteScopeDenied(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys), // 缺 write
	})

	_, err := mr.UpdateAPIKeyProfiles(ctx, &objects.GUID{Type: ent.TypeAPIKey, ID: fx.targetKey.ID}, nil, objects.APIKeyProfiles{
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
		TemplateID: &objects.GUID{Type: ent.TypeAPIKeyProfileTemplate, ID: fx.template.ID},
		APIKeyID:   &objects.GUID{Type: ent.TypeAPIKey, ID: fx.targetKey.ID},
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
		TemplateID: &objects.GUID{Type: ent.TypeAPIKeyProfileTemplate, ID: fx.otherTemplate.ID},
		APIKeyID:   &objects.GUID{Type: ent.TypeAPIKey, ID: fx.targetKey.ID},
	})
	require.Error(t, err)
}

func TestOpenAPIResolver_LoadAPIKeyProfileTemplate_MissingReadScopeDenied(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeWriteAPIKeys), // 缺 read
	})

	_, err := mr.LoadAPIKeyProfileTemplate(ctx, LoadAPIKeyProfileTemplateInput{
		TemplateID: &objects.GUID{Type: ent.TypeAPIKeyProfileTemplate, ID: fx.template.ID},
		APIKeyID:   &objects.GUID{Type: ent.TypeAPIKey, ID: fx.targetKey.ID},
	})
	require.Error(t, err)
}

// setKeyQuotaProfile gives an API key a single "Default" profile carrying an
// all-time quota, written through a privacy-bypass context (same pattern the
// fixtures use). It lets the quota-usage tests exercise a key that actually has
// a quota to report.
func setKeyQuotaProfile(t *testing.T, client *ent.Client, keyID int) {
	t.Helper()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))

	reqs := int64(100)
	tokens := int64(1000)

	_, err := client.APIKey.UpdateOneID(keyID).
		SetProfiles(&objects.APIKeyProfiles{
			ActiveProfile: "Default",
			Profiles: []objects.APIKeyProfile{
				{
					Name: "Default",
					Quota: &objects.APIKeyQuota{
						Requests:    &reqs,
						TotalTokens: &tokens,
						Period: objects.APIKeyQuotaPeriod{
							Type: objects.APIKeyQuotaPeriodTypeAllTime,
						},
					},
				},
			},
		}).
		Save(ctx)
	require.NoError(t, err)
}

func TestOpenAPIResolver_APIKeyQuotaUsages_ByID(t *testing.T) {
	mr, fx, ctx, client := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys),
	})
	setKeyQuotaProfile(t, client, fx.targetKey.ID)

	qr := &queryResolver{mr.Resolver}

	got, err := qr.APIKeyQuotaUsages(ctx, &objects.GUID{Type: ent.TypeAPIKey, ID: fx.targetKey.ID}, nil, nil)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "Default", got[0].ProfileName)
	require.NotNil(t, got[0].Quota)
	require.NotNil(t, got[0].Usage)
	// No usage_log rows → zero usage.
	require.Equal(t, 0, got[0].Usage.RequestCount)
	require.Equal(t, 0, got[0].Usage.TotalTokens)
	require.True(t, got[0].Usage.TotalCost.IsZero())
	require.NotNil(t, got[0].Window)
	// all_time window: open start, end = now.
	require.Nil(t, got[0].Window.Start)
	require.NotNil(t, got[0].Window.End)
}

func TestOpenAPIResolver_APIKeyQuotaUsages_ByKey(t *testing.T) {
	mr, fx, ctx, client := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys),
	})
	setKeyQuotaProfile(t, client, fx.targetKey.ID)

	qr := &queryResolver{mr.Resolver}

	keyVal := fx.targetKey.Key

	got, err := qr.APIKeyQuotaUsages(ctx, nil, &keyVal, nil)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "Default", got[0].ProfileName)
}

// A key whose profiles carry no quota returns an empty (non-nil) list.
func TestOpenAPIResolver_APIKeyQuotaUsages_NoQuotaReturnsEmpty(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys),
	})

	qr := &queryResolver{mr.Resolver}

	// targetKey fixture has a "Default" profile without quota.
	got, err := qr.APIKeyQuotaUsages(ctx, &objects.GUID{Type: ent.TypeAPIKey, ID: fx.targetKey.ID}, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Empty(t, got)
}

func TestOpenAPIResolver_APIKeyQuotaUsages_CrossProjectDenied_ByID(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys),
	})

	qr := &queryResolver{mr.Resolver}

	// Foreign-project key id is filtered out by the privacy project boundary.
	_, err := qr.APIKeyQuotaUsages(ctx, &objects.GUID{Type: ent.TypeAPIKey, ID: fx.otherKey.ID}, nil, nil)
	require.Error(t, err)
}

func TestOpenAPIResolver_APIKeyQuotaUsages_CrossProjectDenied_ByKey(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys),
	})

	qr := &queryResolver{mr.Resolver}

	// Same boundary applies to plaintext-key lookup: the foreign key is invisible,
	// so existence is not leaked (uniform NotFound).
	otherVal := fx.otherKey.Key

	_, err := qr.APIKeyQuotaUsages(ctx, nil, &otherVal, nil)
	require.Error(t, err)
}

func TestOpenAPIResolver_APIKeyQuotaUsages_MissingReadScopeDenied(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeWriteAPIKeys), // 缺 read
	})

	qr := &queryResolver{mr.Resolver}

	_, err := qr.APIKeyQuotaUsages(ctx, &objects.GUID{Type: ent.TypeAPIKey, ID: fx.targetKey.ID}, nil, nil)
	require.Error(t, err)
}

func TestOpenAPIResolver_APIKeyQuotaUsages_RequiresExactlyOneArg(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys),
	})

	qr := &queryResolver{mr.Resolver}

	// Neither provided.
	_, err := qr.APIKeyQuotaUsages(ctx, nil, nil, nil)
	require.Error(t, err)

	// Both provided.
	keyVal := fx.targetKey.Key
	_, err = qr.APIKeyQuotaUsages(ctx, &objects.GUID{Type: ent.TypeAPIKey, ID: fx.targetKey.ID}, &keyVal, nil)
	require.Error(t, err)
}

func TestOpenAPIResolver_APIKeyQuotaUsages_InvalidGUIDType(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys),
	})

	qr := &queryResolver{mr.Resolver}

	// A GUID of the wrong type must be rejected before any DB lookup.
	_, err := qr.APIKeyQuotaUsages(ctx, &objects.GUID{Type: "Channel", ID: fx.targetKey.ID}, nil, nil)
	require.Error(t, err)
}

func TestOpenAPIResolver_UpdateAPIKeyProfiles_ByName(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys),
		string(scopes.ScopeWriteAPIKeys),
	})

	input := objects.APIKeyProfiles{
		ActiveProfile: "Default",
		Profiles:      []objects.APIKeyProfile{{Name: "Default"}},
	}

	got, err := mr.UpdateAPIKeyProfiles(ctx, nil, lo.ToPtr(fx.targetKey.Name), input)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, fx.targetKey.Name, got.Name)
	require.Equal(t, fx.targetKey.ID, got.ID.ID, "name must resolve to the same key as its id")
	require.NotNil(t, got.Profiles)
	require.Equal(t, "Default", got.Profiles.ActiveProfile)
}

func TestOpenAPIResolver_UpdateAPIKeyProfiles_RequiresExactlyOneIdentifier(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys),
		string(scopes.ScopeWriteAPIKeys),
	})

	input := objects.APIKeyProfiles{
		ActiveProfile: "Default",
		Profiles:      []objects.APIKeyProfile{{Name: "Default"}},
	}

	// Neither provided.
	_, err := mr.UpdateAPIKeyProfiles(ctx, nil, nil, input)
	require.Error(t, err)

	// Both provided.
	_, err = mr.UpdateAPIKeyProfiles(ctx, &objects.GUID{Type: ent.TypeAPIKey, ID: fx.targetKey.ID}, lo.ToPtr(fx.targetKey.Name), input)
	require.Error(t, err)
}

func TestOpenAPIResolver_UpdateAPIKeyProfiles_ByName_CrossProjectDenied(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys),
		string(scopes.ScopeWriteAPIKeys),
	})

	// 外项目的 key name 在 privacy 项目过滤下不可见 → NotFound，不泄露存在性。
	_, err := mr.UpdateAPIKeyProfiles(ctx, nil, lo.ToPtr(fx.otherKey.Name), objects.APIKeyProfiles{
		ActiveProfile: "X",
		Profiles:      []objects.APIKeyProfile{{Name: "X"}},
	})
	require.Error(t, err)
}

func TestOpenAPIResolver_UpdateAPIKeyProfiles_InvalidGUIDType(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys),
		string(scopes.ScopeWriteAPIKeys),
	})

	_, err := mr.UpdateAPIKeyProfiles(ctx, &objects.GUID{Type: "Channel", ID: fx.targetKey.ID}, nil, objects.APIKeyProfiles{
		ActiveProfile: "Default",
		Profiles:      []objects.APIKeyProfile{{Name: "Default"}},
	})
	require.Error(t, err)
}

func TestOpenAPIResolver_LoadAPIKeyProfileTemplate_ByNames(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys),
		string(scopes.ScopeWriteAPIKeys),
	})

	got, err := mr.LoadAPIKeyProfileTemplate(ctx, LoadAPIKeyProfileTemplateInput{
		TemplateName: lo.ToPtr(fx.template.Name),
		APIKeyName:   lo.ToPtr(fx.targetKey.Name),
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.Profiles)

	// Same append-only semantics as the by-id path.
	require.Equal(t, "Default", got.Profiles.ActiveProfile)
	require.Len(t, got.Profiles.Profiles, 2)
	require.Equal(t, "Production", got.Profiles.Profiles[1].Name)
}

// Mixed identifiers: id for one target, name for the other — both directions.
func TestOpenAPIResolver_LoadAPIKeyProfileTemplate_MixedIdentifiers(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys),
		string(scopes.ScopeWriteAPIKeys),
	})

	got, err := mr.LoadAPIKeyProfileTemplate(ctx, LoadAPIKeyProfileTemplateInput{
		TemplateID: &objects.GUID{Type: ent.TypeAPIKeyProfileTemplate, ID: fx.template.ID},
		APIKeyName: lo.ToPtr(fx.targetKey.Name),
	})
	require.NoError(t, err)
	require.Len(t, got.Profiles.Profiles, 2)

	got, err = mr.LoadAPIKeyProfileTemplate(ctx, LoadAPIKeyProfileTemplateInput{
		TemplateName: lo.ToPtr(fx.template.Name),
		APIKeyID:     &objects.GUID{Type: ent.TypeAPIKey, ID: fx.targetKey.ID},
	})
	require.NoError(t, err)
	// Second load appends another copy with a deduplicated profile name.
	require.Len(t, got.Profiles.Profiles, 3)
}

func TestOpenAPIResolver_LoadAPIKeyProfileTemplate_RequiresExactlyOneIdentifierPerTarget(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys),
		string(scopes.ScopeWriteAPIKeys),
	})

	// Template identifier missing.
	_, err := mr.LoadAPIKeyProfileTemplate(ctx, LoadAPIKeyProfileTemplateInput{
		APIKeyID: &objects.GUID{Type: ent.TypeAPIKey, ID: fx.targetKey.ID},
	})
	require.Error(t, err)

	// Template identified twice.
	_, err = mr.LoadAPIKeyProfileTemplate(ctx, LoadAPIKeyProfileTemplateInput{
		TemplateID:   &objects.GUID{Type: ent.TypeAPIKeyProfileTemplate, ID: fx.template.ID},
		TemplateName: lo.ToPtr(fx.template.Name),
		APIKeyID:     &objects.GUID{Type: ent.TypeAPIKey, ID: fx.targetKey.ID},
	})
	require.Error(t, err)

	// API key identifier missing.
	_, err = mr.LoadAPIKeyProfileTemplate(ctx, LoadAPIKeyProfileTemplateInput{
		TemplateID: &objects.GUID{Type: ent.TypeAPIKeyProfileTemplate, ID: fx.template.ID},
	})
	require.Error(t, err)

	// API key identified twice.
	_, err = mr.LoadAPIKeyProfileTemplate(ctx, LoadAPIKeyProfileTemplateInput{
		TemplateID: &objects.GUID{Type: ent.TypeAPIKeyProfileTemplate, ID: fx.template.ID},
		APIKeyID:   &objects.GUID{Type: ent.TypeAPIKey, ID: fx.targetKey.ID},
		APIKeyName: lo.ToPtr(fx.targetKey.Name),
	})
	require.Error(t, err)
}

func TestOpenAPIResolver_LoadAPIKeyProfileTemplate_ByName_CrossProjectDenied(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys),
		string(scopes.ScopeWriteAPIKeys),
	})

	// 外项目的模板 name 同样被 privacy 项目过滤挡下。
	_, err := mr.LoadAPIKeyProfileTemplate(ctx, LoadAPIKeyProfileTemplateInput{
		TemplateName: lo.ToPtr(fx.otherTemplate.Name),
		APIKeyName:   lo.ToPtr(fx.targetKey.Name),
	})
	require.Error(t, err)
}

func TestOpenAPIResolver_APIKeyQuotaUsages_ByName(t *testing.T) {
	mr, fx, ctx, client := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys),
	})
	setKeyQuotaProfile(t, client, fx.targetKey.ID)

	qr := &queryResolver{mr.Resolver}

	got, err := qr.APIKeyQuotaUsages(ctx, nil, nil, lo.ToPtr(fx.targetKey.Name))
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "Default", got[0].ProfileName)
}

func TestOpenAPIResolver_APIKey_ByID(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys),
	})

	qr := &queryResolver{mr.Resolver}

	got, err := qr.APIKey(ctx, &objects.GUID{Type: ent.TypeAPIKey, ID: fx.targetKey.ID}, nil, nil)
	require.NoError(t, err)
	require.Equal(t, fx.targetKey.ID, got.ID.ID)
	require.Equal(t, fx.targetKey.Key, got.Key)
	require.Equal(t, fx.targetKey.Name, got.Name)
	require.NotNil(t, got.Profiles)
}

func TestOpenAPIResolver_APIKey_ByKey(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys),
	})

	qr := &queryResolver{mr.Resolver}

	got, err := qr.APIKey(ctx, nil, lo.ToPtr(fx.targetKey.Key), nil)
	require.NoError(t, err)
	require.Equal(t, fx.targetKey.ID, got.ID.ID)
	require.Equal(t, fx.targetKey.Name, got.Name)
}

// The headline use case: resolve a key's id/key/profiles from its name alone.
func TestOpenAPIResolver_APIKey_ByName(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys),
	})

	qr := &queryResolver{mr.Resolver}

	got, err := qr.APIKey(ctx, nil, nil, lo.ToPtr(fx.targetKey.Name))
	require.NoError(t, err)
	require.Equal(t, fx.targetKey.ID, got.ID.ID)
	require.Equal(t, fx.targetKey.Key, got.Key)
	require.Equal(t, fx.targetKey.Name, got.Name)
	require.NotNil(t, got.Profiles)
}

func TestOpenAPIResolver_APIKey_RequiresExactlyOneArg(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys),
	})

	qr := &queryResolver{mr.Resolver}

	// None provided.
	_, err := qr.APIKey(ctx, nil, nil, nil)
	require.Error(t, err)

	// Two provided.
	_, err = qr.APIKey(ctx, &objects.GUID{Type: ent.TypeAPIKey, ID: fx.targetKey.ID}, lo.ToPtr(fx.targetKey.Key), nil)
	require.Error(t, err)

	_, err = qr.APIKey(ctx, nil, lo.ToPtr(fx.targetKey.Key), lo.ToPtr(fx.targetKey.Name))
	require.Error(t, err)
}

func TestOpenAPIResolver_APIKey_CrossProjectDenied(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys),
	})

	qr := &queryResolver{mr.Resolver}

	// Foreign-project key stays invisible no matter which identifier is used.
	_, err := qr.APIKey(ctx, &objects.GUID{Type: ent.TypeAPIKey, ID: fx.otherKey.ID}, nil, nil)
	require.Error(t, err)

	_, err = qr.APIKey(ctx, nil, nil, lo.ToPtr(fx.otherKey.Name))
	require.Error(t, err)
}

func TestOpenAPIResolver_APIKey_MissingReadScopeDenied(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeWriteAPIKeys), // 缺 read
	})

	qr := &queryResolver{mr.Resolver}

	_, err := qr.APIKey(ctx, nil, nil, lo.ToPtr(fx.targetKey.Name))
	require.Error(t, err)
}

func TestOpenAPIResolver_APIKey_InvalidGUIDType(t *testing.T) {
	mr, fx, ctx, _ := setupOpenAPI(t, []string{
		string(scopes.ScopeReadAPIKeys),
	})

	qr := &queryResolver{mr.Resolver}

	_, err := qr.APIKey(ctx, &objects.GUID{Type: "Channel", ID: fx.targetKey.ID}, nil, nil)
	require.Error(t, err)
}

// newOpenAPIGraphqlHandler wires the real production handler (NewGraphqlHandlers)
// around an in-memory ent client, so transport-level behavior is tested as
// shipped — not a test-local server config.
func newOpenAPIGraphqlHandler(t *testing.T) *GraphqlHandler {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent_handler?mode=memory&_fk=1")
	t.Cleanup(func() { _ = client.Close() })

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

	tmplSvc := biz.NewAPIKeyProfileTemplateService(biz.APIKeyProfileTemplateServiceParams{Ent: client})
	systemSvc := biz.NewSystemService(biz.SystemServiceParams{Ent: client})
	quotaSvc := biz.NewQuotaService(client, systemSvc)

	return NewGraphqlHandlers(Dependencies{
		Ent:                          client,
		APIKeyService:                apiKeySvc,
		APIKeyProfileTemplateService: tmplSvc,
		QuotaService:                 quotaSvc,
	})
}

// Regression: the OpenAPI GraphQL endpoint must reject GET so a plaintext `key`
// lookup variable can never travel in the URL. POST must keep working.
func TestOpenAPIHandler_RejectsGET(t *testing.T) {
	h := newOpenAPIGraphqlHandler(t)

	// GET carrying an operation in the query string → no transport matches →
	// gqlgen replies 400 "transport not supported".
	getRec := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/openapi/v1/graphql?query=%7B__typename%7D", nil)
	h.Graphql.ServeHTTP(getRec, getReq)

	require.Equal(t, http.StatusBadRequest, getRec.Code, "GET must be rejected at the transport layer")
	require.Contains(t, getRec.Body.String(), "transport not supported")

	// POST is still served (we only removed GET): a no-auth introspection of the
	// Query root type succeeds at the transport layer (200, no transport error).
	postRec := httptest.NewRecorder()
	postReq := httptest.NewRequest(http.MethodPost, "/openapi/v1/graphql", strings.NewReader(`{"query":"{ __typename }"}`))
	postReq.Header.Set("Content-Type", "application/json")
	h.Graphql.ServeHTTP(postRec, postReq)

	require.Equal(t, http.StatusOK, postRec.Code)
	require.NotContains(t, postRec.Body.String(), "transport not supported")
	require.Contains(t, postRec.Body.String(), "Query")
}
