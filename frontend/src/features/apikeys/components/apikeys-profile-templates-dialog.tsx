import { useState } from 'react';
import { format } from 'date-fns';
import { IconLoader2, IconPencil, IconPlus, IconTemplate, IconTrash } from '@tabler/icons-react';
import { zhCN, enUS } from 'date-fns/locale';
import { useTranslation } from 'react-i18next';
import { useSelectedProjectId } from '@/stores/projectStore';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useApiKeyProfileTemplates, useDeleteApiKeyProfileTemplate } from '../data/apikeys';
import type { ApiKeyProfileTemplate } from '../data/schema';
import { ApiKeyCreateTemplateDialog } from './apikeys-create-template-dialog';
import { ApiKeyEditTemplateDialog } from './apikeys-edit-template-dialog';

interface ApiKeysProfileTemplatesDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

function TemplateItem({
  template,
  onDelete,
  isDeleting,
  onEdit,
  isEditing,
}: {
  template: ApiKeyProfileTemplate;
  onDelete: (template: ApiKeyProfileTemplate) => void;
  isDeleting: boolean;
  onEdit: (template: ApiKeyProfileTemplate) => void;
  isEditing: boolean;
}) {
  const { t, i18n } = useTranslation();
  const locale = i18n.language?.startsWith('zh') ? zhCN : enUS;
  const createdDate = format(new Date(template.createdAt), 'PP', { locale });
  const mappingCount = template.profile?.modelMappings?.length ?? 0;

  return (
    <div className='hover:bg-muted/50 flex items-start gap-3 rounded-md px-3 py-2.5 transition-colors'>
      <IconTemplate className='text-muted-foreground mt-0.5 h-4 w-4 shrink-0' />
      <div className='min-w-0 flex-1'>
        <div className='text-foreground text-sm font-medium'>{template.name}</div>
        {template.description && <div className='text-muted-foreground mt-0.5 truncate text-xs'>{template.description}</div>}
        <div className='text-muted-foreground/70 mt-1 flex items-center gap-3 text-[11px]'>
          <span>{template.profile?.name}</span>
          {mappingCount > 0 && <span>{t('apikeys.profileTemplates.mappingCount', { count: mappingCount })}</span>}
          <span>{createdDate}</span>
        </div>
      </div>
      <button
        type='button'
        className='text-muted-foreground hover:text-foreground mt-0.5 shrink-0 rounded p-1 transition-colors disabled:cursor-not-allowed disabled:opacity-50'
        onClick={() => onEdit(template)}
        disabled={isEditing}
        aria-label={t('apikeys.profileTemplates.editButton')}
      >
        <IconPencil className='h-3.5 w-3.5' />
      </button>
      <button
        type='button'
        className='text-muted-foreground hover:text-destructive mt-0.5 shrink-0 rounded p-1 transition-colors disabled:cursor-not-allowed disabled:opacity-50'
        onClick={() => onDelete(template)}
        disabled={isDeleting}
        aria-label={t('apikeys.templates.deleteButton')}
      >
        {isDeleting ? <IconLoader2 className='h-3.5 w-3.5 animate-spin' /> : <IconTrash className='h-3.5 w-3.5' />}
      </button>
    </div>
  );
}

export function ApiKeysProfileTemplatesDialog({ open, onOpenChange }: ApiKeysProfileTemplatesDialogProps) {
  const { t } = useTranslation();
  const selectedProjectId = useSelectedProjectId();
  const [deleteTarget, setDeleteTarget] = useState<ApiKeyProfileTemplate | null>(null);
  const [deletingTemplateId, setDeletingTemplateId] = useState<string | null>(null);
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<ApiKeyProfileTemplate | null>(null);
  const [editingTemplateId, setEditingTemplateId] = useState<string | null>(null);

  const { data: templates, isLoading: isLoadingTemplates } = useApiKeyProfileTemplates(selectedProjectId);
  const deleteTemplate = useDeleteApiKeyProfileTemplate();

  const confirmDelete = () => {
    if (!deleteTarget) return;
    const targetId = deleteTarget.id;
    setDeletingTemplateId(targetId);
    setDeleteTarget(null);
    deleteTemplate.mutate(targetId, {
      onSettled: () => {
        setDeletingTemplateId(null);
      },
    });
  };

  const templateList = templates ?? [];
  const isEmpty = !isLoadingTemplates && templateList.length === 0;

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className='sm:max-w-lg'>
          <DialogHeader className='text-left'>
            <DialogTitle className='flex items-center gap-2'>
              <IconTemplate className='h-5 w-5' />
              {t('apikeys.profileTemplates.title')}
            </DialogTitle>
            <DialogDescription>{t('apikeys.profileTemplates.description')}</DialogDescription>
          </DialogHeader>

          {!isEmpty && !isLoadingTemplates && (
            <div className='flex justify-end'>
              <Button variant='outline' size='sm' onClick={() => setCreateDialogOpen(true)} className='flex items-center gap-2'>
                <IconPlus className='h-4 w-4' />
                {t('apikeys.profileTemplates.createButton')}
              </Button>
            </div>
          )}

          {isEmpty && (
            <div className='flex flex-col items-center justify-center gap-2 py-8'>
              <IconTemplate className='text-muted-foreground/50 h-8 w-8' />
              <p className='text-muted-foreground text-sm font-medium'>{t('apikeys.templates.emptyTitle')}</p>
              <p className='text-muted-foreground/70 text-center text-xs'>{t('apikeys.profileTemplates.emptyHint')}</p>
            </div>
          )}

          {isLoadingTemplates && !isEmpty && (
            <div className='flex items-center justify-center py-8'>
              <IconLoader2 className='text-muted-foreground h-5 w-5 animate-spin' />
            </div>
          )}

          {!isEmpty && !isLoadingTemplates && (
            <ScrollArea className='max-h-[400px]'>
              <div className='py-1'>
                {templateList.map((template) => (
                  <TemplateItem
                    key={template.id}
                    template={template}
                    onDelete={setDeleteTarget}
                    isDeleting={deletingTemplateId === template.id}
                    onEdit={setEditTarget}
                    isEditing={editingTemplateId === template.id}
                  />
                ))}
              </div>
            </ScrollArea>
          )}

          <div className='flex justify-end gap-2'>
            {isEmpty && (
              <Button onClick={() => setCreateDialogOpen(true)} size='sm' className='flex items-center gap-2'>
                <IconPlus className='h-4 w-4' />
                {t('apikeys.profileTemplates.createButton')}
              </Button>
            )}
            <Button variant='outline' onClick={() => onOpenChange(false)}>
              {t('common.buttons.close')}
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      <AlertDialog open={deleteTarget !== null} onOpenChange={(isOpen) => !isOpen && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('apikeys.templates.deleteConfirmTitle')}</AlertDialogTitle>
            <AlertDialogDescription>{t('apikeys.templates.deleteConfirmDescription', { name: deleteTarget?.name })}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.buttons.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              onClick={confirmDelete}
              disabled={deleteTemplate.isPending}
              className='bg-destructive text-destructive-foreground hover:bg-destructive/90'
            >
              {deleteTemplate.isPending ? t('common.buttons.saving') : t('apikeys.templates.deleteButton')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {createDialogOpen && <ApiKeyCreateTemplateDialog open={createDialogOpen} onOpenChange={setCreateDialogOpen} />}
      {editTarget && <ApiKeyEditTemplateDialog open={!!editTarget} onOpenChange={(open) => { if (!open) setEditTarget(null); }} template={editTarget} />}
    </>
  );
}
