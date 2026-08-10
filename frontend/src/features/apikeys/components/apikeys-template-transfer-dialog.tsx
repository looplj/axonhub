import { useEffect, useMemo, useState } from 'react';
import { IconCopy, IconDownload, IconLoader2, IconUpload } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { z } from 'zod';
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
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { useMyProjects } from '@/features/projects/data/projects';
import { useSelectedProjectId } from '@/stores/projectStore';
import {
  useApiKeyProfileTemplates,
  useCreateApiKeyProfileTemplate,
  useUpdateApiKeyProfileTemplate,
} from '../data/apikeys';
import { apiKeyProfileSchema, type ApiKeyProfile, type ApiKeyProfileTemplate } from '../data/schema';

const TEMPLATE_EXPORT_TYPE = 'axonhub-api-key-profile-template';

const templateExportSchema = z.object({
  version: z.literal(1),
  type: z.literal(TEMPLATE_EXPORT_TYPE),
  exportedAt: z.string().optional(),
  sourceProjectID: z.string().optional().nullable(),
  template: z.object({
    name: z.string().trim().min(1),
    description: z.string().optional().default(''),
    profile: apiKeyProfileSchema,
  }),
});

type TemplateExport = z.infer<typeof templateExportSchema>;
type TransferMode = 'copy' | 'import';

function buildTemplateExport(template: ApiKeyProfileTemplate): TemplateExport {
  return {
    version: 1,
    type: TEMPLATE_EXPORT_TYPE,
    exportedAt: new Date().toISOString(),
    sourceProjectID: template.projectID,
    template: {
      name: template.name,
      description: template.description ?? '',
      profile: template.profile,
    },
  };
}

export function exportApiKeyProfileTemplate(template: ApiKeyProfileTemplate) {
  const payload = buildTemplateExport(template);
  const blob = new Blob([`${JSON.stringify(payload, null, 2)}\n`], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  const safeName = template.name.trim().replace(/[^\p{L}\p{N}._-]+/gu, '-').replace(/^-+|-+$/g, '') || 'template';
  anchor.href = url;
  anchor.download = `${safeName}.axonhub-template.json`;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
}

interface ApiKeyTemplateTransferDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  mode: TransferMode;
  template?: ApiKeyProfileTemplate | null;
}

