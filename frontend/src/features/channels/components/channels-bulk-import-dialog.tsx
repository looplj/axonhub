import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Loader2, Upload, XCircle } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
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

  // Reset preview state when text content changes
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

    // Add existing channel names to the set for duplicate detection
    existingChannelNames.forEach((name) => nameSet.add(name.toLowerCase()));

    lines.forEach((line, index) => {
      const parts = line.split(',').map((part) => part.trim());

      if (parts.length < 5) {
        errors.push(t('channels.dialogs.bulkImport.invalidFormat', { line: index + 1 }));
        return;
      }

      const [type, name, baseURL, apiKey, supportedModelsStr, defaultTestModel] = parts;

      // Validate channel type
      const typeResult = channelTypeSchema.safeParse(type);
      if (!typeResult.success) {
        errors.push(t('channels.dialogs.bulkImport.unsupportedType', { line: index + 1, type }));
        return;
      }

      // Validate required fields
      if (!baseURL || baseURL.trim() === '') {
        errors.push(t('channels.dialogs.bulkImport.baseUrlRequired', { line: index + 1 }));
        return;
      }

      if (!apiKey || apiKey.trim() === '') {
        errors.push(t('channels.dialogs.bulkImport.apiKeyRequired', { line: index + 1 }));
        return;
      }

      // Parse supported models
      const supportedModels = supportedModelsStr
        ? supportedModelsStr
            .split('|')
            .map((m) => m.trim())
            .filter((m) => m)
        : [];

      // Create channel item
      const channelName = name || `Channel ${index + 1}`;
      const channelItem: BulkImportChannelItem = {
        type: typeResult.data,
        name: channelName,
        baseURL: baseURL.trim(),
        apiKey: apiKey.trim(),
        supportedModels,
        defaultTestModel: defaultTestModel || supportedModels[0] || '',
      };

      // Check for duplicate names (both in current batch and with existing channels)
      const lowerCaseName = channelName.toLowerCase();
      if (nameSet.has(lowerCaseName)) {
        // Check if it's a duplicate with existing server data
        const isDuplicateWithExisting = existingChannelNames.some((existingName) => existingName.toLowerCase() === lowerCaseName);

        if (isDuplicateWithExisting) {
          errors.push(t('channels.dialogs.bulkImport.duplicateNameWithExisting', { line: index + 1, name: channelName }));
        } else {
          errors.push(t('channels.dialogs.bulkImport.duplicateName', { line: index + 1, name: channelName }));
        }
        return;
      }
      nameSet.add(lowerCaseName);

      // Validate the channel item
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
    const text = textValue;
    parseChannelData(text);
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
      <DialogContent className='flex max-h-[90vh] flex-col overflow-hidden sm:max-w-4xl'>
        <DialogHeader className='flex-shrink-0 pb-4'>
          <DialogTitle className='flex items-center gap-2 text-lg'>
            <Upload className='h-6 w-6' />
            {t('channels.dialogs.bulkImport.title')}
          </DialogTitle>
          <DialogDescription className='text-sm'>{t('channels.dialogs.bulkImport.description')}</DialogDescription>
        </DialogHeader>

        <div className='flex-1 space-y-6 overflow-y-auto pr-6'>
          {/* Format Instructions */}
          <Card className='flex-shrink-0'>
            <CardHeader className='pb-4'>
              <CardTitle className='text-base font-semibold'>{t('channels.dialogs.bulkImport.formatTitle')}</CardTitle>
              <CardDescription className='space-y-2 text-sm'>
                <div>{t('channels.dialogs.bulkImport.formatDescription')}</div>
                <div>{t('channels.dialogs.bulkImport.formatNote')}</div>
              </CardDescription>
            </CardHeader>
            <CardContent className='pt-0'>
              <div className='bg-muted rounded-md border p-4 font-mono text-xs whitespace-pre-line'>{exampleText}</div>
            </CardContent>
          </Card>

          {/* Input Form */}
          <div className='flex-shrink-0'>
            <Form {...form}>
              <FormField
                control={form.control}
                name='text'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel className='text-base font-semibold'>{t('channels.dialogs.bulkImport.inputLabel')}</FormLabel>
                    <FormControl>
                      <Textarea
                        placeholder={t('channels.dialogs.bulkImport.inputPlaceholder')}
                        className='min-h-[250px] resize-none p-4 font-mono text-sm'
                        {...field}
                      />
                    </FormControl>
                    <FormDescription asChild className='rounded-md border bg-blue-50/50 p-3 text-sm dark:bg-blue-950/20'>
                      <div className='space-y-2'>
                        {isLoadingChannelNames && (
                          <div className='flex items-center gap-2'>
                            <Loader2 className='h-4 w-4 animate-spin' />
                            {t('channels.dialogs.bulkImport.loadingChannelNames')}
                          </div>
                        )}
                        <div>{t('channels.dialogs.bulkImport.supportedTypes')}</div>
                      </div>
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </Form>
          </div>

          {/* Preview Results */}
          {showPreview && (
            <Card className='flex min-h-0 flex-1 flex-col'>
              <CardHeader
                className={`flex-shrink-0 border-b pb-4 ${parseErrors.length > 0 ? 'bg-red-50/30 dark:bg-red-950/10' : 'bg-muted/20'}`}
              >
                <CardTitle className='flex items-center gap-3 text-base font-semibold'>
                  {t('channels.dialogs.bulkImport.previewTitle')}
                  {parsedChannels.length > 0 && (
                    <Badge variant='secondary' className='bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'>
                      {t('channels.dialogs.bulkImport.validRecords', { count: parsedChannels.length })}
                    </Badge>
                  )}
                  {parseErrors.length > 0 && (
                    <Badge variant='destructive' className='bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400'>
                      {t('channels.dialogs.bulkImport.errors', { count: parseErrors.length })}
                    </Badge>
                  )}
                  {/* Import Status Indicator */}
                  {showPreview && (
                    <div className='ml-auto'>
                      {parseErrors.length > 0 ? (
                        <div className='flex items-center gap-2 rounded-md border bg-red-50 px-3 py-1 text-sm font-medium text-red-600 dark:bg-red-950/20 dark:text-red-400'>
                          {t('channels.dialogs.bulkImport.status.blocked')}
                        </div>
                      ) : parsedChannels.length > 0 ? (
                        <div className='flex items-center gap-2 rounded-md border bg-green-50 px-3 py-1 text-sm font-medium text-green-600 dark:bg-green-950/20 dark:text-green-400'>
                          {t('channels.dialogs.bulkImport.status.ready')}
                        </div>
                      ) : null}
                    </div>
                  )}
                </CardTitle>
              </CardHeader>
              <CardContent className='bg-background/50 min-h-0 flex-1'>
                <div
                  className='from-background/80 to-muted/20 h-full space-y-4 overflow-y-auto rounded-lg bg-gradient-to-b p-4 pr-4'
                  style={{ maxHeight: 'calc(95vh - 350px)' }}
                >
                  {/* Errors */}
                  {parseErrors.length > 0 && (
                    <div className='space-y-3 rounded-lg border-2 border-red-200 bg-red-50/50 p-4 dark:border-red-800 dark:bg-red-950/20'>
                      <div className='mb-4 flex items-center gap-2 text-sm font-semibold text-red-700 dark:text-red-400'>
                        {t('channels.dialogs.bulkImport.errorMessages')}
                        <div className='ml-auto rounded-md border bg-red-100 px-2 py-1 text-xs font-medium text-red-800 dark:bg-red-900/40 dark:text-red-300'>
                          {t('channels.dialogs.bulkImport.status.blockedHint')}
                        </div>
                      </div>
                      {parseErrors.map((error, index) => (
                        <Alert key={index} variant='destructive' className='bg-red-50/70 dark:bg-red-950/30'>
                          <XCircle className='h-4 w-4' />
                          <AlertDescription className='text-sm'>{error}</AlertDescription>
                        </Alert>
                      ))}
                    </div>
                  )}

                  {/* Valid Channels */}
                  {parsedChannels.length > 0 && (
                    <div className='bg-muted/30 space-y-4 rounded-lg border p-4'>
                      <div className='mb-4 text-sm font-semibold text-green-600 dark:text-green-400'>
                        {t('channels.dialogs.bulkImport.validChannels')}
                      </div>
                      <div className='grid gap-4'>
                        {parsedChannels.map((channel, index) => (
                          <div key={index} className='bg-background hover:bg-muted/50 space-y-3 rounded-lg border p-4 transition-colors'>
                            <div className='flex items-center gap-3'>
                              <Badge variant='outline' className='px-3 py-1 text-sm font-medium'>
                                {channel.type}
                              </Badge>
                              <span className='text-foreground text-base font-bold'>{channel.name}</span>
                            </div>
                            <div className='grid grid-cols-1 gap-3 text-sm md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4'>
                              {channel.baseURL && (
                                <div className='bg-muted border-border rounded-lg border p-3'>
                                  <span className='text-muted-foreground font-semibold'>
                                    {t('channels.dialogs.bulkImport.fieldLabels.baseUrl')}:
                                  </span>
                                  <div className='mt-1 font-mono text-xs break-all text-blue-600 dark:text-blue-400'>{channel.baseURL}</div>
                                </div>
                              )}
                              {channel.apiKey && (
                                <div className='bg-muted border-border rounded-lg border p-3'>
                                  <span className='text-muted-foreground font-semibold'>
                                    {t('channels.dialogs.bulkImport.fieldLabels.apiKey')}:
                                  </span>
                                  <div className='mt-1 font-mono text-xs text-purple-600 dark:text-purple-400'>
                                    {channel.apiKey.substring(0, 20)}...
                                  </div>
                                </div>
                              )}
                              <div className='bg-muted border-border rounded-lg border p-3'>
                                <span className='text-muted-foreground font-semibold'>
                                  {t('channels.dialogs.bulkImport.fieldLabels.supportedModels')}:
                                </span>
                                <div className='mt-1 flex flex-wrap gap-1'>
                                  {channel.supportedModels.map((model, idx) => (
                                    <span
                                      key={idx}
                                      className='bg-background text-foreground border-border rounded border px-2 py-1 text-xs font-medium'
                                    >
                                      {model}
                                    </span>
                                  ))}
                                </div>
                              </div>
                              <div className='bg-muted border-border rounded-lg border p-3'>
                                <span className='text-muted-foreground font-semibold'>
                                  {t('channels.dialogs.bulkImport.fieldLabels.defaultTestModel')}:
                                </span>
                                <div className='mt-1 text-xs font-medium text-green-600 dark:text-green-400'>
                                  {channel.defaultTestModel}
                                </div>
                              </div>
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              </CardContent>
            </Card>
          )}
        </div>

        <DialogFooter className='border-border bg-muted/30 flex-shrink-0 flex-col gap-3 border-t pt-4 pb-2 sm:flex-row'>
          <Button
            variant='outline'
            onClick={handleClose}
            size='lg'
            className='hover:bg-muted w-full border-2 px-8 py-2 text-sm font-medium sm:w-auto'
          >
            {t('common.buttons.cancel')}
          </Button>
          {!hasPreviewedCurrent ? (
            <Button
              onClick={handlePreview}
              disabled={!textValue?.trim() || isLoadingChannelNames}
              size='lg'
              className='bg-primary hover:bg-primary/90 w-full px-8 py-2 text-sm font-medium sm:w-auto'
            >
              {isLoadingChannelNames && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
              {t('channels.dialogs.bulkImport.previewButton')}
            </Button>
          ) : (
            <Button
              onClick={handleImport}
              disabled={!showPreview || parsedChannels.length === 0 || parseErrors.length > 0 || bulkImportMutation.isPending}
              size='lg'
              className='bg-primary hover:bg-primary/90 w-full px-8 py-2 text-sm font-medium sm:w-auto'
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
