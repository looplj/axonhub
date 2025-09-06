import { getTokenFromStorage } from '@/stores/authStore'

// Same domain, no need to add baseURL.
export const API_BASE_URL = ''

interface ApiRequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH'
  headers?: Record<string, string>
  body?: any
  requireAuth?: boolean
}

class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
    public response?: any
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

export async function apiRequest<T>(
  endpoint: string,
  options: ApiRequestOptions = {}
): Promise<T> {
  const { method = 'GET', headers = {}, body, requireAuth = false } = options

  const url = `${API_BASE_URL}${endpoint}`

  const requestHeaders: Record<string, string> = {
    'Content-Type': 'application/json',
    ...headers,
  }

  // Add Authorization header if auth is required
  if (requireAuth) {
    const token = getTokenFromStorage()
    if (token) {
      requestHeaders['Authorization'] = `Bearer ${token}`
    }
  }

  const requestOptions: RequestInit = {
    method,
    headers: requestHeaders,
  }

  if (body && method !== 'GET') {
    requestOptions.body = JSON.stringify(body)
  }

  try {
    const response = await fetch(url, requestOptions)

    if (!response.ok) {
      let errorMessage = `HTTP ${response.status}: ${response.statusText}`
      let errorData: any = null

      try {
        errorData = await response.json()
        if (errorData.message) {
          errorMessage = errorData.message
        } else if (errorData.error) {
          errorMessage = errorData.error
        }
      } catch {
        // If response is not JSON, use status text
      }

      throw new ApiError(errorMessage, response.status, errorData)
    }

    // Handle empty responses
    const contentType = response.headers.get('content-type')
    if (contentType && contentType.includes('application/json')) {
      return await response.json()
    } else {
      return {} as T
    }
  } catch (error) {
    if (error instanceof ApiError) {
      throw error
    }

    // Network or other errors
    console.error('API request failed:', error)
    throw new ApiError(
      error instanceof Error ? error.message : 'Network error occurred',
      0
    )
  }
}

// System API endpoints
export const systemApi = {
  getStatus: (): Promise<{ isInitialized: boolean }> =>
    apiRequest('/admin/system/status'),

  initialize: (data: {
    ownerEmail: string
    ownerPassword: string
    ownerFirstName: string
    ownerLastName: string
    brandName: string
  }): Promise<{ success: boolean; message: string }> =>
    apiRequest('/admin/system/initialize', {
      method: 'POST',
      body: data,
    }),
}

// Auth API endpoints
export const authApi = {
  signIn: (data: {
    email: string
    password: string
  }): Promise<{
    user: {
      id: string
      email: string
      firstName?: string
      lastName?: string
      isOwner: boolean
      preferLanguage?: string
      scopes: string[]
      roles: Array<{ code: string; name: string }>
    }
    token: string
  }> =>
    apiRequest('/admin/auth/signin', {
      method: 'POST',
      body: data,
    }),
}
