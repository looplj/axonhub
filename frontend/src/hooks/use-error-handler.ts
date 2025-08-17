import { useCallback } from 'react'
import { toast } from 'sonner'
import { ZodError } from 'zod'
import { useTranslation } from 'react-i18next'

export function useErrorHandler() {
  const { t } = useTranslation()
  
  const handleError = useCallback((error: unknown, context?: string) => {
    console.error('Error occurred:', error)

    let errorMessage = t('common.errors.unknownError')
    
    if (error instanceof ZodError) {
      // Schema validation error
      const fieldErrors = error.errors.map(err => {
        const path = err.path.join('.')
        return `${path}: ${err.message}`
      }).join(', ')
      
      errorMessage = t('common.errors.validationFailed', { details: fieldErrors })
      
      toast.error(t('common.errors.validationError'), {
        description: errorMessage,
        duration: 5000,
      })
    } else if (error instanceof Error) {
      errorMessage = error.message
      
      if (context) {
        toast.error(t('common.errors.operationFailed', { operation: context }), {
          description: errorMessage,
          duration: 4000,
        })
      } else {
        toast.error(errorMessage)
      }
    } else {
      // Unknown error type
      if (context) {
        toast.error(t('common.errors.operationFailed', { operation: context }), {
          description: errorMessage,
          duration: 4000,
        })
      } else {
        toast.error(errorMessage)
      }
    }
  }, [t])

  return { handleError }
}