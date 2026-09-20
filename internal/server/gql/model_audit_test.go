package gql

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	gqlclient "github.com/99designs/gqlgen/client"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/scopes"
)

func TestRequestModelAuditCompleteExecutions(t *testing.T) {
	db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	ctx := authz.WithTestBypass(t.Context())
	project := db.Project.Create().SetName("audit").SaveX(ctx)
	expected := map[string]objects.ModelAuditStatus{
		"matched":              objects.ModelAuditMatched,
		"early-mismatch":       objects.ModelAuditMismatched,
		"early-unknown":        objects.ModelAuditUnknown,
		"early-conflict":       objects.ModelAuditConflicting,
		"retry-reported-model": objects.ModelAuditMatched,
		"retry-success":        objects.ModelAuditMatched,
		"retry-canceled":       objects.ModelAuditMatched,
		"retry-mismatch":       objects.ModelAuditMismatched,
		"retry-conflict":       objects.ModelAuditConflicting,
		"retry-final-unknown":  objects.ModelAuditUnknown,
	}
	for name := range expected {
		req := db.Request.Create().SetProjectID(project.ID).SetModelID(name).
			SetStatus(request.StatusCompleted).SetRequestBody([]byte(`{} `)).SaveX(ctx)
		for i := range 11 {
			create := db.RequestExecution.Create().SetRequestID(req.ID).SetProjectID(project.ID).
				SetModelID("routed").SetOutboundModelID("sent").SetUpstreamModelID("sent").
				SetStatus(requestexecution.StatusCompleted).SetCreatedAt(time.Unix(1000+int64(i), 0)).
				SetRequestBody([]byte(`{"prompt":"body must not be loaded for auditing"}`)).
				SetResponseBody([]byte(`{"text":"body must not be loaded for auditing"}`))
			if strings.HasPrefix(name, "retry-") {
				if i < 10 {
					create.SetStatus(requestexecution.StatusFailed).SetUpstreamModelID("")
					if name == "retry-reported-model" {
						create.SetOutboundModelID("sent:free").SetUpstreamModelID("sent:free")
					}
					if name == "retry-canceled" {
						create.SetStatus(requestexecution.StatusCanceled)
					}
				} else if name == "retry-final-unknown" {
					create.SetUpstreamModelID("")
				}
			}
			if i == 0 {
				switch name {
				case "early-mismatch":
					create.SetUpstreamModelID("different")
				case "early-unknown":
					create.SetUpstreamModelID("")
				case "early-conflict":
					create.SetUpstreamModelIds([]string{"sent", "changed"})
				case "retry-mismatch":
					create.SetUpstreamModelID("different")
				case "retry-conflict":
					create.SetUpstreamModelIds([]string{"sent", "changed"})
				}
			}
			create.SaveX(ctx)
		}
	}

	// Batch only audit metadata, leaving stored prompts and response bodies out.
	parents, err := withModelAuditExecutions(db.Request.Query()).All(ctx)
	require.NoError(t, err)
	for _, parent := range parents {
		executions, err := parent.NamedExecutions(modelAuditExecutionsEdge)
		require.NoError(t, err)
		require.Len(t, executions, 11)
		for _, execution := range executions {
			require.NotEmpty(t, execution.Status, "audit must load execution status with model metadata")
			require.Empty(t, execution.RequestBody)
			require.Empty(t, execution.ResponseBody)
		}
	}

	var executionQueries atomic.Int32
	db.RequestExecution.Intercept(ent.InterceptFunc(func(next ent.Querier) ent.Querier {
		return ent.QuerierFunc(func(ctx context.Context, query ent.Query) (ent.Value, error) {
			executionQueries.Add(1)
			return next.Query(ctx, query)
		})
	}))
	handler := NewGraphqlHandlers(Dependencies{Ent: db})
	client := gqlclient.New(handler.Graphql, func(req *gqlclient.Request) {
		req.HTTP = req.HTTP.WithContext(authz.WithTestBypass(req.HTTP.Context()))
	})
	var response struct {
		Requests struct {
			Edges []struct {
				Node struct {
					ID                   string
					ModelID              string
					Audit                objects.RequestModelAudit
					ModelAuditExecutions struct {
						Edges []struct {
							Node struct{ UpstreamModelID string }
						}
					}
				}
			}
		}
	}
	// Fragments, aliases, and a filtered display page cannot narrow the audit.
	err = client.Post(`query Audit($include: Boolean!) {
        requests(first: 10) { edges { node { id modelID ...AuditFields
            modelAuditExecutions: executions(first: 10, orderBy: {field: CREATED_AT, direction: DESC}, where: {upstreamModelID: "sent"}) {
                edges { node { upstreamModelID } }
            }
        } } }
    }
    fragment AuditFields on Request {
        audit: modelAudit @include(if: $include) {
            status matchedUpstreamIds upstreamModelIds mismatchedModelIds conflictingModelIds unknownCount comparedCount conflictCount
        }
    }`, &response, gqlclient.Var("include", true))
	require.NoError(t, err)
	require.Len(t, response.Requests.Edges, len(expected))
	require.EqualValues(t, 2, executionQueries.Load(), "one full-set query plus one display query, not one audit query per request")
	for _, edge := range response.Requests.Edges {
		node := edge.Node
		require.Equal(t, expected[node.ModelID], node.Audit.Status)
		if node.ModelID == "retry-final-unknown" {
			require.Empty(t, node.Audit.MatchedUpstreamIds)
		} else {
			require.Equal(t, []string{"sent"}, node.Audit.MatchedUpstreamIds)
		}
		if node.ModelID == "retry-reported-model" {
			require.ElementsMatch(t, []string{"sent", "sent:free"}, node.Audit.UpstreamModelIds)
			require.Equal(t, 11, node.Audit.ComparedCount)
			require.Zero(t, node.Audit.UnknownCount)
		}
		displayCount := 10
		if strings.HasPrefix(node.ModelID, "retry-") {
			displayCount = 1
			if node.ModelID == "retry-final-unknown" {
				displayCount = 0
			}
		}
		require.Len(t, node.ModelAuditExecutions.Edges, displayCount)
		require.Equal(t, 11, node.Audit.ComparedCount+node.Audit.UnknownCount)
		if node.ModelID == "early-mismatch" {
			require.Equal(t, []string{"different"}, node.Audit.MismatchedModelIds)
		}
		if node.ModelID == "early-unknown" {
			require.Equal(t, 1, node.Audit.UnknownCount)
		}
		if node.ModelID == "retry-success" || node.ModelID == "retry-canceled" {
			require.Equal(t, 10, node.Audit.UnknownCount)
			require.Equal(t, 1, node.Audit.ComparedCount)
		}
		if node.ModelID == "early-conflict" {
			require.Equal(t, 1, node.Audit.ConflictCount)
			require.ElementsMatch(t, []string{"sent", "changed"}, node.Audit.ConflictingModelIds)
		}
		var detail struct {
			Node struct{ ModelAudit objects.RequestModelAudit }
		}
		err := client.Post(`query($id: ID!) { node(id: $id) { ... on Request {
            modelAudit { status matchedUpstreamIds upstreamModelIds mismatchedModelIds conflictingModelIds unknownCount comparedCount conflictCount }
        } } }`, &detail, gqlclient.Var("id", node.ID))
		require.NoError(t, err)
		require.Equal(t, node.Audit, detail.Node.ModelAudit, "node/detail fallback must return the same complete audit")
	}

	for _, selection := range []string{
		"id",
		"id modelAudit @skip(if: true) { status }",
		"id modelAudit @include(if: false) { status }",
	} {
		executionQueries.Store(0)
		var result map[string]any
		require.NoError(t, client.Post("{ requests(first: 10) { edges { node { "+selection+" } } } }", &result))
		require.Zero(t, executionQueries.Load(), "audit metadata is loaded only when requested")
	}
}

