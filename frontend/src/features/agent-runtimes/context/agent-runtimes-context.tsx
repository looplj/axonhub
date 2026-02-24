import React, { useRef, useState } from 'react';
import useDialogState from '@/hooks/use-dialog-state';
import { AgentRuntime } from '../data/schema';

type AgentRuntimesDialogType =
  | 'add'
  | 'edit'
  | 'delete'
  | 'bulkDelete'
  | 'bulkActivate'
  | 'bulkDeactivate';

interface AgentRuntimesContextType {
  open: AgentRuntimesDialogType | null;
  setOpen: (str: AgentRuntimesDialogType | null) => void;
  currentRow: AgentRuntime | null;
  setCurrentRow: React.Dispatch<React.SetStateAction<AgentRuntime | null>>;
  selectedAgentRuntimes: AgentRuntime[];
  setSelectedAgentRuntimes: React.Dispatch<React.SetStateAction<AgentRuntime[]>>;
  resetRowSelection: () => void;
  setResetRowSelection: (fn: () => void) => void;
}

const AgentRuntimesContext = React.createContext<AgentRuntimesContextType | null>(null);

interface Props {
  children: React.ReactNode;
}

export default function AgentRuntimesProvider({ children }: Props) {
  const [open, setOpen] = useDialogState<AgentRuntimesDialogType>(null);
  const [currentRow, setCurrentRow] = useState<AgentRuntime | null>(null);
  const [selectedAgentRuntimes, setSelectedAgentRuntimes] = useState<AgentRuntime[]>([]);
  const resetRowSelectionRef = useRef<() => void>(() => {});

  return (
    <AgentRuntimesContext.Provider
      value={{
        open,
        setOpen,
        currentRow,
        setCurrentRow,
        selectedAgentRuntimes,
        setSelectedAgentRuntimes,
        resetRowSelection: () => resetRowSelectionRef.current(),
        setResetRowSelection: (fn: () => void) => {
          resetRowSelectionRef.current = fn;
        },
      }}
    >
      {children}
    </AgentRuntimesContext.Provider>
  );
}

// eslint-disable-next-line react-refresh/only-export-components
export const useAgentRuntimes = () => {
  const agentRuntimesContext = React.useContext(AgentRuntimesContext);

  if (!agentRuntimesContext) {
    throw new Error('useAgentRuntimes has to be used within <AgentRuntimesContext>');
  }

  return agentRuntimesContext;
};
