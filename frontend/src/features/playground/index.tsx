import { useState, useEffect, useCallback, useMemo, useRef } from 'react'
import { IconTrash, IconRefresh } from '@tabler/icons-react'
import { useChat } from '@ai-sdk/react'
import { DefaultChatTransport } from 'ai'
import { MessageSquare, RefreshCcw, Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/authStore'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Textarea } from '@/components/ui/textarea'
import { TooltipProvider } from '@/components/ui/tooltip'
import { Actions, Action } from '@/components/ai-elements/actions'
import {
  Conversation,
  ConversationContent,
  ConversationEmptyState,
  ConversationScrollButton,
} from '@/components/ai-elements/conversation'
import { Loader } from '@/components/ai-elements/loader'
import { Message, MessageContent } from '@/components/ai-elements/message'
import { PromptInput, PromptInputTextarea, PromptInputSubmit } from '@/components/ai-elements/prompt-input'
import { Reasoning, ReasoningTrigger, ReasoningContent } from '@/components/ai-elements/reasoning'
import { Response as UIResponse } from '@/components/ai-elements/response'
import { AutoCompleteSelect } from '@/components/auto-complete-select'
import { useChannels } from '@/features/channels/data/channels'

export default function Playground() {
  const { t } = useTranslation()
  const [selectedGroupModel, setSelectedGroupModel] = useState('')
  const [model, setModel] = useState('gpt-4o')
  const [selectedChannel, setSelectedChannel] = useState<string | null>(null)
  const [temperature, setTemperature] = useState(0.6)
  const [maxTokens, setMaxTokens] = useState(4096)
  const [systemPrompt, setSystemPrompt] = useState(t('playground.settings.defaultSystemPrompt'))

  // useRef hooks for direct access to current values
  const modelRef = useRef(model)
  const temperatureRef = useRef(temperature)
  const maxTokensRef = useRef(maxTokens)
  const systemPromptRef = useRef(systemPrompt)
  const selectedChannelRef = useRef(selectedChannel)

  // Keep refs synchronized with state
  useEffect(() => {
    modelRef.current = model
  }, [model])

  useEffect(() => {
    temperatureRef.current = temperature
  }, [temperature])

  useEffect(() => {
    maxTokensRef.current = maxTokens
  }, [maxTokens])

  useEffect(() => {
    systemPromptRef.current = systemPrompt
  }, [systemPrompt])

  useEffect(() => {
    selectedChannelRef.current = selectedChannel
  }, [selectedChannel])

  const { accessToken } = useAuthStore((state) => state.auth)
  // 获取 channels 数据
  const { data: channelsData, isLoading: channelsLoading } = useChannels({
    first: 100,
    orderBy: { field: 'ORDERING_WEIGHT', direction: 'DESC' },
    where: {
      statusIn: ['enabled', 'disabled'],
    },
  })

  const [input, setInput] = useState('')

  const { messages, sendMessage, status, setMessages, regenerate, stop } = useChat({
    transport: new DefaultChatTransport({
      api: '/admin/playground/chat',
      credentials: 'include',
      headers: () => {
        return {
          Authorization: 'Bearer ' + accessToken,
          'X-Channel-ID': selectedChannelRef.current || '',
        }
      },
      body: () => {
        return {
          model: modelRef.current,
          temperature: temperatureRef.current,
          max_tokens: maxTokensRef.current,
          system: systemPromptRef.current,
        }
      },
      fetch: async (url, init) => {
        const res = await fetch(url, init)
        if (!res.ok) {
          let message = res.statusText || 'Request failed'
          let code: number | undefined = res.status
          try {
            const body = await res.clone().json()
            const upstream = body?.error?.message || body?.message
            const upstreamCode = typeof body?.error?.code === 'number' ? body.error.code : undefined
            if (upstream) message = upstream
            if (typeof upstreamCode === 'number') code = upstreamCode
          } catch {
            // ignore JSON parse errors
          }
          const err: any = new Error(message)
          err.status = res.status
          err.code = code
          err.response = res
          throw err
        }
        return res
      },
    }),
    onError: (error) => {
      const anyErr = error as any
      const status = anyErr?.status
      const codeFromError = typeof anyErr?.code === 'number' ? anyErr.code : undefined

      // If the error includes a fetch Response, try to parse JSON body
      const response: Response | undefined = anyErr?.response instanceof Response ? anyErr.response : undefined
      if (response) {
        response
          .clone()
          .json()
          .then((data: any) => {
            const upstreamMsg = data?.error?.message || data?.message
            const upstreamCode = typeof data?.error?.code === 'number' ? data.error.code : undefined
            const code = upstreamCode ?? codeFromError ?? status
            const message = upstreamMsg || error.message || 'Unknown error'
            const title = code ? `${code} ${message}` : message
            toast.error(title)
          })
          .catch(() => {
            const upstreamMsg = anyErr?.error?.message || anyErr?.response?.error?.message
            const code = codeFromError ?? status
            const message = upstreamMsg || error.message || 'Unknown error'
            const title = code ? `${code} ${message}` : message
            toast.error(title)
          })
        return
      }

      try {
        const upstreamMsg = anyErr?.error?.message || anyErr?.response?.error?.message
        const code = codeFromError ?? status
        const message = upstreamMsg || error.message || 'Unknown error'
        const title = code ? `${code} ${message}` : message
        toast.error(title)
      } catch {
        toast.error(error.message)
      }
    },
  })
  const isLoading = status === 'submitted' || status === 'streaming'

  // Handle form submission
  const handleSubmit = useCallback(
    (message: { text?: string }, e: React.FormEvent) => {
      e.preventDefault()
      // block submit while a request is in-flight
      if (isLoading) return
      if (message.text?.trim()) {
        sendMessage({ text: message.text })
        setInput('')
      }
    },
    [sendMessage, selectedChannel, isLoading]
  )

  const handleClear = useCallback(() => {
    setMessages([])
  }, [setMessages])

  const handleRetry = useCallback(() => {
    if (messages.length === 0) return

    // 找到最后一个助手消息的索引
    let lastAssistantIndex = -1
    for (let i = messages.length - 1; i >= 0; i--) {
      if (messages[i].role === 'assistant') {
        lastAssistantIndex = i
        break
      }
    }

    if (lastAssistantIndex !== -1) {
      // 移除最后一个助手消息及其之后的所有消息
      const newMessages = messages.slice(0, lastAssistantIndex)
      setMessages(newMessages)

      // 使用 setTimeout 确保状态更新后再调用 regenerate
      setTimeout(() => {
        regenerate()
      }, 100)
    } else {
      // 如果没有助手消息，直接重新发送
      regenerate()
    }
  }, [messages, regenerate, setMessages])

  // 获取按 channel 分组的模型列表
  const groupedModels = useMemo(() => {
    if (!channelsData?.edges) return []

    const channelGroups = channelsData.edges.map((edge) => ({
      channelName: edge.node.name,
      channelType: edge.node.type,
      models: edge.node.supportedModels.map((model) => ({
        value: edge.node.id + '|' + model,
        label: model,
        channel: edge.node,
      })),
    }))

    return channelGroups.filter((group) => group.models.length > 0)
  }, [channelsData])

  // 为选择准备的平面化模型项（value: channelId|model, label: "model — channel"）
  const modelOptions = useMemo(
    () =>
      groupedModels.flatMap((group) =>
        group.models.map((m) => ({ value: m.value, label: `${m.label} — ${group.channelName}` }))
      ),
    [groupedModels]
  )

  // 处理模型选择，同时设置对应的 channel
  const handleModelChange = useCallback(
    (newModel: string) => {
      setSelectedGroupModel(newModel)
      const parts = newModel.split('|')
      setModel(parts[1])
      setSelectedChannel(parts[0])
    },
    [setSelectedGroupModel, setModel, setSelectedChannel]
  )

  useEffect(() => {
    if (!selectedGroupModel && groupedModels.length > 0 && groupedModels[0].models.length > 0) {
      handleModelChange(groupedModels[0].models[0].value)
    }
  }, [groupedModels, handleModelChange, selectedGroupModel])

  return (
    <TooltipProvider>
      {/* {process.env.NODE_ENV === 'development' && (
        <AIDevtools
          config={{
            enabled: true,
            position: 'bottom',
            theme: 'dark',
            streamCapture: {
              enabled: true,
              endpoint: '/admin/playground/chat',
              autoConnect: true,
            },
          }}
          enabled={true}
        />
      )} */}
      <div className='bg-background flex h-screen w-full'>
        {/* Settings Sidebar */}
        <div className='bg-muted/40 flex w-80 flex-col border-r'>
          <div className='border-b p-6'>
            <h1 className='text-2xl font-bold tracking-tight'>{t('playground.title')}</h1>
            <p className='text-muted-foreground mt-2 text-sm leading-relaxed'>{t('playground.description')}</p>
          </div>

          <ScrollArea className='flex-1 p-6'>
            <div className='space-y-8'>
              <div className='space-y-4'>
                <Label htmlFor='model' className='text-sm font-semibold'>
                  {t('playground.settings.model')}
                </Label>
                <AutoCompleteSelect
                  selectedValue={selectedGroupModel as string}
                  onSelectedValueChange={(v) => handleModelChange(v)}
                  items={modelOptions}
                  isLoading={channelsLoading}
                  emptyMessage={t('playground.errors.noChannelsAvailable')}
                  placeholder={channelsLoading ? t('loading') : t('playground.settings.selectModel')}
                />
                {channelsLoading && <p className='text-muted-foreground text-xs'>{t('loading')}...</p>}
                {!channelsLoading && modelOptions.length > 0 && (
                  <p className='text-muted-foreground text-xs'>
                    {t('playground.modelsAvailable', {
                      count: modelOptions.length,
                      channels: groupedModels.length,
                    })}
                  </p>
                )}
              </div>

              <div className='space-y-4'>
                <Label htmlFor='temperature' className='text-sm font-semibold'>
                  {t('playground.settings.temperature')}: {temperature}
                </Label>
                <div className='px-2'>
                  <Input
                    id='temperature'
                    type='range'
                    min='0'
                    max='2'
                    step='0.1'
                    value={temperature}
                    onChange={(e) => setTemperature(parseFloat(e.target.value))}
                    className='bg-muted h-2 w-full cursor-pointer appearance-none rounded-lg'
                  />
                  <div className='text-muted-foreground mt-1 flex justify-between text-xs'>
                    <span>0</span>
                    <span>1</span>
                    <span>2</span>
                  </div>
                </div>
              </div>

              <div className='space-y-4'>
                <Label htmlFor='maxTokens' className='text-sm font-semibold'>
                  {t('playground.settings.maxTokens')}
                </Label>
                <Input
                  id='maxTokens'
                  type='number'
                  min='1'
                  max='4000'
                  value={maxTokens}
                  onChange={(e) => setMaxTokens(parseInt(e.target.value))}
                  className='h-10'
                />
              </div>

              <div className='space-y-4'>
                <Label htmlFor='systemPrompt' className='text-sm font-semibold'>
                  {t('playground.settings.systemPrompt')}
                </Label>
                <Textarea
                  id='systemPrompt'
                  placeholder={t('playground.settings.defaultSystemPrompt')}
                  value={systemPrompt}
                  onChange={(e) => setSystemPrompt(e.target.value)}
                  rows={4}
                  className='min-h-[100px] resize-none'
                />
              </div>
            </div>
          </ScrollArea>

          <div className='space-y-3 border-t p-6'>
            <Button
              onClick={handleRetry}
              variant='outline'
              className='h-10 w-full'
              disabled={isLoading || messages.length === 0 || messages.every((msg) => msg.role !== 'assistant')}
            >
              <IconRefresh className='mr-2 h-4 w-4' />
              {isLoading
                ? t('playground.chat.generating')
                : messages.length === 0
                  ? t('playground.chat.noMessages')
                  : messages.every((msg) => msg.role !== 'assistant')
                    ? t('playground.chat.noMessages')
                    : t('playground.chat.retry')}
            </Button>

            <Button onClick={handleClear} variant='outline' className='h-10 w-full' disabled={isLoading}>
              <IconTrash className='mr-2 h-4 w-4' />
              {t('playground.chat.clear')}
            </Button>
          </div>
        </div>

        {/* Chat Area */}
        <div className='flex flex-1 flex-col p-6'>
          <div className='flex h-full flex-col'>
            <Conversation className='flex-1'>
              <ConversationContent>
                {messages.length === 0 ? (
                  <ConversationEmptyState
                    icon={<MessageSquare className='size-12' />}
                    title={t('playground.chat.startConversation')}
                    description={t('playground.chat.typeMessageBelow')}
                  />
                ) : (
                  (() => {
                    const lastAssistantIndex = (() => {
                      for (let i = messages.length - 1; i >= 0; i--) {
                        if (messages[i].role === 'assistant') return i
                      }
                      return -1
                    })()
                    return messages.map((message, messageIndex) => {
                      const isLastAssistant = message.role === 'assistant' && messageIndex === lastAssistantIndex
                      const textContent = message.parts
                        ?.filter((p) => p.type === 'text')
                        .map((p: any) => p.text)
                        .join('\n')
                      return (
                        <Message from={message.role} key={message.id}>
                          <MessageContent>
                            {message.parts?.map((part, index) => {
                              if (part.type === 'text') {
                                return <UIResponse key={index}>{part.text}</UIResponse>
                              }
                              if (part.type === 'reasoning') {
                                return (
                                  <Reasoning key={index} isStreaming={status === 'streaming'}>
                                    <ReasoningTrigger />
                                    <ReasoningContent>{part.text}</ReasoningContent>
                                  </Reasoning>
                                )
                              }
                              return null
                            })}
                            {isLastAssistant && textContent ? (
                              <Actions className='mt-2'>
                                <Action onClick={() => regenerate()} label={t('playground.chat.retry')}>
                                  <RefreshCcw className='size-3' />
                                </Action>
                                <Action onClick={() => navigator.clipboard.writeText(textContent)} label={t('copy')}>
                                  <Copy className='size-3' />
                                </Action>
                              </Actions>
                            ) : null}
                          </MessageContent>
                        </Message>
                      )
                    })
                  })()
                )}
                {status === 'submitted' && <Loader />}
              </ConversationContent>
              <ConversationScrollButton />
            </Conversation>

            <PromptInput onSubmit={handleSubmit} className='relative mt-4 w-full'>
              <PromptInputTextarea
                value={input}
                placeholder={t('playground.chat.typeMessage')}
                onChange={(e) => setInput(e.currentTarget.value)}
                className='pr-16'
              />
              <PromptInputSubmit
                status={status}
                disabled={status === 'ready' ? !input.trim() : false}
                // className='absolute right-2 top-1/2 -translate-y-1/2'
                className='absolute right-3 bottom-3'

                onClick={(e) => {
                  // When not ready (submitted/streaming/error), treat click as cancel
                  if (status !== 'ready') {
                    e.preventDefault()
                    stop()
                  }
                }}
              />
            </PromptInput>
          </div>
        </div>
      </div>
    </TooltipProvider>
  )
}
