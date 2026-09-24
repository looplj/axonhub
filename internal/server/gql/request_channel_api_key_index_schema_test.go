package gql

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

// The request-log pages build their queries as strings in TypeScript, so
// neither the TypeScript compiler nor the frontend bundle catches a field name
// that the GraphQL schema does not define. These tests mirror the two query
// shapes the frontend sends and validate them against the real embedded
// schema, which is what keeps channelAPIKeyIndex wired end to end.

const requestListQueryWithChannelAPIKeyIndex = `
  query GetRequests(
    $first: Int
    $after: Cursor
    $last: Int
    $before: Cursor
    $orderBy: RequestOrder
    $where: RequestWhereInput
  ) {
    requests(first: $first, after: $after, last: $last, before: $before, orderBy: $orderBy, where: $where) {
      edges {
        node {
          id
          createdAt
          updatedAt
          source
          modelID
          format
          reasoningEffort
          stream
          status
          clientIP
          metricsLatencyMs
          metricsFirstTokenLatencyMs
          metricsReasoningDurationMs
          executions(first: 10, orderBy: { field: CREATED_AT, direction: DESC }) {
            edges {
              node {
                id
                createdAt
                modelID
                format
                status
                reasoningEffort
                passThroughApplied
                channel {
                  id
                  name
                }
                channelAPIKeyIndex
              }
              cursor
            }
            pageInfo {
              hasNextPage
              hasPreviousPage
              startCursor
              endCursor
            }
            totalCount
          }
          usageLogs(first: 1) {
            edges {
              node {
                id
                promptTokens
                completionTokens
                completionReasoningTokens
                totalTokens
                promptCachedTokens
                promptWriteCachedTokens
                totalCost
              }
            }
          }
        }
        cursor
      }
      pageInfo {
        hasNextPage
        hasPreviousPage
        startCursor
        endCursor
      }
    }
  }
`

const requestExecutionsQueryWithChannelAPIKeyIndex = `
  query GetRequestExecutions(
    $requestID: ID!
    $first: Int
    $after: Cursor
    $orderBy: RequestExecutionOrder
    $where: RequestExecutionWhereInput
  ) {
    node(id: $requestID) {
      ... on Request {
        executions(first: $first, after: $after, orderBy: $orderBy, where: $where) {
          edges {
            node {
              id
              createdAt
              updatedAt
              requestID
              channel {
                id
                name
                type
                baseURL
              }
              channelAPIKeyIndex
              modelID
              projectID
              dataStorageID
              requestHeaders
              requestBody
              responseBody
              responseChunks
              errorMessage
              responseStatusCode
              status
              format
              reasoningEffort
              stream
              requestURL
              passThroughApplied
              metricsFirstTokenLatencyMs
              metricsReasoningDurationMs
            }
            cursor
          }
          pageInfo {
            hasNextPage
            hasPreviousPage
            startCursor
            endCursor
          }
          totalCount
        }
      }
    }
  }
`

func TestFrontendRequestQueries_ExposeChannelAPIKeyIndex(t *testing.T) {
	schema := gqlparser.MustLoadSchema(requestExecutionSchemaSources()...)

	tests := []struct {
		name  string
		query string
	}{
		{name: "request list query", query: requestListQueryWithChannelAPIKeyIndex},
		{name: "request executions query", query: requestExecutionsQueryWithChannelAPIKeyIndex},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, errs := gqlparser.LoadQuery(schema, tt.query)
			require.Empty(t, errs, "frontend query must validate against the GraphQL schema")
		})
	}
}

// requestExecutionSchemaSources exposes the embedded SDL for query validation.
func requestExecutionSchemaSources() []*ast.Source {
	return sources
}
