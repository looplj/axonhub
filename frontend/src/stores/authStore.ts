import { create } from 'zustand'

export const ACCESS_TOKEN = 'axonhub_access_token'
const USER_INFO = 'axonhub_user_info'

interface Role {
  id: string
  name: string
}

interface AuthUser {
  email: string
  firstName: string
  lastName: string
  isOwner: boolean
  preferLanguage: string
  avatar?: string
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
export const getTokenFromStorage = (): string => {
  try {
    return localStorage.getItem(ACCESS_TOKEN) || ''
  } catch (error) {
    console.warn('Failed to read token from localStorage:', error)
    return ''
  }
}

export const setTokenToStorage = (token: string): void => {
  try {
    localStorage.setItem(ACCESS_TOKEN, token)
  } catch (error) {
    console.warn('Failed to save token to localStorage:', error)
  }
}

export const removeTokenFromStorage = (): void => {
  try {
    localStorage.removeItem(ACCESS_TOKEN)
  } catch (error) {
    console.warn('Failed to remove token from localStorage:', error)
  }
}

const getUserFromStorage = (): AuthUser | null => {
  try {
    const userStr = localStorage.getItem(USER_INFO)
    return userStr ? JSON.parse(userStr) : null
  } catch (error) {
    console.warn('Failed to read user info from localStorage:', error)
    return null
  }
}

const setUserToStorage = (user: AuthUser | null): void => {
  try {
    if (user) {
      localStorage.setItem(USER_INFO, JSON.stringify(user))
    } else {
      localStorage.removeItem(USER_INFO)
    }
  } catch (error) {
    console.warn('Failed to save user info to localStorage:', error)
  }
}

const removeUserFromStorage = (): void => {
  try {
    localStorage.removeItem(USER_INFO)
  } catch (error) {
    console.warn('Failed to remove user info from localStorage:', error)
  }
}

export const useAuthStore = create<AuthState>()((set) => {
  const initToken = getTokenFromStorage()
  const initUser = getUserFromStorage()
  
  return {
    auth: {
      user: initUser,
      setUser: (user) =>
        set((state) => {
          setUserToStorage(user)
          return { ...state, auth: { ...state.auth, user } }
        }),
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
          removeUserFromStorage()
          return {
            ...state,
            auth: { ...state.auth, user: null, accessToken: '' },
          }
        }),
    },
  }
})

// export const useAuth = () => useAuthStore((state) => state.auth)