export function ApiKeyTemplateTransferDialog({ open, onOpenChange, mode, template }: ApiKeyTemplateTransferDialogProps) {
  const { t } = useTranslation();
  const selectedProjectId = useSelectedProjectId();
  const { data: projects = [], isLoading: isLoadingProjects } = useMyProjects();
  const createTemplate = useCreateApiKeyProfileTemplate();
  const updateTemplate = useUpdateApiKeyProfileTemplate();
  const [payload, setPayload] = useState<TemplateExport | null>(null);
  const [targetProjectID, setTargetProjectID] = useState('');
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [importError, setImportError] = useState('');
  const [confirmOverwrite, setConfirmOverwrite] = useState(false);
  const [fileInputKey, setFileInputKey] = useState(0);

  const availableProjects = useMemo(
    () => projects.filter((project) => project.status === 'active' && (mode !== 'copy' || project.id !== selectedProjectId)),
    [mode, projects, selectedProjectId]
  );

  useEffect(() => {
    if (!open) return;

    const initialPayload = mode === 'copy' && template ? buildTemplateExport(template) : null;
    setPayload(initialPayload);
    setName(initialPayload?.template.name ?? '');
    setDescription(initialPayload?.template.description ?? '');
    setTargetProjectID(mode === 'import' ? selectedProjectId ?? '' : '');
    setImportError('');
    setConfirmOverwrite(false);
    setFileInputKey((key) => key + 1);
  }, [mode, open, selectedProjectId, template]);

  useEffect(() => {
    if (!open || targetProjectID || availableProjects.length === 0) return;
    setTargetProjectID(availableProjects[0].id);
  }, [availableProjects, open, targetProjectID]);

  const { data: targetTemplates, isLoading: isLoadingTargetTemplates } = useApiKeyProfileTemplates(
    open && payload && targetProjectID ? targetProjectID : null
  );
  const matchingTemplate = useMemo(
    () => targetTemplates?.find((item) => item.name.trim().toLowerCase() === name.trim().toLowerCase()),
    [name, targetTemplates]
  );

  const sourceChannelIDs = payload?.template.profile.channelIDs ?? [];
  const crossesProjects = !payload?.sourceProjectID || payload.sourceProjectID !== targetProjectID;
  const removedChannelCount = crossesProjects ? sourceChannelIDs.length : 0;
  const isPending = createTemplate.isPending || updateTemplate.isPending;
  const canSubmit = !!payload && !!targetProjectID && !!name.trim() && !isLoadingTargetTemplates && !isPending;

  const profileForTarget = (): ApiKeyProfile | null => {
    if (!payload) return null;
    return {
      ...payload.template.profile,
      name: name.trim(),
      channelIDs: crossesProjects ? [] : payload.template.profile.channelIDs,
    };
  };

  const saveToTarget = async (overwrite: boolean) => {
    const profile = profileForTarget();
    if (!profile || !targetProjectID) return;

    try {
      if (overwrite && matchingTemplate) {
        await updateTemplate.mutateAsync({
          id: matchingTemplate.id,
          projectID: targetProjectID,
          input: {
            name: name.trim(),
            description,
            profile,
          },
        });
      } else {
        await createTemplate.mutateAsync({
          name: name.trim(),
          description,
          projectID: targetProjectID,
          profile,
        });
      }

      toast.success(
        t(overwrite ? 'apikeys.templateTransfer.overwriteSuccess' : 'apikeys.templateTransfer.saveSuccess', {
          name: name.trim(),
        })
      );
      setConfirmOverwrite(false);
      onOpenChange(false);
    } catch {
      toast.error(t('apikeys.templateTransfer.saveError'));
    }
  };

  const handleSubmit = () => {
    if (!canSubmit) return;
    if (matchingTemplate) {
      setConfirmOverwrite(true);
      return;
    }
    void saveToTarget(false);
  };

  const handleImportFile = async (file: File | undefined) => {
    setImportError('');
    setPayload(null);
    if (!file) return;

    if (file.size > 1024 * 1024) {
      setImportError(t('apikeys.templateTransfer.fileTooLarge'));
      return;
    }

    try {
      const parsed = templateExportSchema.parse(JSON.parse(await file.text()));
      setPayload(parsed);
      setName(parsed.template.name);
      setDescription(parsed.template.description ?? '');
    } catch {
      setImportError(t('apikeys.templateTransfer.invalidFile'));
    }
  };

  const noCopyTargets = mode === 'copy' && !isLoadingProjects && availableProjects.length === 0;

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className='sm:max-w-lg'>
          <DialogHeader className='text-left'>
            <DialogTitle className='flex items-center gap-2'>
              {mode === 'copy' ? <IconCopy className='h-5 w-5' /> : <IconUpload className='h-5 w-5' />}
              {t(mode === 'copy' ? 'apikeys.templateTransfer.copyTitle' : 'apikeys.templateTransfer.importTitle')}
            </DialogTitle>
            <DialogDescription>
              {t(mode === 'copy' ? 'apikeys.templateTransfer.copyDescription' : 'apikeys.templateTransfer.importDescription')}
            </DialogDescription>
          </DialogHeader>

          <div className='space-y-4'>
            {mode === 'import' && (
              <div className='space-y-2'>
                <Label htmlFor='api-key-template-file'>{t('apikeys.templateTransfer.fileLabel')}</Label>
                <Input
                  key={fileInputKey}
                  id='api-key-template-file'
                  type='file'
                  accept='.json,application/json'
                  onChange={(event) => void handleImportFile(event.target.files?.[0])}
                />
                {importError && <p className='text-destructive text-sm'>{importError}</p>}
              </div>
            )}

            {noCopyTargets && <p className='text-muted-foreground text-sm'>{t('apikeys.templateTransfer.noTargetProjects')}</p>}

            {payload && !noCopyTargets && (
              <>
                <div className='space-y-2'>
                  <Label>{t('apikeys.templateTransfer.targetProject')}</Label>
                  <Select value={targetProjectID} onValueChange={setTargetProjectID} disabled={isLoadingProjects || isPending}>
                    <SelectTrigger>
                      <SelectValue placeholder={t('apikeys.templateTransfer.selectProject')} />
                    </SelectTrigger>
                    <SelectContent>
                      {availableProjects.map((project) => (
                        <SelectItem key={project.id} value={project.id}>
                          {project.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                <div className='space-y-2'>
                  <Label htmlFor='transferred-template-name'>{t('apikeys.templates.nameLabel')}</Label>
                  <Input id='transferred-template-name' value={name} onChange={(event) => setName(event.target.value)} disabled={isPending} />
                </div>

                <div className='space-y-2'>
                  <Label htmlFor='transferred-template-description'>{t('apikeys.templates.descriptionLabel')}</Label>
                  <Textarea
                    id='transferred-template-description'
                    value={description}
                    onChange={(event) => setDescription(event.target.value)}
                    rows={3}
                    disabled={isPending}
                  />
                </div>

                {removedChannelCount > 0 && (
                  <div className='rounded-md border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-sm text-amber-700 dark:text-amber-300'>
                    {t('apikeys.templateTransfer.channelIDsRemoved', { count: removedChannelCount })}
                  </div>
                )}

                {matchingTemplate && (
                  <div className='rounded-md border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-sm text-amber-700 dark:text-amber-300'>
                    {t('apikeys.templateTransfer.nameConflict', { name: matchingTemplate.name })}
                  </div>
                )}
              </>
            )}
          </div>

          <DialogFooter className='gap-2'>
            <Button variant='outline' onClick={() => onOpenChange(false)} disabled={isPending}>
              {t('common.buttons.cancel')}
            </Button>
            <Button
              onClick={handleSubmit}
              disabled={!canSubmit}
              variant={matchingTemplate ? 'destructive' : 'default'}
              className='gap-2'
            >
              {isPending && <IconLoader2 className='h-4 w-4 animate-spin' />}
              {matchingTemplate ? t('apikeys.templates.overwriteButton') : t('apikeys.templateTransfer.saveButton')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={confirmOverwrite} onOpenChange={setConfirmOverwrite}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('apikeys.templates.overwriteConfirmTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('apikeys.templates.overwriteConfirmDescription', { name: matchingTemplate?.name ?? name.trim() })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isPending}>{t('common.buttons.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              onClick={(event) => {
                event.preventDefault();
                void saveToTarget(true);
              }}
              disabled={isPending}
              className='bg-destructive text-destructive-foreground hover:bg-destructive/90'
            >
              {isPending ? t('common.buttons.saving') : t('apikeys.templates.overwriteButton')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

export function TemplateExportButton({ template }: { template: ApiKeyProfileTemplate }) {
  const { t } = useTranslation();

  return (
    <button
      type='button'
      className='text-muted-foreground hover:text-foreground mt-0.5 shrink-0 rounded p-1 transition-colors'
      onClick={() => {
        exportApiKeyProfileTemplate(template);
        toast.success(t('apikeys.templateTransfer.exportSuccess', { name: template.name }));
      }}
      aria-label={t('apikeys.templateTransfer.exportButton')}
    >
      <IconDownload className='h-3.5 w-3.5' />
    </button>
  );
}
