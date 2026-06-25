import React, { useRef, useState, useEffect } from 'react';
import useDialogState from '@/hooks/use-dialog-state';
import { Channel } from '../data/schema';

type ChannelsDialogType =
  | 'add'
  | 'duplicate'
  | 'edit'
  | 'delete'
  | 'settings'
  | 'channelSettings'
  | 'modelMapping'
  | 'overrides'
  | 'proxy'
  | 'status'
  | 'test'
  | 'testHistory'
  | 'bulkImport'
  | 'archive'
  | 'bulkOrdering'
  | 'bulkArchive'
  | 'bulkDisable'
  | 'bulkEnable'
  | 'bulkTest'
  | 'bulkDelete'
  | 'bulkApplyTemplate'
  | 'bulkClearTemplate'
  | 'errorResolved'
  | 'viewModels'
  | 'price'
  | 'transformOptions'
  | 'rateLimit'
  | 'testAPIKeys'
  | 'disabledAPIKeys'
  | 'endpoints';

interface ChannelsContextType {
  open: ChannelsDialogType | null;
  setOpen: (str: ChannelsDialogType | null) => void;
  currentRow: Channel | null;
  setCurrentRow: React.Dispatch<React.SetStateAction<Channel | null>>;
  selectedChannels: Channel[];
  setSelectedChannels: React.Dispatch<React.SetStateAction<Channel[]>>;
  resetRowSelection: () => void;
  setResetRowSelection: (fn: () => void) => void;
  showTypeTabs: boolean;
  setShowTypeTabs: React.Dispatch<React.SetStateAction<boolean>>;
}

const ChannelsContext = React.createContext<ChannelsContextType | null>(null);

interface Props {
  children: React.ReactNode;
}

export default function ChannelsProvider({ children }: Props) {
  const [open, setOpen] = useDialogState<ChannelsDialogType>(null);
  const [currentRow, setCurrentRow] = useState<Channel | null>(null);
  const [selectedChannels, setSelectedChannels] = useState<Channel[]>([]);
  const [showTypeTabs, setShowTypeTabs] = useState<boolean>(() => {
    try {
      const stored = localStorage.getItem('channels-show-type-tabs');
      if (stored !== null) {
        const parsed = JSON.parse(stored);
        return typeof parsed === 'boolean' ? parsed : true;
      }
    } catch {
      return true;
    }
    return true;
  });

  useEffect(() => {
    try {
      localStorage.setItem('channels-show-type-tabs', JSON.stringify(showTypeTabs));
    } catch {
      // Ignore storage failures; the in-memory preference still works.
    }
  }, [showTypeTabs]);
  const resetRowSelectionRef = useRef<() => void>(() => {});

  return (
    <ChannelsContext.Provider
      value={{
        open,
        setOpen,
        currentRow,
        setCurrentRow,
        selectedChannels,
        setSelectedChannels,
        resetRowSelection: () => resetRowSelectionRef.current(),
        setResetRowSelection: (fn: () => void) => {
          resetRowSelectionRef.current = fn;
        },
        showTypeTabs,
        setShowTypeTabs,
      }}
    >
      {children}
    </ChannelsContext.Provider>
  );
}

// eslint-disable-next-line react-refresh/only-export-components
export const useChannels = () => {
  const channelsContext = React.useContext(ChannelsContext);

  if (!channelsContext) {
    throw new Error('useChannels has to be used within <ChannelsContext>');
  }

  return channelsContext;
};