func TestRequestModelAuditPrivacy(t *testing.T) {
	db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	setup := authz.WithTestBypass(t.Context())
	project := db.Project.Create().SetName("project").SaveX(setup)
	otherProject := db.Project.Create().SetName("other").SaveX(setup)
	creator := db.User.Create().SetEmail("creator@example.invalid").SetPassword("password").SaveX(setup)
	member := db.User.Create().SetEmail("member@example.invalid").SetPassword("password").SaveX(setup)
	db.UserProject.Create().SetUserID(creator.ID).SetProjectID(project.ID).
		SetScopes([]string{string(scopes.ScopeReadRequests)}).SaveX(setup)
	db.UserProject.Create().SetUserID(member.ID).SetProjectID(project.ID).
		SetScopes([]string{string(scopes.ScopeReadRequests)}).SaveX(setup)
	key := db.APIKey.Create().SetName("personal").SetKey("test-personal-key").SetType(apikey.TypePersonal).
		SetUserID(creator.ID).SetProjectID(project.ID).SaveX(setup)
	public := db.Request.Create().SetProjectID(project.ID).SetModelID("public").
		SetStatus(request.StatusCompleted).SetRequestBody([]byte("{}")).SaveX(setup)
	private := db.Request.Create().SetProjectID(project.ID).SetAPIKeyID(key.ID).SetModelID("private").
		SetStatus(request.StatusCompleted).SetRequestBody([]byte("{}")).SaveX(setup)
	foreign := db.Request.Create().SetProjectID(otherProject.ID).SetModelID("foreign").
		SetStatus(request.StatusCompleted).SetRequestBody([]byte("{}")).SaveX(setup)
	for _, req := range []*ent.Request{public, private, foreign} {
		db.RequestExecution.Create().SetRequestID(req.ID).SetProjectID(req.ProjectID).
			SetModelID(req.ModelID).SetOutboundModelID(req.ModelID).SetUpstreamModelID(req.ModelID).
			SetStatus(requestexecution.StatusCompleted).SetRequestBody([]byte("{}")).SaveX(setup)
	}
	// An inconsistent execution must not carry another project's data into a visible request.
	db.RequestExecution.Create().SetRequestID(public.ID).SetProjectID(otherProject.ID).
		SetModelID("foreign-data").SetOutboundModelID("foreign-data").SetUpstreamModelID("foreign-data").
		SetStatus(requestexecution.StatusCompleted).SetRequestBody([]byte("{}")).SaveX(setup)

	currentUser := db.User.Query().Where(user.ID(member.ID)).WithProjectUsers().OnlyX(setup)
	memberCtx := contexts.WithProjectID(contexts.WithUser(ent.NewContext(t.Context(), db), currentUser), project.ID)
	resolver := &requestResolver{&Resolver{client: db}}
	audit, err := resolver.ModelAudit(memberCtx, &ent.Request{ID: public.ID})
	require.NoError(t, err)
	require.Equal(t, []string{"public"}, audit.UpstreamModelIds)
	require.Equal(t, []string{"public"}, audit.MatchedUpstreamIds)
	require.Equal(t, 1, audit.ComparedCount)
	for _, req := range []*ent.Request{private, foreign} {
		_, err := resolver.ModelAudit(memberCtx, &ent.Request{ID: req.ID})
		require.Error(t, err, "fallback must recheck parent request privacy")
	}

	handler := NewGraphqlHandlers(Dependencies{Ent: db})
	client := gqlclient.New(handler.Graphql, func(req *gqlclient.Request) {
		req.HTTP = req.HTTP.WithContext(memberCtx)
	})
	var response struct {
		Requests struct {
			Edges []struct {
				Node struct{ ModelAudit objects.RequestModelAudit }
			}
		}
	}
	err = client.Post("{ requests(first: 10) { edges { node { modelAudit { status matchedUpstreamIds upstreamModelIds comparedCount } } } } }", &response)
	require.NoError(t, err)
	require.Len(t, response.Requests.Edges, 1)
	require.Equal(t, []string{"public"}, response.Requests.Edges[0].Node.ModelAudit.UpstreamModelIds)

	creatorUser := db.User.Query().Where(user.ID(creator.ID)).WithProjectUsers().OnlyX(setup)
	creatorCtx := contexts.WithProjectID(contexts.WithUser(ent.NewContext(t.Context(), db), creatorUser), project.ID)
	audit, err = resolver.ModelAudit(creatorCtx, &ent.Request{ID: private.ID})
	require.NoError(t, err)
	require.Equal(t, []string{"private"}, audit.UpstreamModelIds)
}
