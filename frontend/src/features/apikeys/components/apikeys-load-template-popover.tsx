import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { format } from 'date-fns';
import { IconFileDownload, IconLoader2, IconTemplate } from '@tabler/icons-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Popover, PopoverTrigger, PopoverContent } from '@/components/ui/popover';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useApiKeyProfileTemplates, useLoadApiKeyProfileTemplate } from '../data/apikeys';
import type { ApiKeyProfileTemplate } from '../data/schema';

interface ApiKeyLoadTemplatePopoverProps {
  apiKeyID: string;
  projectID: string | null;
  onLoadComplete?: () => void;
}

function TemplateItem({
  template,
  onLoad,
  isLoading,
}: {
  template: ApiKeyProfileTemplate;
  onLoad: (template: ApiKeyProfileTemplate) => void;
  isLoading: boolean;
}) {
  const { t, i18n } = useTranslation();
  const createdDate = format(new Date(template.createdAt), 'PP', {
    locale: i18n.language?.startsWith('zh') ? undefined : undefined,
  });

  return (
    <button
      type='button'
      className='hover:bg-muted/50 flex w-full items-start gap-3 rounded-md px-3 py-2.5 text-left transition-colors disabled:opacity-50 disabled:cursor-not-allowed'
      onClick={() => onLoad(template)}
      disabled={isLoading}
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

  const { data: templates, isLoading: isLoadingTemplates } = useApiKeyProfileTemplates(projectID);
  const loadTemplate = useLoadApiKeyProfileTemplate();

  const handleLoad = (template: ApiKeyProfileTemplate) => {
    setLoadingTemplateId(template.id);
    loadTemplate.mutate(
      { templateID: template.id, apiKeyID },
      {
        onSuccess: () => {
          toast.success(t('apikeys.templates.loadSuccessMessage', { name: template.profile.name }));
          setLoadingTemplateId(null);
          setOpen(false);
          onLoadComplete?.();
        },
        onError: () => {
          toast.error(t('apikeys.templates.loadErrorMessage'));
          setLoadingTemplateId(null);
        },
      }
    );
  };

  const templateList = templates ?? [];
  const isEmpty = !isLoadingTemplates && templateList.length === 0;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button variant='outline' size='sm' className='flex items-center gap-2'>
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
                />
              ))}
            </div>
          </ScrollArea>
        )}
      </PopoverContent>
    </Popover>
  );
}
