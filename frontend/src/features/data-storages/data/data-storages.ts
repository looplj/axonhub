import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { graphqlRequest } from '@/gql/graphql'
import { toast } from 'sonner'
import { useErrorHandler } from '@/hooks/use-error-handler'
import i18n from '@/lib/i18n'

// GraphQL queries and mutations
const DATA_STORAGES_QUERY = `
  query DataStorages(
    $first: Int
    $after: Cursor
    $where: DataStorageWhereInput
    $orderBy: DataStorageOrder
  ) {
    dataStorages(
      first: $first
      after: $after
      where: $where
      orderBy: $orderBy
    ) {
      edges {
        node {
          id
          name
          description
          type
          primary
          status
          settings {
            directory
            s3 {
              bucketName
              endpoint
              region
              accessKey
              secretKey
            }
            gcs {
              bucketName
              credential
            }
          }
          createdAt
          updatedAt
        }
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

const CREATE_DATA_STORAGE_MUTATION = `
  mutation CreateDataStorage($input: CreateDataStorageInput!) {
    createDataStorage(input: $input) {
      id
      name
      description
      type
      primary
      status
      settings {
        directory
        s3 {
          bucketName
          endpoint
          region
          accessKey
          secretKey
        }
        gcs {
          bucketName
          credential
        }
      }
      createdAt
      updatedAt
    }
  }
`

const UPDATE_DATA_STORAGE_MUTATION = `
  mutation UpdateDataStorage($id: ID!, $input: UpdateDataStorageInput!) {
    updateDataStorage(id: $id, input: $input) {
      id
      name
      description
      type
      primary
      status
      settings {
        directory
        s3 {
          bucketName
          endpoint
          region
          accessKey
          secretKey
        }
        gcs {
          bucketName
          credential
        }
      }
      createdAt
      updatedAt
    }
  }
`

// Types
export interface S3Settings {
  bucketName: string
  endpoint: string
  region: string
  accessKey: string
  secretKey: string
}

export interface GCSSettings {
  bucketName: string
  credential: string
}

export interface DataStorageSettings {
  directory?: string
  s3?: S3Settings
  gcs?: GCSSettings
}

export interface DataStorage {
  id: string
  name: string
  description: string
  type: 'database' | 'fs' | 's3' | 'gcs'
  primary: boolean
  status: 'active' | 'archived'
  settings: DataStorageSettings
  createdAt: string
  updatedAt: string
}

export interface DataStorageEdge {
  node: DataStorage
}

export interface PageInfo {
  hasNextPage: boolean
  hasPreviousPage: boolean
  startCursor?: string
  endCursor?: string
}

export interface DataStoragesData {
  edges: DataStorageEdge[]
  pageInfo: PageInfo
  totalCount: number
}

export interface DataStoragesQueryVariables {
  first?: number
  after?: string
  where?: Record<string, any>
  orderBy?: {
    field: string
    direction: 'ASC' | 'DESC'
  }
}

export interface CreateDataStorageInput {
  name: string
  description?: string
  type: 'database' | 'fs' | 's3' | 'gcs'
  settings: DataStorageSettings
}

export interface UpdateDataStorageInput {
  name?: string
  description?: string
  settings?: DataStorageSettings
}

// Hooks
export function useDataStorages(variables?: DataStoragesQueryVariables) {
  const { handleError } = useErrorHandler()

  return useQuery({
    queryKey: ['dataStorages', variables],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ dataStorages: DataStoragesData }>(
          DATA_STORAGES_QUERY,
          variables
        )
        return data.dataStorages
      } catch (error) {
        handleError(error, '获取数据存储列表')
        throw error
      }
    },
  })
}

export function useCreateDataStorage() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (input: CreateDataStorageInput) => {
      const data = await graphqlRequest<{ createDataStorage: DataStorage }>(
        CREATE_DATA_STORAGE_MUTATION,
        { input }
      )
      return data.createDataStorage
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['dataStorages'] })
      toast.success(i18n.t('common.success.created'))
    },
    onError: () => {
      toast.error(i18n.t('common.errors.createFailed'))
    },
  })
}

export function useUpdateDataStorage() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async ({ id, input }: { id: string; input: UpdateDataStorageInput }) => {
      const data = await graphqlRequest<{ updateDataStorage: DataStorage }>(
        UPDATE_DATA_STORAGE_MUTATION,
        { id, input }
      )
      return data.updateDataStorage
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['dataStorages'] })
      toast.success(i18n.t('common.success.updated'))
    },
    onError: () => {
      toast.error(i18n.t('common.errors.updateFailed'))
    },
  })
}
