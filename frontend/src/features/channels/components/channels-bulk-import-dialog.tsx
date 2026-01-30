import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import {
  AlertCircle,
  CheckCircle2,
  FileText,
  Key,
  Layers,
  Link,
  Loader2,
  Sparkles,
  Upload,
  XCircle,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Separator } from '@/components/ui/separator';
import { Textarea } from '@/components/ui/textarea';
import { useAllChannelNames, useBulkImportChannels } from '../data/channels';
import {
  type BulkImportChannelItem,
  bulkImportChannelItemSchema,
  type BulkImportText,
  bulkImportTextSchema,
  channelTypeSchema,
} from '../data/schema';

interface ChannelsBulkImportDialogProps {
  isOpen: boolean;
  onClose: () => void;
}

export function ChannelsBulkImportDialog({ isOpen, onClose }: ChannelsBulkImportDialogProps) {
  const { t } = useTranslation();
  const [parsedChannels, setParsedChannels] = useState<BulkImportChannelItem[]>([]);
  const [parseErrors, setParseErrors] = useState<string[]>([]);
  const [showPreview, setShowPreview] = useState(false);
  const [hasPreviewedCurrent, setHasPreviewedCurrent] = useState(false);

  const bulkImportMutation = useBulkImportChannels();
  const { data: existingChannelNames = [], isLoading: isLoadingChannelNames } = useAllChannelNames();

  const form = useForm<BulkImportText>({
    resolver: zodResolver(bulkImportTextSchema),
    defaultValues: {
      text: '',
    },
  });

  const textValue = form.watch('text');

  useEffect(() => {
    setHasPreviewedCurrent(false);
    setShowPreview(false);
  }, [textValue]);

  const parseChannelData = (text: string) => {
    const lines = text
      .trim()
      .split('\n')
      .filter((line) => line.trim());
    const channels: BulkImportChannelItem[] = [];
    const errors: string[] = [];
    const nameSet = new Set<string>();

    existingChannelNames.forEach((name) => nameSet.add(name.toLowerCase()));

    lines.forEach((line, index) => {
      const parts = line.split(',').map((part) => part.trim());

      if (parts.length < 5) {
        errors.push(t('channels.dialogs.bulkImport.invalidFormat', { line: index + 1 }));
        return;
      }

      const [type, name, baseURL, apiKey, supportedModelsStr, defaultTestModel] = parts;

      const typeResult = channelTypeSchema.safeParse(type);
      if (!typeResult.success) {
        errors.push(t('channels.dialogs.bulkImport.unsupportedType', { line: index + 1, type }));
        return;
      }

      if (!baseURL || baseURL.trim() === '') {
        errors.push(t('channels.dialogs.bulkImport.baseUrlRequired', { line: index + 1 }));
        return;
      }

      if (!apiKey || apiKey.trim() === '') {
        errors.push(t('channels.dialogs.bulkImport.apiKeyRequired', { line: index + 1 }));
        return;
      }

      const supportedModels = supportedModelsStr
        ? supportedModelsStr
            .split('|')
            .map((m) => m.trim())
            .filter((m) => m)
        : [];

      const channelName = name || `Channel ${index + 1}`;
      const channelItem: BulkImportChannelItem = {
        type: typeResult.data,
        name: channelName,
        baseURL: baseURL.trim(),
        apiKey: apiKey.trim(),
        supportedModels,
        defaultTestModel: defaultTestModel || supportedModels[0] || '',
      };

      const lowerCaseName = channelName.toLowerCase();
      if (nameSet.has(lowerCaseName)) {
        const isDuplicateWithExisting = existingChannelNames.some(
          (existingName) => existingName.toLowerCase() === lowerCaseName
        );

        if (isDuplicateWithExisting) {
          errors.push(t('channels.dialogs.bulkImport.duplicateNameWithExisting', { line: index + 1, name: channelName }));
        } else {
          errors.push(t('channels.dialogs.bulkImport.duplicateName', { line: index + 1, name: channelName }));
        }
        return;
      }
      nameSet.add(lowerCaseName);

      const result = bulkImportChannelItemSchema.safeParse(channelItem);
      if (!result.success) {
        const fieldErrors = result.error.issues.map((err) => `${err.path.join('.')}: ${err.message}`).join(', ');
        errors.push(
          t('channels.dialogs.bulkImport.validationError', {
            line: index + 1,
            name: channelName,
            error: fieldErrors,
          })
        );
        return;
      }

      channels.push(channelItem);
    });

    setParsedChannels(channels);
    setParseErrors(errors);
    setShowPreview(true);
    setHasPreviewedCurrent(true);
  };

  const handlePreview = () => {
    parseChannelData(textValue);
  };

  const handleImport = async () => {
    if (parsedChannels.length === 0) return;

    try {
      await bulkImportMutation.mutateAsync({
        channels: parsedChannels,
      });
      onClose();
      form.reset();
      setParsedChannels([]);
      setParseErrors([]);
      setShowPreview(false);
    } catch (_error) {
      // Bulk import failed - error is already handled by mutation hook
    }
  };

  const handleClose = () => {
    onClose();
    form.reset();
    setParsedChannels([]);
    setParseErrors([]);
    setShowPreview(false);
    setHasPreviewedCurrent(false);
  };

  const exampleText = `openai,OpenAI GPT,https://api.openai.com/v1,sk-xxx,gpt-4|gpt-3.5-turbo,gpt-4
anthropic,Anthropic Claude,https://api.anthropic.com,claude-xxx,claude-3-opus|claude-3-sonnet,claude-3-opus
deepseek,DeepSeek AI,https://api.deepseek.com,sk-xxx,deepseek-chat|deepseek-coder,deepseek-chat
deepseek_anthropic,DeepSeek Anthropic,https://api.deepseek.com/anthropic,sk-xxx,deepseek-chat|deepseek-coder,deepseek-chat`;

  return (
    <Dialog open={isOpen} onOpenChange={handleClose}>
      <DialogContent className='flex max-h-[90vh] flex-col overflow-hidden sm:max-w-5xl'>
        <DialogHeader className='flex-shrink-0 border-b pb-4'>
          <div className='flex items-center gap-3'>
            <div className='bg-primary/10 text-primary dark:bg-primary/20 rounded-lg p-2'>
              <Upload className='h-5 w-5' />
            </div>
            <div>
              <DialogTitle className='text-lg font-semibold'>{t('channels.dialogs.bulkImport.title')}</DialogTitle>
              <DialogDescription className='text-sm text-muted-foreground'>
                {t('channels.dialogs.bulkImport.description')}
              </DialogDescription>
            </div>
          </div>
        </DialogHeader>

        <div className='flex-1 overflow-y-auto px-1 py-2'>
          <div className='space-y-6'>
            {/* Format Instructions Card */}
            <Card className='border-l-4 border-l-primary'>
              <CardHeader className='pb-3'>
                <div className='flex items-center gap-2'>
                  <div className='bg-primary/10 text-primary dark:bg-primary/20 rounded-lg p-1.5'>
                    <FileText className='h-4 w-4' />
                  </div>
                  <CardTitle className='text-sm font-medium'>{t('channels.dialogs.bulkImport.formatTitle')}</CardTitle>
                </div>
                <CardDescription className='pt-2 text-sm'>
                  {t('channels.dialogs.bulkImport.formatDescription')}
                </CardDescription>
              </CardHeader>
              <CardContent className='space-y-4 pt-0'>
                <div className='bg-muted/50 rounded-lg border p-3'>
                  <code className='text-muted-foreground block text-xs whitespace-pre-wrap font-mono leading-relaxed'>
                    {exampleText}
                  </code>
                </div>
                <p className='text-muted-foreground text-xs'>{t('channels.dialogs.bulkImport.formatNote')}</p>
              </CardContent>
            </Card>

            {/* Input Form */}
            <Card>
              <CardHeader className='pb-3'>
                <div className='flex items-center gap-2'>
                  <div className='bg-primary/10 text-primary dark:bg-primary/20 rounded-lg p-1.5'>
                    <Layers className='h-4 w-4' />
                  </div>
                  <CardTitle className='text-sm font-medium'>{t('channels.dialogs.bulkImport.inputLabel')}</CardTitle>
                </div>
              </CardHeader>
              <CardContent className='space-y-4 pt-0'>
                <Form {...form}>
                  <FormField
                    control={form.control}
                    name='text'
                    render={({ field }) => (
                      <FormItem className='space-y-3'>
                        <FormControl>
                          <Textarea
                            placeholder={t('channels.dialogs.bulkImport.inputPlaceholder')}
                            className='min-h-[180px] resize-none font-mono text-sm leading-relaxed'
                            {...field}
                          />
                        </FormControl>
                        <FormDescription className='flex items-center gap-2 text-xs'>
                          {isLoadingChannelNames ? (
                            <>
                              <Loader2 className='h-3 w-3 animate-spin' />
                              {t('channels.dialogs.bulkImport.loadingChannelNames')}
                            </>
                          ) : (
                            <>
                              <Sparkles className='text-primary h-3 w-3' />
                              {t('channels.dialogs.bulkImport.supportedTypes')}
                            </>
                          )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </Form>
              </CardContent>
            </Card>

            {/* Preview Results */}
            {showPreview && (
              <Card
                className={`${parseErrors.length > 0 ? 'border-l-4 border-l-red-500' : parsedChannels.length > 0 ? 'border-l-4 border-l-green-500' : ''}`}
              >
                <CardHeader className='pb-3'>
                  <div className='flex items-center justify-between'>
                    <div className='flex items-center gap-3'>
                      <div className='bg-primary/10 text-primary dark:bg-primary/20 rounded-lg p-1.5'>
                        <Sparkles className='h-4 w-4' />
                      </div>
                      <CardTitle className='text-sm font-medium'>{t('channels.dialogs.bulkImport.previewTitle')}</CardTitle>
                      <div className='flex items-center gap-2'>
                        {parsedChannels.length > 0 && (
                          <Badge
                            variant='secondary'
                            className='bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400 border-0'
                          >
                            <CheckCircle2 className='mr-1 h-3 w-3' />
                            {t('channels.dialogs.bulkImport.validRecords', { count: parsedChannels.length })}
                          </Badge>
                        )}
                        {parseErrors.length > 0 && (
                          <Badge
                            variant='secondary'
                            className='bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400 border-0'
                          >
                            <AlertCircle className='mr-1 h-3 w-3' />
                            {t('channels.dialogs.bulkImport.errors', { count: parseErrors.length })}
                          </Badge>
                        )}
                      </div>
                    </div>
                    {parseErrors.length > 0 ? (
                      <div className='flex items-center gap-2 text-xs font-medium text-red-600 dark:text-red-400'>
                        <XCircle className='h-4 w-4' />
                        {t('channels.dialogs.bulkImport.status.blocked')}
                      </div>
                    ) : parsedChannels.length > 0 ? (
                      <div className='flex items-center gap-2 text-xs font-medium text-green-600 dark:text-green-400'>
                        <CheckCircle2 className='h-4 w-4' />
                        {t('channels.dialogs.bulkImport.status.ready')}
                      </div>
                    ) : null}
                  </div>
                </CardHeader>
                <CardContent className='space-y-4 pt-0'>
                  {/* Errors */}
                  {parseErrors.length > 0 && (
                    <div className='space-y-2'>
                      <div className='flex items-center justify-between'>
                        <span className='text-xs font-medium text-red-600 dark:text-red-400'>
                          {t('channels.dialogs.bulkImport.errorMessages')}
                        </span>
                        <span className='text-muted-foreground text-xs'>{t('channels.dialogs.bulkImport.status.blockedHint')}</span>
                      </div>
                      <div className='space-y-2'>
                        {parseErrors.map((error, index) => (
                          <Alert
                            key={index}
                            variant='destructive'
                            className='border-red-200 bg-red-50/50 py-2 dark:border-red-800 dark:bg-red-950/20'
                          >
                            <XCircle className='h-4 w-4 shrink-0' />
                            <AlertDescription className='text-xs'>{error}</AlertDescription>
                          </Alert>
                        ))}
                      </div>
                    </div>
                  )}

                  <Separator />

                  {/* Valid Channels */}
                  {parsedChannels.length > 0 && (
                    <div className='space-y-3'>
                      <div className='flex items-center gap-2 text-xs font-medium text-green-600 dark:text-green-400'>
                        <CheckCircle2 className='h-4 w-4' />
                        {t('channels.dialogs.bulkImport.validChannels')}
                      </div>
                      <div className='grid gap-3'>
                        {parsedChannels.map((channel, index) => (
                          <Card key={index} className='overflow-hidden border-l-2 border-l-primary/50'>
                            <CardHeader className='bg-muted/30 py-3'>
                              <div className='flex items-center gap-3'>
                                <Badge variant='outline' className='text-xs font-medium'>
                                  {channel.type}
                                </Badge>
                                <span className='text-sm font-semibold'>{channel.name}</span>
                              </div>
                            </CardHeader>
                            <CardContent className='grid gap-3 py-3 sm:grid-cols-2'>
                              <div className='flex items-start gap-2'>
                                <div className='bg-primary/10 text-primary dark:bg-primary/20 rounded-lg p-1.5'>
                                  <Link className='h-3.5 w-3.5' />
                                </div>
                                <div className='min-w-0 flex-1'>
                                  <div className='text-muted-foreground text-xs'>
                                    {t('channels.dialogs.bulkImport.fieldLabels.baseUrl')}
                                  </div>
                                  <div className='truncate font-mono text-xs text-blue-600 dark:text-blue-400'>
                                    {channel.baseURL}
                                  </div>
                                </div>
                              </div>
                              <div className='flex items-start gap-2'>
                                <div className='bg-primary/10 text-primary dark:bg-primary/20 rounded-lg p-1.5'>
                                  <Key className='h-3.5 w-3.5' />
                                </div>
                                <div className='min-w-0 flex-1'>
                                  <div className='text-muted-foreground text-xs'>
                                    {t('channels.dialogs.bulkImport.fieldLabels.apiKey')}
                                  </div>
                                  <div className='font-mono text-xs text-purple-600 dark:text-purple-400'>
                                    {channel.apiKey.substring(0, 16)}...
                                  </div>
                                </div>
                              </div>
                              <div className='flex items-start gap-2 sm:col-span-2'>
                                <div className='bg-primary/10 text-primary dark:bg-primary/20 rounded-lg p-1.5'>
                                  <Layers className='h-3.5 w-3.5' />
                                </div>
                                <div className='min-w-0 flex-1'>
                                  <div className='text-muted-foreground text-xs'>
                                    {t('channels.dialogs.bulkImport.fieldLabels.supportedModels')}
                                  </div>
                                  <div className='mt-1 flex flex-wrap gap-1'>
                                    {channel.supportedModels.map((model, idx) => (
                                      <Badge
                                        key={idx}
                                        variant='secondary'
                                        className='bg-muted text-muted-foreground text-xs font-normal'
                                      >
                                        {model}
                                      </Badge>
                                    ))}
                                  </div>
                                </div>
                              </div>
                            </CardContent>
                          </Card>
                        ))}
                      </div>
                    </div>
                  )}
                </CardContent>
              </Card>
            )}
          </div>
        </div>

        <DialogFooter className='border-t bg-muted/30 flex-shrink-0 gap-3 pt-4'>
          <Button variant='outline' onClick={handleClose} className='min-w-[100px]'>
            {t('common.buttons.cancel')}
          </Button>
          {!hasPreviewedCurrent ? (
            <Button onClick={handlePreview} disabled={!textValue?.trim() || isLoadingChannelNames} className='min-w-[100px]'>
              {isLoadingChannelNames && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
              {t('channels.dialogs.bulkImport.previewButton')}
            </Button>
          ) : (
            <Button
              onClick={handleImport}
              disabled={!showPreview || parsedChannels.length === 0 || parseErrors.length > 0 || bulkImportMutation.isPending}
              className='min-w-[100px]'
            >
              {bulkImportMutation.isPending && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
              {t('channels.dialogs.bulkImport.importButton', { count: parsedChannels.length })}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
