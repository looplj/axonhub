import { useCallback, useEffect, useMemo, useRef } from 'react';
import { useForm, useFieldArray } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { IconPlus, IconTrash } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { extractNumberID } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { TagsAutocompleteInput } from '@/components/ui/tags-autocomplete-input';
import { useAllChannelSummarys } from '@/features/channels/data/channels';
import { updateProjectProfilesInputSchemaFactory, type ProjectProfile, type UpdateProjectProfilesInput } from '../data/schema';

interface ProjectProfilesDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (data: UpdateProjectProfilesInput) => void;
  loading?: boolean;
  initialData?: {
    activeProfile: string;
    profiles: ProjectProfile[];
  };
}

export function ProjectProfilesDialog({ open, onOpenChange, onSubmit, loading = false, initialData }: ProjectProfilesDialogProps) {
  const { t } = useTranslation();
  const { data: channelsData } = useAllChannelSummarys();

  const allTags = useMemo(() => {
    if (!channelsData?.edges) return [];
    const tagSet = new Set<string>();
    channelsData.edges.forEach((edge) => {
      edge.node.tags?.forEach((tag: string) => tagSet.add(tag));
    });
    return Array.from(tagSet).sort();
  }, [channelsData]);

  const defaultValues = useMemo(
    () => ({
      activeProfile: '',
      profiles: [] as ProjectProfile[],
    }),
    []
  );

  const form = useForm<UpdateProjectProfilesInput>({
    resolver: zodResolver(updateProjectProfilesInputSchemaFactory(t)),
    defaultValues,
  });

  const lastInitialDataRef = useRef<string | null>(null);

  useEffect(() => {
    if (!open) {
      lastInitialDataRef.current = null;
      return;
    }

    const dataKey = JSON.stringify(initialData);
    if (dataKey === lastInitialDataRef.current) return;
    lastInitialDataRef.current = dataKey;

    if (initialData && initialData.profiles.length > 0) {
      form.reset({
        activeProfile: initialData.activeProfile || '',
        profiles: initialData.profiles.map((p) => ({
          name: p.name || '',
          channelIDs: p.channelIDs || [],
          channelTags: p.channelTags || [],
          channelTagsMatchMode: p.channelTagsMatchMode || 'any',
        })),
      });
    } else {
      form.reset(defaultValues);
    }
  }, [open, initialData, form, defaultValues]);

  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: 'profiles',
  });

  const watchedProfiles = form.watch('profiles');
  const activeProfile = form.watch('activeProfile');

  const handleAddProfile = useCallback(() => {
    const newName = `Profile ${fields.length + 1}`;
    append({
      name: newName,
      channelIDs: [],
      channelTags: [],
      channelTagsMatchMode: 'any',
    });
    if (!activeProfile) {
      form.setValue('activeProfile', newName);
    }
  }, [fields.length, append, activeProfile, form]);

  const handleRemoveProfile = useCallback(
    (index: number) => {
      const removedName = watchedProfiles[index]?.name;
      remove(index);
      if (removedName === activeProfile) {
        const remaining = watchedProfiles.filter((_, i) => i !== index);
        form.setValue('activeProfile', remaining[0]?.name || '');
      }
    },
    [watchedProfiles, activeProfile, form, remove]
  );

  const handleSubmit = useCallback(
    (data: UpdateProjectProfilesInput) => {
      onSubmit(data);
    },
    [onSubmit]
  );

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='flex max-h-[90vh] flex-col sm:max-w-4xl'>
        <DialogHeader className='shrink-0 text-left'>
          <DialogTitle>{t('projects.profiles.title')}</DialogTitle>
          <DialogDescription>{t('projects.profiles.description')}</DialogDescription>
        </DialogHeader>

        <div className='flex min-h-0 flex-1 flex-col'>
          {/* Fixed Add Profile Section at Top */}
          <div className='bg-background shrink-0 border-b p-4'>
            <Form {...form}>
              <form id='project-profiles-form' onSubmit={form.handleSubmit(handleSubmit)} className='space-y-6'>
                <div className='flex items-center justify-between'>
                  <h3 className='text-lg font-medium'>{t('projects.profiles.profilesTitle')}</h3>
                  <Button type='button' variant='outline' size='sm' onClick={handleAddProfile} className='flex items-center gap-2'>
                    <IconPlus className='h-4 w-4' />
                    {t('projects.profiles.addProfile')}
                  </Button>
                </div>
              </form>
            </Form>
          </div>

          {/* Scrollable Profiles Section */}
          {fields.length > 0 && (
            <div className='flex-1 overflow-y-auto py-1'>
              <Form {...form}>
                <form onSubmit={form.handleSubmit(handleSubmit)} className='space-y-6 px-4'>
                  <div className='space-y-4'>
                    {fields.map((field, profileIndex) => {
                      const isExcludeMode = form.watch(`profiles.${profileIndex}.channelTagsMatchMode`) === 'none';

                      return (
              <Card key={field.id}>
                <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
                  <CardTitle className='text-base'>
                    {watchedProfiles[profileIndex]?.name || `Profile ${profileIndex + 1}`}
                    {watchedProfiles[profileIndex]?.name === activeProfile && (
                      <span className='bg-primary/10 text-primary ml-2 rounded-full px-2 py-0.5 text-xs'>
                        {t('projects.profiles.activeProfile')}
                      </span>
                    )}
                  </CardTitle>
                  <Button type='button' variant='ghost' size='icon' onClick={() => handleRemoveProfile(profileIndex)}>
                    <IconTrash className='h-4 w-4' />
                  </Button>
                </CardHeader>
                <CardContent className='space-y-6'>
                  {/* Profile Name */}
                  <FormField
                    control={form.control}
                    name={`profiles.${profileIndex}.name`}
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('projects.profiles.profileName')}</FormLabel>
                        <FormControl>
                          <Input {...field} placeholder={t('projects.profiles.profileNamePlaceholder')} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  {/* Channel IDs */}
                  <div className='border-t pt-6'>
                    <h4 className='mb-3 text-sm font-medium'>{t('projects.profiles.allowedChannels')}</h4>
                    <p className='text-muted-foreground mb-3 text-xs'>{t('projects.profiles.allowedChannelsDescription')}</p>
                    <FormField
                      control={form.control}
                      name={`profiles.${profileIndex}.channelIDs`}
                      render={({ field }) => (
                        <FormItem>
                          <FormControl>
                            <TagsAutocompleteInput
                              value={(field.value || []).map((id) => {
                                const channel = channelsData?.edges?.find((edge) => parseInt(extractNumberID(edge.node.id), 10) === id);
                                return channel?.node.name || id.toString();
                              })}
                              onChange={(tags) => {
                                const ids = tags
                                  .map((tag) => {
                                    const channel = channelsData?.edges?.find((edge) => edge.node.name === tag);
                                    return channel ? parseInt(extractNumberID(channel.node.id), 10) : parseInt(tag);
                                  })
                                  .filter((id) => !isNaN(id));
                                field.onChange(ids);
                              }}
                              placeholder={t('projects.profiles.allowedChannels')}
                              suggestions={channelsData?.edges?.map((edge) => edge.node.name) || []}
                              className='h-auto min-h-9 py-1'
                            />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>

                  {/* Channel Tags */}
                  <div className='border-t pt-6'>
                    <div className='mb-3 flex items-start justify-between gap-3'>
                      <div>
                        <h4 className='text-sm font-medium'>
                          {t(isExcludeMode ? 'projects.profiles.excludedChannelTags' : 'projects.profiles.allowedChannelTags')}
                        </h4>
                        <p className='text-muted-foreground mt-1 text-xs'>
                          {t(isExcludeMode ? 'projects.profiles.excludedChannelTagsDescription' : 'projects.profiles.allowedChannelTagsDescription')}
                        </p>
                      </div>
                      <FormField
                        control={form.control}
                        name={`profiles.${profileIndex}.channelTagsMatchMode`}
                        render={({ field }) => (
                          <FormItem className='w-[180px]'>
                            <FormLabel>{t('projects.profiles.allowedChannelTagsMatchMode')}</FormLabel>
                            <FormControl>
                              <Select value={field.value || 'any'} onValueChange={field.onChange}>
                                <SelectTrigger>
                                  <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                  <SelectItem value='any'>{t('projects.profiles.allowedChannelTagsMatchModeAny')}</SelectItem>
                                  <SelectItem value='all'>{t('projects.profiles.allowedChannelTagsMatchModeAll')}</SelectItem>
                                  <SelectItem value='none'>{t('projects.profiles.allowedChannelTagsMatchModeNone')}</SelectItem>
                                </SelectContent>
                              </Select>
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                    </div>
                    <FormField
                      control={form.control}
                      name={`profiles.${profileIndex}.channelTags`}
                      render={({ field }) => (
                        <FormItem>
                          <FormControl>
                            <TagsAutocompleteInput
                              value={field.value || []}
                              onChange={field.onChange}
                              placeholder={t(isExcludeMode ? 'projects.profiles.excludedChannelTags' : 'projects.profiles.allowedChannelTags')}
                              suggestions={allTags}
                              className='h-auto min-h-9 py-1'
                            />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>
                </CardContent>
              </Card>
                      );
                    })}
                  </div>
                </form>
              </Form>
            </div>
          )}

          {/* Fixed Active Profile Section at Bottom */}
          <div className='bg-background mt-4 shrink-0 border-t px-4 py-2'>
            <Form {...form}>
              <FormField
                control={form.control}
                name='activeProfile'
                render={({ field }) => (
                  <FormItem className='flex items-center space-y-0 gap-x-3'>
                    <FormLabel className='shrink-0 font-medium'>{t('projects.profiles.activeProfile')}</FormLabel>
                    <FormControl>
                      <Select
                        value={field.value || 'none'}
                        onValueChange={(v) => field.onChange(v === 'none' ? '' : v)}
                      >
                        <SelectTrigger>
                          <SelectValue placeholder={t('projects.profiles.noRestriction')} />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value='none'>{t('projects.profiles.noRestriction')}</SelectItem>
                          {watchedProfiles.map((p, i) => (
                            <SelectItem key={i} value={p.name || `profile-${i}`}>
                              {p.name || `Profile ${i + 1}`}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </Form>
          </div>
        </div>

        <DialogFooter className='flex-col items-stretch gap-2 sm:flex-row sm:items-center sm:justify-end'>
          <div className='flex w-full gap-2 sm:w-auto'>
            <Button type='button' variant='outline' onClick={() => onOpenChange(false)} disabled={loading}>
              {t('common.buttons.cancel')}
            </Button>
            <Button type='submit' form='project-profiles-form' disabled={loading}>
              {loading ? t('common.buttons.saving') : t('common.buttons.save')}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
