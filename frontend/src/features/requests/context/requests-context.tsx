'use client';

import { createContext, useContext, useState, ReactNode } from 'react';
import { Request, RequestExecution } from '../data/schema';

interface RequestsContextType {
  // Dialog states
  detailDialogOpen: boolean;
  setDetailDialogOpen: (open: boolean) => void;

  // Execution detail dialog states
  executionDetailOpen: boolean;
  setExecutionDetailOpen: (open: boolean) => void;

  // Executions drawer states
  executionsDrawerOpen: boolean;
  setExecutionsDrawerOpen: (open: boolean) => void;

  // Current selected items
  currentRequest: Request | null;
  setCurrentRequest: (request: Request | null) => void;

  currentExecution: RequestExecution | null;
  setCurrentExecution: (execution: RequestExecution | null) => void;

  // Table selection
  selectedRequests: string[];
  setSelectedRequests: (ids: string[]) => void;
}

const RequestsContext = createContext<RequestsContextType | undefined>(undefined);

interface RequestsProviderProps {
  children: ReactNode;
}

export default function RequestsProvider({ children }: RequestsProviderProps) {
  const [detailDialogOpen, setDetailDialogOpen] = useState(false);
  const [executionDetailOpen, setExecutionDetailOpen] = useState(false);
  const [executionsDrawerOpen, setExecutionsDrawerOpen] = useState(false);
  const [currentRequest, setCurrentRequest] = useState<Request | null>(null);
  const [currentExecution, setCurrentExecution] = useState<RequestExecution | null>(null);
  const [selectedRequests, setSelectedRequests] = useState<string[]>([]);

  const value: RequestsContextType = {
    detailDialogOpen,
    setDetailDialogOpen,
    executionDetailOpen,
    setExecutionDetailOpen,
    executionsDrawerOpen,
    setExecutionsDrawerOpen,
    currentRequest,
    setCurrentRequest,
    currentExecution,
    setCurrentExecution,
    selectedRequests,
    setSelectedRequests,
  };

  return <RequestsContext.Provider value={value}>{children}</RequestsContext.Provider>;
}

// Also export as named export for convenience
export { RequestsProvider };

export function useRequestsContext() {
  const context = useContext(RequestsContext);
  if (context === undefined) {
    throw new Error('useRequestsContext must be used within a RequestsProvider');
  }
  return context;
}
