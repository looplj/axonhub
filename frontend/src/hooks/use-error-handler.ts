import { useCallback } from 'react'
import { toast } from 'sonner'
import { ZodError } from 'zod'

export function useErrorHandler() {
  const handleError = useCallback((error: unknown, context?: string) => {
    console.error('Error occurred:', error)

    let errorMessage = '发生未知错误'
    
    if (error instanceof ZodError) {
      // Schema validation error
      const fieldErrors = error.errors.map(err => {
        const path = err.path.join('.')
        return `${path}: ${err.message}`
      }).join(', ')
      
      errorMessage = `数据校验失败: ${fieldErrors}`
      
      toast.error('数据校验错误', {
        description: errorMessage,
        duration: 5000,
      })
    } else if (error instanceof Error) {
      errorMessage = error.message
      
      if (context) {
        toast.error(`${context}失败`, {
          description: errorMessage,
          duration: 4000,
        })
      } else {
        toast.error(errorMessage)
      }
    } else {
      // Unknown error type
      if (context) {
        toast.error(`${context}失败`, {
          description: errorMessage,
          duration: 4000,
        })
      } else {
        toast.error(errorMessage)
      }
    }
  }, [])

  return { handleError }
}