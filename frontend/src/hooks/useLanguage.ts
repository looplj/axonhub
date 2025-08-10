import { useTranslation } from 'react-i18next'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useAuthStore } from '@/stores/authStore'
import { graphqlRequest } from '@/gql/graphql'
import { UPDATE_ME_MUTATION } from '@/gql/users'
import { toast } from 'sonner'

export function useLanguage() {
  const { i18n, t } = useTranslation()
  const auth = useAuthStore((state) => state.auth)
  const queryClient = useQueryClient()

  // Mutation for updating user language preference
  const updateLanguageMutation = useMutation({
    mutationFn: async (language: string) => {
      if (!auth.user) {
        throw new Error('User not authenticated')
      }

      const response = (await graphqlRequest(UPDATE_ME_MUTATION, {
        input: {
          preferLanguage: language,
        },
      })) as { updateMe: any }
      return response.updateMe
    },
    onSuccess: (updatedUser, language) => {
      // Update the auth store with new language preference
      if (auth.user) {
        auth.setUser({
          ...auth.user,
          preferLanguage: updatedUser.preferLanguage,
        })
      }

      // Invalidate and refetch user data
      queryClient.invalidateQueries({ queryKey: ['me'] })

      const languageName = updatedUser.preferLanguage === 'en' ? 'English' : '中文'
      toast.success(t('language.changeSuccess', { language: languageName }))
    },
    onError: (error: any) => {
      toast.error(t('language.changeError', { error: error.message }))
      // Revert i18n language on error
      if (auth.user?.preferLanguage) {
        i18n.changeLanguage(auth.user.preferLanguage)
      }
    },
  })

  const changeLanguage = async (language: string) => {
    try {
      // Immediately change the UI language for better UX
      await i18n.changeLanguage(language)
      
      // Update user preference in the backend if user is authenticated
      if (auth.user && auth.accessToken) {
        updateLanguageMutation.mutate(language)
      }
    } catch (error) {
      toast.error(t('language.changeError', { error: String(error) }))
    }
  }

  const initializeLanguage = (userLanguage?: string) => {
    if (userLanguage && userLanguage !== i18n.language) {
      i18n.changeLanguage(userLanguage)
    }
  }

  return {
    currentLanguage: i18n.language,
    changeLanguage,
    initializeLanguage,
    isUpdating: updateLanguageMutation.isPending,
  }
}