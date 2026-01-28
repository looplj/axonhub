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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { useDataStoragesContext } from '../context/data-storages-context';
import { useCreateDataStorage, CreateDataStorageInput } from '../data/data-storages';
import { DataStorageFormData } from './types';

export function CreateDataStorageDialog() {
  const { t } = useTranslation();
  const { isCreateDialogOpen, setIsCreateDialogOpen } = useDataStoragesContext();
  const createMutation = useCreateDataStorage();

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
        webdavURL: '',
        webdavUsername: '',
        webdavPassword: '',
        webdavPath: '',
        webdavInsecureSkipTLS: false,
      });
    }
  }, [isCreateDialogOpen, reset]);

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
        webdav:
          data.type === 'webdav'
            ? {
                url: data.webdavURL,
                username: data.webdavUsername,
                password: data.webdavPassword,
                path: data.webdavPath,
                insecure_skip_tls: data.webdavInsecureSkipTLS,
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

  return (
    <Dialog open={isCreateDialogOpen} onOpenChange={setIsCreateDialogOpen}>
      <DialogContent className='sm:max-w-[700px]'>
        <DialogHeader>
          <DialogTitle>{t('dataStorages.dialogs.create.title')}</DialogTitle>
          <DialogDescription>{t('dataStorages.dialogs.create.description')}</DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit(onCreateSubmit, () => {})} noValidate>
          <div className='grid max-h-[85vh] gap-4 overflow-y-auto py-4'>
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
              <Select
                value={selectedType}
                onValueChange={(value) => setValue('type', value as DataStorageFormData['type'])}
              >
                <SelectTrigger id='create-type'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='fs'>{t('dataStorages.types.fs')}</SelectItem>
                  <SelectItem value='s3'>{t('dataStorages.types.s3')}</SelectItem>
                  <SelectItem value='gcs'>{t('dataStorages.types.gcs')}</SelectItem>
                  <SelectItem value='webdav'>{t('dataStorages.types.webdav')}</SelectItem>
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
                  {errors.s3BucketName && (
                    <span className='text-sm text-red-500'>{errors.s3BucketName.message}</span>
                  )}
                </div>
                <div className='grid gap-2'>
                  <Label htmlFor='create-s3-endpoint'>{t('dataStorages.fields.s3Endpoint')}</Label>
                  <Input
                    id='create-s3-endpoint'
                    {...register('s3Endpoint')}
                    placeholder='https://s3.amazonaws.com'
                  />
                </div>
                <div className='grid gap-2'>
                  <Label htmlFor='create-s3-region'>{t('dataStorages.fields.s3Region')}</Label>
                  <Input
                    id='create-s3-region'
                    {...register('s3Region')}
                    placeholder='us-east-1'
                  />
                </div>
                <div className='grid gap-2'>
                  <Label htmlFor='create-s3-access-key'>
                    {t('dataStorages.fields.s3AccessKey')} *
                  </Label>
                  <Input
                    id='create-s3-access-key'
                    {...register('s3AccessKey', {
                      validate: (value) => {
                        if (watch('type') === 's3' && !value) {
                          return t('dataStorages.validation.s3AccessKeyRequired');
                        }
                        return true;
                      },
                    })}
                  />
                  {errors.s3AccessKey && (
                    <span className='text-sm text-red-500'>{errors.s3AccessKey.message}</span>
                  )}
                </div>
                <div className='grid gap-2'>
                  <Label htmlFor='create-s3-secret-key'>
                    {t('dataStorages.fields.s3SecretKey')} *
                  </Label>
                  <Input
                    id='create-s3-secret-key'
                    type='password'
                    {...register('s3SecretKey', {
                      validate: (value) => {
                        if (watch('type') === 's3' && !value) {
                          return t('dataStorages.validation.s3SecretKeyRequired');
                        }
                        return true;
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
                  {errors.gcsBucketName && (
                    <span className='text-sm text-red-500'>{errors.gcsBucketName.message}</span>
                  )}
                </div>
                <div className='grid gap-2'>
                  <Label htmlFor='create-gcs-credential'>
                    {t('dataStorages.fields.gcsCredential')} *
                  </Label>
                  <Textarea
                    id='create-gcs-credential'
                    {...register('gcsCredential', {
                      validate: (value) => {
                        if (watch('type') === 'gcs') {
                          const trimmedValue = value?.trim() ?? '';
                          if (!trimmedValue) {
                            return t('dataStorages.validation.gcsCredentialRequired');
                          }
                          try {
                            const parsed = JSON.parse(trimmedValue);
                            if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
                              return t('dataStorages.validation.gcsCredentialInvalid');
                            }
                          } catch (_error) {
                            return t('dataStorages.validation.gcsCredentialInvalid');
                          }
                        }
                        return true;
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

            {selectedType === 'webdav' && (
              <>
                <div className='grid gap-2'>
                  <Label htmlFor='create-webdav-url'>{t('dataStorages.fields.webdavURL')}</Label>
                  <Input
                    id='create-webdav-url'
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
                  <Label htmlFor='create-webdav-username'>
                    {t('dataStorages.fields.webdavUsername')}
                  </Label>
                  <Input
                    id='create-webdav-username'
                    {...register('webdavUsername')}
                    placeholder='username'
                  />
                </div>
                <div className='grid gap-2'>
                  <Label htmlFor='create-webdav-password'>
                    {t('dataStorages.fields.webdavPassword')} *
                  </Label>
                  <Input
                    id='create-webdav-password'
                    type='password'
                    {...register('webdavPassword', {
                      validate: (value) => {
                        if (watch('type') === 'webdav' && !value) {
                          return t('dataStorages.validation.webdavPasswordRequired');
                        }
                        return true;
                      },
                    })}
                  />
                  {errors.webdavPassword && (
                    <span className='text-sm text-red-500'>{errors.webdavPassword.message}</span>
                  )}
                </div>
                <div className='grid gap-2'>
                  <Label htmlFor='create-webdav-path'>{t('dataStorages.fields.webdavPath')}</Label>
                  <Input
                    id='create-webdav-path'
                    {...register('webdavPath')}
                    placeholder='/remote.php/dav/files/user/'
                  />
                </div>
                <div className='flex items-center space-x-2'>
                  <input
                    type='checkbox'
                    id='create-webdav-insecure'
                    {...register('webdavInsecureSkipTLS')}
                    className='h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-600'
                  />
                  <Label htmlFor='create-webdav-insecure'>
                    {t('dataStorages.fields.webdavInsecureSkipTLS')}
                  </Label>
                </div>
              </>
            )}
          </div>
          <DialogFooter>
            <Button type='button' variant='outline' onClick={() => setIsCreateDialogOpen(false)}>
              {t('common.buttons.cancel')}
            </Button>
            <Button type='submit' disabled={createMutation.isPending}>
              {createMutation.isPending ? t('common.buttons.creating') : t('common.buttons.create')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
