'use client';

import { useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { useDataStoragesContext } from '../context/data-storages-context';
import {
  useCreateDataStorage,
  useUpdateDataStorage,
  useArchiveDataStorage,
  CreateDataStorageInput,
  UpdateDataStorageInput,
} from '../data/data-storages';

interface DataStorageFormData {
  name: string;
  description: string;
  type: 'database' | 'fs' | 's3' | 'gcs';
  directory: string;
  // S3 fields
  s3BucketName: string;
  s3Endpoint: string;
  s3Region: string;
  s3AccessKey: string;
  s3SecretKey: string;
  // GCS fields
  gcsBucketName: string;
  gcsCredential: string;
}

export function DataStorageDialogs() {
  const { t } = useTranslation();
  const {
    isCreateDialogOpen,
    setIsCreateDialogOpen,
    isEditDialogOpen,
    setIsEditDialogOpen,
    isArchiveDialogOpen,
    setIsArchiveDialogOpen,
    editingDataStorage,
    setEditingDataStorage,
    archiveDataStorage,
    setArchiveDataStorage,
  } = useDataStoragesContext();

  const createMutation = useCreateDataStorage();
  const updateMutation = useUpdateDataStorage();
  const archiveMutation = useArchiveDataStorage();

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
  });

  const selectedType = watch('type');

  // Clear errors for fields that are not relevant to the current type
  useEffect(() => {
    // Clear errors for fields not relevant to current type
    if (selectedType === 'fs') {
      clearErrors(['s3BucketName', 's3Endpoint', 's3AccessKey', 's3SecretKey']);
      clearErrors(['gcsBucketName', 'gcsCredential']);
    } else if (selectedType === 's3') {
      clearErrors(['directory']);
      clearErrors(['gcsBucketName', 'gcsCredential']);
    } else if (selectedType === 'gcs') {
      clearErrors(['directory']);
      clearErrors(['s3BucketName', 's3Endpoint', 's3AccessKey', 's3SecretKey']);
    }
  }, [selectedType, clearErrors]);

  // Reset form when dialogs open/close
  useEffect(() => {
    if (isCreateDialogOpen) {
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
      });
    }
  }, [isCreateDialogOpen, reset]);

  useEffect(() => {
    if (isEditDialogOpen && editingDataStorage) {
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
      });
    }
  }, [isEditDialogOpen, editingDataStorage, reset]);

  const resetArchiveContext = () => {
    setIsArchiveDialogOpen(false);
    setArchiveDataStorage(null);
  };

  const onCreateSubmit = async (data: DataStorageFormData) => {
    const input: CreateDataStorageInput = {
      name: data.name,
      description: data.description,
      type: data.type,
      settings: {
        directory: data.type === 'fs' ? data.directory : undefined,
        s3:
          data.type === 's3'
            ? {
                bucketName: data.s3BucketName,
                endpoint: data.s3Endpoint,
                region: data.s3Region,
                accessKey: data.s3AccessKey,
                secretKey: data.s3SecretKey,
              }
            : undefined,
        gcs:
          data.type === 'gcs'
            ? {
                bucketName: data.gcsBucketName,
                credential: data.gcsCredential,
              }
            : undefined,
      },
    };

    try {
      await createMutation.mutateAsync(input);
      setIsCreateDialogOpen(false);
      reset();
    } catch (error) {
      throw error;
    }
  };

  const onEditSubmit = async (data: DataStorageFormData) => {
    if (!editingDataStorage) {
      return;
    }

    // Build settings, only including non-empty values
    const settings: any = {};
    if (data.type === 'fs' && data.directory) {
      settings.directory = data.directory;
    } else if (data.type === 's3') {
      // Only include S3 if at least one field is provided
      const s3Data: any = {};
      if (data.s3BucketName) s3Data.bucketName = data.s3BucketName;
      if (data.s3Endpoint) s3Data.endpoint = data.s3Endpoint;
      if (data.s3Region) s3Data.region = data.s3Region;
      if (data.s3AccessKey) s3Data.accessKey = data.s3AccessKey;
      if (data.s3SecretKey) s3Data.secretKey = data.s3SecretKey;

      // Only include S3 object if it has at least one field
      if (Object.keys(s3Data).length > 0) {
        settings.s3 = s3Data;
      }
    } else if (data.type === 'gcs') {
      // Only include GCS if at least one field is provided
      const gcsData: any = {};
      if (data.gcsBucketName) gcsData.bucketName = data.gcsBucketName;
      if (data.gcsCredential) gcsData.credential = data.gcsCredential;

      // Only include GCS object if it has at least one field
      if (Object.keys(gcsData).length > 0) {
        settings.gcs = gcsData;
      }
    }

    const input: UpdateDataStorageInput = {
      name: data.name,
      description: data.description,
      settings,
    };

    try {
      await updateMutation.mutateAsync({
        id: editingDataStorage.id,
        input,
      });
      setIsEditDialogOpen(false);
      setEditingDataStorage(null);
      reset();
    } catch (error) {
      throw error;
    }
  };

  return (
    <>
      {/* Create Dialog */}
      <Dialog open={isCreateDialogOpen} onOpenChange={setIsCreateDialogOpen}>
        <DialogContent className='sm:max-w-[525px]'>
          <DialogHeader>
            <DialogTitle>{t('dataStorages.dialogs.create.title')}</DialogTitle>
            <DialogDescription>{t('dataStorages.dialogs.create.description')}</DialogDescription>
          </DialogHeader>
          <form
            onSubmit={handleSubmit(onCreateSubmit, (errors) => {})}
            noValidate
          >
            <div className='grid max-h-[60vh] gap-4 overflow-y-auto py-4'>
              <div className='grid gap-2'>
                <Label htmlFor='create-name'>{t('dataStorages.fields.name')}</Label>
                <Input
                  id='create-name'
                  {...register('name', {
                    required: t('dataStorages.validation.nameRequired'),
                  })}
                />
                {errors.name && <span className='text-sm text-red-500'>{errors.name.message}</span>}
              </div>

              <div className='grid gap-2'>
                <Label htmlFor='create-description'>{t('dataStorages.fields.description')}</Label>
                <Textarea id='create-description' {...register('description')} rows={3} />
              </div>

              <div className='grid gap-2'>
                <Label htmlFor='create-type'>{t('dataStorages.fields.type')}</Label>
                <Select value={selectedType} onValueChange={(value) => setValue('type', value as DataStorageFormData['type'])}>
                  <SelectTrigger id='create-type'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='fs'>{t('dataStorages.types.fs')}</SelectItem>
                    <SelectItem value='s3'>{t('dataStorages.types.s3')}</SelectItem>
                    <SelectItem value='gcs'>{t('dataStorages.types.gcs')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              {selectedType === 'fs' && (
                <div className='grid gap-2'>
                  <Label htmlFor='create-directory'>{t('dataStorages.fields.directory')}</Label>
                  <Input
                    id='create-directory'
                    {...register('directory', {
                      validate: (value) => {
                        // Only validate if the current type is 'fs'
                        if (watch('type') === 'fs' && !value) {
                          return t('dataStorages.validation.directoryRequired');
                        }
                        return true;
                      },
                    })}
                    placeholder='/var/axonhub/data'
                  />
                  {errors.directory && <span className='text-sm text-red-500'>{errors.directory.message}</span>}
                </div>
              )}

              {selectedType === 's3' && (
                <>
                  <div className='grid gap-2'>
                    <Label htmlFor='create-s3-bucket'>{t('dataStorages.fields.s3BucketName')}</Label>
                    <Input
                      id='create-s3-bucket'
                      {...register('s3BucketName', {
                        validate: (value) => {
                          if (watch('type') === 's3' && !value) {
                            return t('dataStorages.validation.s3BucketRequired');
                          }
                          return true;
                        },
                      })}
                      placeholder='my-bucket'
                    />
                    {errors.s3BucketName && <span className='text-sm text-red-500'>{errors.s3BucketName.message}</span>}
                  </div>
                  <div className='grid gap-2'>
                    <Label htmlFor='create-s3-endpoint'>{t('dataStorages.fields.s3Endpoint')}</Label>
                    <Input id='create-s3-endpoint' {...register('s3Endpoint')} placeholder='https://s3.amazonaws.com' />
                  </div>
                  <div className='grid gap-2'>
                    <Label htmlFor='create-s3-region'>{t('dataStorages.fields.s3Region')}</Label>
                    <Input id='create-s3-region' {...register('s3Region')} placeholder='us-east-1' />
                  </div>
                  <div className='grid gap-2'>
                    <Label htmlFor='create-s3-access-key'>
                      {t('dataStorages.fields.s3AccessKey')} {isEditDialogOpen ? '' : '*'}
                    </Label>
                    <Input
                      id='create-s3-access-key'
                      {...register('s3AccessKey', {
                        validate: (value) => {
                          if (watch('type') === 's3') {
                            // Only require for create, not for edit
                            if (!isEditDialogOpen && !value) {
                              return t('dataStorages.validation.s3AccessKeyRequired');
                            }
                          }
                          return true;
                        },
                      })}
                      placeholder={isEditDialogOpen ? 'Leave empty to keep current value' : ''}
                    />
                    {errors.s3AccessKey && <span className='text-sm text-red-500'>{errors.s3AccessKey.message}</span>}
                  </div>
                  <div className='grid gap-2'>
                    <Label htmlFor='create-s3-secret-key'>
                      {t('dataStorages.fields.s3SecretKey')} {isEditDialogOpen ? '' : '*'}
                    </Label>
                    <Input
                      id='create-s3-secret-key'
                      type='password'
                      {...register('s3SecretKey', {
                        validate: (value) => {
                          if (watch('type') === 's3') {
                            // Only require for create, not for edit
                            if (!isEditDialogOpen && !value) {
                              return t('dataStorages.validation.s3SecretKeyRequired');
                            }
                          }
                          return true;
                        },
                      })}
                      placeholder={isEditDialogOpen ? 'Leave empty to keep current value' : ''}
                    />
                    {errors.s3SecretKey && <span className='text-sm text-red-500'>{errors.s3SecretKey.message}</span>}
                  </div>
                </>
              )}

              {selectedType === 'gcs' && (
                <>
                  <div className='grid gap-2'>
                    <Label htmlFor='create-gcs-bucket'>{t('dataStorages.fields.gcsBucketName')}</Label>
                    <Input
                      id='create-gcs-bucket'
                      {...register('gcsBucketName', {
                        validate: (value) => {
                          if (watch('type') === 'gcs' && !value) {
                            return t('dataStorages.validation.gcsBucketRequired');
                          }
                          return true;
                        },
                      })}
                      placeholder='my-bucket'
                    />
                    {errors.gcsBucketName && <span className='text-sm text-red-500'>{errors.gcsBucketName.message}</span>}
                  </div>
                  <div className='grid gap-2'>
                    <Label htmlFor='create-gcs-credential'>
                      {t('dataStorages.fields.gcsCredential')} {isEditDialogOpen ? '' : '*'}
                    </Label>
                    <Textarea
                      id='create-gcs-credential'
                      {...register('gcsCredential', {
                        validate: (value) => {
                          if (watch('type') === 'gcs') {
                            // Only require for create, not for edit
                            const trimmedValue = value?.trim() ?? '';

                            if (!isEditDialogOpen && !trimmedValue) {
                              return t('dataStorages.validation.gcsCredentialRequired');
                            }

                            if (trimmedValue) {
                              try {
                                const parsed = JSON.parse(trimmedValue);
                                if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
                                  return t('dataStorages.validation.gcsCredentialInvalid');
                                }
                              } catch (_error) {
                                return t('dataStorages.validation.gcsCredentialInvalid');
                              }
                            }
                          }
                          return true;
                        },
                      })}
                      className='max-h-48 overflow-auto'
                      rows={5}
                      placeholder={isEditDialogOpen ? 'Leave empty to keep current value' : '{"type": "service_account", ...}'}
                    />
                    {errors.gcsCredential && <span className='text-sm text-red-500'>{errors.gcsCredential.message}</span>}
                  </div>
                </>
              )}
            </div>
            <DialogFooter>
              <Button type='button' variant='outline' onClick={() => setIsCreateDialogOpen(false)}>
                {t('common.buttons.cancel')}
              </Button>
              <Button
                type='submit'
                disabled={createMutation.isPending}
              >
                {createMutation.isPending ? t('common.buttons.creating') : t('common.buttons.create')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Edit Dialog */}
      <Dialog
        open={isEditDialogOpen}
        onOpenChange={(open) => {
          setIsEditDialogOpen(open);
          if (!open) setEditingDataStorage(null);
        }}
      >
        <DialogContent className='sm:max-w-[525px]'>
          <DialogHeader>
            <DialogTitle>{t('dataStorages.dialogs.edit.title')}</DialogTitle>
            <DialogDescription>{t('dataStorages.dialogs.edit.description')}</DialogDescription>
          </DialogHeader>
          <form
            onSubmit={handleSubmit(onEditSubmit, (errors) => {})}
            noValidate
          >
            <div className='grid max-h-[60vh] gap-4 overflow-y-auto py-4'>
              <div className='grid gap-2'>
                <Label htmlFor='edit-name'>{t('dataStorages.fields.name')}</Label>
                <Input
                  id='edit-name'
                  {...register('name', {
                    required: t('dataStorages.validation.nameRequired'),
                  })}
                />
                {errors.name && <span className='text-sm text-red-500'>{errors.name.message}</span>}
              </div>

              <div className='grid gap-2'>
                <Label htmlFor='edit-description'>{t('dataStorages.fields.description')}</Label>
                <Textarea id='edit-description' {...register('description')} rows={3} />
              </div>

              {selectedType === 'fs' && (
                <div className='grid gap-2'>
                  <Label htmlFor='edit-directory'>{t('dataStorages.fields.directory')}</Label>
                  <Input
                    id='edit-directory'
                    {...register('directory', {
                      validate: (value) => {
                        // Only validate if the current type is 'fs'
                        if (watch('type') === 'fs' && !value) {
                          return t('dataStorages.validation.directoryRequired');
                        }
                        return true;
                      },
                    })}
                    placeholder='/var/axonhub/data'
                  />
                  {errors.directory && <span className='text-sm text-red-500'>{errors.directory.message}</span>}
                </div>
              )}

              {selectedType === 's3' && (
                <>
                  <div className='grid gap-2'>
                    <Label htmlFor='edit-s3-bucket'>{t('dataStorages.fields.s3BucketName')}</Label>
                    <Input
                      id='edit-s3-bucket'
                      {...register('s3BucketName', {
                        validate: (value) => {
                          if (watch('type') === 's3' && !value) {
                            return t('dataStorages.validation.s3BucketRequired');
                          }
                          return true;
                        },
                      })}
                      placeholder='my-bucket'
                    />
                    {errors.s3BucketName && <span className='text-sm text-red-500'>{errors.s3BucketName.message}</span>}
                  </div>
                  <div className='grid gap-2'>
                    <Label htmlFor='edit-s3-endpoint'>{t('dataStorages.fields.s3Endpoint')}</Label>
                    <Input id='edit-s3-endpoint' {...register('s3Endpoint')} placeholder='https://s3.amazonaws.com' />
                  </div>
                  <div className='grid gap-2'>
                    <Label htmlFor='edit-s3-region'>{t('dataStorages.fields.s3Region')}</Label>
                    <Input id='edit-s3-region' {...register('s3Region')} placeholder='us-east-1' />
                  </div>
                  <div className='grid gap-2'>
                    <Label htmlFor='edit-s3-access-key'>{t('dataStorages.fields.s3AccessKey')}</Label>
                    <Input
                      id='edit-s3-access-key'
                      {...register('s3AccessKey', {
                        validate: (value) => {
                          // Only required for new data storage (no editingDataStorage)
                          // For updates, empty value means keep current value
                          if (watch('type') === 's3' && !value && !editingDataStorage) {
                            return t('dataStorages.validation.s3AccessKeyRequired');
                          }
                          return true;
                        },
                      })}
                      placeholder={t('dataStorages.dialogs.fields.s3AccessKey.editPlaceholder')}
                    />
                    {errors.s3AccessKey && <span className='text-sm text-red-500'>{errors.s3AccessKey.message}</span>}
                  </div>
                  <div className='grid gap-2'>
                    <Label htmlFor='edit-s3-secret-key'>{t('dataStorages.fields.s3SecretKey')}</Label>
                    <Input
                      id='edit-s3-secret-key'
                      type='password'
                      {...register('s3SecretKey', {
                        validate: (value) => {
                          // Only required for new data storage (no editingDataStorage)
                          // For updates, empty value means keep current value
                          if (watch('type') === 's3' && !value && !editingDataStorage) {
                            return t('dataStorages.validation.s3SecretKeyRequired');
                          }
                          return true;
                        },
                      })}
                      placeholder={t('dataStorages.dialogs.fields.s3SecretKey.editPlaceholder')}
                    />
                    {errors.s3SecretKey && <span className='text-sm text-red-500'>{errors.s3SecretKey.message}</span>}
                  </div>
                </>
              )}

              {selectedType === 'gcs' && (
                <>
                  <div className='grid gap-2'>
                    <Label htmlFor='edit-gcs-bucket'>{t('dataStorages.fields.gcsBucketName')}</Label>
                    <Input
                      id='edit-gcs-bucket'
                      {...register('gcsBucketName', {
                        validate: (value) => {
                          if (watch('type') === 'gcs' && !value) {
                            return t('dataStorages.validation.gcsBucketRequired');
                          }
                          return true;
                        },
                      })}
                      placeholder='my-bucket'
                    />
                    {errors.gcsBucketName && <span className='text-sm text-red-500'>{errors.gcsBucketName.message}</span>}
                  </div>
                  <div className='grid gap-2'>
                    <Label htmlFor='edit-gcs-credential'>{t('dataStorages.fields.gcsCredential')}</Label>
                    <Textarea
                      id='edit-gcs-credential'
                      {...register('gcsCredential', {
                        validate: (value) => {
                          // Only required for new data storage (no editingDataStorage)
                          // For updates, empty value means keep current value
                          if (watch('type') === 'gcs') {
                            const trimmedValue = value?.trim() ?? '';

                            if (!editingDataStorage && !trimmedValue) {
                              return t('dataStorages.validation.gcsCredentialRequired');
                            }

                            if (trimmedValue) {
                              try {
                                const parsed = JSON.parse(trimmedValue);
                                if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
                                  return t('dataStorages.validation.gcsCredentialInvalid');
                                }
                              } catch (_error) {
                                return t('dataStorages.validation.gcsCredentialInvalid');
                              }
                            }
                          }
                          return true;
                        },
                      })}
                      className='max-h-48 overflow-auto'
                      rows={5}
                      placeholder={t('dataStorages.dialogs.fields.gcsCredential.editPlaceholder')}
                    />
                    {errors.gcsCredential && <span className='text-sm text-red-500'>{errors.gcsCredential.message}</span>}
                  </div>
                </>
              )}
            </div>
            <DialogFooter>
              <Button
                type='button'
                variant='outline'
                onClick={() => {
                  setIsEditDialogOpen(false);
                  setEditingDataStorage(null);
                }}
              >
                {t('common.buttons.cancel')}
              </Button>
              <Button
                type='submit'
                disabled={updateMutation.isPending}
              >
                {updateMutation.isPending ? t('common.buttons.saving') : t('common.buttons.save')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
      <Dialog open={isArchiveDialogOpen} onOpenChange={setIsArchiveDialogOpen}>
        <DialogContent className='sm:max-w-[480px]'>
          <DialogHeader>
            <DialogTitle>{t('dataStorages.dialogs.status.archiveTitle')}</DialogTitle>
            <DialogDescription>
              {t('dataStorages.dialogs.status.archiveDescription', {
                name: archiveDataStorage?.name ?? '',
              })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button type='button' variant='outline' onClick={resetArchiveContext}>
              {t('common.buttons.cancel')}
            </Button>
            <Button
              type='button'
              variant='destructive'
              disabled={archiveMutation.isPending}
              onClick={async () => {
                if (!archiveDataStorage) return;
                try {
                  await archiveMutation.mutateAsync(archiveDataStorage.id);
                  resetArchiveContext();
                } catch (_error) {
                  // handled in mutation
                }
              }}
            >
              {archiveMutation.isPending ? t('common.buttons.archiving') : t('common.buttons.archive')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
