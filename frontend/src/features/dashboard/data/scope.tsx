import { createContext, useContext } from 'react';
import { useSelectedProjectId } from '@/stores/projectStore';

interface DashboardScopeContextValue {
  projectScoped: boolean;
}

const DashboardScopeContext = createContext<DashboardScopeContextValue>({
  projectScoped: false,
});

interface DashboardScopeProviderProps {
  projectScoped: boolean;
  children: React.ReactNode;
}

export function DashboardScopeProvider({ projectScoped, children }: DashboardScopeProviderProps) {
  return <DashboardScopeContext.Provider value={{ projectScoped }}>{children}</DashboardScopeContext.Provider>;
}

export function useDashboardScope() {
  const { projectScoped } = useContext(DashboardScopeContext);
  const selectedProjectId = useSelectedProjectId();

  const scopedHeaders = projectScoped && selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;

  return {
    projectScoped,
    selectedProjectId,
    scopedHeaders,
  };
}
