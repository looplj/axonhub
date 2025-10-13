import { create } from 'zustand'

const PROJECT_STORAGE_KEY = 'axonhub_selected_project_id'

interface ProjectState {
  selectedProjectId: string | null
  setSelectedProjectId: (projectId: string | null) => void
  clearSelectedProjectId: () => void
}

// Helper functions for localStorage
const getProjectIdFromStorage = (): string | null => {
  try {
    return localStorage.getItem(PROJECT_STORAGE_KEY)
  } catch (error) {
    console.warn('Failed to read project ID from localStorage:', error)
    return null
  }
}

const setProjectIdToStorage = (projectId: string): void => {
  try {
    localStorage.setItem(PROJECT_STORAGE_KEY, projectId)
  } catch (error) {
    console.warn('Failed to save project ID to localStorage:', error)
  }
}

const removeProjectIdFromStorage = (): void => {
  try {
    localStorage.removeItem(PROJECT_STORAGE_KEY)
  } catch (error) {
    console.warn('Failed to remove project ID from localStorage:', error)
  }
}

export const useProjectStore = create<ProjectState>()((set) => {
  const initProjectId = getProjectIdFromStorage()
  
  return {
    selectedProjectId: initProjectId,
    setSelectedProjectId: (projectId) => {
      set({ selectedProjectId: projectId })
      if (projectId) {
        setProjectIdToStorage(projectId)
      } else {
        removeProjectIdFromStorage()
      }
    },
    clearSelectedProjectId: () => {
      set({ selectedProjectId: null })
      removeProjectIdFromStorage()
    },
  }
})

// Convenience hook to get the selected project ID
export const useSelectedProjectId = () => useProjectStore((state) => state.selectedProjectId)
