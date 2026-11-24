import { Fragment, useCallback, useRef, useState } from 'react';
import { z } from 'zod';
import { ImageIcon } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { appPath } from '@/lib/app-path';
import { ensureFreshAccessToken } from '@/lib/auth-session';
import { Conversation, ConversationContent, ConversationEmptyState, ConversationScrollButton } from '@/components/ai-elements/conversation';
import { Loader } from '@/components/ai-elements/loader';
import { Message, MessageContent } from '@/components/ai-elements/message';
import type { PlaygroundModelSource } from './selection';

// Upstream-controlled schema: declare only the fields we read.
const imagesResponseSchema = z.object({
  data: z
    .array(
      z.object({
        url: z.string().optional(),
        b64_json: z.string().optional(),
      })
    )
    .default([]),
  output_format: z.string().optional(),
});

const playgroundErrorSchema = z.object({
  error: z.object({
    code: z.number().optional(),
    message: z.string().optional(),
  }),
});

export type ImageGeneration = {
  id: string;
  prompt: string;
  status: 'pending' | 'done' | 'error';
  images: string[];
  error?: string;
};

export type ImageGenerationParams = {
  projectId: string | null;
  modelSource: PlaygroundModelSource;
  selectedChannel: string;
  model: string;
  size: string;
  count: number;
};

export function useImageGeneration(params: ImageGenerationParams) {
  const [generations, setGenerations] = useState<ImageGeneration[]>([]);
  const [isGenerating, setIsGenerating] = useState(false);
  const abortRef = useRef<AbortController | null>(null);

  const generate = useCallback(
    async (prompt: string) => {
      if (isGenerating) return;
      const entry: ImageGeneration = {
        id: crypto.randomUUID(),
        prompt,
        status: 'pending',
        images: [],
      };
      setGenerations((current) => [...current, entry]);
      setIsGenerating(true);

      const controller = new AbortController();
      abortRef.current = controller;

      try {
        const token = await ensureFreshAccessToken();
        const headers: Record<string, string> = {
          'Content-Type': 'application/json',
          'X-Project-ID': params.projectId || '',
        };
        if (token) {
          headers.Authorization = `Bearer ${token}`;
        }
        if (params.modelSource === 'channel' && params.selectedChannel) {
          headers['X-Channel-ID'] = params.selectedChannel;
        }
        const body: Record<string, unknown> = {
          model: params.model,
          prompt,
        };
        const size = params.size.trim();
        if (size) {
          body.size = size;
        }
        if (params.count > 1) {
          body.n = params.count;
        }

        const res = await fetch(appPath('/admin/playground/images'), {
          method: 'POST',
          credentials: 'include',
          headers,
          body: JSON.stringify(body),
          signal: controller.signal,
        });

        let parsedBody: unknown = null;
        try {
          parsedBody = await res.json();
        } catch {
          // Non-JSON error bodies fall through to the status text below.
        }

        if (!res.ok) {
          const parsed = playgroundErrorSchema.safeParse(parsedBody);
          const message =
            parsed.success && parsed.data.error.message
              ? `${parsed.data.error.code ? `${parsed.data.error.code} ` : ''}${parsed.data.error.message}`
              : res.statusText || 'Request failed';
          throw new Error(message);
        }

        const parsed = imagesResponseSchema.parse(parsedBody);
        const mediaType = parsed.output_format ? `image/${parsed.output_format}` : 'image/png';
        const images = parsed.data
          .map((item) => item.url || (item.b64_json ? `data:${mediaType};base64,${item.b64_json}` : ''))
          .filter((url) => url !== '');

        setGenerations((current) => current.map((item) => (item.id === entry.id ? { ...item, status: 'done', images } : item)));
      } catch (error) {
        if (error instanceof DOMException && error.name === 'AbortError') {
          setGenerations((current) => current.filter((item) => item.id !== entry.id));
        } else {
          const message = error instanceof Error ? error.message : 'Unknown error';
          setGenerations((current) => current.map((item) => (item.id === entry.id ? { ...item, status: 'error', error: message } : item)));
          toast.error(message);
        }
      } finally {
        abortRef.current = null;
        setIsGenerating(false);
      }
    },
    [isGenerating, params]
  );

  const stop = useCallback(() => {
    abortRef.current?.abort();
  }, []);

  const clear = useCallback(() => {
    abortRef.current?.abort();
    setGenerations([]);
  }, []);

  const retryLast = useCallback(() => {
    const lastPrompt = generations.at(-1)?.prompt;
    if (lastPrompt) {
      void generate(lastPrompt);
    }
  }, [generate, generations]);

  return { generations, isGenerating, generate, stop, clear, retryLast };
}

export function ImageConversation({ generations }: { generations: ImageGeneration[] }) {
  const { t } = useTranslation();

  return (
    <Conversation className='max-h-[50vh] min-h-0 flex-1 lg:max-h-none'>
      <ConversationContent>
        {generations.length === 0 ? (
          <ConversationEmptyState
            icon={<ImageIcon className='size-12' />}
            title={t('playground.image.startGenerating')}
            description={t('playground.image.typePromptBelow')}
          />
        ) : (
          generations.map((generation) => (
            <Fragment key={generation.id}>
              <Message from='user'>
                <MessageContent>{generation.prompt}</MessageContent>
              </Message>
              <Message from='assistant'>
                <MessageContent>
                  {generation.status === 'pending' && <Loader />}
                  {generation.status === 'error' && <p className='text-destructive text-sm whitespace-pre-wrap'>{generation.error}</p>}
                  {generation.status === 'done' &&
                    generation.images.map((image, index) => (
                      <img
                        key={index}
                        alt={generation.prompt}
                        className='max-h-[384px] w-auto rounded-md border object-contain'
                        src={image}
                      />
                    ))}
                  {generation.status === 'done' && generation.images.length === 0 && (
                    <p className='text-muted-foreground text-sm'>{t('playground.image.emptyResponse')}</p>
                  )}
                </MessageContent>
              </Message>
            </Fragment>
          ))
        )}
      </ConversationContent>
      <ConversationScrollButton />
    </Conversation>
  );
}
