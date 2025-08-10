import { useState } from 'react'
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
import { useCreateApiKey } from '../data/apikeys'
import { CreateApiKeyInput, createApiKeyInputSchemaFactory } from '../data/schema'

export function ApiKeysCreateDialog() {
  const { t } = useTranslation()
  const { isDialogOpen, closeDialog } = useApiKeysContext()
  const createApiKey = useCreateApiKey()
  const [isSubmitting, setIsSubmitting] = useState(false)

  const form = useForm<CreateApiKeyInput>({
    resolver: zodResolver(createApiKeyInputSchemaFactory(t)),
    defaultValues: {
      name: '',
      userID: '',
      key: '',
    },
  })

  const onSubmit = async (data: CreateApiKeyInput) => {
    setIsSubmitting(true)
    try {
      await createApiKey.mutateAsync(data)
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
          <DialogTitle>{t('apikeys.dialog.create.title')}</DialogTitle>
          <DialogDescription>
            {t('apikeys.dialog.create.description')}
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-4'>
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('apikeys.dialog.fields.name.label')}</FormLabel>
                  <FormControl>
                    <Input placeholder={t('apikeys.dialog.fields.name.placeholder')} {...field} />
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
                  <FormLabel>{t('apikeys.dialog.fields.userId.label')}</FormLabel>
                  <FormControl>
                    <Input placeholder={t('apikeys.dialog.fields.userId.placeholder')} {...field} />
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
                  <FormLabel>{t('apikeys.dialog.fields.key.label')}</FormLabel>
                  <FormControl>
                    <Input placeholder={t('apikeys.dialog.fields.key.placeholder')} {...field} />
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
                {t('apikeys.dialog.buttons.cancel')}
              </Button>
              <Button type='submit' disabled={isSubmitting}>
                {isSubmitting ? t('apikeys.dialog.buttons.creating') : t('apikeys.dialog.buttons.create')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}