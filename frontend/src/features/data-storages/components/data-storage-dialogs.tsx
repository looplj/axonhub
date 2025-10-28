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
    formState: { errors },
  } = useForm<DataStorageFormData>({
    defaultValues: {
      name: '',
      description: '',
      type: 'fs',
      directory: '',
    },
  })

  const selectedType = watch('type')

  // Reset form when dialogs open/close
  useEffect(() => {
    if (isCreateDialogOpen) {
      reset({
        name: '',
        description: '',
        type: 'fs',
        directory: '',
      })
    }
  }, [isCreateDialogOpen, reset])

  useEffect(() => {
    if (isEditDialogOpen && editingDataStorage) {
      reset({
        name: editingDataStorage.name,
        description: editingDataStorage.description,
        type: editingDataStorage.type,
        directory: editingDataStorage.settings.directory || '',
      })
    }
  }, [isEditDialogOpen, editingDataStorage, reset])

  const onCreateSubmit = async (data: DataStorageFormData) => {
    const input: CreateDataStorageInput = {
      name: data.name,
      description: data.description,
      type: data.type,
      settings: {
        directory: data.type === 'fs' ? data.directory : undefined,
      },
    }

    await createMutation.mutateAsync(input)
    setIsCreateDialogOpen(false)
    reset()
  }

  const onEditSubmit = async (data: DataStorageFormData) => {
    if (!editingDataStorage) return

    const input: UpdateDataStorageInput = {
      name: data.name,
      description: data.description,
      settings: {
        directory: data.type === 'fs' ? data.directory : undefined,
      },
    }

    await updateMutation.mutateAsync({
      id: editingDataStorage.id,
      input,
    })
    setIsEditDialogOpen(false)
    setEditingDataStorage(null)
    reset()
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
          <form onSubmit={handleSubmit(onCreateSubmit)}>
            <div className='grid gap-4 py-4'>
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
                      required:
                        selectedType === 'fs'
                          ? t(
                              'dataStorages.validation.directoryRequired',
                              '目录路径不能为空'
                            )
                          : false,
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
            </div>
            <DialogFooter>
              <Button
                type='button'
                variant='outline'
                onClick={() => setIsCreateDialogOpen(false)}
              >
                {t('common.cancel')}
              </Button>
              <Button type='submit' disabled={createMutation.isPending}>
                {createMutation.isPending
                  ? t('common.creating')
                  : t('common.create')}
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
          <form onSubmit={handleSubmit(onEditSubmit)}>
            <div className='grid gap-4 py-4'>
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
                      required:
                        selectedType === 'fs'
                          ? t(
                              'dataStorages.validation.directoryRequired',
                              '目录路径不能为空'
                            )
                          : false,
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
                {t('common.cancel')}
              </Button>
              <Button type='submit' disabled={updateMutation.isPending}>
                {updateMutation.isPending
                  ? t('common.saving')
                  : t('common.save')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </>
  )
}
