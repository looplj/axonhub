import React, { useState, useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Textarea } from '@/components/ui/textarea'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { 
  bulkImportTextSchema, 
  type BulkImportText, 
  type BulkImportChannelItem,
  bulkImportChannelItemSchema,
  channelTypeSchema
} from '../data/schema'
import { useBulkImportChannels, useAllChannelNames } from '../data/channels'
import { Loader2, Upload, XCircle } from 'lucide-react'

interface ChannelsBulkImportDialogProps {
  isOpen: boolean
  onClose: () => void
}

export function ChannelsBulkImportDialog({ isOpen, onClose }: ChannelsBulkImportDialogProps) {
  const { t } = useTranslation()
  const [parsedChannels, setParsedChannels] = useState<BulkImportChannelItem[]>([])
  const [parseErrors, setParseErrors] = useState<string[]>([])
  const [showPreview, setShowPreview] = useState(false)
  const [hasPreviewedCurrent, setHasPreviewedCurrent] = useState(false)
  
  const bulkImportMutation = useBulkImportChannels()
  const { data: existingChannelNames = [], isLoading: isLoadingChannelNames } = useAllChannelNames()

  const form = useForm<BulkImportText>({
    resolver: zodResolver(bulkImportTextSchema),
    defaultValues: {
      text: '',
    },
  })

  const textValue = form.watch('text')

  // Reset preview state when text content changes
  useEffect(() => {
    setHasPreviewedCurrent(false)
    setShowPreview(false)
  }, [textValue])

  const parseChannelData = (text: string) => {
    const lines = text.trim().split('\n').filter(line => line.trim())
    const channels: BulkImportChannelItem[] = []
    const errors: string[] = []
    const nameSet = new Set<string>()
    
    // Add existing channel names to the set for duplicate detection
    existingChannelNames.forEach(name => nameSet.add(name.toLowerCase()))

    lines.forEach((line, index) => {
      const parts = line.split(',').map(part => part.trim())
      
      if (parts.length < 5) {
        errors.push(t('channels.dialogs.bulkImport.invalidFormat', { line: index + 1 }))
        return
      }

      const [type, name, baseURL, apiKey, supportedModelsStr, defaultTestModel] = parts
      
      // Validate channel type
      const typeResult = channelTypeSchema.safeParse(type)
      if (!typeResult.success) {
        errors.push(t('channels.dialogs.bulkImport.unsupportedType', { line: index + 1, type }))
        return
      }

      // Validate required fields
      if (!baseURL || baseURL.trim() === '') {
        errors.push(t('channels.dialogs.bulkImport.baseUrlRequired', { line: index + 1 }))
        return
      }
      
      if (!apiKey || apiKey.trim() === '') {
        errors.push(t('channels.dialogs.bulkImport.apiKeyRequired', { line: index + 1 }))
        return
      }

      // Parse supported models
      const supportedModels = supportedModelsStr ? supportedModelsStr.split('|').map(m => m.trim()).filter(m => m) : []
      
      // Create channel item
      const channelName = name || `Channel ${index + 1}`
      const channelItem: BulkImportChannelItem = {
        type: typeResult.data,
        name: channelName,
        baseURL: baseURL.trim(),
        apiKey: apiKey.trim(),
        supportedModels,
        defaultTestModel: defaultTestModel || supportedModels[0] || '',
      }

      // Check for duplicate names (both in current batch and with existing channels)
      const lowerCaseName = channelName.toLowerCase()
      if (nameSet.has(lowerCaseName)) {
        // Check if it's a duplicate with existing server data
        const isDuplicateWithExisting = existingChannelNames.some(
          existingName => existingName.toLowerCase() === lowerCaseName
        )
        
        if (isDuplicateWithExisting) {
          errors.push(t('channels.dialogs.bulkImport.duplicateNameWithExisting', { line: index + 1, name: channelName }))
        } else {
          errors.push(t('channels.dialogs.bulkImport.duplicateName', { line: index + 1, name: channelName }))
        }
        return
      }
      nameSet.add(lowerCaseName)

      // Validate the channel item
      const result = bulkImportChannelItemSchema.safeParse(channelItem)
      if (!result.success) {
        const fieldErrors = result.error.errors.map(err => `${err.path.join('.')}: ${err.message}`).join(', ')
        errors.push(t('channels.dialogs.bulkImport.validationError', { line: index + 1, name: channelName, error: fieldErrors }))
        return
      }

      channels.push(channelItem)
    })

    setParsedChannels(channels)
    setParseErrors(errors)
    setShowPreview(true)
    setHasPreviewedCurrent(true)
  }

  const handlePreview = () => {
    const text = textValue
    parseChannelData(text)
  }

  const handleImport = async () => {
    if (parsedChannels.length === 0) return

    try {
      await bulkImportMutation.mutateAsync({
        channels: parsedChannels
      })
      onClose()
      form.reset()
      setParsedChannels([])
      setParseErrors([])
      setShowPreview(false)
    } catch (_error) {
      // Bulk import failed - error is already handled by mutation hook
    }
  }

  const handleClose = () => {
    onClose()
    form.reset()
    setParsedChannels([])
    setParseErrors([])
    setShowPreview(false)
    setHasPreviewedCurrent(false)
  }

  const exampleText = `openai,OpenAI GPT,https://api.openai.com/v1,sk-xxx,gpt-4|gpt-3.5-turbo,gpt-4
anthropic,Anthropic Claude,https://api.anthropic.com,claude-xxx,claude-3-opus|claude-3-sonnet,claude-3-opus
deepseek,DeepSeek AI,https://api.deepseek.com,sk-xxx,deepseek-chat|deepseek-coder,deepseek-chat`

  return (
    <Dialog open={isOpen} onOpenChange={handleClose}>
      <DialogContent className="sm:max-w-4xl max-h-[90vh] flex flex-col overflow-hidden">
        <DialogHeader className="pb-4 flex-shrink-0">
          <DialogTitle className="flex items-center gap-2 text-lg">
            <Upload className="w-6 h-6" />
            {t('channels.dialogs.bulkImport.title')}
          </DialogTitle>
          <DialogDescription className="text-sm">
            {t('channels.dialogs.bulkImport.description')}
          </DialogDescription>
        </DialogHeader>

        <div className="flex-1 overflow-y-auto space-y-6 pr-6">
          {/* Format Instructions */}
          <Card className="flex-shrink-0">
            <CardHeader className="pb-4">
              <CardTitle className="text-lg font-bold">{t('channels.dialogs.bulkImport.formatTitle')}</CardTitle>
              <CardDescription className="text-sm space-y-3">
                <div className="font-semibold">{t('channels.dialogs.bulkImport.formatDescription')}</div>
                <div>{t('channels.dialogs.bulkImport.formatNote')}</div>
              </CardDescription>
            </CardHeader>
            <CardContent className="pt-0">
              <div className="text-sm font-mono bg-muted p-6 rounded-xl whitespace-pre-line border-2 border-dashed border-muted-foreground/40">
                {exampleText}
              </div>
            </CardContent>
          </Card>

          {/* Input Form */}
          <div className="flex-shrink-0">
            <Form {...form}>
              <FormField
                control={form.control}
                name="text"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel className="text-base font-semibold">{t('channels.dialogs.bulkImport.inputLabel')}</FormLabel>
                    <FormControl>
                      <Textarea
                        placeholder={t('channels.dialogs.bulkImport.inputPlaceholder')}
                        className="min-h-[250px] font-mono text-sm resize-none border-2 focus:border-primary p-4"
                        {...field}
                      />
                    </FormControl>
                    <FormDescription className="text-sm bg-blue-50/80 dark:bg-blue-950/30 p-4 rounded-lg border border-blue-200 dark:border-blue-800">
                      {isLoadingChannelNames ? (
                        <div className="flex items-center gap-2">
                          <Loader2 className="h-4 w-4 animate-spin" />
                          {t('channels.dialogs.bulkImport.supportedTypes')} (Loading existing channel names...)
                        </div>
                      ) : (
                        t('channels.dialogs.bulkImport.supportedTypes')
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </Form>
          </div>

          {/* Preview Results */}
          {showPreview && (
            <Card className="flex-1 min-h-0 flex flex-col bg-card border-border">
              <CardHeader className={`pb-4 flex-shrink-0 border-b border-border ${
                parseErrors.length > 0 ? 'bg-red-50/30 dark:bg-red-950/10' : 'bg-muted/20'
              }`}>
                <CardTitle className="text-lg font-bold flex items-center gap-3 text-foreground">
                  {t('channels.dialogs.bulkImport.previewTitle')}
                  {parsedChannels.length > 0 && (
                    <Badge variant="secondary" className="text-sm px-4 py-2 bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-400 border border-green-300 dark:border-green-700">
                      ✅ {t('channels.dialogs.bulkImport.validRecords', { count: parsedChannels.length })}
                    </Badge>
                  )}
                  {parseErrors.length > 0 && (
                    <Badge variant="destructive" className="text-sm px-4 py-2 bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-400 border border-red-300 dark:border-red-700">
                      ❌ {t('channels.dialogs.bulkImport.errors', { count: parseErrors.length })}
                    </Badge>
                  )}
                  {/* Import Status Indicator */}
                  {showPreview && (
                    <div className="ml-auto">
                      {parseErrors.length > 0 ? (
                        <div className="flex items-center gap-2 text-sm font-medium text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-950/20 px-3 py-1 rounded-md border border-red-200 dark:border-red-800">
                          🚫 Import Blocked
                        </div>
                      ) : parsedChannels.length > 0 ? (
                        <div className="flex items-center gap-2 text-sm font-medium text-green-600 dark:text-green-400 bg-green-50 dark:bg-green-950/20 px-3 py-1 rounded-md border border-green-200 dark:border-green-800">
                          ✅ Ready to Import
                        </div>
                      ) : null}
                    </div>
                  )}
                </CardTitle>
              </CardHeader>
              <CardContent className="flex-1 min-h-0 bg-background/50">
                <div className="h-full overflow-y-auto pr-4 space-y-4 bg-gradient-to-b from-background/80 to-muted/20 rounded-lg p-4" style={{ maxHeight: 'calc(95vh - 350px)' }}>
                  {/* Errors */}
                  {parseErrors.length > 0 && (
                    <div className="space-y-3 bg-red-50/50 dark:bg-red-950/20 p-4 rounded-lg border-2 border-red-200 dark:border-red-800">
                      <div className="text-base font-semibold text-red-700 dark:text-red-400 flex items-center gap-2 py-3 px-2 mb-4">
                        ❌ {t('channels.dialogs.bulkImport.errorMessages')}
                        <div className="ml-auto text-sm font-medium bg-red-100 dark:bg-red-900/40 text-red-800 dark:text-red-300 px-3 py-1 rounded-md border border-red-300 dark:border-red-700">
                          Import blocked - fix errors first
                        </div>
                      </div>
                      {parseErrors.map((error, index) => (
                        <Alert key={index} variant="destructive" className="border-red-300 dark:border-red-700 bg-red-50/70 dark:bg-red-950/30">
                          <XCircle className="h-4 w-4" />
                          <AlertDescription className="text-sm font-medium text-red-800 dark:text-red-300">{error}</AlertDescription>
                        </Alert>
                      ))}
                    </div>
                  )}

                  {/* Valid Channels */}
                  {parsedChannels.length > 0 && (
                    <div className="space-y-4 bg-muted/30 p-4 rounded-lg border border-border">
                      <div className="text-base font-semibold text-green-600 dark:text-green-400 flex items-center gap-2 py-3 px-2 mb-4">
                        ✅ {t('channels.dialogs.bulkImport.validChannels')}
                      </div>
                      <div className="grid gap-4">
                        {parsedChannels.map((channel, index) => (
                          <div key={index} className="border border-border rounded-lg p-4 space-y-3 bg-background hover:bg-muted/50 transition-colors duration-200">
                            <div className="flex items-center gap-3">
                              <Badge variant="outline" className="text-sm px-3 py-1 font-medium">
                                {channel.type}
                              </Badge>
                              <span className="font-bold text-base text-foreground">{channel.name}</span>
                            </div>
                            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3 text-sm">
                              {channel.baseURL && (
                                <div className="bg-muted p-3 rounded-lg border border-border">
                                  <span className="font-semibold text-muted-foreground">{t('channels.dialogs.bulkImport.fieldLabels.baseUrl')}:</span>
                                  <div className="font-mono text-blue-600 dark:text-blue-400 mt-1 break-all text-xs">{channel.baseURL}</div>
                                </div>
                              )}
                              {channel.apiKey && (
                                <div className="bg-muted p-3 rounded-lg border border-border">
                                  <span className="font-semibold text-muted-foreground">{t('channels.dialogs.bulkImport.fieldLabels.apiKey')}:</span>
                                  <div className="font-mono text-purple-600 dark:text-purple-400 mt-1 text-xs">{channel.apiKey.substring(0, 20)}...</div>
                                </div>
                              )}
                              <div className="bg-muted p-3 rounded-lg border border-border">
                                <span className="font-semibold text-muted-foreground">{t('channels.dialogs.bulkImport.fieldLabels.supportedModels')}:</span>
                                <div className="mt-1 flex flex-wrap gap-1">
                                  {channel.supportedModels.map((model, idx) => (
                                    <span key={idx} className="bg-background text-foreground px-2 py-1 rounded text-xs font-medium border border-border">
                                      {model}
                                    </span>
                                  ))}
                                </div>
                              </div>
                              <div className="bg-muted p-3 rounded-lg border border-border">
                                <span className="font-semibold text-muted-foreground">{t('channels.dialogs.bulkImport.fieldLabels.defaultTestModel')}:</span>
                                <div className="font-medium text-green-600 dark:text-green-400 mt-1 text-xs">{channel.defaultTestModel}</div>
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

        <DialogFooter className="pt-4 pb-2 gap-3 flex-col sm:flex-row flex-shrink-0 border-t border-border bg-muted/30">
          <Button 
            variant="outline" 
            onClick={handleClose} 
            size="lg" 
            className="px-8 py-2 w-full sm:w-auto text-sm font-medium border-2 hover:bg-muted"
          >
            {t('channels.dialogs.bulkImport.cancel')}
          </Button>
          {!hasPreviewedCurrent ? (
            <Button
              onClick={handlePreview}
              disabled={!textValue?.trim() || isLoadingChannelNames}
              size="lg"
              className="px-8 py-2 w-full sm:w-auto text-sm font-medium bg-primary hover:bg-primary/90"
            >
              {isLoadingChannelNames && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {t('channels.dialogs.bulkImport.previewButton')}
            </Button>
          ) : (
            <Button
              onClick={handleImport}
              disabled={!showPreview || parsedChannels.length === 0 || parseErrors.length > 0 || bulkImportMutation.isPending}
              size="lg"
              className="px-8 py-2 w-full sm:w-auto text-sm font-medium bg-primary hover:bg-primary/90"
            >
              {bulkImportMutation.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {t('channels.dialogs.bulkImport.importButton', { count: parsedChannels.length })}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}