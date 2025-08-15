import { HTMLAttributes } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { PasswordInput } from '@/components/password-input'
import { useInitializeSystem } from '@/features/auth/data/initialization'

type InitializationFormProps = HTMLAttributes<HTMLFormElement>

// Create form schema factory to support i18n
const createFormSchema = (t: (key: string) => string) => z.object({
  ownerEmail: z
    .string()
    .min(1, { message: t('initialization.form.validation.ownerEmailRequired') })
    .email({ message: t('initialization.form.validation.ownerEmailInvalid') }),
  ownerPassword: z
    .string()
    .min(1, {
      message: t('initialization.form.validation.ownerPasswordRequired'),
    })
    .min(8, {
      message: t('initialization.form.validation.ownerPasswordMinLength'),
    }),
  ownerFirstName: z
    .string()
    .min(1, { message: t('initialization.form.validation.ownerFirstNameRequired') }),
  ownerLastName: z
    .string()
    .min(1, { message: t('initialization.form.validation.ownerLastNameRequired') }),
  brandName: z
    .string()
    .min(1, { message: t('initialization.form.validation.brandNameRequired') }),
})

export function InitializationForm({
  className,
  ...props
}: InitializationFormProps) {
  const { t } = useTranslation()
  const initializeSystemMutation = useInitializeSystem()
  
  const formSchema = createFormSchema(t)
  type FormData = z.infer<typeof formSchema>

  const form = useForm<FormData>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      ownerEmail: '',
      ownerPassword: '',
      ownerFirstName: '',
      ownerLastName: '',
      brandName: '',
    },
  })

  function onSubmit(data: FormData) {
    const input = {
      ownerEmail: data.ownerEmail,
      ownerPassword: data.ownerPassword,
      ownerFirstName: data.ownerFirstName,
      ownerLastName: data.ownerLastName,
      brandName: data.brandName,
    }
    initializeSystemMutation.mutate(input)
  }

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className={cn('grid gap-4', className)}
        {...props}
      >
        <FormField
          control={form.control}
          name='ownerFirstName'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('initialization.form.ownerFirstName')}</FormLabel>
              <FormControl>
                <Input placeholder={t('initialization.form.placeholders.ownerFirstName')} {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='ownerLastName'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('initialization.form.ownerLastName')}</FormLabel>
              <FormControl>
                <Input placeholder={t('initialization.form.placeholders.ownerLastName')} {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='ownerEmail'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('initialization.form.ownerEmail')}</FormLabel>
              <FormControl>
                <Input placeholder={t('initialization.form.placeholders.ownerEmail')} {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='ownerPassword'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('initialization.form.ownerPassword')}</FormLabel>
              <FormControl>
                <PasswordInput placeholder={t('initialization.form.placeholders.ownerPassword')} {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='brandName'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('initialization.form.brandName')}</FormLabel>
              <FormControl>
                <Input placeholder={t('initialization.form.placeholders.brandName')} {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <Button
          type='submit'
          className='mt-2'
          disabled={initializeSystemMutation.isPending}
        >
          {initializeSystemMutation.isPending
            ? t('initialization.form.submitting')
            : t('initialization.form.submit')}
        </Button>
      </form>
    </Form>
  )
}
