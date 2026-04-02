'use client';

import { Loader2, Pencil, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useProxyPresets, useDeleteProxyPreset } from '../data/system';
import { ProxyPresetEditDialog } from './proxy-preset-edit-dialog';

export function ProxyPresetsSettings() {
  const { t } = useTranslation();
  const { data: presets, isLoading } = useProxyPresets();
  const deletePreset = useDeleteProxyPreset();

  if (isLoading) {
    return (
      <div className='flex h-32 items-center justify-center'>
        <Loader2 className='h-6 w-6 animate-spin' />
        <span className='text-muted-foreground ml-2'>{t('common.loading')}</span>
      </div>
    );
  }

  return (
    <div className='space-y-6'>
      <Card>
        <CardHeader>
          <CardTitle>{t('system.proxy.title')}</CardTitle>
          <CardDescription>{t('system.proxy.description')}</CardDescription>
        </CardHeader>
        <CardContent>
          {!presets || presets.length === 0 ? (
            <p className='text-muted-foreground text-sm'>{t('system.proxy.empty')}</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('system.proxy.columns.name')}</TableHead>
                  <TableHead>{t('system.proxy.columns.url')}</TableHead>
                  <TableHead>{t('system.proxy.columns.username')}</TableHead>
                  <TableHead className='w-[100px] text-right'>{t('system.proxy.columns.actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {presets.map((preset) => (
                  <TableRow key={preset.url}>
                    <TableCell className='font-medium'>{preset.name || '-'}</TableCell>
                    <TableCell className='font-mono text-sm'>{preset.url}</TableCell>
                    <TableCell className='text-muted-foreground text-sm'>{preset.username || '-'}</TableCell>
                    <TableCell className='text-right'>
                      <div className='flex justify-end gap-1'>
                        <ProxyPresetEditDialog preset={preset} />
                        <Button
                          variant='ghost'
                          size='sm'
                          className='hover:text-destructive h-8 w-8 p-0'
                          onClick={() => deletePreset.mutate(preset.url)}
                          disabled={deletePreset.isPending}
                        >
                          <Trash2 className='h-4 w-4' />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
