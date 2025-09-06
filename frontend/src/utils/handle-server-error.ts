import { toast } from 'sonner'
import i18n from '@/lib/i18n'

export function handleServerError(error: unknown) {
  // eslint-disable-next-line no-console
  console.log(error)

  let errMsg = i18n.t('common.errors.somethingWentWrong')

  if (
    error &&
    typeof error === 'object' &&
    'status' in error &&
    Number(error.status) === 204
  ) {
    errMsg = i18n.t('common.errors.contentNotFound')
  }

  // Handle fetch API errors (Response objects) or objects with data property
  if (error instanceof Response) {
    // For Response objects, we can try to parse the body to get the error message
    error.clone().json().then(data => {
      if (data?.title) {
        toast.error(data.title)
      } else {
        toast.error(errMsg)
      }
    }).catch(() => {
      toast.error(errMsg)
    })
    return
  } else if (error && typeof error === 'object' && 'data' in error && error.data) {
    // For objects with a data property (similar to AxiosError structure)
    const data = error.data as { title?: string }
    if (data.title) {
      errMsg = data.title
    }
  }

  toast.error(errMsg)
}
