'use client';

import { useState, useEffect } from 'react';
import { IconSearch, IconPlayerPlay } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useTestChannel } from '../data/channels';
import { Channel } from '../data/schema';

type TestStatus = 'not_started' | 'testing' | 'success' | 'failed';

interface ModelTestResult {
  modelName: string;
  status: TestStatus;
  latency?: number;
  error?: string;
}

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  channel: Channel;
}

export function ChannelsTestDialog({ open, onOpenChange, channel }: Props) {
  const { t } = useTranslation();
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedModels, setSelectedModels] = useState<string[]>([]);
  const [testResults, setTestResults] = useState<Record<string, ModelTestResult>>({});
  const [isTesting, setIsTesting] = useState(false);
  const testChannel = useTestChannel();

  // Filter models based on search query
  const filteredModels = channel.supportedModels.filter((model) => model.toLowerCase().includes(searchQuery.toLowerCase()));

  // Initialize test results when dialog opens
  useEffect(() => {
    if (open) {
      const initialResults: Record<string, ModelTestResult> = {};
      channel.supportedModels.forEach((model) => {
        initialResults[model] = {
          modelName: model,
          status: 'not_started',
        };
      });
      setTestResults(initialResults);
      setSelectedModels([]);
      setSearchQuery('');
    }
  }, [open, channel.supportedModels]);

  // Handle model selection
  const handleModelSelect = (modelName: string, checked: boolean) => {
    if (checked) {
      setSelectedModels((prev) => [...prev, modelName]);
    } else {
      setSelectedModels((prev) => prev.filter((m) => m !== modelName));
    }
  };

  // Handle select all
  const handleSelectAll = (checked: boolean) => {
    if (checked) {
      setSelectedModels(filteredModels);
    } else {
      setSelectedModels([]);
    }
  };

  // Test a single model
  const testModel = async (modelName: string) => {
    setTestResults((prev) => ({
      ...prev,
      [modelName]: { ...prev[modelName], status: 'testing' },
    }));

    try {
      const startTime = Date.now();
      const result = await testChannel.mutateAsync({
        channelID: channel.id,
        modelID: modelName,
      });
      const latency = (Date.now() - startTime) / 1000;

      setTestResults((prev) => ({
        ...prev,
        [modelName]: {
          ...prev[modelName],
          status: result.success ? 'success' : 'failed',
          latency: result.success ? result.latency || latency : undefined,
          error: result.success ? undefined : result.error || 'Test failed',
        },
      }));
    } catch (error) {
      setTestResults((prev) => ({
        ...prev,
        [modelName]: {
          ...prev[modelName],
          status: 'failed',
          error: error instanceof Error ? error.message : 'Unknown error',
        },
      }));
    }
  };

  // Test selected models
  const handleTestSelected = async () => {
    if (selectedModels.length === 0) return;

    setIsTesting(true);

    // Test models in parallel
    await Promise.all(selectedModels.map((model) => testModel(model)));

    setIsTesting(false);
  };

  // Get status badge
  const getStatusBadge = (status: TestStatus) => {
    switch (status) {
      case 'testing':
        return <Badge variant='secondary'>{t('channels.dialogs.test.testingModel')}</Badge>;
      case 'success':
        return (
          <Badge variant='default' className='border-green-200 bg-green-100 text-green-800'>
            {t('channels.dialogs.test.testSuccess')}
          </Badge>
        );
      case 'failed':
        return <Badge variant='destructive'>{t('channels.dialogs.test.testFailed')}</Badge>;
      default:
        return <Badge variant='outline'>{t('channels.dialogs.test.notStarted')}</Badge>;
    }
  };

  const isAllSelected = filteredModels.length > 0 && filteredModels.every((model) => selectedModels.includes(model));
  const isIndeterminate = selectedModels.length > 0 && !isAllSelected;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='flex max-h-[90vh] flex-col sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('channels.dialogs.test.title')}</DialogTitle>
          <DialogDescription>{t('channels.dialogs.test.description', { name: channel.name })}</DialogDescription>
        </DialogHeader>

        <div className='min-h-0 flex-1 space-y-4'>
          {/* Search */}
          <div className='relative'>
            <IconSearch className='text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2 transform' />
            <Input
              placeholder={t('channels.dialogs.test.searchPlaceholder')}
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className='pl-10'
            />
          </div>

          {/* Models Table */}
          <div className='min-h-0 flex-1 overflow-hidden rounded-lg border'>
            <div className='max-h-96 overflow-auto'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className='w-12'>
                      <Checkbox
                        checked={isAllSelected}
                        onCheckedChange={handleSelectAll}
                        ref={(el) => {
                          if (el) {
                            const input = el.querySelector('input') as HTMLInputElement;
                            if (input) {
                              input.indeterminate = isIndeterminate;
                            }
                          }
                        }}
                      />
                    </TableHead>
                    <TableHead>{t('channels.dialogs.test.modelNameColumn')}</TableHead>
                    <TableHead className='w-32'>{t('channels.dialogs.test.statusColumn')}</TableHead>
                    <TableHead className='w-24'></TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filteredModels.map((model) => {
                    const result = testResults[model];
                    return (
                      <TableRow key={model}>
                        <TableCell>
                          <Checkbox
                            checked={selectedModels.includes(model)}
                            onCheckedChange={(checked) => handleModelSelect(model, !!checked)}
                          />
                        </TableCell>
                        <TableCell className='font-medium'>
                          <div>{model}</div>
                          {result?.error && <div className='mt-1 text-xs text-red-600'>{result.error}</div>}
                        </TableCell>
                        <TableCell>
                          {getStatusBadge(result?.status || 'not_started')}
                          {result?.latency && <div className='text-muted-foreground mt-1 text-xs'>{result.latency.toFixed(2)}s</div>}
                        </TableCell>
                        <TableCell>
                          <Button
                            size='sm'
                            variant='outline'
                            onClick={() => testModel(model)}
                            disabled={result?.status === 'testing' || testChannel.isPending}
                          >
                            <IconPlayerPlay className='mr-1 h-3 w-3' />
                            {result?.status === 'testing' ? t('channels.dialogs.test.testingModel') : t('channels.dialogs.test.testModel')}
                          </Button>
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </div>
          </div>
        </div>

        <DialogFooter className='flex justify-between'>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('common.buttons.cancel')}
          </Button>
          <Button onClick={handleTestSelected} disabled={selectedModels.length === 0 || isTesting}>
            <IconPlayerPlay className='mr-2 h-4 w-4' />
            {t('channels.dialogs.test.testAllButton', { count: selectedModels.length })}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
