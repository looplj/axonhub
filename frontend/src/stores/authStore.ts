import { create } from 'zustand';

const USER_INFO = 'axonhub_user_info';

try {
  localStorage.removeItem('axonhub_access_token');
} catch {
}

interface Role {
  code: string;
  name: string;
}

interface Project {
  projectID: string;
  isOwner: boolean;
  scopes: string[];
  effectiveScopes?: string[];
  roles: Role[];
}

export interface AuthUser {
  id: string;
  email: string;
  firstName: string;
  lastName: string;
  isOwner: boolean;
  preferLanguage: string;
  avatar?: string;
  scopes: string[];
  roles: Role[];
  projects: Project[];
  oidcIdentities?: { id: string; idpName: string; issuer: string; subject: string; email: string }[];
  hasPassword?: boolean;
}

interface AuthState {
  auth: {
    user: AuthUser | null;
    setUser: (user: AuthUser | null) => void;
    sessionGeneration: number;
    startSession: () => void;
    reset: () => void;
  };
}

const getUserFromStorage = (): AuthUser | null => {
  try {
    const userStr = localStorage.getItem(USER_INFO);
    return userStr ? JSON.parse(userStr) : null;
  } catch (error) {
    return null;
  }
};

const setUserToStorage = (user: AuthUser | null): void => {
  try {
    if (user) {
      localStorage.setItem(USER_INFO, JSON.stringify(user));
    } else {
      localStorage.removeItem(USER_INFO);
    }
  } catch (error) {
  }
};

const removeUserFromStorage = (): void => {
  try {
    localStorage.removeItem(USER_INFO);
  } catch (error) {
  }
};

export const useAuthStore = create<AuthState>()((set) => {
  const initUser = getUserFromStorage();

  return {
    auth: {
      user: initUser,
      setUser: (user) =>
        set((state) => {
          setUserToStorage(user);
          return { ...state, auth: { ...state.auth, user } };
        }),
      sessionGeneration: 0,
      startSession: () =>
        set((state) => {
          return { ...state, auth: { ...state.auth, sessionGeneration: state.auth.sessionGeneration + 1 } };
        }),
      reset: () =>
        set((state) => {
          if (!state.auth.user) return state;
          removeUserFromStorage();
          return {
            ...state,
            auth: { ...state.auth, user: null, sessionGeneration: state.auth.sessionGeneration + 1 },
          };
        }),
    },
  };
});

// export const useAuth = () => useAuthStore((state) => state.auth)
