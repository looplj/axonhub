import { useModels } from '../context/models-context'
import { ModelsActionDialog } from './models-action-dialog'
import { ModelsDeleteDialog } from './models-delete-dialog'
import { ModelsAssociationDialog } from './models-association-dialog'
import { ModelSettingsDialog } from './models-settings-dialog'

export function ModelsDialogs() {
  const { open } = useModels()

  return (
    <>
      {(open === 'create' || open === 'edit') && <ModelsActionDialog />}
      {open === 'delete' && <ModelsDeleteDialog />}
      {open === 'association' && <ModelsAssociationDialog />}
      {open === 'settings' && <ModelSettingsDialog />}
    </>
  )
}
