import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { graphqlRequest } from '@/gql/graphql'
import { UPDATE_ME_MUTATION } from '@/gql/users'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/authStore'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useMe } from '@/features/auth/data/auth'

const profileFormSchema = z.object({
  firstName: z
    .string()
    .min(1, {
      message: 'First name is required.',
    })
    .max(50, {
      message: 'First name must not be longer than 50 characters.',
    }),
  lastName: z
    .string()
    .min(1, {
      message: 'Last name is required.',
    })
    .max(50, {
      message: 'Last name must not be longer than 50 characters.',
    }),
  email: z
    .string({
      required_error: 'Email is required.',
    })
    .email('Please enter a valid email address.'),
  preferLanguage: z.string().min(1, {
    message: 'Please select a preferred language.',
  }),
})

type ProfileFormValues = z.infer<typeof profileFormSchema>

export default function ProfileForm() {
  const auth = useAuthStore((state) => state.auth)
  const queryClient = useQueryClient()

  // Get current user data
  const { data: currentUser, isLoading } = useMe()

  const form = useForm<ProfileFormValues>({
    resolver: zodResolver(profileFormSchema),
    values: {
      firstName: currentUser?.firstName || '',
      lastName: currentUser?.lastName || '',
      email: currentUser?.email || '',
      preferLanguage: currentUser?.preferLanguage || 'en',
    },
    mode: 'onChange',
  })

  // Mutation for updating user profile
  const updateProfileMutation = useMutation({
    mutationFn: async (data: ProfileFormValues) => {
      const response = (await graphqlRequest(UPDATE_ME_MUTATION, {
        input: {
          email: data.email,
          firstName: data.firstName,
          lastName: data.lastName,
          preferLanguage: data.preferLanguage,
        },
      })) as { updateMe: any }
      return response.updateMe
    },
    onSuccess: (updatedUser) => {
      // Update the auth store with new user data
      auth.setUser({
        ...auth.user!,
        firstName: updatedUser.firstName,
        lastName: updatedUser.lastName,
        email: updatedUser.email,
        preferLanguage: updatedUser.preferLanguage,
      })

      // Invalidate and refetch user data
      queryClient.invalidateQueries({ queryKey: ['me'] })

      toast.success('Profile updated successfully!')
    },
    onError: (error: any) => {
      toast.error(`Failed to update profile: ${error.message}`)
    },
  })

  const onSubmit = (data: ProfileFormValues) => {
    updateProfileMutation.mutate(data)
  }

  if (isLoading) {
    return <div>Loading...</div>
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-8'>
        <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
          <FormField
            control={form.control}
            name='firstName'
            render={({ field }) => (
              <FormItem>
                <FormLabel>First Name</FormLabel>
                <FormControl>
                  <Input placeholder='Enter your first name' {...field} />
                </FormControl>
                <FormDescription>
                  Your first name as it will appear to other users.
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='lastName'
            render={({ field }) => (
              <FormItem>
                <FormLabel>Last Name</FormLabel>
                <FormControl>
                  <Input placeholder='Enter your last name' {...field} />
                </FormControl>
                <FormDescription>
                  Your last name as it will appear to other users.
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>

        <FormField
          control={form.control}
          name='email'
          render={({ field }) => (
            <FormItem>
              <FormLabel>Email</FormLabel>
              <FormControl>
                <Input
                  type='email'
                  placeholder='Enter your email address'
                  {...field}
                />
              </FormControl>
              <FormDescription>
                Your email address for account notifications and login.
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name='preferLanguage'
          render={({ field }) => (
            <FormItem>
              <FormLabel>Preferred Language</FormLabel>
              <Select onValueChange={field.onChange} value={field.value}>
                <FormControl>
                  <SelectTrigger>
                    <SelectValue placeholder='Select your preferred language' />
                  </SelectTrigger>
                </FormControl>
                <SelectContent>
                  <SelectItem value='en'>English</SelectItem>
                  <SelectItem value='zh'>中文</SelectItem>
                  {/* <SelectItem value='ja'>日本語</SelectItem> */}
                  {/* <SelectItem value='ko'>한국어</SelectItem> */}
                </SelectContent>
              </Select>
              <FormDescription>
                Choose your preferred language for the interface.
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />

        <Button type='submit'>Update Profile</Button>
      </form>
    </Form>
  )
}
