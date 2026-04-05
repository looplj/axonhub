import { useCallback } from 'react';
import { ZodError } from 'zod';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { 
  parseGraphQLErrors, 
  getErrorI18nKey, 
  getErrorI18nParams,
  isDuplicateError,
  getDuplicateErrorInfo,
  type ParsedGraphQLError 
} from '@/lib/error-parser';

export interface ErrorHandlerOptions {
  /** Context/operation name for the error (e.g., "Create API Key") */
  context?: string;
  /** Specific error code to handle differently */
  onDuplicate?: (info: { resource?: string; field?: string; value?: string }) => void;
  /** Callback for specific error codes */
  onErrorCode?: (code: string, error: ParsedGraphQLError) => boolean | void;
  /** Whether to show toast notification */
  showToast?: boolean;
}

export function useErrorHandler() {
  const { t } = useTranslation();

  const handleError = useCallback(
    (error: unknown, options?: string | ErrorHandlerOptions) => {
      // Normalize options
      const opts: ErrorHandlerOptions = typeof options === 'string' 
        ? { context: options } 
        : options || {};
      
      const { context, onDuplicate, onErrorCode, showToast = true } = opts;

      // Handle Zod validation errors
      if (error instanceof ZodError) {
        const fieldErrors =
          error.issues
            ?.map((err: any) => {
              const path = err.path.join('.');
              return `${path}: ${err.message}`;
            })
            .join(', ') || 'Validation failed';

        const message = t('common.errors.validationFailed', { details: fieldErrors });

        if (showToast) {
          toast.error(t('common.errors.validationError'), {
            description: message,
            duration: 5000,
          });
        }
        return { type: 'validation', message };
      }

      // Try to parse GraphQL errors with extensions.code
      const graphqlErrors = parseGraphQLErrors(error);
      
      if (graphqlErrors.length > 0) {
        const firstError = graphqlErrors[0];
        const { code, extensions } = firstError;

        // Check for custom error code handler
        if (onErrorCode) {
          const handled = onErrorCode(code, firstError);
          if (handled === true) return { type: code, handled: true };
        }

        // Handle duplicate errors specially
        if ((code === 'DUPLICATE_NAME' || code === 'ALREADY_EXISTS') && onDuplicate) {
          const info = getDuplicateErrorInfo(error);
          if (info) {
            onDuplicate(info);
            return { type: 'duplicate', ...info };
          }
        }

        // Get i18n key and params
        const i18nKey = getErrorI18nKey(code);
        const params = getErrorI18nParams(firstError);
        
        // Build error message
        let message: string;
        try {
          message = t(i18nKey, params);
        } catch {
          // Fallback to raw message if translation fails
          message = firstError.message;
        }

        if (showToast) {
          // 直接展示具体错误消息
          toast.error(message, { duration: 5000 });
        }

        return { 
          type: code.toLowerCase(), 
          code, 
          message,
          extensions 
        };
      }

      // Handle generic errors
      const errorMessage = error instanceof Error ? error.message : t('common.errors.unknownError');

      if (showToast) {
        // 直接展示具体错误消息
        toast.error(errorMessage, { duration: 5000 });
      }

      return { type: 'unknown', message: errorMessage };
    },
    [t]
  );

  /**
   * Check if an error is a duplicate error
   */
  const checkIsDuplicateError = useCallback((error: unknown) => {
    return isDuplicateError(error);
  }, []);

  /**
   * Get duplicate error info
   */
  const getDuplicateInfo = useCallback((error: unknown) => {
    return getDuplicateErrorInfo(error);
  }, []);

  return { 
    handleError, 
    isDuplicateError: checkIsDuplicateError,
    getDuplicateInfo 
  };
}
