import { useState, useEffect, useMemo } from 'react';
import { Copy, Eye, EyeOff, AlertTriangle, Link, CheckIcon } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import { MaskedCodeBlock, MaskedCodeBlockCopyButton, highlightMaskedCode } from '@/components/ai-elements/masked-code-block';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard';
import { useQueryAllModels } from '@/features/models/data/models';
import { useApiKey } from '../data/apikeys';
import { buildPiModelsConfig, serializePiModelsConfig } from '../data/pi-models-config';
import { useApiKeysContext } from '../context/apikeys-context';

function CopyBaseUrlButton({ baseUrl }: { baseUrl: string }) {
  const { t } = useTranslation();
  const { isCopied, handleCopy } = useCopyToClipboard({
    text: baseUrl,
    copyMessage: t('apikeys.messages.baseUrlCopied'),
  });

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button size='icon' variant='ghost' className='shrink-0' onClick={handleCopy}>
          {isCopied ? <CheckIcon className='h-3.5 w-3.5 text-green-500' /> : <Link className='h-3.5 w-3.5' />}
        </Button>
      </TooltipTrigger>
      <TooltipContent>{baseUrl}</TooltipContent>
    </Tooltip>
  );
}

export function ApiKeysViewDialog() {
  const { t } = useTranslation();
  const { isDialogOpen, closeDialog, selectedApiKey } = useApiKeysContext();
  const [isVisible, setIsVisible] = useState(false);
  const [preRenderedCode, setPreRenderedCode] = useState<Record<string, { light: string; dark: string }>>({});
  const [piPreRendered, setPiPreRendered] = useState<{ source: string; light: string; dark: string }>();
  const [activeExampleTab, setActiveExampleTab] = useState('claudeCode');
  const { handleCopy: copyApiKey } = useCopyToClipboard({
    text: selectedApiKey?.key ?? '',
    copyMessage: t('apikeys.messages.copied'),
  });

  const apiKey = selectedApiKey?.key || '';
  const maskedApiKey = selectedApiKey?.key ? selectedApiKey.key.slice(0, 3) + '...' + selectedApiKey.key.slice(-4) : '';

  const currentOrigin = typeof window !== 'undefined' ? window.location.origin : 'http://localhost:8090';

  // The key detail carries the active profile's model allowlist, which the list row omits.
  const detailApiKeyId = isDialogOpen.view ? selectedApiKey?.id ?? '' : '';
  const { data: apiKeyDetail } = useApiKey(detailApiKeyId);
  const { data: enabledModels } = useQueryAllModels({ where: { statusIn: ['enabled'] } }, { enabled: !!detailApiKeyId });

  const piConfig = useMemo(() => {
    // The list row omits profiles, so wait for the detail before trusting the allowlist.
    if (!detailApiKeyId || !apiKey || !apiKeyDetail) return undefined;

    const profiles = apiKeyDetail.profiles;
    const modelIDs = profiles?.profiles?.find((profile) => profile.name === profiles.activeProfile)?.modelIDs ?? [];
    const models = (enabledModels?.edges ?? [])
      .map((edge) => edge.node)
      .filter((model) => modelIDs.length === 0 || modelIDs.includes(model.modelID));

    return {
      config: buildPiModelsConfig({ origin: currentOrigin, apiKey, models }),
      masked: buildPiModelsConfig({ origin: currentOrigin, apiKey: maskedApiKey, models }),
    };
  }, [detailApiKeyId, apiKeyDetail, enabledModels, currentOrigin, apiKey, maskedApiKey]);

  const piConfigCode = useMemo(
    () =>
      piConfig && {
        baseUrl: `${currentOrigin}/v1`,
        display: serializePiModelsConfig(piConfig.masked),
        real: serializePiModelsConfig(piConfig.config),
      },
    [piConfig, currentOrigin]
  );

  // Highlighting a full model list is expensive, so cache it per rendered content.
  const piPreRenderedCode =
    piPreRendered && piConfigCode && piPreRendered.source === piConfigCode.display ? piPreRendered : undefined;

  useEffect(() => {
    if (activeExampleTab !== 'pi' || !piConfigCode || piPreRenderedCode) return;

    let cancelled = false;

    highlightMaskedCode(piConfigCode.display, 'json').then(([light, dark]) => {
      if (!cancelled) setPiPreRendered({ source: piConfigCode.display, light, dark });
    });

    return () => {
      cancelled = true;
    };
  }, [activeExampleTab, piConfigCode, piPreRenderedCode]);

  const codeExamples = useMemo(() => {
    if (!selectedApiKey?.key) return {};

    return {
      codex: {
        baseUrl: `${currentOrigin}/v1`,
        display: `# Set your API key as an environment variable
export AXONHUB_API_KEY="${maskedApiKey}"

# Edit \${HOME}/.codex/config.toml and configure AxonHub:
model = "gpt-5"
model_provider = "axonhub-responses"

[model_providers.axonhub-responses]
name = "AxonHub using Chat Completions"
base_url = "${currentOrigin}/v1"
env_key = "AXONHUB_API_KEY"
wire_api = "responses"
query_params = {}

# Restart Codex to apply the configuration`,
        real: `# Set your API key as an environment variable
export AXONHUB_API_KEY="${apiKey}"

# Edit \${HOME}/.codex/config.toml and configure AxonHub:
model = "gpt-5"
model_provider = "axonhub-responses"

[model_providers.axonhub-responses]
name = "AxonHub using Chat Completions"
base_url = "${currentOrigin}/v1"
env_key = "AXONHUB_API_KEY"
wire_api = "responses"
query_params = {}

# Restart Codex to apply the configuration`
      },
      claudeCode: {
        baseUrl: `${currentOrigin}/anthropic`,
        display: `# In your terminal, set the API key
export ANTHROPIC_AUTH_TOKEN="${maskedApiKey}"
export ANTHROPIC_BASE_URL="${currentOrigin}/anthropic"

# Then launch Claude Code
claude

# Or use the --api-key flag with the base URL
claude --api-key "${maskedApiKey}" --base-url "${currentOrigin}/anthropic" "Hello, Claude!"

# The configuration will be stored in ~/.config/claude/config.json`,
        real: `# In your terminal, set the API key
export ANTHROPIC_AUTH_TOKEN="${apiKey}"
export ANTHROPIC_BASE_URL="${currentOrigin}/anthropic"

# Then launch Claude Code
claude

# Or use the --api-key flag with the base URL
claude --api-key "${apiKey}" --base-url "${currentOrigin}/anthropic" "Hello, Claude!"

# The configuration will be stored in ~/.config/claude/config.json`
      },
      anthropicSDK: {
        baseUrl: `${currentOrigin}/anthropic`,
        display: `from anthropic import Anthropic

client = Anthropic(
    api_key="${maskedApiKey}",
    base_url="${currentOrigin}/anthropic"
)

message = client.messages.create(
    model="claude-sonnet-4-5-20250929",
    max_tokens=1024,
    messages=[
        {
            "role": "user",
            "content": "Hello, Claude!"
        }
    ]
)

print(message.content)`,
        real: `from anthropic import Anthropic

client = Anthropic(
    api_key="${apiKey}",
    base_url="${currentOrigin}/anthropic"
)

message = client.messages.create(
    model="claude-sonnet-4-5-20250929",
    max_tokens=1024,
    messages=[
        {
            "role": "user",
            "content": "Hello, Claude!"
        }
    ]
)

print(message.content)`
      },
      openAISDK: {
        baseUrl: `${currentOrigin}/v1`,
        display: `from openai import OpenAI

client = OpenAI(
    api_key="${maskedApiKey}",
    base_url="${currentOrigin}/v1"
)

response = client.responses.create(
    model="gpt-4o",
    input="Hello, Claude!"
)

print(response.output_text)`,
        real: `from openai import OpenAI

client = OpenAI(
    api_key="${apiKey}",
    base_url="${currentOrigin}/v1"
)

response = client.responses.create(
    model="gpt-4o",
    input="Hello, Claude!"
)

print(response.output_text)`
      },
      geminiSDK: {
        baseUrl: `${currentOrigin}/gemini`,
        display: `from google import genai
from google.genai import types

client = genai.Client(
    api_key="${maskedApiKey}",
    base_url="${currentOrigin}/gemini"
)

response = client.models.generate_content(
    model='gemini-2.5-flash',
    contents=types.Part.from_text(text='Hello!'),
    config=types.GenerateContentConfig(
        temperature=0.7,
        max_output_tokens=1024,
    ),
)

print(response.text)`,
        real: `from google import genai
from google.genai import types

client = genai.Client(
    api_key="${apiKey}",
    base_url="${currentOrigin}/gemini"
)

response = client.models.generate_content(
    model='gemini-2.5-flash',
    contents=types.Part.from_text(text='Hello!'),
    config=types.GenerateContentConfig(
        temperature=0.7,
        max_output_tokens=1024,
    ),
)

print(response.text)`
      }
    };
  }, [selectedApiKey?.key, apiKey, maskedApiKey]);

  useEffect(() => {
    if (!selectedApiKey?.key || Object.keys(codeExamples).length === 0) return;

    const languages: Record<string, 'bash' | 'python'> = {
      codex: 'bash',
      claudeCode: 'bash',
      anthropicSDK: 'python',
      openAISDK: 'python',
      geminiSDK: 'python'
    };

    const renderAllCodeBlocks = async () => {
      const results: Record<string, { light: string; dark: string }> = {};
      
      await Promise.all(
        Object.entries(codeExamples).map(async ([key, example]) => {
          const [light, dark] = await highlightMaskedCode(example.display, languages[key]);
          results[key] = { light, dark };
        })
      );

      setPreRenderedCode(results);
    };

    renderAllCodeBlocks();
  }, [selectedApiKey?.key, codeExamples]);

  const maskedKey = selectedApiKey?.key ? selectedApiKey.key.replace(/./g, '*').slice(0, -4) + selectedApiKey.key.slice(-4) : '';

  return (
    <Dialog open={isDialogOpen.view} onOpenChange={() => closeDialog()}>
      <DialogContent className='flex max-h-[90vh] flex-col sm:max-w-3xl'>
        <DialogHeader>
          <DialogTitle>{t('apikeys.dialogs.view.title')}</DialogTitle>
          <DialogDescription>{t('apikeys.dialogs.view.description')}</DialogDescription>
        </DialogHeader>

        <Alert className='border-orange-200 bg-orange-50 dark:border-orange-800 dark:bg-orange-950 shrink-0'>
          <AlertTriangle className='h-4 w-4 text-orange-600 dark:text-orange-400' />
          <AlertDescription className='text-orange-800 dark:text-orange-200'>{t('apikeys.dialogs.view.warning')}</AlertDescription>
        </Alert>

        <div className='space-y-4 shrink-0'>
          <div>
            <label className='text-sm font-medium'>{t('common.columns.name')}</label>
            <div className='bg-muted mt-1 rounded-md p-3'>{selectedApiKey?.name}</div>
          </div>

          <div>
            <label className='text-sm font-medium'>{t('apikeys.columns.key')}</label>
            <div className='mt-1 flex items-center space-x-2'>
              <code className='bg-muted flex-1 rounded-md p-3 font-mono text-sm break-all'>
                {isVisible ? selectedApiKey?.key : maskedKey}
              </code>
              <Button variant='outline' size='sm' onClick={() => setIsVisible(!isVisible)} className='flex-shrink-0'>
                {isVisible ? <EyeOff className='h-4 w-4' /> : <Eye className='h-4 w-4' />}
              </Button>
              <Button variant='outline' size='sm' onClick={copyApiKey} className='flex-shrink-0'>
                <Copy className='h-4 w-4' />
              </Button>
            </div>
          </div>
        </div>

        <div className='flex-1 overflow-hidden flex flex-col'>
          <label className='text-sm font-medium'>{t('apikeys.dialogs.view.usageExamples')}</label>
          {selectedApiKey?.type === 'user' || selectedApiKey?.type === 'personal' ? (
            <Tabs value={activeExampleTab} onValueChange={setActiveExampleTab} className='mt-2 flex-1 flex flex-col min-h-0'>
              <TabsList className='grid w-full grid-cols-6 shrink-0'>
                <TabsTrigger value='claudeCode'>{t('apikeys.dialogs.view.tabs.claudeCode')}</TabsTrigger>
                <TabsTrigger value='codex'>{t('apikeys.dialogs.view.tabs.codex')}</TabsTrigger>
                <TabsTrigger value='pi'>{t('apikeys.dialogs.view.tabs.pi')}</TabsTrigger>
                <TabsTrigger value='anthropicSDK'>{t('apikeys.dialogs.view.tabs.anthropicSDK')}</TabsTrigger>
                <TabsTrigger value='openAISDK'>{t('apikeys.dialogs.view.tabs.openAISDK')}</TabsTrigger>
                <TabsTrigger value='geminiSDK'>{t('apikeys.dialogs.view.tabs.geminiSDK')}</TabsTrigger>
              </TabsList>
              <TabsContent value='pi' className='mt-3 min-h-0 flex-1 flex flex-col gap-2 overflow-y-auto'>
                <p className='text-muted-foreground text-xs'>{t('apikeys.dialogs.view.piHint')}</p>
                <MaskedCodeBlock displayCode={piConfigCode?.display || ''} realCode={piConfigCode?.real || ''} language='json' className='overflow-visible' preRenderedHtml={piPreRenderedCode}>
                  <CopyBaseUrlButton baseUrl={piConfigCode?.baseUrl || ''} />
                  <MaskedCodeBlockCopyButton />
                </MaskedCodeBlock>
              </TabsContent>
              <TabsContent value='anthropicSDK' className='mt-3 min-h-0 flex-1 overflow-y-auto'>
                <MaskedCodeBlock displayCode={codeExamples?.anthropicSDK?.display || ''} realCode={codeExamples?.anthropicSDK?.real || ''} language='python' className='overflow-visible' preRenderedHtml={preRenderedCode.anthropicSDK}>
                  <CopyBaseUrlButton baseUrl={codeExamples?.anthropicSDK?.baseUrl || ''} />
                  <MaskedCodeBlockCopyButton />
                </MaskedCodeBlock>
              </TabsContent>
              <TabsContent value='openAISDK' className='mt-3 min-h-0 flex-1 overflow-y-auto'>
                <MaskedCodeBlock displayCode={codeExamples?.openAISDK?.display || ''} realCode={codeExamples?.openAISDK?.real || ''} language='python' className='overflow-visible' preRenderedHtml={preRenderedCode.openAISDK}>
                  <CopyBaseUrlButton baseUrl={codeExamples?.openAISDK?.baseUrl || ''} />
                  <MaskedCodeBlockCopyButton />
                </MaskedCodeBlock>
              </TabsContent>
              <TabsContent value='codex' className='mt-3 min-h-0 flex-1 overflow-y-auto'>
                <MaskedCodeBlock displayCode={codeExamples?.codex?.display || ''} realCode={codeExamples?.codex?.real || ''} language='bash' className='overflow-visible' preRenderedHtml={preRenderedCode.codex}>
                  <CopyBaseUrlButton baseUrl={codeExamples?.codex?.baseUrl || ''} />
                  <MaskedCodeBlockCopyButton />
                </MaskedCodeBlock>
              </TabsContent>
              <TabsContent value='claudeCode' className='mt-3 min-h-0 flex-1 overflow-y-auto'>
                <MaskedCodeBlock displayCode={codeExamples?.claudeCode?.display || ''} realCode={codeExamples?.claudeCode?.real || ''} language='bash' className='overflow-visible' preRenderedHtml={preRenderedCode.claudeCode}>
                  <CopyBaseUrlButton baseUrl={codeExamples?.claudeCode?.baseUrl || ''} />
                  <MaskedCodeBlockCopyButton />
                </MaskedCodeBlock>
              </TabsContent>
              <TabsContent value='geminiSDK' className='mt-3 min-h-0 flex-1 overflow-y-auto'>
                <MaskedCodeBlock displayCode={codeExamples?.geminiSDK?.display || ''} realCode={codeExamples?.geminiSDK?.real || ''} language='python' className='overflow-visible' preRenderedHtml={preRenderedCode.geminiSDK}>
                  <CopyBaseUrlButton baseUrl={codeExamples?.geminiSDK?.baseUrl || ''} />
                  <MaskedCodeBlockCopyButton />
                </MaskedCodeBlock>
              </TabsContent>
            </Tabs>
          ) : (
            <div className='mt-2 flex-1 flex items-center justify-center text-muted-foreground text-sm'>
              {t('apikeys.dialogs.view.noExamples')}
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
