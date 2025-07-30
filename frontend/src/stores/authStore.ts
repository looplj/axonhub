import { create } from 'zustand'

const ACCESS_TOKEN = 'axonhub_access_token'

interface Role {
  id: string
  name: string
}

interface AuthUser {
  email: string
  firstName: string
  lastName: string
  isOwner: boolean
  scopes: string[]
  roles: Role[]
}

interface AuthState {
  auth: {
    user: AuthUser | null
    setUser: (user: AuthUser | null) => void
    accessToken: string
    setAccessToken: (accessToken: string) => void
    resetAccessToken: () => void
    reset: () => void
  }
}

// Helper functions for localStorage
const getTokenFromStorage = (): string => {
  try {
    return localStorage.getItem(ACCESS_TOKEN) || ''
  } catch (error) {
    console.warn('Failed to read token from localStorage:', error)
    return ''
  }
}

const setTokenToStorage = (token: string): void => {
  try {
    localStorage.setItem(ACCESS_TOKEN, token)
  } catch (error) {
    console.warn('Failed to save token to localStorage:', error)
  }
}

const removeTokenFromStorage = (): void => {
  try {
    localStorage.removeItem(ACCESS_TOKEN)
  } catch (error) {
    console.warn('Failed to remove token from localStorage:', error)
  }
}

export const useAuthStore = create<AuthState>()((set) => {
  const initToken = getTokenFromStorage()
  
  return {
    auth: {
      user: null,
      setUser: (user) =>
        set((state) => ({ ...state, auth: { ...state.auth, user } })),
      accessToken: initToken,
      setAccessToken: (accessToken) =>
        set((state) => {
          setTokenToStorage(accessToken)
          return { ...state, auth: { ...state.auth, accessToken } }
        }),
      resetAccessToken: () =>
        set((state) => {
          removeTokenFromStorage()
          return { ...state, auth: { ...state.auth, accessToken: '' } }
        }),
      reset: () =>
        set((state) => {
          removeTokenFromStorage()
          return {
            ...state,
            auth: { ...state.auth, user: null, accessToken: '' },
          }
        }),
    },
  }
})

// export const useAuth = () => useAuthStore((state) => state.auth)
