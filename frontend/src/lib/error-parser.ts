/**
 * GraphQL Error Parser
 * 
 * Parses GraphQL errors and extracts structured information from extensions.code
 * for client-side error handling and i18n translation.
 */

export type GraphQLErrorCode =
  | 'INVALID_INPUT'
  | 'VALIDATION_FAILED'
  | 'DUPLICATE_NAME'
  | 'ALREADY_EXISTS'
  | 'NOT_FOUND'
  | 'UNAUTHENTICATED'
  | 'FORBIDDEN'
  | 'INTERNAL_SERVER_ERROR'
  | string;

export interface ParsedGraphQLError {
  /** Machine-readable error code from extensions.code */
  code: GraphQLErrorCode;
  /** Human-readable error message */
  message: string;
  /** Additional data from extensions (e.g., resource, field, value) */
  extensions: Record<string, unknown>;
  /** Original error object */
  originalError: unknown;
}

/**
 * Check if an error is a GraphQL error with extensions
 */
function isGraphQLError(error: unknown): error is {
  message: string;
  extensions?: Record<string, unknown>;
} {
  return (
    typeof error === 'object' &&
    error !== null &&
    'message' in error &&
    typeof (error as { message: string }).message === 'string'
  );
}

/**
 * Extract extensions from error
 * Handles both direct extensions and nested in GraphQLRequestError
 */
function getErrorExtensions(error: unknown): Record<string, unknown> | undefined {
  if (!isGraphQLError(error)) {
    return undefined;
  }

  // Direct extensions property
  if (error.extensions) {
    return error.extensions as Record<string, unknown>;
  }

  return undefined;
}

/**
 * Parse a single GraphQL error
 */
function parseSingleError(error: unknown): ParsedGraphQLError | null {
  if (!isGraphQLError(error)) {
    return null;
  }

  const extensions = getErrorExtensions(error) || {};
  const code = (extensions.code as GraphQLErrorCode) || 'INTERNAL_SERVER_ERROR';

  return {
    code,
    message: error.message,
    extensions,
    originalError: error,
  };
}

/**
 * Parse GraphQL errors from various error formats
 *
 * Supported formats:
 * 1. { errors: [{ message, extensions }] }
 * 2. { response: { errors: [...] } }
 * 3. Raw error with message and extensions (e.g., GraphQLRequestError)
 * 4. Array of errors
 */
export function parseGraphQLErrors(error: unknown): ParsedGraphQLError[] {
  if (!error) {
    return [];
  }

  // Format 1: { errors: [...] }
  if (typeof error === 'object' && error !== null && 'errors' in error) {
    const errors = (error as { errors: unknown[] }).errors;
    if (Array.isArray(errors)) {
      return errors.map(parseSingleError).filter((e): e is ParsedGraphQLError => e !== null);
    }
  }

  // Format 2: { response: { errors: [...] } }
  if (typeof error === 'object' && error !== null && 'response' in error) {
    const response = (error as { response: unknown }).response;
    if (typeof response === 'object' && response !== null && 'errors' in response) {
      const errors = (response as { errors: unknown[] }).errors;
      if (Array.isArray(errors)) {
        return errors.map(parseSingleError).filter((e): e is ParsedGraphQLError => e !== null);
      }
    }
  }

  // Format 3: Array of errors
  if (Array.isArray(error)) {
    return error.map(parseSingleError).filter((e): e is ParsedGraphQLError => e !== null);
  }

  // Format 4: Single error with extensions directly (e.g., GraphQLRequestError)
  if (isGraphQLError(error)) {
    const extensions = getErrorExtensions(error) || {};
    const code = (extensions.code as GraphQLErrorCode) || 'INTERNAL_SERVER_ERROR';
    return [{
      code,
      message: error.message,
      extensions,
      originalError: error,
    }];
  }

  return [];
}

/**
 * Get the first error code from a GraphQL error
 */
export function getErrorCode(error: unknown): GraphQLErrorCode | null {
  const errors = parseGraphQLErrors(error);
  return errors.length > 0 ? errors[0].code : null;
}

/**
 * Check if error is a specific code
 */
export function isErrorCode(error: unknown, code: GraphQLErrorCode): boolean {
  return getErrorCode(error) === code;
}

/**
 * Check if error is a duplicate error
 */
export function isDuplicateError(error: unknown): boolean {
  const code = getErrorCode(error);
  return code === 'DUPLICATE_NAME' || code === 'ALREADY_EXISTS';
}

/**
 * Get duplicate error info
 */
export function getDuplicateErrorInfo(error: unknown): { 
  resource?: string; 
  field?: string; 
  value?: string;
} | null {
  const errors = parseGraphQLErrors(error);
  if (errors.length === 0) return null;
  
  const err = errors[0];
  if (err.code !== 'DUPLICATE_NAME' && err.code !== 'ALREADY_EXISTS') {
    return null;
  }

  return {
    resource: err.extensions.resource as string | undefined,
    field: err.extensions.field as string | undefined,
    value: err.extensions.value as string | undefined,
  };
}

/**
 * Get i18n key for error code
 */
export function getErrorI18nKey(code: GraphQLErrorCode | null): string {
  const i18nMap: Record<string, string> = {
    'DUPLICATE_NAME': 'common.errors.duplicateName',
    'ALREADY_EXISTS': 'common.errors.duplicateKey',
    'NOT_FOUND': 'common.errors.notFound',
    'INVALID_INPUT': 'common.errors.validationFailed',
    'VALIDATION_FAILED': 'common.errors.validationFailed',
    'UNAUTHENTICATED': 'common.errors.sessionExpired',
    'FORBIDDEN': 'common.errors.forbidden',
    'INTERNAL_SERVER_ERROR': 'common.errors.internalServerError',
  };

  return code && i18nMap[code] ? i18nMap[code] : 'common.errors.internalServerError';
}

/**
 * Get i18n parameters for error
 */
export function getErrorI18nParams(error: ParsedGraphQLError): Record<string, string> {
  const params: Record<string, string> = {};

  // Add common extensions as params
  if (error.extensions.resource) {
    params.resource = String(error.extensions.resource);
  }
  if (error.extensions.field) {
    params.field = String(error.extensions.field);
  }
  if (error.extensions.value) {
    params.value = String(error.extensions.value);
    params.name = String(error.extensions.value); // alias for convenience
  }

  return params;
}
