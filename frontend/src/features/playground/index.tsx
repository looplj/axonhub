import { useState, useEffect, useCallback, useMemo, useRef } from 'react'
import { IconTrash, IconRefresh } from '@tabler/icons-react'
import { AIDevtools } from '@ai-sdk-tools/devtools'
import { useChat } from '@ai-sdk/react'
import { DefaultChatTransport } from 'ai'
import { MessageSquare } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/authStore'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Textarea } from '@/components/ui/textarea'
import { TooltipProvider } from '@/components/ui/tooltip'
import {
  Conversation,
  ConversationContent,
  ConversationEmptyState,
  ConversationScrollButton,
} from '@/components/ai-elements/conversation'
import { Message, MessageContent } from '@/components/ai-elements/message'
import { PromptInput, PromptInputTextarea, PromptInputSubmit } from '@/components/ai-elements/prompt-input'
import { Response } from '@/components/ai-elements/response'
import { Reasoning, ReasoningTrigger, ReasoningContent } from '@/components/ai-elements/reasoning'
import { useChannels } from '@/features/channels/data/channels'
import type { Channel } from '@/features/channels/data/schema'

export default function Playground() {
  const { t } = useTranslation()
  const [selectedGroupModel, setSelectedGroupModel] = useState('')
  const [model, setModel] = useState('gpt-4o')
  const [selectedChannel, setSelectedChannel] = useState<string | null>(null)
  const [temperature, setTemperature] = useState(0.7)
  const [maxTokens, setMaxTokens] = useState(1000)
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

  const { messages, sendMessage, status, setMessages, regenerate } = useChat({
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
    }),
  })
  const isLoading = status === 'submitted' || status === 'streaming'

  // Handle form submission
  const handleSubmit = useCallback(
    (message: { text?: string }, e: React.FormEvent) => {
      e.preventDefault()
      if (message.text?.trim()) {
        sendMessage({ text: message.text })
        setInput('')
      }
    },
    [sendMessage, selectedChannel]
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

  // 获取扁平化的模型列表（用于搜索）
  const allModels = useMemo(() => {
    if (!channelsData?.edges) return []

    const models = new Set<string>()
    const modelMap = new Map<
      string,
      {
        value: string
        label: string
        channel: Channel
      }
    >()

    channelsData.edges.forEach((edge) => {
      edge.node.supportedModels.forEach((model: string) => {
        if (!models.has(model)) {
          models.add(model)
          modelMap.set(model, {
            value: model,
            label: model,
            channel: edge.node,
          })
        }
      })
    })

    return Array.from(models).map((model) => modelMap.get(model)!)
  }, [channelsData])

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
                <Select value={selectedGroupModel} onValueChange={handleModelChange} disabled={channelsLoading}>
                  <SelectTrigger className='h-10'>
                    <SelectValue placeholder={channelsLoading ? t('loading') : t('playground.settings.selectModel')} />
                  </SelectTrigger>
                  <SelectContent className='max-h-[300px]'>
                    {groupedModels.length > 0 ? (
                      groupedModels.map((group, groupIndex) => (
                        <div key={group.channelName}>
                          {groupIndex > 0 && <Separator className='my-2' />}
                          <div className='text-muted-foreground px-2 py-1.5 text-xs font-semibold'>
                            {group.channelName} ({group.channelType})
                          </div>
                          {group.models.map((modelOption) => (
                            <SelectItem key={`${group.channelName}-${modelOption.value}`} value={modelOption.value}>
                              <div className='flex flex-col items-start'>
                                <span className='font-medium'>{modelOption.label}</span>
                                <span className='text-muted-foreground text-xs'>{group.channelName}</span>
                              </div>
                            </SelectItem>
                          ))}
                        </div>
                      ))
                    ) : (
                      <SelectItem value='gpt-4o' disabled>
                        {channelsLoading ? t('loading') : t('playground.errors.noChannelsAvailable')}
                      </SelectItem>
                    )}
                  </SelectContent>
                </Select>
                {channelsLoading && <p className='text-muted-foreground text-xs'>{t('loading')}...</p>}
                {!channelsLoading && allModels.length > 0 && (
                  <p className='text-muted-foreground text-xs'>
                    {t('playground.modelsAvailable', {
                      count: allModels.length,
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
                  messages.map((message) => (
                    <Message from={message.role} key={message.id}>
                      <MessageContent>
                        {message.parts?.map((part, index) => {
                          if (part.type === 'text') {
                            return <Response key={index}>{part.text}</Response>
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
                      </MessageContent>
                    </Message>
                  ))
                )}
              </ConversationContent>
              <ConversationScrollButton />
            </Conversation>

            <PromptInput onSubmit={handleSubmit} className='mt-4 w-full'>
              <PromptInputTextarea
                value={input}
                placeholder={t('playground.chat.typeMessage')}
                onChange={(e) => setInput(e.currentTarget.value)}
                className='pr-12'
              />
              <PromptInputSubmit
                status={status === 'streaming' ? 'streaming' : 'ready'}
                disabled={!input.trim()}
                className='absolute right-1 bottom-1'
              />
            </PromptInput>
          </div>
        </div>
      </div>
    </TooltipProvider>
  )
}
