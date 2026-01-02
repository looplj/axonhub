import React, { createContext, useContext, useState, useCallback, useMemo } from 'react';
import { Model } from '../data/schema';

type DialogType =
  | 'create'
  | 'batchCreate'
  | 'edit'
  | 'delete'
  | 'archive'
  | 'association'
  | 'settings'
  | 'bulkEnable'
  | 'bulkDisable'
  | 'unassociated'
  | null;

interface ModelsContextType {
  open: DialogType;
  setOpen: (open: DialogType) => void;
  currentRow: Model | null;
  setCurrentRow: (row: Model | null) => void;
  selectedModels: Model[];
  setSelectedModels: (models: Model[]) => void;
  resetRowSelection: (() => void) | null;
  setResetRowSelection: (fn: (() => void) | null) => void;
}

const ModelsContext = createContext<ModelsContextType | undefined>(undefined);

export function ModelsProvider({ children }: { children: React.ReactNode }) {
  const [open, setOpen] = useState<DialogType>(null);
  const [currentRow, setCurrentRow] = useState<Model | null>(null);
  const [selectedModels, setSelectedModels] = useState<Model[]>([]);
  const [resetRowSelection, setResetRowSelection] = useState<(() => void) | null>(null);

  const handleSetOpen = useCallback((newOpen: DialogType) => {
    setOpen(newOpen);
  }, []);

  const handleSetCurrentRow = useCallback((row: Model | null) => {
    setCurrentRow(row);
  }, []);

  const handleSetSelectedModels = useCallback((models: Model[]) => {
    setSelectedModels(models);
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
      selectedModels,
      setSelectedModels: handleSetSelectedModels,
      resetRowSelection,
      setResetRowSelection: handleSetResetRowSelection,
    }),
    [
      open,
      handleSetOpen,
      currentRow,
      handleSetCurrentRow,
      selectedModels,
      handleSetSelectedModels,
      resetRowSelection,
      handleSetResetRowSelection,
    ]
  );

  return <ModelsContext.Provider value={value}>{children}</ModelsContext.Provider>;
}

export function useModels() {
  const context = useContext(ModelsContext);
  if (context === undefined) {
    throw new Error('useModels must be used within a ModelsProvider');
  }
  return context;
}

export default ModelsProvider;
