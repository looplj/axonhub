import { useState, useRef, useEffect, useCallback, useMemo } from 'react'
import { IconTrash, IconRefresh } from '@tabler/icons-react'
import { useChat } from '@ai-sdk/react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/authStore'
import { Button } from '@/components/ui/button'
import { Chat } from '@/components/ui/chat'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Textarea } from '@/components/ui/textarea'
import { TooltipProvider } from '@/components/ui/tooltip'
import { useChannels } from '@/features/channels/data/channels'
import type { Channel } from '@/features/channels/data/schema'

export default function Playground() {
  const { t } = useTranslation()
  const [selectedGroupModel, setSelectedGroupModel] = useState('')
  const [model, setModel] = useState('gpt-4o')
  const [selectedChannel, setSelectedChannel] = useState<string | null>(null)
  const [temperature, setTemperature] = useState(0.7)
  const [maxTokens, setMaxTokens] = useState(1000)
  const [systemPrompt, setSystemPrompt] = useState(
    t('playground.settings.defaultSystemPrompt')
  )
  const { accessToken } = useAuthStore((state) => state.auth)
  // 获取 channels 数据
  const { data: channelsData, isLoading: channelsLoading } = useChannels({
    first: 100,
    orderBy: { field: 'CREATED_AT', direction: 'DESC' },
    where: {
      statusIn: ['enabled', 'disabled'],
    },
  })

  const inputRef = useRef<HTMLTextAreaElement>(null)

  const {
    messages,
    input,
    handleInputChange,
    handleSubmit,
    status,
    reload,
    stop,
    setMessages,
  } = useChat({
    // streamProtocol: 'text',
    api: '/admin/v1/playground/chat',
    initialMessages: [],
    credentials: 'include',
    headers: {
      Authorization: 'Bearer ' + accessToken,
    },
    body: {
      // stream: true,
      model,
      temperature,
      max_tokens: maxTokens,
      system: systemPrompt,
    },
    fetch: async (
      input: string | URL | globalThis.Request,
      init?: RequestInit
    ) => {
      // eslint-disable-next-line @typescript-eslint/ban-ts-comment
      //@ts-ignore
      if (input.headers) {
        // eslint-disable-next-line @typescript-eslint/ban-ts-comment
        //@ts-ignore
        input.headers.set('X-Channel-ID', selectedChannel)
      } else {
        input = input + '?channel_id=' + selectedChannel
      }
      return fetch(input, init)
    },
  })
  const isLoading = status === 'submitted' || status === 'streaming'

  // Focus input on mount
  useEffect(() => {
    if (inputRef.current) {
      inputRef.current.focus()
    }
  }, [])

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

      // 使用 setTimeout 确保状态更新后再调用 reload
      setTimeout(() => {
        reload()
      }, 100)
    } else {
      // 如果没有助手消息，直接重新发送
      reload()
    }
  }, [messages, reload, setMessages])

  // 重试特定消息
  const handleRetryFromMessage = (messageId: string) => {
    const messageIndex = messages.findIndex((msg) => msg.id === messageId)
    if (messageIndex === -1) return

    // 移除该消息及其之后的所有消息
    const newMessages = messages.slice(0, messageIndex)
    setMessages(newMessages)

    // 使用 setTimeout 确保状态更新后再调用 reload
    setTimeout(() => {
      reload()
    }, 100)
  }

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
    if (!selectedGroupModel) {
      handleModelChange(groupedModels[0].models[0].value)
    }
  }, [groupedModels, handleModelChange])

  // 处理消息评分和重试
  const handleRateResponse = (
    messageId: string,
    rating: 'thumbs-up' | 'thumbs-down'
  ) => {
    if (rating === 'thumbs-down') {
      // 当用户点击 thumbs-down 时，触发重试
      handleRetryFromMessage(messageId)
    }
    // 处理 thumbs-up 可以在这里添加其他逻辑
  }

  return (
    <TooltipProvider>
      <div className='bg-background flex h-screen w-full'>
        {/* Settings Sidebar */}
        <div className='bg-muted/40 flex w-80 flex-col border-r'>
          <div className='border-b p-6'>
            <h1 className='text-2xl font-bold tracking-tight'>
              {t('playground.title')}
            </h1>
            <p className='text-muted-foreground mt-2 text-sm leading-relaxed'>
              {t('playground.description')}
            </p>
          </div>

          <ScrollArea className='flex-1 p-6'>
            <div className='space-y-8'>
              <div className='space-y-4'>
                <Label htmlFor='model' className='text-sm font-semibold'>
                  {t('playground.settings.model')}
                </Label>
                <Select
                  value={selectedGroupModel}
                  onValueChange={handleModelChange}
                  disabled={channelsLoading}
                >
                  <SelectTrigger className='h-10'>
                    <SelectValue
                      placeholder={
                        channelsLoading
                          ? t('loading')
                          : t('playground.settings.selectModel')
                      }
                    />
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
                            <SelectItem
                              key={`${group.channelName}-${modelOption.value}`}
                              value={modelOption.value}
                            >
                              <div className='flex flex-col items-start'>
                                <span className='font-medium'>
                                  {modelOption.label}
                                </span>
                                <span className='text-muted-foreground text-xs'>
                                  {group.channelName}
                                </span>
                              </div>
                            </SelectItem>
                          ))}
                        </div>
                      ))
                    ) : (
                      <SelectItem value='gpt-4o' disabled>
                        {channelsLoading
                          ? t('loading')
                          : t('playground.errors.noChannelsAvailable')}
                      </SelectItem>
                    )}
                  </SelectContent>
                </Select>
                {channelsLoading && (
                  <p className='text-muted-foreground text-xs'>
                    {t('loading')}...
                  </p>
                )}
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
              disabled={
                isLoading ||
                messages.length === 0 ||
                messages.every((msg) => msg.role !== 'assistant')
              }
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

            <Button
              onClick={handleClear}
              variant='outline'
              className='h-10 w-full'
              disabled={isLoading}
            >
              <IconTrash className='mr-2 h-4 w-4' />
              {t('playground.chat.clear')}
            </Button>
          </div>
        </div>

        {/* Chat Area */}
        <div className='flex flex-1 flex-col'>
          <Chat
            messages={messages}
            input={input}
            handleInputChange={handleInputChange}
            handleSubmit={handleSubmit}
            isGenerating={isLoading}
            stop={stop}
            setMessages={setMessages}
            onRateResponse={handleRateResponse}
            className='h-full p-6'
          />
        </div>
      </div>
    </TooltipProvider>
  )
}
