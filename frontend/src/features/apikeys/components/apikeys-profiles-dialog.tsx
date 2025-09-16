import { useEffect, useState } from 'react'
import { useForm, useFieldArray } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { IconPlus, IconTrash, IconSettings } from '@tabler/icons-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { AutoComplete } from '@/components/auto-complete'
import { defaultModels } from '../../channels/components/channels-action-dialog'
import { useApiKeysContext } from '../context/apikeys-context'
import {
  updateApiKeyProfilesInputSchemaFactory,
  type UpdateApiKeyProfilesInput,
  type ApiKeyProfile,
} from '../data/schema'

interface ApiKeyProfilesDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (data: UpdateApiKeyProfilesInput) => void
  loading?: boolean
  initialData?: {
    activeProfile: string
    profiles: ApiKeyProfile[]
  }
}

export function ApiKeyProfilesDialog({
  open,
  onOpenChange,
  onSubmit,
  loading = false,
  initialData,
}: ApiKeyProfilesDialogProps) {
  const { t } = useTranslation()
  const { selectedApiKey } = useApiKeysContext()

  const form = useForm<UpdateApiKeyProfilesInput>({
    resolver: zodResolver(updateApiKeyProfilesInputSchemaFactory(t)),
    defaultValues: {
      activeProfile: '',
      profiles: [
        {
          name: 'Default',
          modelMappings: [],
        },
      ],
    },
  })

  const {
    fields: profileFields,
    append: appendProfile,
    remove: removeProfile,
  } = useFieldArray({
    control: form.control,
    name: 'profiles',
  })

  // Watch profile names to update activeProfile dropdown options
  const profileNames = form.watch('profiles')?.map((profile) => profile.name) || []

  // Reset form when dialog opens/closes or initial data changes
  useEffect(() => {
    if (open && initialData) {
      form.reset(initialData)
    } else if (open) {
      form.reset({
        activeProfile: 'Default',
        profiles: [
          {
            name: 'Default',
            modelMappings: [],
          },
        ],
      })
    }
  }, [open, initialData, form])

  const handleSubmit = (data: UpdateApiKeyProfilesInput) => {
    onSubmit(data)
  }

  const addProfile = () => {
    appendProfile({
      name: `Profile ${profileFields.length + 1}`,
      modelMappings: [],
    })
  }

  const removeProfileHandler = (index: number) => {
    if (profileFields.length > 1) {
      removeProfile(index)
      // If we're removing the active profile, set active to the first remaining profile
      const currentActiveProfile = form.getValues('activeProfile')
      const removedProfile = profileFields[index]
      if (currentActiveProfile === removedProfile.name) {
        const remainingProfiles = profileFields.filter((_, i) => i !== index)
        if (remainingProfiles.length > 0) {
          form.setValue('activeProfile', remainingProfiles[0].name)
        }
      }
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[90vh] sm:max-w-4xl'>
        <DialogHeader className='text-left'>
          <DialogTitle className='flex items-center gap-2'>
            <IconSettings className='h-5 w-5' />
            {t('apikeys.profiles.title')}
          </DialogTitle>
          <DialogDescription>
            {t('apikeys.profiles.description', {
              name: selectedApiKey?.name,
            })}
          </DialogDescription>
        </DialogHeader>
        <div className='flex h-[36rem] flex-col'>
          {/* Fixed Add Profile Section at Top */}
          <div className='bg-background border-b p-4'>
            <Form {...form}>
              <form id='apikey-profiles-form' onSubmit={form.handleSubmit(handleSubmit)} className='space-y-6'>
                <div className='flex items-center justify-between'>
                  <h3 className='text-lg font-medium'>{t('apikeys.profiles.profilesTitle')}</h3>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={addProfile}
                    className='flex items-center gap-2'
                  >
                    <IconPlus className='h-4 w-4' />
                    {t('apikeys.profiles.addProfile')}
                  </Button>
                </div>
              </form>
            </Form>
          </div>

          {/* Scrollable Profiles Section */}
          <div className='-mr-4 flex-1 overflow-y-auto py-1 pr-4'>
            <Form {...form}>
              <form onSubmit={form.handleSubmit(handleSubmit)} className='space-y-6'>
                <div className='space-y-4'>
                  <div className='space-y-4'>
                    {profileFields.map((profile, profileIndex) => (
                      <ProfileCard
                        key={profile.id}
                        profileIndex={profileIndex}
                        form={form}
                        onRemove={() => removeProfileHandler(profileIndex)}
                        canRemove={profileFields.length > 1}
                        t={t}
                      />
                    ))}
                  </div>
                </div>
              </form>
            </Form>
          </div>

          {/* Fixed Active Profile Section at Bottom */}
          <div className='bg-background border-t p-4'>
            <Form {...form}>
              <Separator className='mb-4' />
              <FormField
                control={form.control}
                name='activeProfile'
                render={({ field }) => (
                  <FormItem className='grid grid-cols-8 items-start space-y-0 gap-x-6 gap-y-1'>
                    <FormLabel className='col-span-2 pt-2 text-right font-medium'>
                      {t('apikeys.profiles.activeProfile')}
                    </FormLabel>
                    <FormControl className='col-span-6'>
                      <Select onValueChange={field.onChange} value={field.value}>
                        <SelectTrigger>
                          <SelectValue placeholder={t('apikeys.profiles.selectActiveProfile')} />
                        </SelectTrigger>
                        <SelectContent>
                          {profileNames
                            .filter((name) => name.trim() !== '')
                            .map((profileName) => (
                              <SelectItem key={profileName} value={profileName}>
                                {profileName}
                              </SelectItem>
                            ))}
                        </SelectContent>
                      </Select>
                    </FormControl>
                    <div className='col-span-6 col-start-3 min-h-[1.25rem]'>
                      <FormMessage />
                    </div>
                  </FormItem>
                )}
              />
            </Form>
          </div>
        </div>

        <DialogFooter>
          <Button type='button' variant='outline' onClick={() => onOpenChange(false)} disabled={loading}>
            {t('common.cancel')}
          </Button>
          <Button type='submit' form='apikey-profiles-form' disabled={loading}>
            {loading ? t('common.saving') : t('common.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

interface ProfileCardProps {
  profileIndex: number
  form: any
  onRemove: () => void
  canRemove: boolean
  t: (key: string) => string
}

function ProfileCard({ profileIndex, form, onRemove, canRemove, t }: ProfileCardProps) {
  const {
    fields: mappingFields,
    append: appendMapping,
    remove: removeMapping,
  } = useFieldArray({
    control: form.control,
    name: `profiles.${profileIndex}.modelMappings`,
  })

  const addMapping = () => {
    appendMapping({ from: '', to: '' })
  }

  // Get all available models from all providers
  const getAllAvailableModels = () => {
    const allModels = new Set<string>()
    Object.values(defaultModels).forEach((models) => {
      models.forEach((model) => allModels.add(model))
    })
    return Array.from(allModels).sort()
  }

  const availableModels = getAllAvailableModels()

  return (
    <Card>
      <CardHeader className='pb-3'>
        <div className='flex items-center justify-between'>
          <CardTitle className='text-base'>
            <FormField
              control={form.control}
              name={`profiles.${profileIndex}.name`}
              render={({ field }) => (
                <FormItem>
                  <FormControl>
                    <Input {...field} placeholder={t('apikeys.profiles.profileName')} className='font-medium' />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </CardTitle>
          {canRemove && (
            <Button
              type='button'
              variant='ghost'
              size='sm'
              onClick={onRemove}
              className='text-destructive hover:text-destructive'
            >
              <IconTrash className='h-4 w-4' />
            </Button>
          )}
        </div>
      </CardHeader>
      <CardContent className='space-y-4'>
        <div className='flex items-center justify-between'>
          <h4 className='text-sm font-medium'>{t('apikeys.profiles.modelMappings')}</h4>
          <Button type='button' variant='outline' size='sm' onClick={addMapping} className='flex items-center gap-2'>
            <IconPlus className='h-4 w-4' />
            {t('apikeys.profiles.addMapping')}
          </Button>
        </div>

        {mappingFields.length === 0 && (
          <p className='text-muted-foreground py-4 text-center text-sm'>{t('apikeys.profiles.noMappings')}</p>
        )}

        <div className='space-y-3'>
          {mappingFields.map((mapping, mappingIndex) => (
            <MappingRow
              key={mapping.id}
              profileIndex={profileIndex}
              mappingIndex={mappingIndex}
              form={form}
              onRemove={() => removeMapping(mappingIndex)}
              availableModels={availableModels}
              t={t}
            />
          ))}
        </div>
      </CardContent>
    </Card>
  )
}

interface MappingRowProps {
  profileIndex: number
  mappingIndex: number
  form: any
  onRemove: () => void
  availableModels: string[]
  t: (key: string) => string
}

function MappingRow({ profileIndex, mappingIndex, form, onRemove, availableModels, t }: MappingRowProps) {
  const [fromSearch, setFromSearch] = useState('')
  const [toSearch, setToSearch] = useState('')

  // Filter models based on search
  const filteredFromModels = availableModels
    .filter((model) => model.toLowerCase().includes(fromSearch.toLowerCase()))
    .map((model) => ({ value: model, label: model }))

  const filteredToModels = availableModels
    .filter((model) => model.toLowerCase().includes(toSearch.toLowerCase()))
    .map((model) => ({ value: model, label: model }))

  return (
    <div className='flex items-start gap-3'>
      <FormField
        control={form.control}
        name={`profiles.${profileIndex}.modelMappings.${mappingIndex}.from`}
        render={({ field }) => (
          <FormItem className='flex-1'>
            <FormControl>
              <AutoComplete
                selectedValue={field.value || ''}
                onSelectedValueChange={(value) => {
                  field.onChange(value)
                  setFromSearch(value)
                }}
                searchValue={fromSearch || field.value || ''}
                onSearchValueChange={setFromSearch}
                items={filteredFromModels}
                placeholder={t('apikeys.profiles.sourceModel')}
                emptyMessage={t('apikeys.profiles.noModelsFound')}
              />
            </FormControl>
            <div className='text-muted-foreground mt-1 text-xs'>{t('apikeys.profiles.regexSupported')}</div>
            <FormMessage />
          </FormItem>
        )}
      />
      <span className='text-muted-foreground'>→</span>
      <FormField
        control={form.control}
        name={`profiles.${profileIndex}.modelMappings.${mappingIndex}.to`}
        render={({ field }) => (
          <FormItem className='flex-1'>
            <FormControl>
              <AutoComplete
                selectedValue={field.value || ''}
                onSelectedValueChange={(value) => {
                  field.onChange(value)
                  setToSearch(value)
                }}
                searchValue={toSearch || field.value || ''}
                onSearchValueChange={setToSearch}
                items={filteredToModels}
                placeholder={t('apikeys.profiles.targetModel')}
                emptyMessage={t('apikeys.profiles.noModelsFound')}
              />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />
      <Button
        type='button'
        variant='ghost'
        size='sm'
        onClick={onRemove}
        className='text-destructive hover:text-destructive'
      >
        <IconTrash className='h-4 w-4' />
      </Button>
    </div>
  )
}
