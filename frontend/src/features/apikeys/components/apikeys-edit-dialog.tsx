import { useEffect, useState } from 'react'
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
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
import { UpdateApiKeyInput, updateApiKeyInputSchemaFactory } from '../data/schema'

export function ApiKeysEditDialog() {
  const { t } = useTranslation()
  const { isDialogOpen, closeDialog, selectedApiKey } = useApiKeysContext()
  const updateApiKey = useUpdateApiKey()
  const [isSubmitting, setIsSubmitting] = useState(false)

  const form = useForm<UpdateApiKeyInput>({
    resolver: zodResolver(updateApiKeyInputSchemaFactory(t)),
    defaultValues: {
      name: '',
    },
  })

  useEffect(() => {
    if (selectedApiKey && isDialogOpen.edit) {
      form.reset({
        name: selectedApiKey.name,
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
          <DialogTitle>{t('apikeys.dialogs.edit.title')}</DialogTitle>
          <DialogDescription>
            {t('apikeys.dialogs.edit.description')}
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-4'>
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('apikeys.dialogs.fields.name.label')}</FormLabel>
                  <FormControl>
                    <Input placeholder={t('apikeys.dialogs.fields.name.placeholder')} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <div className='space-y-4'>
              <div>
                <label className='text-sm font-medium text-muted-foreground'>
                  {t('apikeys.dialogs.fields.userId.label')}
                </label>
                <p className='text-sm text-foreground mt-1'>{selectedApiKey?.user?.email || selectedApiKey?.user?.id}</p>
              </div>
              <div>
                <label className='text-sm font-medium text-muted-foreground'>
                  {t('apikeys.dialogs.fields.key.label')}
                </label>
                <p className='text-sm text-foreground mt-1 font-mono'>{selectedApiKey?.key}</p>
              </div>
            </div>
            <DialogFooter>
              <Button
                type='button'
                variant='outline'
                onClick={handleClose}
                disabled={isSubmitting}
              >
                {t('apikeys.dialogs.buttons.cancel')}
              </Button>
              <Button type='submit' disabled={isSubmitting}>
                {isSubmitting ? t('apikeys.dialogs.buttons.saving') : t('apikeys.dialogs.buttons.save')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}