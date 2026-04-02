import React, { createContext, useCallback, useContext, useMemo, useState } from 'react';
import { PromptProtectionRule } from '../data/schema';

type DialogType = 'create' | 'edit' | 'delete' | 'bulkEnable' | 'bulkDisable' | 'bulkDelete' | null;

interface RulesContextType {
  open: DialogType;
  setOpen: (open: DialogType) => void;
  currentRow: PromptProtectionRule | null;
  setCurrentRow: (row: PromptProtectionRule | null) => void;
  selectedRules: PromptProtectionRule[];
  setSelectedRules: (rules: PromptProtectionRule[]) => void;
  resetRowSelection: (() => void) | null;
  setResetRowSelection: (fn: (() => void) | null) => void;
}

const RulesContext = createContext<RulesContextType | undefined>(undefined);

export function PromptProtectionRulesProvider({ children }: { children: React.ReactNode }) {
  const [open, setOpen] = useState<DialogType>(null);
  const [currentRow, setCurrentRow] = useState<PromptProtectionRule | null>(null);
  const [selectedRules, setSelectedRules] = useState<PromptProtectionRule[]>([]);
  const [resetRowSelection, setResetRowSelection] = useState<(() => void) | null>(null);

  const handleSetOpen = useCallback((nextOpen: DialogType) => {
    setOpen(nextOpen);
    if (nextOpen !== 'edit' && nextOpen !== 'delete') {
      setCurrentRow(null);
    }
  }, []);

  const value = useMemo(
    () => ({
      open,
      setOpen: handleSetOpen,
      currentRow,
      setCurrentRow,
      selectedRules,
      setSelectedRules,
      resetRowSelection,
      setResetRowSelection,
    }),
    [open, handleSetOpen, currentRow, selectedRules, resetRowSelection]
  );

  return <RulesContext.Provider value={value}>{children}</RulesContext.Provider>;
}

export function usePromptProtectionRules() {
  const context = useContext(RulesContext);
  if (!context) {
    throw new Error('usePromptProtectionRules must be used within PromptProtectionRulesProvider');
  }

  return context;
}

export default PromptProtectionRulesProvider;
