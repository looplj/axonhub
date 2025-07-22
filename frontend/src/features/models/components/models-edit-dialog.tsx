import { useEffect, useState } from 'react'
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
import { useUpdateModel } from '../data/models'
import { UpdateModelInput, updateModelInputSchema } from '../data/schema'

export function ModelsEditDialog() {
  const { isDialogOpen, closeDialog, selectedModel } = useModelsContext()
  const updateModel = useUpdateModel()
  const [isSubmitting, setIsSubmitting] = useState(false)

  const form = useForm<UpdateModelInput>({
    resolver: zodResolver(updateModelInputSchema),
    defaultValues: {
      name: '',
      description: '',
      provider: '',
      modelID: '',
      config: {},
    },
  })

  useEffect(() => {
    if (selectedModel && isDialogOpen.edit) {
      form.reset({
        name: selectedModel.name,
        description: selectedModel.description || '',
        provider: selectedModel.provider,
        modelID: selectedModel.modelID,
        config: selectedModel.config || {},
      })
    }
  }, [selectedModel, isDialogOpen.edit, form])

  const onSubmit = async (data: UpdateModelInput) => {
    if (!selectedModel) return

    setIsSubmitting(true)
    try {
      await updateModel.mutateAsync({
        id: selectedModel.id,
        input: data,
      })
      closeDialog('edit')
    } catch (error) {
      // Error is handled by the mutation
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleClose = () => {
    form.reset()
    closeDialog('edit')
  }

  return (
    <Dialog open={isDialogOpen.edit} onOpenChange={handleClose}>
      <DialogContent className='sm:max-w-[425px]'>
        <DialogHeader>
          <DialogTitle>编辑模型</DialogTitle>
          <DialogDescription>
            修改模型的配置信息。
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
                {isSubmitting ? '保存中...' : '保存'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}