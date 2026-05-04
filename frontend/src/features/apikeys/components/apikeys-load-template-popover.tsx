import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { format } from 'date-fns';
import { zhCN } from 'date-fns/locale/zh-CN';
import { IconFileDownload, IconLoader2, IconTemplate, IconTrash } from '@tabler/icons-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
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
import { Popover, PopoverTrigger, PopoverContent } from '@/components/ui/popover';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useApiKeyProfileTemplates, useLoadApiKeyProfileTemplate, useDeleteApiKeyProfileTemplate } from '../data/apikeys';
import type { ApiKeyProfileTemplate, ApiKeyProfile } from '../data/schema';

interface ApiKeyLoadTemplatePopoverProps {
  apiKeyID: string;
  projectID: string | null;
  onLoadComplete?: (loadedProfiles: { activeProfile: string; profiles: ApiKeyProfile[] }) => void;
}

function TemplateItem({
  template,
  onLoad,
  isLoading,
  onDelete,
  isDeleting,
}: {
  template: ApiKeyProfileTemplate;
  onLoad: (template: ApiKeyProfileTemplate) => void;
  isLoading: boolean;
  onDelete: (template: ApiKeyProfileTemplate) => void;
  isDeleting: boolean;
}) {
  const { t, i18n } = useTranslation();
  const createdDate = format(new Date(template.createdAt), 'PP', {
    locale: i18n.language?.startsWith('zh') ? zhCN : undefined,
  });

  return (
    <div className='hover:bg-muted/50 flex items-start gap-3 rounded-md px-3 py-2.5 transition-colors'>
      <button
        type='button'
        className='flex min-w-0 flex-1 items-start gap-3 text-left disabled:opacity-50 disabled:cursor-not-allowed'
        onClick={() => onLoad(template)}
        disabled={isLoading || isDeleting}
      >
        <IconTemplate className='text-muted-foreground mt-0.5 h-4 w-4 shrink-0' />
        <div className='min-w-0 flex-1'>
          <div className='text-foreground text-sm font-medium'>{template.name}</div>
          {template.description && (
            <div className='text-muted-foreground mt-0.5 truncate text-xs'>
              {template.description}
            </div>
          )}
          <div className='text-muted-foreground/70 mt-1 text-[11px]'>{createdDate}</div>
        </div>
      </button>
      <button
        type='button'
        className='text-muted-foreground hover:text-destructive mt-0.5 shrink-0 rounded p-1 transition-colors disabled:opacity-50 disabled:cursor-not-allowed'
        onClick={(e) => {
          e.stopPropagation();
          onDelete(template);
        }}
        disabled={isDeleting}
        aria-label={t('apikeys.templates.deleteButton')}
      >
        {isDeleting ? <IconLoader2 className='h-3.5 w-3.5 animate-spin' /> : <IconTrash className='h-3.5 w-3.5' />}
      </button>
    </div>
  );
}

export function ApiKeyLoadTemplatePopover({
  apiKeyID,
  projectID,
  onLoadComplete,
}: ApiKeyLoadTemplatePopoverProps) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [loadingTemplateId, setLoadingTemplateId] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<ApiKeyProfileTemplate | null>(null);

  const { data: templates, isLoading: isLoadingTemplates } = useApiKeyProfileTemplates(projectID);
  const loadTemplate = useLoadApiKeyProfileTemplate();
  const deleteTemplate = useDeleteApiKeyProfileTemplate();
  const [deletingTemplateId, setDeletingTemplateId] = useState<string | null>(null);

  const handleLoad = (template: ApiKeyProfileTemplate) => {
    setLoadingTemplateId(template.id);
    loadTemplate.mutate(
      { templateID: template.id, apiKeyID },
      {
        onSuccess: (result) => {
          toast.success(t('apikeys.templates.loadSuccessMessage', { name: template.profile.name }));
          setLoadingTemplateId(null);
          setOpen(false);
          const profilesData = result?.loadApiKeyProfileTemplate?.profiles;
          if (profilesData) {
            onLoadComplete?.({
              activeProfile: profilesData.activeProfile || '',
              profiles: profilesData.profiles || [],
            });
          } else {
            onLoadComplete?.({ activeProfile: '', profiles: [] });
          }
        },
        onError: () => {
          toast.error(t('apikeys.templates.loadErrorMessage'));
          setLoadingTemplateId(null);
        },
      }
    );
  };

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
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button type='button' variant='outline' size='sm' className='flex items-center gap-2'>
            <IconFileDownload className='h-4 w-4' />
            {t('apikeys.templates.loadButton')}
          </Button>
        </PopoverTrigger>
        <PopoverContent className='w-80 p-0' align='start'>
          <div className='px-4 py-3 border-b'>
            <h4 className='text-sm font-medium'>{t('apikeys.templates.loadTitle')}</h4>
          </div>

          {isEmpty && (
            <div className='flex flex-col items-center justify-center gap-2 px-4 py-8'>
              <IconTemplate className='text-muted-foreground/50 h-8 w-8' />
              <p className='text-muted-foreground text-sm font-medium'>
                {t('apikeys.templates.emptyTitle')}
              </p>
              <p className='text-muted-foreground/70 text-center text-xs'>
                {t('apikeys.templates.emptyMessage')}
              </p>
            </div>
          )}

          {isLoadingTemplates && !isEmpty && (
            <div className='flex items-center justify-center py-8'>
              <IconLoader2 className='text-muted-foreground h-5 w-5 animate-spin' />
            </div>
          )}

          {!isEmpty && !isLoadingTemplates && (
            <ScrollArea className='max-h-[280px]'>
              <div className='py-1'>
                {templateList.map((template) => (
                  <TemplateItem
                    key={template.id}
                    template={template}
                    onLoad={handleLoad}
                    isLoading={loadingTemplateId === template.id}
                    onDelete={setDeleteTarget}
                    isDeleting={deletingTemplateId === template.id}
                  />
                ))}
              </div>
            </ScrollArea>
          )}
        </PopoverContent>
      </Popover>

      <AlertDialog open={deleteTarget !== null} onOpenChange={(isOpen) => !isOpen && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('apikeys.templates.deleteConfirmTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('apikeys.templates.deleteConfirmDescription', { name: deleteTarget?.name })}
            </AlertDialogDescription>
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
    </>
  );
}
