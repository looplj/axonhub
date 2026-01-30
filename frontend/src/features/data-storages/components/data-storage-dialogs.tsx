'use client';

import { CreateDataStorageDialog } from './create-data-storage-dialog';
import { EditDataStorageDialog } from './edit-data-storage-dialog';
import { ArchiveDataStorageDialog } from './archive-data-storage-dialog';

export function DataStorageDialogs() {
  return (
    <>
      <CreateDataStorageDialog />
      <EditDataStorageDialog />
      <ArchiveDataStorageDialog />
    </>
  );
}
