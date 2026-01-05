import { useState, useEffect, useMemo } from 'react';
import { Copy, Eye, EyeOff, AlertTriangle } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import { MaskedCodeBlock, MaskedCodeBlockCopyButton, highlightMaskedCode } from '@/components/ai-elements/masked-code-block';
import { useApiKeysContext } from '../context/apikeys-context';

export function ApiKeysViewDialog() {
  const { t } = useTranslation();
  const { isDialogOpen, closeDialog, selectedApiKey } = useApiKeysContext();
  const [isVisible, setIsVisible] = useState(false);
  const [preRenderedCode, setPreRenderedCode] = useState<Record<string, { light: string; dark: string }>>({});

  const apiKey = selectedApiKey?.key || '';
  const maskedApiKey = selectedApiKey?.key ? 'sk-...' + selectedApiKey.key.slice(-4) : '';

  const codeExamples = useMemo(() => {
    if (!selectedApiKey?.key) return {};

    return {
      codex: {
        display: `# Set your API key as an environment variable
export AXONHUB_API_KEY="${maskedApiKey}"

# Edit \${HOME}/.codex/config.toml and configure AxonHub:
model = "gpt-5"
model_provider = "axonhub-responses"

[model_providers.axonhub-responses]
name = "AxonHub using Chat Completions"
base_url = "http://127.0.0.1:8090/v1"
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
base_url = "http://127.0.0.1:8090/v1"
env_key = "AXONHUB_API_KEY"
wire_api = "responses"
query_params = {}

# Restart Codex to apply the configuration`
      },
      claudeCode: {
        display: `# In your terminal, set the API key
export ANTHROPIC_AUTH_TOKEN="${maskedApiKey}"
export ANTHROPIC_BASE_URL="http://localhost:8090/anthropic"

# Then launch Claude Code
claude

# Or use the --api-key flag with the base URL
claude --api-key "${maskedApiKey}" --base-url "http://localhost:8090/anthropic" "Hello, Claude!"

# The configuration will be stored in ~/.config/claude/config.json`,
        real: `# In your terminal, set the API key
export ANTHROPIC_AUTH_TOKEN="${apiKey}"
export ANTHROPIC_BASE_URL="http://localhost:8090/anthropic"

# Then launch Claude Code
claude

# Or use the --api-key flag with the base URL
claude --api-key "${apiKey}" --base-url "http://localhost:8090/anthropic" "Hello, Claude!"

# The configuration will be stored in ~/.config/claude/config.json`
      },
      anthropicSDK: {
        display: `import anthropic

client = anthropic.Anthropic(
    api_key="${maskedApiKey}",
    base_url="http://localhost:8090/anthropic"
)

message = client.messages.create(
    model="claude-3-5-sonnet-20241022",
    max_tokens=1024,
    messages=[
        {
            "role": "user",
            "content": "Hello, Claude!"
        }
    ]
)

print(message.content[0].text)`,
        real: `import anthropic

client = anthropic.Anthropic(
    api_key="${apiKey}",
    base_url="http://localhost:8090/anthropic"
)

message = client.messages.create(
    model="claude-3-5-sonnet-20241022",
    max_tokens=1024,
    messages=[
        {
            "role": "user",
            "content": "Hello, Claude!"
        }
    ]
)

print(message.content[0].text)`
      },
      openAISDK: {
        display: `from openai import OpenAI

client = OpenAI(
    api_key="${maskedApiKey}",
    base_url="http://localhost:8090/v1"
)

response = client.chat.completions.create(
    model="gpt-4o",
    messages=[
        {"role": "user", "content": "Hello, Claude!"}
    ]
)

print(response.choices[0].message.content)`,
        real: `from openai import OpenAI

client = OpenAI(
    api_key="${apiKey}",
    base_url="http://localhost:8090/v1"
)

response = client.chat.completions.create(
    model="gpt-4o",
    messages=[
        {"role": "user", "content": "Hello, Claude!"}
    ]
)

print(response.choices[0].message.content)`
      },
      geminiSDK: {
        display: `import google.generativeai as genai

genai.configure(api_key="${maskedApiKey}")

model = genai.GenerativeModel(
    'gemini-pro',
    generation_config=genai.GenerationConfig(
        temperature=0.7,
        max_output_tokens=1024,
    )
)

response = model.generate_content("Hello!")

print(response.text)`,
        real: `import google.generativeai as genai

genai.configure(api_key="${apiKey}")

model = genai.GenerativeModel(
    'gemini-pro',
    generation_config=genai.GenerationConfig(
        temperature=0.7,
        max_output_tokens=1024,
    )
)

response = model.generate_content("Hello!")

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

  const copyToClipboard = () => {
    if (selectedApiKey?.key) {
      navigator.clipboard.writeText(selectedApiKey.key);
      toast.success(t('apikeys.messages.copied'));
    }
  };

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
            <label className='text-sm font-medium'>{t('apikeys.columns.name')}</label>
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
              <Button variant='outline' size='sm' onClick={copyToClipboard} className='flex-shrink-0'>
                <Copy className='h-4 w-4' />
              </Button>
            </div>
          </div>
        </div>

        <div className='flex-1 overflow-hidden flex flex-col'>
          <label className='text-sm font-medium'>{t('apikeys.dialogs.view.usageExamples')}</label>
          {selectedApiKey?.type === 'user' ? (
            <Tabs defaultValue='claudeCode' className='mt-2 flex-1 flex flex-col min-h-0'>
              <TabsList className='grid w-full grid-cols-5 shrink-0'>
                <TabsTrigger value='claudeCode'>{t('apikeys.dialogs.view.tabs.claudeCode')}</TabsTrigger>
                <TabsTrigger value='codex'>{t('apikeys.dialogs.view.tabs.codex')}</TabsTrigger>
                <TabsTrigger value='anthropicSDK'>{t('apikeys.dialogs.view.tabs.anthropicSDK')}</TabsTrigger>
                <TabsTrigger value='openAISDK'>{t('apikeys.dialogs.view.tabs.openAISDK')}</TabsTrigger>
                <TabsTrigger value='geminiSDK'>{t('apikeys.dialogs.view.tabs.geminiSDK')}</TabsTrigger>
              </TabsList>
              <TabsContent value='anthropicSDK' className='mt-3 min-h-0 flex-1 overflow-y-auto'>
                <MaskedCodeBlock displayCode={codeExamples?.anthropicSDK?.display || ''} realCode={codeExamples?.anthropicSDK?.real || ''} language='python' className='overflow-visible' preRenderedHtml={preRenderedCode.anthropicSDK}>
                  <MaskedCodeBlockCopyButton />
                </MaskedCodeBlock>
              </TabsContent>
              <TabsContent value='openAISDK' className='mt-3 min-h-0 flex-1 overflow-y-auto'>
                <MaskedCodeBlock displayCode={codeExamples?.openAISDK?.display || ''} realCode={codeExamples?.openAISDK?.real || ''} language='python' className='overflow-visible' preRenderedHtml={preRenderedCode.openAISDK}>
                  <MaskedCodeBlockCopyButton />
                </MaskedCodeBlock>
              </TabsContent>
              <TabsContent value='codex' className='mt-3 min-h-0 flex-1 overflow-y-auto'>
                <MaskedCodeBlock displayCode={codeExamples?.codex?.display || ''} realCode={codeExamples?.codex?.real || ''} language='bash' className='overflow-visible' preRenderedHtml={preRenderedCode.codex}>
                  <MaskedCodeBlockCopyButton />
                </MaskedCodeBlock>
              </TabsContent>
              <TabsContent value='claudeCode' className='mt-3 min-h-0 flex-1 overflow-y-auto'>
                <MaskedCodeBlock displayCode={codeExamples?.claudeCode?.display || ''} realCode={codeExamples?.claudeCode?.real || ''} language='bash' className='overflow-visible' preRenderedHtml={preRenderedCode.claudeCode}>
                  <MaskedCodeBlockCopyButton />
                </MaskedCodeBlock>
              </TabsContent>
              <TabsContent value='geminiSDK' className='mt-3 min-h-0 flex-1 overflow-y-auto'>
                <MaskedCodeBlock displayCode={codeExamples?.geminiSDK?.display || ''} realCode={codeExamples?.geminiSDK?.real || ''} language='python' className='overflow-visible' preRenderedHtml={preRenderedCode.geminiSDK}>
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
