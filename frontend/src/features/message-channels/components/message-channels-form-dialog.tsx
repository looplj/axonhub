import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { useForm, useFieldArray } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { IconPlus, IconTrash } from '@tabler/icons-react';
import { useCreateMessageChannel, useUpdateMessageChannel } from '../data/message-channels';
import {
  type CreateMessageChannelInput,
  type UpdateMessageChannelInput,
  type MessageChannel,
  createMessageChannelInputSchema,
  updateMessageChannelInputSchema,
} from '../data/schema';

interface MessageChannelsFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentRow?: MessageChannel | null;
}

export function MessageChannelsFormDialog({
  open,
  onOpenChange,
  currentRow,
}: MessageChannelsFormDialogProps) {
  const { t } = useTranslation();
  const isEdit = !!currentRow;
  const createMessageChannel = useCreateMessageChannel();
  const updateMessageChannel = useUpdateMessageChannel();

  const form = useForm<CreateMessageChannelInput | UpdateMessageChannelInput>({
    resolver: zodResolver(isEdit ? updateMessageChannelInputSchema : createMessageChannelInputSchema),
    defaultValues: {
      name: '',
      description: '',
      type: 'feishu',
      status: 'enabled',
      settings: {
        feishu: {
          appId: '',
          appSecret: '',
          encryptKey: '',
          verificationToken: '',
          allowFrom: [],
          excludeKeywords: [],
        },
      },
    },
  });

  const allowFromFields = useFieldArray({
    control: form.control,
    name: 'settings.feishu.allowFrom' as const,
  });

  const excludeKeywordsFields = useFieldArray({
    control: form.control,
    name: 'settings.feishu.excludeKeywords' as const,
  });

  useEffect(() => {
    if (open && currentRow) {
      form.reset({
        name: currentRow.name,
        description: currentRow.description,
        type: currentRow.type,
        status: currentRow.status,
        settings: currentRow.settings || {
          feishu: {
            appId: '',
            appSecret: '',
            encryptKey: '',
            verificationToken: '',
            allowFrom: [],
            excludeKeywords: [],
          },
        },
      });
    } else if (open) {
      form.reset({
        name: '',
        description: '',
        type: 'feishu',
        status: 'enabled',
        settings: {
          feishu: {
            appId: '',
            appSecret: '',
            encryptKey: '',
            verificationToken: '',
            allowFrom: [],
            excludeKeywords: [],
          },
        },
      });
    }
  }, [open, currentRow, form]);

  const onSubmit = async (values: CreateMessageChannelInput | UpdateMessageChannelInput) => {
    if (isEdit && currentRow) {
      await updateMessageChannel.mutateAsync({ id: currentRow.id, input: values });
    } else {
      await createMessageChannel.mutateAsync(values as CreateMessageChannelInput);
    }
    onOpenChange(false);
  };

  const isPending = createMessageChannel.isPending || updateMessageChannel.isPending;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex h-[85vh] max-h-[800px] flex-col sm:max-w-2xl">
        <DialogHeader className="shrink-0 text-left">
          <DialogTitle>
            {isEdit
              ? t('messageChannels.dialogs.edit.title')
              : t('messageChannels.dialogs.create.title')}
          </DialogTitle>
          <DialogDescription>
            {isEdit
              ? t('messageChannels.dialogs.edit.description')
              : t('messageChannels.dialogs.create.description')}
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="flex min-h-0 flex-1 flex-col gap-4">
            <Tabs defaultValue="basic" className="flex min-h-0 flex-1 flex-col">
              <TabsList className="grid w-full grid-cols-2 shrink-0">
                <TabsTrigger value="basic">{t('messageChannels.tabs.basic')}</TabsTrigger>
                <TabsTrigger value="settings">{t('messageChannels.tabs.settings')}</TabsTrigger>
              </TabsList>
              <TabsContent value="basic" className="flex-1 overflow-y-auto space-y-4 pt-4">
                <FormField
                  control={form.control}
                  name="name"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('messageChannels.form.name.label')}</FormLabel>
                      <FormControl>
                        <Input
                          placeholder={t('messageChannels.form.name.placeholder')}
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="description"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('messageChannels.form.description.label')}</FormLabel>
                      <FormControl>
                        <Textarea
                          placeholder={t('messageChannels.form.description.placeholder')}
                          {...field}
                          value={field.value || ''}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="type"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('messageChannels.form.type.label')}</FormLabel>
                      <Select onValueChange={field.onChange} value={field.value}>
                        <FormControl>
                          <SelectTrigger>
                            <SelectValue placeholder={t('messageChannels.form.type.placeholder')} />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          <SelectItem value="feishu">{t('messageChannels.types.feishu')}</SelectItem>
                        </SelectContent>
                      </Select>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="status"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('messageChannels.form.status.label')}</FormLabel>
                      <Select onValueChange={field.onChange} value={field.value}>
                        <FormControl>
                          <SelectTrigger>
                            <SelectValue placeholder={t('messageChannels.form.status.placeholder')} />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          <SelectItem value="enabled">{t('common.status.enabled')}</SelectItem>
                          <SelectItem value="disabled">{t('common.status.disabled')}</SelectItem>
                        </SelectContent>
                      </Select>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </TabsContent>
              <TabsContent value="settings" className="flex-1 overflow-y-auto space-y-4 pt-4">
                <FormField
                  control={form.control}
                  name="settings.feishu.appId"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('messageChannels.form.feishu.appId.label')}</FormLabel>
                      <FormControl>
                        <Input
                          placeholder={t('messageChannels.form.feishu.appId.placeholder')}
                          {...field}
                          value={field.value || ''}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="settings.feishu.appSecret"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('messageChannels.form.feishu.appSecret.label')}</FormLabel>
                      <FormControl>
                        <Input
                          type="password"
                          placeholder={t('messageChannels.form.feishu.appSecret.placeholder')}
                          {...field}
                          value={field.value || ''}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="settings.feishu.encryptKey"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('messageChannels.form.feishu.encryptKey.label')}</FormLabel>
                      <FormControl>
                        <Input
                          placeholder={t('messageChannels.form.feishu.encryptKey.placeholder')}
                          {...field}
                          value={field.value || ''}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="settings.feishu.verificationToken"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('messageChannels.form.feishu.verificationToken.label')}</FormLabel>
                      <FormControl>
                        <Input
                          placeholder={t('messageChannels.form.feishu.verificationToken.placeholder')}
                          {...field}
                          value={field.value || ''}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                {/* Allow From */}
                <div className="space-y-2">
                  <FormLabel>{t('messageChannels.form.feishu.allowFrom.label')}</FormLabel>
                  <div className="space-y-2">
                    {allowFromFields.fields.map((field, index) => (
                      <div key={field.id} className="flex items-center gap-2">
                        <FormField
                          control={form.control}
                          name={`settings.feishu.allowFrom.${index}`}
                          render={({ field }) => (
                            <FormItem className="flex-1">
                              <FormControl>
                                <Input
                                  placeholder={t('messageChannels.form.feishu.allowFrom.placeholder')}
                                  {...field}
                                />
                              </FormControl>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          onClick={() => allowFromFields.remove(index)}
                        >
                          <IconTrash className="h-4 w-4" />
                        </Button>
                      </div>
                    ))}
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => allowFromFields.append('')}
                    >
                      <IconPlus className="mr-2 h-4 w-4" />
                      {t('messageChannels.form.feishu.allowFrom.add')}
                    </Button>
                  </div>
                </div>

                {/* Exclude Keywords */}
                <div className="space-y-2">
                  <FormLabel>{t('messageChannels.form.feishu.excludeKeywords.label')}</FormLabel>
                  <div className="space-y-2">
                    {excludeKeywordsFields.fields.map((field, index) => (
                      <div key={field.id} className="flex items-center gap-2">
                        <FormField
                          control={form.control}
                          name={`settings.feishu.excludeKeywords.${index}`}
                          render={({ field }) => (
                            <FormItem className="flex-1">
                              <FormControl>
                                <Input
                                  placeholder={t('messageChannels.form.feishu.excludeKeywords.placeholder')}
                                  {...field}
                                />
                              </FormControl>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          onClick={() => excludeKeywordsFields.remove(index)}
                        >
                          <IconTrash className="h-4 w-4" />
                        </Button>
                      </div>
                    ))}
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => excludeKeywordsFields.append('')}
                    >
                      <IconPlus className="mr-2 h-4 w-4" />
                      {t('messageChannels.form.feishu.excludeKeywords.add')}
                    </Button>
                  </div>
                </div>
              </TabsContent>
            </Tabs>
            <DialogFooter className="shrink-0 border-t pt-4">
              <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
                {t('common.buttons.cancel')}
              </Button>
              <Button type="submit" disabled={isPending}>
                {isPending
                  ? t('common.buttons.processing')
                  : isEdit
                    ? t('common.buttons.update')
                    : t('common.buttons.create')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
