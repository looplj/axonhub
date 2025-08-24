import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { useRef } from 'react'
import { User, Upload } from 'lucide-react'
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
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { useMe } from '@/features/auth/data/auth'

type ProfileFormValues = {
  firstName: string
  lastName: string
  email: string
  preferLanguage: string
  avatar?: string
}

export default function ProfileForm() {
  const { t } = useTranslation()
  const auth = useAuthStore((state) => state.auth)
  const queryClient = useQueryClient()
  const fileInputRef = useRef<HTMLInputElement>(null)

  const profileFormSchema = z.object({
    firstName: z
      .string()
      .min(1, {
        message: t('profile.form.validation.firstNameRequired'),
      })
      .max(50, {
        message: t('profile.form.validation.firstNameTooLong'),
      }),
    lastName: z
      .string()
      .min(1, {
        message: t('profile.form.validation.lastNameRequired'),
      })
      .max(50, {
        message: t('profile.form.validation.lastNameTooLong'),
      }),
    email: z
      .string({
        required_error: t('profile.form.validation.emailRequired'),
      })
      .email(t('profile.form.validation.emailInvalid')),
    preferLanguage: z.string().min(1, {
      message: t('profile.form.validation.languageRequired'),
    }),
    avatar: z.string().optional(),
  })

  // Get current user data
  const { data: currentUser, isLoading } = useMe()

  const form = useForm<ProfileFormValues>({
    resolver: zodResolver(profileFormSchema),
    values: {
      firstName: currentUser?.firstName || '',
      lastName: currentUser?.lastName || '',
      email: currentUser?.email || '',
      preferLanguage: currentUser?.preferLanguage || 'en',
      avatar: currentUser?.avatar || '',
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
          avatar: data.avatar,
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
        avatar: updatedUser.avatar,
      })

      // Invalidate and refetch user data
      queryClient.invalidateQueries({ queryKey: ['me'] })

      toast.success(t('profile.form.messages.updateSuccess'))
    },
    onError: (error: any) => {
      toast.error(t('profile.form.messages.updateError', { error: error.message }))
    },
  })

  const handleAvatarUpload = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (file) {
      // For now, we'll use a simple file reader to convert to base64
      // In a real app, you'd upload to a file storage service
      const reader = new FileReader()
      reader.onload = (e) => {
        const result = e.target?.result as string
        form.setValue('avatar', result)
      }
      reader.readAsDataURL(file)
    }
  }

  const onSubmit = (data: ProfileFormValues) => {
    updateProfileMutation.mutate(data)
  }

  if (isLoading) {
    return <div>{t('loading')}</div>
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-8'>
        {/* Avatar Upload Section */}
        <FormField
          control={form.control}
          name='avatar'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('profile.form.fields.avatar.label')}</FormLabel>
              <FormControl>
                <div className='flex items-center space-x-4'>
                  <Avatar className='h-20 w-20'>
                    <AvatarImage src={field.value} alt='Avatar' />
                    <AvatarFallback>
                      <User className='h-10 w-10' />
                    </AvatarFallback>
                  </Avatar>
                  <div className='flex flex-col space-y-2'>
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      onClick={() => fileInputRef.current?.click()}
                    >
                      <Upload className='h-4 w-4 mr-2' />
                      {t('profile.form.fields.avatar.upload')}
                    </Button>
                    <input
                      ref={fileInputRef}
                      type='file'
                      accept='image/*'
                      onChange={handleAvatarUpload}
                      className='hidden'
                    />
                  </div>
                </div>
              </FormControl>
              <FormDescription>
                {t('profile.form.fields.avatar.description')}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />

        <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
          <FormField
            control={form.control}
            name='firstName'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('profile.form.fields.firstName.label')}</FormLabel>
                <FormControl>
                  <Input placeholder={t('profile.form.fields.firstName.placeholder')} {...field} />
                </FormControl>
                <FormDescription>
                  {t('profile.form.fields.firstName.description')}
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
                <FormLabel>{t('profile.form.fields.lastName.label')}</FormLabel>
                <FormControl>
                  <Input placeholder={t('profile.form.fields.lastName.placeholder')} {...field} />
                </FormControl>
                <FormDescription>
                  {t('profile.form.fields.lastName.description')}
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
              <FormLabel>{t('profile.form.fields.email.label')}</FormLabel>
              <FormControl>
                <Input
                  type='email'
                  placeholder={t('profile.form.fields.email.placeholder')}
                  {...field}
                />
              </FormControl>
              <FormDescription>
                {t('profile.form.fields.email.description')}
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
              <FormLabel>{t('profile.form.fields.preferLanguage.label')}</FormLabel>
              <Select onValueChange={field.onChange} value={field.value}>
                <FormControl>
                  <SelectTrigger>
                    <SelectValue placeholder={t('profile.form.fields.preferLanguage.placeholder')} />
                  </SelectTrigger>
                </FormControl>
                <SelectContent>
                  <SelectItem value='en'>{t('profile.form.fields.preferLanguage.options.en')}</SelectItem>
                  <SelectItem value='zh'>{t('profile.form.fields.preferLanguage.options.zh')}</SelectItem>
                  {/* <SelectItem value='ja'>日本語</SelectItem> */}
                  {/* <SelectItem value='ko'>한국어</SelectItem> */}
                </SelectContent>
              </Select>
              <FormDescription>
                {t('profile.form.fields.preferLanguage.description')}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />

        <div className="flex justify-end">
            <Button type="submit" disabled={updateProfileMutation.isPending}>
              {updateProfileMutation.isPending ? t('profile.form.buttons.updating') : t('profile.form.buttons.update')}
            </Button>
          </div>
      </form>
    </Form>
  )
}
