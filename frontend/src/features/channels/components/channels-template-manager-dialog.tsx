// Standalone manager for channel override templates: list, create, edit, and
// delete reusable header/body override configurations in one place.
import { useCallback, useEffect, useMemo, useState } from 'react';
import { ClipboardList, Loader2, Pencil, Plus, Search, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
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
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useDebounce } from '@/hooks/use-debounce';
import {
  ChannelOverrideTemplate,
  useChannelOverrideTemplates,
  useCreateChannelOverrideTemplate,
  useDeleteChannelOverrideTemplate,
  useUpdateChannelOverrideTemplate,
} from '../data/templates';
import { ChannelOverrideTemplateForm, ChannelOverrideTemplateFormValue } from './channel-override-template-form';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function ChannelsTemplateManagerDialog({ open, onOpenChange }: Props) {
  const { t } = useTranslation();
  const [searchValue, setSearchValue] = useState('');
  const debouncedSearch = useDebounce(searchValue, 300);
  const [selectedTemplateId, setSelectedTemplateId] = useState<string | null>(null);
  const [selectedSnapshot, setSelectedSnapshot] = useState<ChannelOverrideTemplate | null>(null);
  const [isCreating, setIsCreating] = useState(false);
  const [pendingDelete, setPendingDelete] = useState<ChannelOverrideTemplate | null>(null);

  const createTemplate = useCreateChannelOverrideTemplate();
  const updateTemplate = useUpdateChannelOverrideTemplate();
  const deleteTemplate = useDeleteChannelOverrideTemplate();

  const { data, isLoading } = useChannelOverrideTemplates({ search: debouncedSearch, first: 100 }, { enabled: open });

  const templates = useMemo(() => data?.edges?.map((edge) => edge.node) || [], [data]);

  const selectedTemplate = useMemo(
    () =>
      templates.find((template) => template.id === selectedTemplateId) ??
      (selectedSnapshot?.id === selectedTemplateId ? selectedSnapshot : null),
    [templates, selectedTemplateId, selectedSnapshot]
  );

  useEffect(() => {
    if (!open) {
      setSearchValue('');
      setSelectedTemplateId(null);
      setSelectedSnapshot(null);
      setIsCreating(false);
      setPendingDelete(null);
    }
  }, [open]);

  const handleCreate = useCallback(
    async (value: ChannelOverrideTemplateFormValue) => {
      try {
        const created = await createTemplate.mutateAsync({
          name: value.name,
          description: value.description || undefined,
          headerOverrideOperations: value.headerOverrideOperations,
          bodyOverrideOperations: value.bodyOverrideOperations,
        });
        setIsCreating(false);
        setSelectedTemplateId(created.id);
        setSelectedSnapshot(created);
      } catch (_error) {
        // Error already handled by mutation
      }
    },
    [createTemplate]
  );

  const handleUpdate = useCallback(
    async (value: ChannelOverrideTemplateFormValue) => {
      if (!selectedTemplate) return;

      try {
        await updateTemplate.mutateAsync({
          id: selectedTemplate.id,
          input: {
            name: value.name,
            description: value.description,
            clearDescription: value.description === '',
            headerOverrideOperations: value.headerOverrideOperations,
            bodyOverrideOperations: value.bodyOverrideOperations,
          },
        });
      } catch (_error) {
        // Error already handled by mutation
      }
    },
    [selectedTemplate, updateTemplate]
  );

  const handleDelete = useCallback(async () => {
    if (!pendingDelete) return;

    const deletingId = pendingDelete.id;
    try {
      await deleteTemplate.mutateAsync(deletingId);
      if (selectedTemplateId === deletingId) {
        setSelectedTemplateId(null);
        setSelectedSnapshot(null);
      }
    } catch (_error) {
      // Error already handled by mutation
    } finally {
      setPendingDelete(null);
    }
  }, [pendingDelete, deleteTemplate, selectedTemplateId]);

  const showEditor = isCreating || selectedTemplate !== null;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='flex h-[85vh] flex-col p-0 sm:max-w-[960px]'>
        <DialogHeader className='shrink-0 border-b px-6 py-4 pr-12'>
          <DialogTitle className='flex items-center gap-2'>
            <ClipboardList className='h-5 w-5' />
            {t('channels.templates.manager.title')}
          </DialogTitle>
          <DialogDescription>{t('channels.templates.manager.description')}</DialogDescription>
        </DialogHeader>

        <div className='flex min-h-0 flex-1'>
          <div className='flex w-[280px] shrink-0 flex-col border-r'>
            <div className='space-y-3 p-4'>
              <div className='relative'>
                <Search className='text-muted-foreground absolute top-1/2 left-2.5 h-4 w-4 -translate-y-1/2' />
                <Input
                  className='pl-8'
                  placeholder={t('channels.templates.searchPlaceholder')}
                  value={searchValue}
                  onChange={(event) => setSearchValue(event.target.value)}
                />
              </div>
              <Button
                type='button'
                className='w-full'
                onClick={() => {
                  setIsCreating(true);
                  setSelectedTemplateId(null);
                }}
                data-testid='template-manager-create'
              >
                <Plus className='mr-2 h-4 w-4' />
                {t('channels.templates.manager.createButton')}
              </Button>
            </div>

            <ScrollArea className='min-h-0 flex-1'>
              <div className='space-y-1 px-4 pb-4'>
                {isLoading && (
                  <div className='flex justify-center py-8'>
                    <Loader2 className='text-muted-foreground h-5 w-5 animate-spin' />
                  </div>
                )}

                {!isLoading && templates.length === 0 && (
                  <p className='text-muted-foreground py-8 text-center text-sm'>{t('channels.templates.noTemplates')}</p>
                )}

                {templates.map((template) => {
                  const isActive = !isCreating && template.id === selectedTemplateId;
                  return (
                    <div
                      key={template.id}
                      className={`group flex items-start gap-1 rounded-md border p-2 transition-colors ${
                        isActive ? 'bg-accent border-accent-foreground/20' : 'hover:bg-accent/50 border-transparent'
                      }`}
                    >
                      <button
                        type='button'
                        className='min-w-0 flex-1 text-left'
                        data-testid={`template-manager-item-${template.id}`}
                        onClick={() => {
                          setIsCreating(false);
                          setSelectedTemplateId(template.id);
                          setSelectedSnapshot(template);
                        }}
                      >
                        <span className='block truncate text-sm font-medium'>{template.name}</span>
                        <span className='text-muted-foreground block truncate text-xs'>
                          {template.description ||
                            t('channels.templates.manager.operationSummary', {
                              headers: template.headerOverrideOperations?.length ?? 0,
                              body: template.bodyOverrideOperations?.length ?? 0,
                            })}
                        </span>
                      </button>
                      <div className='flex shrink-0 items-center gap-0.5'>
                        <Button
                          type='button'
                          variant='ghost'
                          size='icon'
                          className='h-6 w-6'
                          title={t('common.buttons.edit')}
                          onClick={() => {
                            setIsCreating(false);
                            setSelectedTemplateId(template.id);
                            setSelectedSnapshot(template);
                          }}
                        >
                          <Pencil className='h-3.5 w-3.5' />
                        </Button>
                        <Button
                          type='button'
                          variant='ghost'
                          size='icon'
                          className='text-muted-foreground hover:text-destructive h-6 w-6'
                          title={t('common.buttons.delete')}
                          onClick={() => setPendingDelete(template)}
                        >
                          <Trash2 className='h-3.5 w-3.5' />
                        </Button>
                      </div>
                    </div>
                  );
                })}
              </div>
            </ScrollArea>
          </div>

          <div className='flex min-h-0 flex-1 flex-col p-6'>
            {!showEditor && (
              <div className='text-muted-foreground flex flex-1 items-center justify-center text-center text-sm'>
                {templates.length === 0
                  ? t('channels.templates.manager.emptyState')
                  : t('channels.templates.manager.selectHint')}
              </div>
            )}

            {isCreating && (
              <ChannelOverrideTemplateForm
                key='template-create'
                isSaving={createTemplate.isPending}
                submitLabel={t('common.buttons.create')}
                onSubmit={handleCreate}
              />
            )}

            {!isCreating && selectedTemplate && (
              <ChannelOverrideTemplateForm
                key={selectedTemplate.id}
                initialValue={{
                  name: selectedTemplate.name,
                  description: selectedTemplate.description ?? '',
                  headerOverrideOperations: selectedTemplate.headerOverrideOperations,
                  bodyOverrideOperations: selectedTemplate.bodyOverrideOperations,
                }}
                isSaving={updateTemplate.isPending}
                submitLabel={t('common.buttons.save')}
                onSubmit={handleUpdate}
              />
            )}
          </div>
        </div>
      </DialogContent>

      <AlertDialog open={pendingDelete !== null} onOpenChange={(isOpen) => !isOpen && setPendingDelete(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('channels.templates.manager.deleteTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('channels.templates.manager.deleteDescription', { name: pendingDelete?.name ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteTemplate.isPending}>{t('common.buttons.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              onClick={(event) => {
                event.preventDefault();
                void handleDelete();
              }}
              disabled={deleteTemplate.isPending}
            >
              {deleteTemplate.isPending ? t('channels.templates.manager.deleting') : t('common.buttons.delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Dialog>
  );
}
