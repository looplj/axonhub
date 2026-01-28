'use client';

import { useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { useDataStoragesContext } from '../context/data-storages-context';
import { useUpdateDataStorage, UpdateDataStorageInput } from '../data/data-storages';
import { DataStorageFormData } from './types';

export function EditDataStorageDialog() {
  const { t } = useTranslation();
  const {
    isEditDialogOpen,
    setIsEditDialogOpen,
    editingDataStorage,
    setEditingDataStorage,
  } = useDataStoragesContext();
  const updateMutation = useUpdateDataStorage();

  const {
    register,
    handleSubmit,
    reset,
    watch,
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
      webdavURL: '',
      webdavUsername: '',
      webdavPassword: '',
      webdavPath: '',
      webdavInsecureSkipTLS: false,
    },
  });

  const selectedType = watch('type');

  // Clear errors for fields that are not relevant to the current type
  useEffect(() => {
    if (selectedType === 'fs') {
      clearErrors(['s3BucketName', 's3Endpoint', 's3AccessKey', 's3SecretKey']);
      clearErrors(['gcsBucketName', 'gcsCredential']);
      clearErrors(['webdavURL', 'webdavUsername', 'webdavPassword', 'webdavPath']);
    } else if (selectedType === 's3') {
      clearErrors(['directory']);
      clearErrors(['gcsBucketName', 'gcsCredential']);
      clearErrors(['webdavURL', 'webdavUsername', 'webdavPassword', 'webdavPath']);
    } else if (selectedType === 'gcs') {
      clearErrors(['directory']);
      clearErrors(['s3BucketName', 's3Endpoint', 's3AccessKey', 's3SecretKey']);
      clearErrors(['webdavURL', 'webdavUsername', 'webdavPassword', 'webdavPath']);
    } else if (selectedType === 'webdav') {
      clearErrors(['directory']);
      clearErrors(['s3BucketName', 's3Endpoint', 's3AccessKey', 's3SecretKey']);
      clearErrors(['gcsBucketName', 'gcsCredential']);
    }
  }, [selectedType, clearErrors]);

  // Reset form when dialog opens
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
        webdavURL: editingDataStorage.settings.webdav?.url || '',
        webdavUsername: editingDataStorage.settings.webdav?.username || '',
        webdavPassword: editingDataStorage.settings.webdav?.password || '',
        webdavPath: editingDataStorage.settings.webdav?.path || '',
        webdavInsecureSkipTLS: editingDataStorage.settings.webdav?.insecure_skip_tls || false,
      });
    }
  }, [isEditDialogOpen, editingDataStorage, reset]);

  const onEditSubmit = async (data: DataStorageFormData) => {
    if (!editingDataStorage) {
      return;
    }

    // Build settings, only including non-empty values
    const settings: any = {};
    if (data.type === 'fs' && data.directory) {
      settings.directory = data.directory;
    } else if (data.type === 's3') {
      const s3Data: any = {};
      if (data.s3BucketName) s3Data.bucketName = data.s3BucketName;
      if (data.s3Endpoint) s3Data.endpoint = data.s3Endpoint;
      if (data.s3Region) s3Data.region = data.s3Region;
      if (data.s3AccessKey) s3Data.accessKey = data.s3AccessKey;
      if (data.s3SecretKey) s3Data.secretKey = data.s3SecretKey;

      if (Object.keys(s3Data).length > 0) {
        settings.s3 = s3Data;
      }
    } else if (data.type === 'gcs') {
      const gcsData: any = {};
      if (data.gcsBucketName) gcsData.bucketName = data.gcsBucketName;
      if (data.gcsCredential) gcsData.credential = data.gcsCredential;

      if (Object.keys(gcsData).length > 0) {
        settings.gcs = gcsData;
      }
    } else if (data.type === 'webdav') {
      const webdavData: any = {};
      if (data.webdavURL) webdavData.url = data.webdavURL;
      if (data.webdavUsername) webdavData.username = data.webdavUsername;
      if (data.webdavPassword) webdavData.password = data.webdavPassword;
      if (data.webdavPath) webdavData.path = data.webdavPath;
      webdavData.insecure_skip_tls = data.webdavInsecureSkipTLS;

      if (Object.keys(webdavData).length > 0) {
        settings.webdav = webdavData;
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
    <Dialog
      open={isEditDialogOpen}
      onOpenChange={(open) => {
        setIsEditDialogOpen(open);
        if (!open) setEditingDataStorage(null);
      }}
    >
      <DialogContent className='sm:max-w-[700px]'>
        <DialogHeader>
          <DialogTitle>{t('dataStorages.dialogs.edit.title')}</DialogTitle>
          <DialogDescription>{t('dataStorages.dialogs.edit.description')}</DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit(onEditSubmit, () => {})} noValidate>
          <div className='grid max-h-[85vh] gap-4 overflow-y-auto py-4'>
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
                      if (watch('type') === 'fs' && !value) {
                        return t('dataStorages.validation.directoryRequired');
                      }
                      return true;
                    },
                  })}
                  placeholder='/var/axonhub/data'
                />
                {errors.directory && (
                  <span className='text-sm text-red-500'>{errors.directory.message}</span>
                )}
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
                  {errors.s3BucketName && (
                    <span className='text-sm text-red-500'>{errors.s3BucketName.message}</span>
                  )}
                </div>
                <div className='grid gap-2'>
                  <Label htmlFor='edit-s3-endpoint'>{t('dataStorages.fields.s3Endpoint')}</Label>
                  <Input
                    id='edit-s3-endpoint'
                    {...register('s3Endpoint')}
                    placeholder='https://s3.amazonaws.com'
                  />
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
                        if (watch('type') === 's3' && !value && !editingDataStorage) {
                          return t('dataStorages.validation.s3AccessKeyRequired');
                        }
                        return true;
                      },
                    })}
                    placeholder={t('dataStorages.dialogs.fields.s3AccessKey.editPlaceholder')}
                  />
                  {errors.s3AccessKey && (
                    <span className='text-sm text-red-500'>{errors.s3AccessKey.message}</span>
                  )}
                </div>
                <div className='grid gap-2'>
                  <Label htmlFor='edit-s3-secret-key'>{t('dataStorages.fields.s3SecretKey')}</Label>
                  <Input
                    id='edit-s3-secret-key'
                    type='password'
                    {...register('s3SecretKey', {
                      validate: (value) => {
                        if (watch('type') === 's3' && !value && !editingDataStorage) {
                          return t('dataStorages.validation.s3SecretKeyRequired');
                        }
                        return true;
                      },
                    })}
                    placeholder={t('dataStorages.dialogs.fields.s3SecretKey.editPlaceholder')}
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
                  {errors.gcsBucketName && (
                    <span className='text-sm text-red-500'>{errors.gcsBucketName.message}</span>
                  )}
                </div>
                <div className='grid gap-2'>
                  <Label htmlFor='edit-gcs-credential'>{t('dataStorages.fields.gcsCredential')}</Label>
                  <Textarea
                    id='edit-gcs-credential'
                    {...register('gcsCredential', {
                      validate: (value) => {
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
                  {errors.gcsCredential && (
                    <span className='text-sm text-red-500'>{errors.gcsCredential.message}</span>
                  )}
                </div>
              </>
            )}

            {selectedType === 'webdav' && (
              <>
                <div className='grid gap-2'>
                  <Label htmlFor='edit-webdav-url'>{t('dataStorages.fields.webdavURL')}</Label>
                  <Input
                    id='edit-webdav-url'
                    {...register('webdavURL', {
                      validate: (value) => {
                        if (watch('type') === 'webdav' && !value) {
                          return t('dataStorages.validation.webdavURLRequired');
                        }
                        return true;
                      },
                    })}
                    placeholder='https://webdav.example.com'
                  />
                  {errors.webdavURL && (
                    <span className='text-sm text-red-500'>{errors.webdavURL.message}</span>
                  )}
                </div>
                <div className='grid gap-2'>
                  <Label htmlFor='edit-webdav-username'>
                    {t('dataStorages.fields.webdavUsername')}
                  </Label>
                  <Input
                    id='edit-webdav-username'
                    {...register('webdavUsername')}
                    placeholder='username'
                  />
                </div>
                <div className='grid gap-2'>
                  <Label htmlFor='edit-webdav-password'>
                    {t('dataStorages.fields.webdavPassword')}
                  </Label>
                  <Input
                    id='edit-webdav-password'
                    type='password'
                    {...register('webdavPassword', {
                      validate: (value) => {
                        if (watch('type') === 'webdav' && !value && !editingDataStorage) {
                          return t('dataStorages.validation.webdavPasswordRequired');
                        }
                        return true;
                      },
                    })}
                    placeholder={t('dataStorages.dialogs.fields.webdavPassword.editPlaceholder')}
                  />
                  {errors.webdavPassword && (
                    <span className='text-sm text-red-500'>{errors.webdavPassword.message}</span>
                  )}
                </div>
                <div className='grid gap-2'>
                  <Label htmlFor='edit-webdav-path'>{t('dataStorages.fields.webdavPath')}</Label>
                  <Input
                    id='edit-webdav-path'
                    {...register('webdavPath')}
                    placeholder='/remote.php/dav/files/user/'
                  />
                </div>
                <div className='flex items-center space-x-2'>
                  <input
                    type='checkbox'
                    id='edit-webdav-insecure'
                    {...register('webdavInsecureSkipTLS')}
                    className='h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-600'
                  />
                  <Label htmlFor='edit-webdav-insecure'>
                    {t('dataStorages.fields.webdavInsecureSkipTLS')}
                  </Label>
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
            <Button type='submit' disabled={updateMutation.isPending}>
              {updateMutation.isPending ? t('common.buttons.saving') : t('common.buttons.save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
