import React, { createContext, useContext, useState, useCallback, useMemo } from 'react';
import { Prompt } from '../data/schema';

type DialogType =
  | 'create'
  | 'edit'
  | 'delete'
  | 'bulkEnable'
  | 'bulkDisable'
  | 'bulkDelete'
  | null;

interface PromptsContextType {
  open: DialogType;
  setOpen: (open: DialogType) => void;
  currentRow: Prompt | null;
  setCurrentRow: (row: Prompt | null) => void;
  selectedPrompts: Prompt[];
  setSelectedPrompts: (prompts: Prompt[]) => void;
  resetRowSelection: (() => void) | null;
  setResetRowSelection: (fn: (() => void) | null) => void;
}

const PromptsContext = createContext<PromptsContextType | undefined>(undefined);

export function PromptsProvider({ children }: { children: React.ReactNode }) {
  const [open, setOpen] = useState<DialogType>(null);
  const [currentRow, setCurrentRow] = useState<Prompt | null>(null);
  const [selectedPrompts, setSelectedPrompts] = useState<Prompt[]>([]);
  const [resetRowSelection, setResetRowSelection] = useState<(() => void) | null>(null);

  const handleSetOpen = useCallback((newOpen: DialogType) => {
    setOpen(newOpen);
  }, []);

  const handleSetCurrentRow = useCallback((row: Prompt | null) => {
    setCurrentRow(row);
  }, []);

  const handleSetSelectedPrompts = useCallback((prompts: Prompt[]) => {
    setSelectedPrompts(prompts);
  }, []);

  const handleSetResetRowSelection = useCallback((fn: (() => void) | null) => {
    setResetRowSelection(() => fn);
  }, []);

  const value = useMemo(
    () => ({
      open,
      setOpen: handleSetOpen,
      currentRow,
      setCurrentRow: handleSetCurrentRow,
      selectedPrompts,
      setSelectedPrompts: handleSetSelectedPrompts,
      resetRowSelection,
      setResetRowSelection: handleSetResetRowSelection,
    }),
    [
      open,
      handleSetOpen,
      currentRow,
      handleSetCurrentRow,
      selectedPrompts,
      handleSetSelectedPrompts,
      resetRowSelection,
      handleSetResetRowSelection,
    ]
  );

  return <PromptsContext.Provider value={value}>{children}</PromptsContext.Provider>;
}

export function usePrompts() {
  const context = useContext(PromptsContext);
  if (context === undefined) {
    throw new Error('usePrompts must be used within a PromptsProvider');
  }
  return context;
}

export default PromptsProvider;
