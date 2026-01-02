import { useEffect, useState, useMemo, useCallback } from 'react';
import { IconPlus, IconTrash } from '@tabler/icons-react';
import { toc } from '@lobehub/icons';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { AutoComplete } from '@/components/auto-complete';
import { AutoCompleteSelect } from '@/components/auto-complete-select';
import { useModels } from '../context/models-context';
import { DEVELOPER_IDS, DEVELOPER_ICONS } from '../data/constants';
import { useBulkCreateModels } from '../data/models';
import providersDataRaw from '../data/providers.json';
import { providersDataSchema, type ProvidersData } from '../data/providers.schema';
import { CreateModelInput, ModelCard } from '../data/schema';

interface ModelRow {
  id: string;
  modelId: string;
  developer: string;
  name: string;
  icon: string;
  group: string;
  modelCard: ModelCard | null;
}

interface ValidationErrors {
  [rowId: string]: {
    developer?: boolean;
    modelId?: boolean;
    name?: boolean;
    icon?: boolean;
  };
}

const MAX_ROWS = 10;

function generateId(): string {
  return `${Date.now()}-${Math.random().toString(36).substring(2, 9)}`;
}

function isDeveloper(provider: string) {
  return DEVELOPER_IDS.includes(provider);
}

export function ModelsBatchCreateDialog() {
  const { t } = useTranslation();
  const { open, setOpen } = useModels();
  const bulkCreateModels = useBulkCreateModels();
  const [rows, setRows] = useState<ModelRow[]>([]);
  const [validationErrors, setValidationErrors] = useState<ValidationErrors>({});
  const [dialogContent, setDialogContent] = useState<HTMLDivElement | null>(null);

  const isOpen = open === 'batchCreate';

  const providersData = useMemo((): ProvidersData => {
    return providersDataSchema.parse(providersDataRaw);
  }, []);

  const providers = useMemo(() => {
    return Object.entries(providersData.providers)
      .filter(([key]) => isDeveloper(key))
      .map(([key, provider]) => ({
        id: key,
        name: provider.display_name || provider.name,
        models: provider.models || [],
      }));
  }, [providersData]);

  const developerOptions = useMemo(() => {
    return DEVELOPER_IDS.map((id) => ({
      value: id,
      label: id,
    }));
  }, []);

  const iconOptions = useMemo(() => {
    return (
      Object.entries(toc)
        // @ts-ignore
        .filter(([_, value]) => value.group == 'provider' || value.group == 'model')
        .map(([_, value]) => ({
          // @ts-ignore
          value: value.id,
          // @ts-ignore
          label: value.id,
        }))
    );
  }, []);

  useEffect(() => {
    if (isOpen && rows.length === 0) {
      setRows([
        {
          id: generateId(),
          modelId: '',
          developer: '',
          name: '',
          icon: '',
          group: '',
          modelCard: null,
        },
      ]);
    }
  }, [isOpen, rows.length]);

  const handleAddRow = useCallback(() => {
    if (rows.length >= MAX_ROWS) {
      toast.error(t('models.dialogs.batchCreate.maxRowsReached', { max: MAX_ROWS }));
      return;
    }
    setRows((prev) => [
      ...prev,
      {
        id: generateId(),
        modelId: '',
        developer: '',
        name: '',
        icon: '',
        group: '',
        modelCard: null,
      },
    ]);
  }, [rows.length, t]);

  const handleRemoveRow = useCallback((id: string) => {
    setRows((prev) => prev.filter((row) => row.id !== id));
  }, []);

  const handleModelIdChange = useCallback(
    (id: string, modelId: string) => {
      setRows((prev) =>
        prev.map((row) => {
          if (row.id !== id) return row;

          if (!row.developer) {
            return { ...row, modelId, name: '', group: '', modelCard: null };
          }

          const provider = providers.find((p) => p.id === row.developer);
          if (!provider) {
            return { ...row, modelId, name: '', group: '', modelCard: null };
          }

          const selectedModel = provider.models.find((m) => m.id === modelId);
          if (selectedModel) {
            const modelCard: ModelCard = {
              reasoning: {
                supported: selectedModel.reasoning?.supported || false,
                default: selectedModel.reasoning?.default || false,
              },
              toolCall: selectedModel.tool_call || false,
              temperature: selectedModel.temperature || false,
              modalities: {
                input: selectedModel.modalities?.input || [],
                output: selectedModel.modalities?.output || [],
              },
              vision: selectedModel.attachment || false,
              cost: {
                input: selectedModel.cost?.input || 0,
                output: selectedModel.cost?.output || 0,
                cacheRead: selectedModel.cost?.cache_read,
                cacheWrite: selectedModel.cost?.cache_write,
              },
              limit: {
                context: selectedModel.limit?.context || 0,
                output: selectedModel.limit?.output || 0,
              },
              knowledge: selectedModel.knowledge,
              releaseDate: selectedModel.release_date,
              lastUpdated: selectedModel.last_updated,
            };
            return {
              ...row,
              modelId,
              name: selectedModel.display_name || selectedModel.name || '',
              group: selectedModel.family || row.developer,
              modelCard,
            };
          }

          return { ...row, modelId, name: '', group: row.developer, modelCard: null };
        })
      );
    },
    [providers]
  );

  const handleDeveloperChange = useCallback((id: string, developer: string) => {
    setRows((prev) =>
      prev.map((row) => {
        if (row.id !== id) return row;
        const icon = DEVELOPER_ICONS[developer] || developer;
        return {
          ...row,
          developer,
          icon,
          modelId: '',
          name: '',
          group: '',
          modelCard: null,
        };
      })
    );
  }, []);

  const handleNameChange = useCallback((id: string, name: string) => {
    setRows((prev) => prev.map((row) => (row.id === id ? { ...row, name } : row)));
  }, []);

  const handleIconChange = useCallback((id: string, icon: string) => {
    setRows((prev) => prev.map((row) => (row.id === id ? { ...row, icon } : row)));
  }, []);

  const clearValidationError = useCallback(
    (rowId: string, field: keyof ValidationErrors[string]) => {
      if (!validationErrors[rowId]?.[field]) return;

      setValidationErrors((prev) => {
        const newErrors = { ...prev };
        if (newErrors[rowId]) {
          const rowErrors = { ...newErrors[rowId] };
          delete rowErrors[field];
          if (Object.keys(rowErrors).length === 0) {
            delete newErrors[rowId];
          } else {
            newErrors[rowId] = rowErrors;
          }
        }
        return newErrors;
      });
    },
    [validationErrors]
  );

  const handleSubmit = useCallback(async () => {
    const errors: ValidationErrors = {};
    rows.forEach((row) => {
      const rowErrors: ValidationErrors[string] = {};
      if (!row.developer) rowErrors.developer = true;
      if (!row.modelId) rowErrors.modelId = true;
      if (!row.name) rowErrors.name = true;
      if (!row.icon) rowErrors.icon = true;
      if (Object.keys(rowErrors).length > 0) {
        errors[row.id] = rowErrors;
      }
    });

    if (Object.keys(errors).length > 0) {
      setValidationErrors(errors);
      return;
    }

    setValidationErrors({});

    const validRows = rows.filter((row) => row.modelId && row.developer && row.name && row.icon && row.group);

    if (validRows.length === 0) {
      return;
    }

    const inputs: CreateModelInput[] = validRows.map((row) => ({
      developer: row.developer,
      modelID: row.modelId,
      type: 'chat',
      name: row.name,
      icon: row.icon,
      group: row.group,
      modelCard: row.modelCard || {
        reasoning: { supported: false, default: false },
        toolCall: false,
        temperature: false,
        modalities: { input: [], output: [] },
        vision: false,
        cost: { input: 0, output: 0 },
        limit: { context: 0, output: 0 },
      },
      settings: {
        associations: [
          {
            type: 'model',
            priority: 0,
            modelId: {
              modelId: row.modelId,
            },
          },
        ],
      },
    }));

    try {
      await bulkCreateModels.mutateAsync(inputs);
      handleClose();
    } catch (_error) {
      // Error is handled by mutation
    }
  }, [rows, bulkCreateModels, t]);

  const handleClose = useCallback(() => {
    setOpen(null);
    setRows([]);
    setValidationErrors({});
  }, [setOpen]);

  const getModelIdOptions = useCallback(
    (developer: string) => {
      const provider = providers.find((p) => p.id === developer);
      if (!provider) return [];
      return provider.models.map((m) => ({
        value: m.id,
        label: m.id,
      }));
    },
    [providers]
  );

  return (
    <Dialog open={isOpen} onOpenChange={handleClose}>
      <DialogContent ref={setDialogContent} className='flex flex-col overflow-hidden sm:max-w-4xl' style={{ height: '600px' }}>
        <DialogHeader className='flex-shrink-0 text-left'>
          <DialogTitle>{t('models.dialogs.batchCreate.title')}</DialogTitle>
          <DialogDescription>{t('models.dialogs.batchCreate.description')}</DialogDescription>
        </DialogHeader>

        <div className='min-h-0 flex-1 overflow-y-auto pr-2'>
          <div className='space-y-2'>
            {rows.map((row) => (
              <div key={row.id} className='rounded-lg border p-2'>
                <div className='flex items-start gap-2'>
                  <div className='flex-[2] space-y-1'>
                    <AutoComplete
                      selectedValue={row.developer}
                      onSelectedValueChange={(value) => {
                        handleDeveloperChange(row.id, value);
                        clearValidationError(row.id, 'developer');
                        clearValidationError(row.id, 'icon');
                      }}
                      searchValue={row.developer}
                      onSearchValueChange={(value) => handleDeveloperChange(row.id, value)}
                      items={developerOptions}
                      placeholder={t('models.fields.developer')}
                      emptyMessage={t('models.fields.noModels')}
                      portalContainer={dialogContent}
                    />
                    {validationErrors[row.id]?.developer && (
                      <p className='text-xs text-red-600'>{t('models.dialogs.batchCreate.required')}</p>
                    )}
                  </div>
                  <div className='flex-[3] space-y-1'>
                    <AutoComplete
                      selectedValue={row.modelId}
                      onSelectedValueChange={(value) => {
                        handleModelIdChange(row.id, value);
                        clearValidationError(row.id, 'modelId');
                        clearValidationError(row.id, 'name');
                        clearValidationError(row.id, 'icon');
                      }}
                      searchValue={row.modelId}
                      onSearchValueChange={(value) => handleModelIdChange(row.id, value)}
                      items={row.developer ? getModelIdOptions(row.developer) : []}
                      placeholder={t('models.fields.modelId')}
                      emptyMessage={t('models.fields.noModels')}
                      portalContainer={dialogContent}
                    />
                    {validationErrors[row.id]?.modelId && (
                      <p className='text-xs text-red-600'>{t('models.dialogs.batchCreate.required')}</p>
                    )}
                  </div>
                  <div className='flex-[2] space-y-1'>
                    <Input
                      value={row.name}
                      onChange={(e) => {
                        handleNameChange(row.id, e.target.value);
                        clearValidationError(row.id, 'name');
                      }}
                      placeholder={t('models.fields.name')}
                    />
                    {validationErrors[row.id]?.name && <p className='text-xs text-red-600'>{t('models.dialogs.batchCreate.required')}</p>}
                  </div>
                  <div className='flex-[3] space-y-1'>
                    <AutoCompleteSelect
                      selectedValue={row.icon}
                      onSelectedValueChange={(value) => {
                        handleIconChange(row.id, value);
                        clearValidationError(row.id, 'icon');
                      }}
                      items={iconOptions}
                      placeholder={t('models.fields.icon')}
                      emptyMessage={t('models.fields.noIcons')}
                      portalContainer={dialogContent}
                    />
                    {validationErrors[row.id]?.icon && <p className='text-xs text-red-600'>{t('models.dialogs.batchCreate.required')}</p>}
                  </div>
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    className='h-8 w-8 flex-shrink-0'
                    onClick={() => handleRemoveRow(row.id)}
                    disabled={rows.length === 1}
                  >
                    <IconTrash className='h-4 w-4' />
                  </Button>
                </div>
              </div>
            ))}
          </div>
        </div>

        <div className='flex flex-shrink-0 flex-col gap-3 border-t pt-4'>
          <Button type='button' variant='outline' onClick={handleAddRow} disabled={rows.length >= MAX_ROWS}>
            <IconPlus className='mr-2 h-4 w-4' />
            {t('models.dialogs.batchCreate.addRow')} ({rows.length}/{MAX_ROWS})
          </Button>
          <div className='flex justify-end gap-2'>
            <Button type='button' variant='outline' onClick={handleClose}>
              {t('common.buttons.cancel')}
            </Button>
            <Button type='button' onClick={handleSubmit} disabled={bulkCreateModels.isPending}>
              {t('common.buttons.create')}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
