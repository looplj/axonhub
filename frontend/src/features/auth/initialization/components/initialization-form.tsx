import { HTMLAttributes } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
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

const formSchema = z.object({
  ownerEmail: z
    .string()
    .min(1, { message: 'Please enter owner email' })
    .email({ message: 'Invalid email address' }),
  ownerPassword: z
    .string()
    .min(1, {
      message: 'Please enter owner password',
    })
    .min(8, {
      message: 'Password must be at least 8 characters long',
    }),
})

export function InitializationForm({
  className,
  ...props
}: InitializationFormProps) {
  const initializeSystemMutation = useInitializeSystem()

  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      ownerEmail: '',
      ownerPassword: '',
    },
  })

  function onSubmit(data: z.infer<typeof formSchema>) {
    const input = {
      ownerEmail: data.ownerEmail,
      ownerPassword: data.ownerPassword,
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
          name='ownerEmail'
          render={({ field }) => (
            <FormItem>
              <FormLabel>Owner Email</FormLabel>
              <FormControl>
                <Input placeholder='admin@example.com' {...field} />
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
              <FormLabel>Owner Password</FormLabel>
              <FormControl>
                <PasswordInput placeholder='********' {...field} />
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
            ? 'Initializing...'
            : 'Initialize System'}
        </Button>
      </form>
    </Form>
  )
}
