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
import { CreateApiKeyInput, createApiKeyInputSchema } from '../data/schema'

export function ApiKeysCreateDialog() {
  const { t } = useTranslation()
  const { isDialogOpen, closeDialog } = useApiKeysContext()
  const createApiKey = useCreateApiKey()
  const [isSubmitting, setIsSubmitting] = useState(false)

  const form = useForm<CreateApiKeyInput>({
    resolver: zodResolver(createApiKeyInputSchema),
    defaultValues: {
      name: '',
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
          <DialogTitle>{t('apikeys.dialogs.create.title')}</DialogTitle>
          <DialogDescription>
            {t('apikeys.dialogs.create.description')}
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
                {isSubmitting ? t('apikeys.dialogs.buttons.creating') : t('apikeys.dialogs.buttons.create')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}