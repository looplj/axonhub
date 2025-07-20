'use client'

import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
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
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { SelectDropdown } from '@/components/select-dropdown'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { X } from 'lucide-react'
import { Channel, channelTypeSchema, createChannelInputSchema, updateChannelInputSchema } from '../data/schema'
import { useCreateChannel, useUpdateChannel } from '../data/channels'
import { useState } from 'react'

const channelTypes = [
  { value: 'openai', label: 'OpenAI' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'deepseek', label: 'DeepSeek' },
  { value: 'doubao', label: 'Doubao' },
  { value: 'kimi', label: 'Kimi' },
]

interface Props {
  currentRow?: Channel
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function ChannelsActionDialog({ currentRow, open, onOpenChange }: Props) {
  const isEdit = !!currentRow
  const createChannel = useCreateChannel()
  const updateChannel = useUpdateChannel()
  const [supportedModels, setSupportedModels] = useState<string[]>(
    currentRow?.supportedModels || []
  )
  const [newModel, setNewModel] = useState('')

  const formSchema = isEdit ? updateChannelInputSchema : createChannelInputSchema
  
  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: isEdit && currentRow
      ? {
          type: currentRow.type,
          baseURL: currentRow.baseURL,
          name: currentRow.name,
          supportedModels: currentRow.supportedModels,
          defaultTestModel: currentRow.defaultTestModel,
        }
      : {
          type: 'openai',
          baseURL: '',
          name: '',
          apiKey: '',
          supportedModels: [],
          defaultTestModel: '',
        },
  })

  const onSubmit = async (values: z.infer<typeof formSchema>) => {
    try {
      const dataWithModels = {
        ...values,
        supportedModels,
      }

      if (isEdit && currentRow) {
        await updateChannel.mutateAsync({
          id: currentRow.id,
          input: dataWithModels,
        })
      } else {
        await createChannel.mutateAsync(dataWithModels as any)
      }
      
      form.reset()
      setSupportedModels([])
      onOpenChange(false)
    } catch (error) {
      console.error('Failed to save channel:', error)
    }
  }

  const addModel = () => {
    if (newModel.trim() && !supportedModels.includes(newModel.trim())) {
      setSupportedModels([...supportedModels, newModel.trim()])
      setNewModel('')
    }
  }

  const removeModel = (model: string) => {
    setSupportedModels(supportedModels.filter(m => m !== model))
  }

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      addModel()
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(state) => {
        if (!state) {
          form.reset()
          setSupportedModels(currentRow?.supportedModels || [])
        }
        onOpenChange(state)
      }}
    >
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader className='text-left'>
          <DialogTitle>{isEdit ? '编辑 Channel' : '添加新 Channel'}</DialogTitle>
          <DialogDescription>
            {isEdit ? '在此更新 Channel 信息。' : '在此创建新的 Channel。'}
            完成后点击保存。
          </DialogDescription>
        </DialogHeader>
        <div className='-mr-4 h-[28rem] w-full overflow-y-auto py-1 pr-4'>
          <Form {...form}>
            <form
              id='channel-form'
              onSubmit={form.handleSubmit(onSubmit)}
              className='space-y-4 p-0.5'
            >
              <FormField
                control={form.control}
                name='type'
                disabled={isEdit}
                render={({ field }) => (
                  <FormItem className='grid grid-cols-6 items-center space-y-0 gap-x-4 gap-y-1'>
                    <FormLabel className='col-span-2 text-right'>
                      类型
                    </FormLabel>
                    <FormControl>
                      <SelectDropdown
                        defaultValue={field.value}
                        onValueChange={field.onChange}
                        items={channelTypes}
                        placeholder='选择 Channel 类型'
                        className='col-span-4'
                        isControlled={true}
                        disabled={isEdit}
                      />
                    </FormControl>
                    <FormMessage className='col-span-4 col-start-3' />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem className='grid grid-cols-6 items-center space-y-0 gap-x-4 gap-y-1'>
                    <FormLabel className='col-span-2 text-right'>
                      名称
                    </FormLabel>
                    <FormControl>
                      <Input
                        placeholder='输入 Channel 名称'
                        className='col-span-4'
                        autoComplete='off'
                        {...field}
                      />
                    </FormControl>
                    <FormMessage className='col-span-4 col-start-3' />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='baseURL'
                render={({ field }) => (
                  <FormItem className='grid grid-cols-6 items-center space-y-0 gap-x-4 gap-y-1'>
                    <FormLabel className='col-span-2 text-right'>
                      Base URL
                    </FormLabel>
                    <FormControl>
                      <Input
                        placeholder='https://api.example.com'
                        className='col-span-4'
                        autoComplete='off'
                        {...field}
                      />
                    </FormControl>
                    <FormMessage className='col-span-4 col-start-3' />
                  </FormItem>
                )}
              />

              {!isEdit && (
                <FormField
                  control={form.control}
                  name='apiKey'
                  render={({ field }) => (
                    <FormItem className='grid grid-cols-6 items-center space-y-0 gap-x-4 gap-y-1'>
                      <FormLabel className='col-span-2 text-right'>
                        API Key
                      </FormLabel>
                      <FormControl>
                        <Input
                          type='password'
                          placeholder='输入 API Key'
                          className='col-span-4'
                          autoComplete='off'
                          {...field}
                        />
                      </FormControl>
                      <FormMessage className='col-span-4 col-start-3' />
                    </FormItem>
                  )}
                />
              )}

              <div className='grid grid-cols-6 items-start space-y-0 gap-x-4 gap-y-1'>
                <label className='col-span-2 text-right text-sm font-medium'>
                  支持的模型
                </label>
                <div className='col-span-4 space-y-2'>
                  <div className='flex gap-2'>
                    <Input
                      placeholder='输入模型名称'
                      value={newModel}
                      onChange={(e) => setNewModel(e.target.value)}
                      onKeyPress={handleKeyPress}
                      className='flex-1'
                    />
                    <Button type='button' onClick={addModel} size='sm'>
                      添加
                    </Button>
                  </div>
                  <div className='flex flex-wrap gap-1'>
                    {supportedModels.map((model) => (
                      <Badge key={model} variant='secondary' className='text-xs'>
                        {model}
                        <button
                          type='button'
                          onClick={() => removeModel(model)}
                          className='ml-1 hover:text-destructive'
                        >
                          <X size={12} />
                        </button>
                      </Badge>
                    ))}
                  </div>
                  {supportedModels.length === 0 && (
                    <p className='text-sm text-muted-foreground'>
                      请至少添加一个支持的模型
                    </p>
                  )}
                </div>
              </div>

              <FormField
                control={form.control}
                name='defaultTestModel'
                render={({ field }) => (
                  <FormItem className='grid grid-cols-6 items-center space-y-0 gap-x-4 gap-y-1'>
                    <FormLabel className='col-span-2 text-right'>
                      默认测试模型
                    </FormLabel>
                    <FormControl>
                      <SelectDropdown
                        defaultValue={field.value}
                        onValueChange={field.onChange}
                        items={supportedModels.map(model => ({ value: model, label: model }))}
                        placeholder='选择默认测试模型'
                        className='col-span-4'
                        disabled={supportedModels.length === 0}
                        isControlled={true}
                      />
                    </FormControl>
                    <FormMessage className='col-span-4 col-start-3' />
                  </FormItem>
                )}
              />
            </form>
          </Form>
        </div>
        <DialogFooter>
          <Button
            type='submit'
            form='channel-form'
            disabled={
              createChannel.isPending || 
              updateChannel.isPending || 
              supportedModels.length === 0
            }
          >
            {createChannel.isPending || updateChannel.isPending ? '保存中...' : '保存'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}