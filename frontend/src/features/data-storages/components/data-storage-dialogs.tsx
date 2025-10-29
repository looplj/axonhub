'use client'

import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useDataStoragesContext } from '../context/data-storages-context'
import {
  useCreateDataStorage,
  useUpdateDataStorage,
  CreateDataStorageInput,
  UpdateDataStorageInput,
} from '../data/data-storages'

interface DataStorageFormData {
  name: string
  description: string
  type: 'database' | 'fs' | 's3' | 'gcs'
  directory: string
  // S3 fields
  s3BucketName: string
  s3Endpoint: string
  s3Region: string
  s3AccessKey: string
  s3SecretKey: string
  // GCS fields
  gcsBucketName: string
  gcsCredential: string
}

export function DataStorageDialogs() {
  const { t } = useTranslation()
  const {
    isCreateDialogOpen,
    setIsCreateDialogOpen,
    isEditDialogOpen,
    setIsEditDialogOpen,
    editingDataStorage,
    setEditingDataStorage,
  } = useDataStoragesContext()

  const createMutation = useCreateDataStorage()
  const updateMutation = useUpdateDataStorage()

  const {
    register,
    handleSubmit,
    reset,
    watch,
    setValue,
    clearErrors,
    formState: { errors },
  } = useForm<DataStorageFormData>({
    defaultValues: {
      name: '',
      description: '',
      type: 'fs',
      directory: '',
      s3BucketName: '',
      s3Endpoint: '',
      s3Region: '',
      s3AccessKey: '',
      s3SecretKey: '',
      gcsBucketName: '',
      gcsCredential: '',
    },
  })

  const selectedType = watch('type')

  // Clear errors for fields that are not relevant to the current type
  useEffect(() => {
    console.log('[DataStorageDialogs] selectedType changed:', selectedType)
    
    // Clear errors for fields not relevant to current type
    if (selectedType === 'fs') {
      clearErrors(['s3BucketName', 's3Endpoint', 's3AccessKey', 's3SecretKey'])
      clearErrors(['gcsBucketName', 'gcsCredential'])
    } else if (selectedType === 's3') {
      clearErrors(['directory'])
      clearErrors(['gcsBucketName', 'gcsCredential'])
    } else if (selectedType === 'gcs') {
      clearErrors(['directory'])
      clearErrors(['s3BucketName', 's3Endpoint', 's3AccessKey', 's3SecretKey'])
    }
    
    console.log('[DataStorageDialogs] form errors after clear:', errors)
    console.log('[DataStorageDialogs] form values:', watch())
  }, [selectedType, clearErrors])

  // Reset form when dialogs open/close
  useEffect(() => {
    if (isCreateDialogOpen) {
      console.log('[DataStorageDialogs] Create dialog opened, resetting form')
      reset({
        name: '',
        description: '',
        type: 'fs',
        directory: '',
        s3BucketName: '',
        s3Endpoint: '',
        s3AccessKey: '',
        s3SecretKey: '',
        gcsBucketName: '',
        gcsCredential: '',
      })
    }
  }, [isCreateDialogOpen, reset])

  useEffect(() => {
    if (isEditDialogOpen && editingDataStorage) {
      console.log('[DataStorageDialogs] Edit dialog opened with data:', editingDataStorage)
      reset({
        name: editingDataStorage.name,
        description: editingDataStorage.description,
        type: editingDataStorage.type,
        directory: editingDataStorage.settings.directory || '',
        s3BucketName: editingDataStorage.settings.s3?.bucketName || '',
        s3Endpoint: editingDataStorage.settings.s3?.endpoint || '',
        s3Region: editingDataStorage.settings.s3?.region || '',
        s3AccessKey: editingDataStorage.settings.s3?.accessKey || '',
        s3SecretKey: editingDataStorage.settings.s3?.secretKey || '',
        gcsBucketName: editingDataStorage.settings.gcs?.bucketName || '',
        gcsCredential: editingDataStorage.settings.gcs?.credential || '',
      })
    }
  }, [isEditDialogOpen, editingDataStorage, reset])

  const onCreateSubmit = async (data: DataStorageFormData) => {
    console.log('[DataStorageDialogs] onCreateSubmit called')
    console.log('[DataStorageDialogs] Form data:', data)
    console.log('[DataStorageDialogs] Data type:', data.type)
    
    const input: CreateDataStorageInput = {
      name: data.name,
      description: data.description,
      type: data.type,
      settings: {
        directory: data.type === 'fs' ? data.directory : undefined,
        s3: data.type === 's3' ? {
          bucketName: data.s3BucketName,
          endpoint: data.s3Endpoint,
          region: data.s3Region,
          accessKey: data.s3AccessKey,
          secretKey: data.s3SecretKey,
        } : undefined,
        gcs: data.type === 'gcs' ? {
          bucketName: data.gcsBucketName,
          credential: data.gcsCredential,
        } : undefined,
      },
    }

    console.log('[DataStorageDialogs] API input:', JSON.stringify(input, null, 2))
    
    try {
      console.log('[DataStorageDialogs] Calling createMutation...')
      await createMutation.mutateAsync(input)
      console.log('[DataStorageDialogs] Create successful')
      setIsCreateDialogOpen(false)
      reset()
    } catch (error) {
      console.error('[DataStorageDialogs] Create failed:', error)
      throw error
    }
  }

  const onEditSubmit = async (data: DataStorageFormData) => {
    console.log('[DataStorageDialogs] onEditSubmit called')
    
    if (!editingDataStorage) {
      console.error('[DataStorageDialogs] No editingDataStorage found!')
      return
    }

    console.log('[DataStorageDialogs] Form data:', data)
    console.log('[DataStorageDialogs] Data type:', data.type)
    console.log('[DataStorageDialogs] Editing storage ID:', editingDataStorage.id)

    const input: UpdateDataStorageInput = {
      name: data.name,
      description: data.description,
      settings: {
        directory: data.type === 'fs' ? data.directory : undefined,
        s3: data.type === 's3' ? {
          bucketName: data.s3BucketName,
          endpoint: data.s3Endpoint,
          region: data.s3Region,
          accessKey: data.s3AccessKey,
          secretKey: data.s3SecretKey,
        } : undefined,
        gcs: data.type === 'gcs' ? {
          bucketName: data.gcsBucketName,
          credential: data.gcsCredential,
        } : undefined,
      },
    }

    console.log('[DataStorageDialogs] API input:', JSON.stringify(input, null, 2))

    try {
      console.log('[DataStorageDialogs] Calling updateMutation...')
      await updateMutation.mutateAsync({
        id: editingDataStorage.id,
        input,
      })
      console.log('[DataStorageDialogs] Update successful')
      setIsEditDialogOpen(false)
      setEditingDataStorage(null)
      reset()
    } catch (error) {
      console.error('[DataStorageDialogs] Update failed:', error)
      throw error
    }
  }

  return (
    <>
      {/* Create Dialog */}
      <Dialog open={isCreateDialogOpen} onOpenChange={setIsCreateDialogOpen}>
        <DialogContent className='sm:max-w-[525px]'>
          <DialogHeader>
            <DialogTitle>
              {t('dataStorages.dialogs.create.title', '创建数据存储')}
            </DialogTitle>
            <DialogDescription>
              {t(
                'dataStorages.dialogs.create.description',
                '配置新的数据存储位置'
              )}
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={handleSubmit(
            onCreateSubmit, 
            (errors) => {
              console.error('[DataStorageDialogs] ❌ Form validation FAILED!')
              console.error('[DataStorageDialogs] Validation errors:', errors)
              console.error('[DataStorageDialogs] Current form values:', watch())
              console.error('[DataStorageDialogs] Selected type:', selectedType)
            }
          )} noValidate>
            <div className='grid gap-4 py-4 max-h-[60vh] overflow-y-auto'>
              <div className='grid gap-2'>
                <Label htmlFor='create-name'>
                  {t('dataStorages.fields.name', '名称')}
                </Label>
                <Input
                  id='create-name'
                  {...register('name', {
                    required: t('dataStorages.validation.nameRequired', '名称不能为空'),
                  })}
                />
                {errors.name && (
                  <span className='text-sm text-red-500'>
                    {errors.name.message}
                  </span>
                )}
              </div>

              <div className='grid gap-2'>
                <Label htmlFor='create-description'>
                  {t('dataStorages.fields.description', '描述')}
                </Label>
                <Textarea
                  id='create-description'
                  {...register('description')}
                  rows={3}
                />
              </div>

              <div className='grid gap-2'>
                <Label htmlFor='create-type'>
                  {t('dataStorages.fields.type', '类型')}
                </Label>
                <Select
                  value={selectedType}
                  onValueChange={(value) =>
                    setValue('type', value as DataStorageFormData['type'])
                  }
                >
                  <SelectTrigger id='create-type'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='fs'>
                      {t('dataStorages.types.fs', '文件系统')}
                    </SelectItem>
                    <SelectItem value='s3'>
                      {t('dataStorages.types.s3', 'S3')}
                    </SelectItem>
                    <SelectItem value='gcs'>
                      {t('dataStorages.types.gcs', 'GCS')}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>

              {selectedType === 'fs' && (
                <div className='grid gap-2'>
                  <Label htmlFor='create-directory'>
                    {t('dataStorages.fields.directory', '目录路径')}
                  </Label>
                  <Input
                    id='create-directory'
                    {...register('directory', {
                      validate: (value) => {
                        // Only validate if the current type is 'fs'
                        if (watch('type') === 'fs' && !value) {
                          return t(
                            'dataStorages.validation.directoryRequired',
                            '目录路径不能为空'
                          )
                        }
                        return true
                      },
                    })}
                    placeholder='/var/axonhub/data'
                  />
                  {errors.directory && (
                    <span className='text-sm text-red-500'>
                      {errors.directory.message}
                    </span>
                  )}
                </div>
              )}

              {selectedType === 's3' && (
                <>
                  <div className='grid gap-2'>
                    <Label htmlFor='create-s3-bucket'>
                      {t('dataStorages.fields.s3BucketName', 'Bucket 名称')}
                    </Label>
                    <Input
                      id='create-s3-bucket'
                      {...register('s3BucketName', {
                        validate: (value) => {
                          if (watch('type') === 's3' && !value) {
                            return t('dataStorages.validation.s3BucketRequired', 'Bucket 名称不能为空')
                          }
                          return true
                        },
                      })}
                      placeholder='my-bucket'
                    />
                    {errors.s3BucketName && (
                      <span className='text-sm text-red-500'>{errors.s3BucketName.message}</span>
                    )}
                  </div>
                  <div className='grid gap-2'>
                    <Label htmlFor='create-s3-endpoint'>
                      {t('dataStorages.fields.s3Endpoint', 'Endpoint (可选)')}
                    </Label>
                    <Input
                      id='create-s3-endpoint'
                      {...register('s3Endpoint')}
                      placeholder='https://s3.amazonaws.com'
                    />
                  </div>
                  <div className='grid gap-2'>
                    <Label htmlFor='create-s3-region'>
                      {t('dataStorages.fields.s3Region', 'Region (可选)')}
                    </Label>
                    <Input
                      id='create-s3-region'
                      {...register('s3Region')}
                      placeholder='us-east-1'
                    />
                  </div>
                  <div className='grid gap-2'>
                    <Label htmlFor='create-s3-access-key'>
                      {t('dataStorages.fields.s3AccessKey', 'Access Key')}
                    </Label>
                    <Input
                      id='create-s3-access-key'
                      {...register('s3AccessKey', {
                        validate: (value) => {
                          if (watch('type') === 's3' && !value) {
                            return t('dataStorages.validation.s3AccessKeyRequired', 'Access Key 不能为空')
                          }
                          return true
                        },
                      })}
                    />
                    {errors.s3AccessKey && (
                      <span className='text-sm text-red-500'>{errors.s3AccessKey.message}</span>
                    )}
                  </div>
                  <div className='grid gap-2'>
                    <Label htmlFor='create-s3-secret-key'>
                      {t('dataStorages.fields.s3SecretKey', 'Secret Key')}
                    </Label>
                    <Input
                      id='create-s3-secret-key'
                      type='password'
                      {...register('s3SecretKey', {
                        validate: (value) => {
                          if (watch('type') === 's3' && !value) {
                            return t('dataStorages.validation.s3SecretKeyRequired', 'Secret Key 不能为空')
                          }
                          return true
                        },
                      })}
                    />
                    {errors.s3SecretKey && (
                      <span className='text-sm text-red-500'>{errors.s3SecretKey.message}</span>
                    )}
                  </div>
                </>
              )}

              {selectedType === 'gcs' && (
                <>
                  <div className='grid gap-2'>
                    <Label htmlFor='create-gcs-bucket'>
                      {t('dataStorages.fields.gcsBucketName', 'Bucket 名称')}
                    </Label>
                    <Input
                      id='create-gcs-bucket'
                      {...register('gcsBucketName', {
                        validate: (value) => {
                          if (watch('type') === 'gcs' && !value) {
                            return t('dataStorages.validation.gcsBucketRequired', 'Bucket 名称不能为空')
                          }
                          return true
                        },
                      })}
                      placeholder='my-bucket'
                    />
                    {errors.gcsBucketName && (
                      <span className='text-sm text-red-500'>{errors.gcsBucketName.message}</span>
                    )}
                  </div>
                  <div className='grid gap-2'>
                    <Label htmlFor='create-gcs-credential'>
                      {t('dataStorages.fields.gcsCredential', 'Service Account JSON')}
                    </Label>
                    <Textarea
                      id='create-gcs-credential'
                      {...register('gcsCredential', {
                        validate: (value) => {
                          if (watch('type') === 'gcs' && !value) {
                            return t('dataStorages.validation.gcsCredentialRequired', 'Service Account JSON 不能为空')
                          }
                          return true
                        },
                      })}
                      className='max-h-48 overflow-auto'
                      rows={5}
                      placeholder='{"type": "service_account", ...}'
                    />
                    {errors.gcsCredential && (
                      <span className='text-sm text-red-500'>{errors.gcsCredential.message}</span>
                    )}
                  </div>
                </>
              )}
            </div>
            <DialogFooter>
              <Button
                type='button'
                variant='outline'
                onClick={() => setIsCreateDialogOpen(false)}
              >
                {t('common.buttons.cancel')}
              </Button>
              <Button 
                type='submit' 
                disabled={createMutation.isPending}
                onClick={() => console.log('[DataStorageDialogs] Create button clicked')}
              >
                {createMutation.isPending
                  ? t('common.buttons.creating')
                  : t('common.buttons.create')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Edit Dialog */}
      <Dialog
        open={isEditDialogOpen}
        onOpenChange={(open) => {
          setIsEditDialogOpen(open)
          if (!open) setEditingDataStorage(null)
        }}
      >
        <DialogContent className='sm:max-w-[525px]'>
          <DialogHeader>
            <DialogTitle>
              {t('dataStorages.dialogs.edit.title', '编辑数据存储')}
            </DialogTitle>
            <DialogDescription>
              {t(
                'dataStorages.dialogs.edit.description',
                '修改数据存储配置'
              )}
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={handleSubmit(
            onEditSubmit,
            (errors) => {
              console.error('[DataStorageDialogs] ❌ Edit form validation FAILED!')
              console.error('[DataStorageDialogs] Validation errors:', errors)
              console.error('[DataStorageDialogs] Current form values:', watch())
              console.error('[DataStorageDialogs] Selected type:', selectedType)
            }
          )} noValidate>
            <div className='grid gap-4 py-4 max-h-[60vh] overflow-y-auto'>
              <div className='grid gap-2'>
                <Label htmlFor='edit-name'>
                  {t('dataStorages.fields.name', '名称')}
                </Label>
                <Input
                  id='edit-name'
                  {...register('name', {
                    required: t('dataStorages.validation.nameRequired', '名称不能为空'),
                  })}
                />
                {errors.name && (
                  <span className='text-sm text-red-500'>
                    {errors.name.message}
                  </span>
                )}
              </div>

              <div className='grid gap-2'>
                <Label htmlFor='edit-description'>
                  {t('dataStorages.fields.description', '描述')}
                </Label>
                <Textarea
                  id='edit-description'
                  {...register('description')}
                  rows={3}
                />
              </div>

              {selectedType === 'fs' && (
                <div className='grid gap-2'>
                  <Label htmlFor='edit-directory'>
                    {t('dataStorages.fields.directory', '目录路径')}
                  </Label>
                  <Input
                    id='edit-directory'
                    {...register('directory', {
                      validate: (value) => {
                        // Only validate if the current type is 'fs'
                        if (watch('type') === 'fs' && !value) {
                          return t(
                            'dataStorages.validation.directoryRequired',
                            '目录路径不能为空'
                          )
                        }
                        return true
                      },
                    })}
                    placeholder='/var/axonhub/data'
                  />
                  {errors.directory && (
                    <span className='text-sm text-red-500'>
                      {errors.directory.message}
                    </span>
                  )}
                </div>
              )}

              {selectedType === 's3' && (
                <>
                  <div className='grid gap-2'>
                    <Label htmlFor='edit-s3-bucket'>
                      {t('dataStorages.fields.s3BucketName', 'Bucket 名称')}
                    </Label>
                    <Input
                      id='edit-s3-bucket'
                      {...register('s3BucketName', {
                        validate: (value) => {
                          if (watch('type') === 's3' && !value) {
                            return t('dataStorages.validation.s3BucketRequired', 'Bucket 名称不能为空')
                          }
                          return true
                        },
                      })}
                      placeholder='my-bucket'
                    />
                    {errors.s3BucketName && (
                      <span className='text-sm text-red-500'>{errors.s3BucketName.message}</span>
                    )}
                  </div>
                  <div className='grid gap-2'>
                    <Label htmlFor='edit-s3-endpoint'>
                      {t('dataStorages.fields.s3Endpoint', 'Endpoint (可选)')}
                    </Label>
                    <Input
                      id='edit-s3-endpoint'
                      {...register('s3Endpoint')}
                      placeholder='https://s3.amazonaws.com'
                    />
                  </div>
                  <div className='grid gap-2'>
                    <Label htmlFor='edit-s3-region'>
                      {t('dataStorages.fields.s3Region', 'Region (可选)')}
                    </Label>
                    <Input
                      id='edit-s3-region'
                      {...register('s3Region')}
                      placeholder='us-east-1'
                    />
                  </div>
                  <div className='grid gap-2'>
                    <Label htmlFor='edit-s3-access-key'>
                      {t('dataStorages.fields.s3AccessKey', 'Access Key')}
                    </Label>
                    <Input
                      id='edit-s3-access-key'
                      {...register('s3AccessKey', {
                        validate: (value) => {
                          if (watch('type') === 's3' && !value) {
                            return t('dataStorages.validation.s3AccessKeyRequired', 'Access Key 不能为空')
                          }
                          return true
                        },
                      })}
                    />
                    {errors.s3AccessKey && (
                      <span className='text-sm text-red-500'>{errors.s3AccessKey.message}</span>
                    )}
                  </div>
                  <div className='grid gap-2'>
                    <Label htmlFor='edit-s3-secret-key'>
                      {t('dataStorages.fields.s3SecretKey', 'Secret Key')}
                    </Label>
                    <Input
                      id='edit-s3-secret-key'
                      type='password'
                      {...register('s3SecretKey', {
                        validate: (value) => {
                          if (watch('type') === 's3' && !value) {
                            return t('dataStorages.validation.s3SecretKeyRequired', 'Secret Key 不能为空')
                          }
                          return true
                        },
                      })}
                    />
                    {errors.s3SecretKey && (
                      <span className='text-sm text-red-500'>{errors.s3SecretKey.message}</span>
                    )}
                  </div>
                </>
              )}

              {selectedType === 'gcs' && (
                <>
                  <div className='grid gap-2'>
                    <Label htmlFor='edit-gcs-bucket'>
                      {t('dataStorages.fields.gcsBucketName', 'Bucket 名称')}
                    </Label>
                    <Input
                      id='edit-gcs-bucket'
                      {...register('gcsBucketName', {
                        validate: (value) => {
                          if (watch('type') === 'gcs' && !value) {
                            return t('dataStorages.validation.gcsBucketRequired', 'Bucket 名称不能为空')
                          }
                          return true
                        },
                      })}
                      placeholder='my-bucket'
                    />
                    {errors.gcsBucketName && (
                      <span className='text-sm text-red-500'>{errors.gcsBucketName.message}</span>
                    )}
                  </div>
                  <div className='grid gap-2'>
                    <Label htmlFor='edit-gcs-credential'>
                      {t('dataStorages.fields.gcsCredential', 'Service Account JSON')}
                    </Label>
                    <Textarea
                      id='edit-gcs-credential'
                      {...register('gcsCredential', {
                        validate: (value) => {
                          if (watch('type') === 'gcs' && !value) {
                            return t('dataStorages.validation.gcsCredentialRequired', 'Service Account JSON 不能为空')
                          }
                          return true
                        },
                      })}
                      className='max-h-48 overflow-auto'
                      rows={5}
                      placeholder='{"type": "service_account", ...}'
                    />
                    {errors.gcsCredential && (
                      <span className='text-sm text-red-500'>{errors.gcsCredential.message}</span>
                    )}
                  </div>
                </>
              )}
            </div>
            <DialogFooter>
              <Button
                type='button'
                variant='outline'
                onClick={() => {
                  setIsEditDialogOpen(false)
                  setEditingDataStorage(null)
                }}
              >
                {t('common.buttons.cancel')}
              </Button>
              <Button 
                type='submit' 
                disabled={updateMutation.isPending}
                onClick={() => console.log('[DataStorageDialogs] Save button clicked')}
              >
                {updateMutation.isPending
                  ? t('common.buttons.saving')
                  : t('common.buttons.save')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </>
  )
}
