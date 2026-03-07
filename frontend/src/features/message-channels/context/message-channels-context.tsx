import React, { createContext, useContext, useRef, useState, useCallback, useMemo } from 'react';
import { MessageChannel } from '../data/schema';

type DialogType = 'create' | 'edit' | 'delete' | 'manageBindings' | null;

interface MessageChannelsContextType {
  open: DialogType;
  setOpen: (open: DialogType) => void;
  currentRow: MessageChannel | null;
  setCurrentRow: (row: MessageChannel | null) => void;
  selectedMessageChannels: MessageChannel[];
  setSelectedMessageChannels: (channels: MessageChannel[]) => void;
  resetRowSelection: () => void;
  setResetRowSelection: (fn: () => void) => void;
}

const MessageChannelsContext = createContext<MessageChannelsContextType | undefined>(undefined);

export function MessageChannelsProvider({ children }: { children: React.ReactNode }) {
  const [open, setOpen] = useState<DialogType>(null);
  const [currentRow, setCurrentRow] = useState<MessageChannel | null>(null);
  const [selectedMessageChannels, setSelectedMessageChannels] = useState<MessageChannel[]>([]);
  const resetRowSelectionRef = useRef<() => void>(() => {});

  const handleSetOpen = useCallback((newOpen: DialogType) => {
    setOpen(newOpen);
  }, []);

  const handleSetCurrentRow = useCallback((row: MessageChannel | null) => {
    setCurrentRow(row);
  }, []);

  const handleSetSelectedMessageChannels = useCallback((channels: MessageChannel[]) => {
    setSelectedMessageChannels(channels);
  }, []);

  const handleSetResetRowSelection = useCallback((fn: () => void) => {
    resetRowSelectionRef.current = fn;
  }, []);

  const value = useMemo(
    () => ({
      open,
      setOpen: handleSetOpen,
      currentRow,
      setCurrentRow: handleSetCurrentRow,
      selectedMessageChannels,
      setSelectedMessageChannels: handleSetSelectedMessageChannels,
      resetRowSelection: () => resetRowSelectionRef.current(),
      setResetRowSelection: handleSetResetRowSelection,
    }),
    [
      open,
      handleSetOpen,
      currentRow,
      handleSetCurrentRow,
      selectedMessageChannels,
      handleSetSelectedMessageChannels,
      handleSetResetRowSelection,
    ]
  );

  return <MessageChannelsContext.Provider value={value}>{children}</MessageChannelsContext.Provider>;
}

export function useMessageChannels() {
  const ctx = useContext(MessageChannelsContext);
  if (!ctx) {
    throw new Error('useMessageChannels must be used within a MessageChannelsProvider');
  }
  return ctx;
}

export default MessageChannelsProvider;
