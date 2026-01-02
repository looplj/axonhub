'use client';

import React, { createContext, useContext, useState } from 'react';
import { DataStorage } from '../data/data-storages';

interface DataStoragesContextType {
  isCreateDialogOpen: boolean;
  setIsCreateDialogOpen: (open: boolean) => void;
  isEditDialogOpen: boolean;
  setIsEditDialogOpen: (open: boolean) => void;
  isArchiveDialogOpen: boolean;
  setIsArchiveDialogOpen: (open: boolean) => void;
  editingDataStorage: DataStorage | null;
  setEditingDataStorage: (dataStorage: DataStorage | null) => void;
  archiveDataStorage: DataStorage | null;
  setArchiveDataStorage: (dataStorage: DataStorage | null) => void;
}

const DataStoragesContext = createContext<DataStoragesContextType | undefined>(undefined);

export function useDataStoragesContext() {
  const context = useContext(DataStoragesContext);
  if (!context) {
    throw new Error('useDataStoragesContext must be used within DataStoragesProvider');
  }
  return context;
}

interface DataStoragesProviderProps {
  children: React.ReactNode;
}

export default function DataStoragesProvider({ children }: DataStoragesProviderProps) {
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false);
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false);
  const [isArchiveDialogOpen, setIsArchiveDialogOpen] = useState(false);
  const [editingDataStorage, setEditingDataStorage] = useState<DataStorage | null>(null);
  const [archiveDataStorage, setArchiveDataStorage] = useState<DataStorage | null>(null);

  return (
    <DataStoragesContext.Provider
      value={{
        isCreateDialogOpen,
        setIsCreateDialogOpen,
        isEditDialogOpen,
        setIsEditDialogOpen,
        isArchiveDialogOpen,
        setIsArchiveDialogOpen,
        editingDataStorage,
        setEditingDataStorage,
        archiveDataStorage,
        setArchiveDataStorage,
      }}
    >
      {children}
    </DataStoragesContext.Provider>
  );
}
