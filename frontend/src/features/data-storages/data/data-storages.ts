import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { graphqlRequest } from '@/gql/graphql'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useErrorHandler } from '@/hooks/use-error-handler'
import {
  dataStorageSchema,
  dataStoragesConnectionSchema,
  createDataStorageInputSchema,
  updateDataStorageInputSchema,
  dataStorageWithCredentialsSchema,
  type DataStorage,
  type DataStoragesConnection,
  type CreateDataStorageInput,
  type UpdateDataStorageInput,
} from './schema'

export type { DataStorage, DataStoragesConnection, CreateDataStorageInput, UpdateDataStorageInput }

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
            }
            gcs {
              bucketName
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
        }
        gcs {
          bucketName
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
        }
        gcs {
          bucketName
        }
      }
      createdAt
      updatedAt
    }
  }
`

const UPDATE_DATA_STORAGE_STATUS_MUTATION = `
  mutation UpdateDataStorageStatus($id: ID!, $input: UpdateDataStorageInput!) {
    updateDataStorage(id: $id, input: $input) {
      id
      status
      updatedAt
    }
  }
`

// Hooks
export function useDataStorages(variables?: Record<string, any>) {
  const { t } = useTranslation()
  const { handleError } = useErrorHandler()

  return useQuery({
    queryKey: ['dataStorages', variables],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ dataStorages: DataStoragesConnection }>(DATA_STORAGES_QUERY, variables)
        return dataStoragesConnectionSchema.parse(data.dataStorages)
      } catch (error) {
        handleError(error, t('dataStorages.errors.fetchData'))
        throw error
      }
    },
  })
}

export function useArchiveDataStorage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (id: string) => {
      const data = await graphqlRequest<{ updateDataStorage: Pick<DataStorage, 'id' | 'status'> }>(
        UPDATE_DATA_STORAGE_STATUS_MUTATION,
        {
          id,
          input: {
            status: 'archived',
          },
        }
      )

      return dataStorageSchema.pick({ id: true, status: true }).parse(data.updateDataStorage)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['dataStorages'] })
      toast.success(t('dataStorages.messages.archiveSuccess'))
    },
    onError: (error: any) => {
      toast.error(t('dataStorages.messages.archiveError'))
      console.error('Archive data storage error:', error)
    },
  })
}

export function useCreateDataStorage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (input: CreateDataStorageInput) => {
      // Validate input
      const validatedInput = createDataStorageInputSchema.parse(input)
      const data = await graphqlRequest<{ createDataStorage: DataStorage }>(CREATE_DATA_STORAGE_MUTATION, {
        input: validatedInput,
      })
      // Validate response data
      return dataStorageSchema.parse(data.createDataStorage)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['dataStorages'] })
      toast.success(t('common.messages.success'))
    },
    onError: (error: any) => {
      toast.error(t('dataStorages.errors.createError'))
      console.error('Create data storage error:', error)
    },
  })
}

export function useUpdateDataStorage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async ({ id, input }: { id: string; input: UpdateDataStorageInput }) => {
      // Validate input
      const validatedInput = updateDataStorageInputSchema.parse(input)
      const data = await graphqlRequest<{ updateDataStorage: DataStorage }>(UPDATE_DATA_STORAGE_MUTATION, {
        id,
        input: validatedInput,
      })
      // Use schema that includes credentials since this is for update
      return dataStorageWithCredentialsSchema.parse(data.updateDataStorage)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['dataStorages'] })
      toast.success(t('common.messages.success'))
    },
    onError: (error: any) => {
      toast.error(t('dataStorages.errors.updateError'))
      console.error('Update data storage error:', error)
    },
  })
}
