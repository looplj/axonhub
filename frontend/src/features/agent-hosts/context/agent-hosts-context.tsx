import React, { useRef, useState } from 'react';
import useDialogState from '@/hooks/use-dialog-state';
import { AgentHost } from '../data/schema';

type AgentHostsDialogType =
  | 'add'
  | 'edit'
  | 'delete'
  | 'bulkDelete'
  | 'bulkActivate'
  | 'bulkDeactivate';

interface AgentHostsContextType {
  open: AgentHostsDialogType | null;
  setOpen: (str: AgentHostsDialogType | null) => void;
  currentRow: AgentHost | null;
  setCurrentRow: React.Dispatch<React.SetStateAction<AgentHost | null>>;
  selectedAgentHosts: AgentHost[];
  setSelectedAgentHosts: React.Dispatch<React.SetStateAction<AgentHost[]>>;
  resetRowSelection: () => void;
  setResetRowSelection: (fn: () => void) => void;
}

const AgentHostsContext = React.createContext<AgentHostsContextType | null>(null);

interface Props {
  children: React.ReactNode;
}

export default function AgentHostsProvider({ children }: Props) {
  const [open, setOpen] = useDialogState<AgentHostsDialogType>(null);
  const [currentRow, setCurrentRow] = useState<AgentHost | null>(null);
  const [selectedAgentHosts, setSelectedAgentHosts] = useState<AgentHost[]>([]);
  const resetRowSelectionRef = useRef<() => void>(() => {});

  return (
    <AgentHostsContext.Provider
      value={{
        open,
        setOpen,
        currentRow,
        setCurrentRow,
        selectedAgentHosts,
        setSelectedAgentHosts,
        resetRowSelection: () => resetRowSelectionRef.current(),
        setResetRowSelection: (fn: () => void) => {
          resetRowSelectionRef.current = fn;
        },
      }}
    >
      {children}
    </AgentHostsContext.Provider>
  );
}

// eslint-disable-next-line react-refresh/only-export-components
export const useAgentHosts = () => {
  const agentHostsContext = React.useContext(AgentHostsContext);

  if (!agentHostsContext) {
    throw new Error('useAgentHosts has to be used within <AgentHostsContext>');
  }

  return agentHostsContext;
};
