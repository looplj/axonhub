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
import { useApiKeysContext } from '../context/apikeys-context'
import { useUpdateApiKey } from '../data/apikeys'
import { UpdateApiKeyInput, updateApiKeyInputSchema } from '../data/schema'

export function ApiKeysEditDialog() {
  const { isDialogOpen, closeDialog, selectedApiKey } = useApiKeysContext()
  const updateApiKey = useUpdateApiKey()
  const [isSubmitting, setIsSubmitting] = useState(false)

  const form = useForm<UpdateApiKeyInput>({
    resolver: zodResolver(updateApiKeyInputSchema),
    defaultValues: {
      name: '',
      userID: '',
      key: '',
    },
  })

  useEffect(() => {
    if (selectedApiKey && isDialogOpen.edit) {
      form.reset({
        name: selectedApiKey.name,
        userID: selectedApiKey.userID,
        key: selectedApiKey.key,
      })
    }
  }, [selectedApiKey, isDialogOpen.edit, form])

  const onSubmit = async (data: UpdateApiKeyInput) => {
    if (!selectedApiKey) return

    setIsSubmitting(true)
    try {
      await updateApiKey.mutateAsync({
        id: selectedApiKey.id,
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
          <DialogTitle>编辑 API Key</DialogTitle>
          <DialogDescription>
            修改 API Key 的信息。
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
                    <Input placeholder='输入 API Key 名称' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='userID'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>用户ID</FormLabel>
                  <FormControl>
                    <Input placeholder='输入用户ID' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='key'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>API Key</FormLabel>
                  <FormControl>
                    <Input placeholder='输入 API Key' {...field} />
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