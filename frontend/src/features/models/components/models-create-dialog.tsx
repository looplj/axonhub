import { useState } from 'react'
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
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
import { Textarea } from '@/components/ui/textarea'
import { useModelsContext } from '../context/models-context'
import { useCreateModel } from '../data/models'
import { CreateModelInput, createModelInputSchema } from '../data/schema'

export function ModelsCreateDialog() {
  const { isDialogOpen, closeDialog } = useModelsContext()
  const createModel = useCreateModel()
  const [isSubmitting, setIsSubmitting] = useState(false)

  const form = useForm<CreateModelInput>({
    resolver: zodResolver(createModelInputSchema),
    defaultValues: {
      name: '',
      description: '',
      provider: '',
      modelID: '',
      config: {},
    },
  })

  const onSubmit = async (data: CreateModelInput) => {
    setIsSubmitting(true)
    try {
      await createModel.mutateAsync(data)
      form.reset()
      closeDialog('create')
    } catch (error) {
      // Error is handled by the mutation
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleClose = () => {
    form.reset()
    closeDialog('create')
  }

  return (
    <Dialog open={isDialogOpen.create} onOpenChange={handleClose}>
      <DialogContent className='sm:max-w-[425px]'>
        <DialogHeader>
          <DialogTitle>创建模型</DialogTitle>
          <DialogDescription>
            创建一个新的 AI 模型配置。
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-4'>
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>名称</FormLabel>
                  <FormControl>
                    <Input placeholder='输入模型名称' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='provider'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>提供商</FormLabel>
                  <FormControl>
                    <Input placeholder='如: OpenAI, Anthropic' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='modelID'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>模型ID</FormLabel>
                  <FormControl>
                    <Input placeholder='如: gpt-4, claude-3' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='description'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>描述</FormLabel>
                  <FormControl>
                    <Textarea
                      placeholder='输入模型描述（可选）'
                      className='resize-none'
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <DialogFooter>
              <Button
                type='button'
                variant='outline'
                onClick={handleClose}
                disabled={isSubmitting}
              >
                取消
              </Button>
              <Button type='submit' disabled={isSubmitting}>
                {isSubmitting ? '创建中...' : '创建'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}