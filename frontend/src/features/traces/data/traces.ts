import { useQuery } from '@tanstack/react-query'
import { graphqlRequest } from '@/gql/graphql'
import { useErrorHandler } from '@/hooks/use-error-handler'
import { useSelectedProjectId } from '@/stores/projectStore'
import {
  Trace,
  TraceConnection,
  TraceDetail,
  traceConnectionSchema,
  traceDetailSchema,
} from './schema'

// GraphQL query for traces
function buildTracesQuery() {
  return `
    query GetTraces(
      $first: Int
      $after: Cursor
      $orderBy: TraceOrder
      $where: TraceWhereInput
    ) {
      traces(first: $first, after: $after, orderBy: $orderBy, where: $where) {
        edges {
          node {
            id
            traceID
            firstUserQuery
            createdAt
            updatedAt
            thread {
              id
              threadID
            }
            requests(where: { status: completed }) {
              totalCount
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
        totalCount
      }
    }
  `
}

// GraphQL query for trace detail
function buildTraceDetailQuery() {
  return `
    query GetTraceDetail($id: ID!) {
      node(id: $id) {
        ... on Trace {
          id
          traceID
          createdAt
          updatedAt
          project {
            id
            name
          }
          thread {
            id
            threadID
          }
          requests(where: { status: completed }) {
            totalCount
          }
        }
      }
    }
  `
}

// GraphQL query for trace with request traces
function buildTraceWithRequestTracesQuery() {
  return `
    fragment SpanValue on SpanValue {
      systemInstruction {
        instruction
      }
      userQuery {
        text
      }
      userImageUrl {
        url
      }
      text {
        text
      }
      thinking {
        thinking
      }
      imageUrl {
        url
      }
      toolUse {
        id
        name
        arguments
      }
      toolResult {
        toolCallID
        isError
        text
      }
    }

    fragment SpanFields on Span {
      id
      type
      startTime
      endTime
      value {
        ...SpanValue
      }
    }

    fragment SegmentFields on Segment {
      id
      parentId
      model
      duration
      startTime
      endTime
      metadata {
        itemCount
        inputTokens
        outputTokens
        totalTokens
        cachedTokens
      }
      requestSpans {
        ...SpanFields
      }
      responseSpans {
        ...SpanFields
      }
    }

    fragment SegmentRecursive on Segment {
      ...SegmentFields
      children {
        ...SegmentFields
        children {
          ...SegmentFields
          children {
            ...SegmentFields
            children {
              ...SegmentFields
              # Support up to 5 levels of nesting
            }
          }
        }
      }
    }

    query GetTraceWithSegments($id: ID!) {
      node(id: $id) {
        ... on Trace {
          id
          traceID
          createdAt
          updatedAt
          project {
            id
            name
          }
          thread {
            id
            threadID
          }
          requests(where: { status: completed }) {
            totalCount
          }
          rawRootSegment
          rootSegment {
            ...SegmentRecursive
          }
        }
      }
    }
  `
}

// Query hooks
export function useTraces(variables?: {
  first?: number
  after?: string
  orderBy?: { field: 'CREATED_AT'; direction: 'ASC' | 'DESC' }
  where?: {
    projectID?: string
    threadID?: string
    traceID?: string
    [key: string]: any
  }
}) {
  const { handleError } = useErrorHandler()
  const selectedProjectId = useSelectedProjectId()
  
  return useQuery({
    queryKey: ['traces', variables, selectedProjectId],
    queryFn: async () => {
      try {
        const query = buildTracesQuery()
        const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined
        
        // Add project filter if project is selected
        const finalVariables = {
          ...variables,
          where: {
            ...variables?.where,
            ...(selectedProjectId && { projectID: selectedProjectId }),
          },
        }
        
        const data = await graphqlRequest<{ traces: TraceConnection }>(
          query,
          finalVariables,
          headers
        )
        return traceConnectionSchema.parse(data?.traces)
      } catch (error) {
        handleError(error, '获取追踪数据')
        throw error
      }
    },
    enabled: true, // Traces can be queried without project selection for admin users
  })
}

export function useTrace(id: string) {
  const { handleError } = useErrorHandler()
  const selectedProjectId = useSelectedProjectId()
  
  return useQuery({
    queryKey: ['trace', id, selectedProjectId],
    queryFn: async () => {
      try {
        const query = buildTraceDetailQuery()
        const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined
        const data = await graphqlRequest<{ node: Trace }>(
          query,
          { id },
          headers
        )
        if (!data.node) {
          throw new Error('Trace not found')
        }
        return traceDetailSchema.parse(data.node)
      } catch (error) {
        handleError(error, '获取追踪详情')
        throw error
      }
    },
    enabled: !!id,
  })
}

export function useTraceWithSegments(id: string) {
  const { handleError } = useErrorHandler()
  const selectedProjectId = useSelectedProjectId()
  
  return useQuery({
    queryKey: ['trace-with-segments', id, selectedProjectId],
    queryFn: async () => {
      try {
        const query = buildTraceWithRequestTracesQuery()
        const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined
        const data = await graphqlRequest<{ node: TraceDetail }>(
          query,
          { id },
          headers
        )
        if (!data.node) {
          throw new Error('Trace not found')
        }
        return traceDetailSchema.parse(data.node)
      } catch (error) {
        handleError(error, '获取追踪详情')
        throw error
      }
    },
    enabled: !!id,
  })
}

// Backward compatibility alias
export const useTraceWithRequestTraces = useTraceWithSegments
