import { createContext, useContext, useState, ReactNode } from 'react';
import { Project } from '../data/schema';

interface ProjectsContextType {
  editingProject: Project | null;
  setEditingProject: (project: Project | null) => void;
  archivingProject: Project | null;
  setArchivingProject: (project: Project | null) => void;
  activatingProject: Project | null;
  setActivatingProject: (project: Project | null) => void;
  isCreateDialogOpen: boolean;
  setIsCreateDialogOpen: (open: boolean) => void;
}

const ProjectsContext = createContext<ProjectsContextType | undefined>(undefined);

export function useProjectsContext() {
  const context = useContext(ProjectsContext);
  if (!context) {
    throw new Error('useProjectsContext must be used within a ProjectsProvider');
  }
  return context;
}

interface ProjectsProviderProps {
  children: ReactNode;
}

export default function ProjectsProvider({ children }: ProjectsProviderProps) {
  const [editingProject, setEditingProject] = useState<Project | null>(null);
  const [archivingProject, setArchivingProject] = useState<Project | null>(null);
  const [activatingProject, setActivatingProject] = useState<Project | null>(null);
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false);

  return (
    <ProjectsContext.Provider
      value={{
        editingProject,
        setEditingProject,
        archivingProject,
        setArchivingProject,
        activatingProject,
        setActivatingProject,
        isCreateDialogOpen,
        setIsCreateDialogOpen,
      }}
    >
      {children}
    </ProjectsContext.Provider>
  );
}
