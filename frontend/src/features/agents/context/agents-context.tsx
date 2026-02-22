import React, { createContext, useContext, useMemo, useState, useCallback } from 'react';
import { Agent } from '../data/schema';

type DialogType = 'delete' | 'viewKey' | null;

interface AgentsContextType {
  open: DialogType;
  setOpen: (open: DialogType) => void;
  currentRow: Agent | null;
  setCurrentRow: (row: Agent | null) => void;
  createdApiKey: { key: string; name: string } | null;
  setCreatedApiKey: (value: { key: string; name: string } | null) => void;
}

const AgentsContext = createContext<AgentsContextType | undefined>(undefined);

export function AgentsProvider({ children }: { children: React.ReactNode }) {
  const [open, setOpen] = useState<DialogType>(null);
  const [currentRow, setCurrentRow] = useState<Agent | null>(null);
  const [createdApiKey, setCreatedApiKey] = useState<{ key: string; name: string } | null>(null);

  const handleSetOpen = useCallback((newOpen: DialogType) => {
    setOpen(newOpen);
  }, []);

  const value = useMemo(
    () => ({
      open,
      setOpen: handleSetOpen,
      currentRow,
      setCurrentRow,
      createdApiKey,
      setCreatedApiKey,
    }),
    [open, handleSetOpen, currentRow, createdApiKey]
  );

  return <AgentsContext.Provider value={value}>{children}</AgentsContext.Provider>;
}

export function useAgents() {
  const ctx = useContext(AgentsContext);
  if (!ctx) {
    throw new Error('useAgents must be used within an AgentsProvider');
  }
  return ctx;
}

export default AgentsProvider;

