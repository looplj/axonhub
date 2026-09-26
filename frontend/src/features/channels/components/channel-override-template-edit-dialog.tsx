// Inline editor for an existing channel override template, opened from the
// template picker inside the per-channel override dialog.
import { useTranslation } from 'react-i18next';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { ChannelOverrideTemplate, useUpdateChannelOverrideTemplate } from '../data/templates';
import { ChannelOverrideTemplateForm } from './channel-override-template-form';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  template: ChannelOverrideTemplate | null;
}

export function ChannelOverrideTemplateEditDialog({ open, onOpenChange, template }: Props) {
  const { t } = useTranslation();
  const updateTemplate = useUpdateChannelOverrideTemplate();

  const handleSubmit = async (value: {
    name: string;
    description: string;
    headerOverrideOperations: ChannelOverrideTemplate['headerOverrideOperations'];
    bodyOverrideOperations: ChannelOverrideTemplate['bodyOverrideOperations'];
  }) => {
    if (!template) return;

    try {
      await updateTemplate.mutateAsync({
        id: template.id,
        input: {
          name: value.name,
          description: value.description,
          clearDescription: value.description === '',
          headerOverrideOperations: value.headerOverrideOperations,
          bodyOverrideOperations: value.bodyOverrideOperations,
        },
      });
      onOpenChange(false);
    } catch (_error) {
      // Error already handled by mutation
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='flex h-[80vh] flex-col p-0 sm:max-w-[760px]'>
        <DialogHeader className='border-b px-6 py-4'>
          <DialogTitle>{t('channels.templates.dialogs.edit.title')}</DialogTitle>
          <DialogDescription>{t('channels.templates.dialogs.edit.description')}</DialogDescription>
        </DialogHeader>

        <div className='flex min-h-0 flex-1 flex-col px-6 py-4'>
          {open && template && (
            <ChannelOverrideTemplateForm
              key={template.id}
              initialValue={{
                name: template.name,
                description: template.description ?? '',
                headerOverrideOperations: template.headerOverrideOperations,
                bodyOverrideOperations: template.bodyOverrideOperations,
              }}
              isSaving={updateTemplate.isPending}
              submitLabel={t('common.buttons.save')}
              onSubmit={handleSubmit}
            />
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}

