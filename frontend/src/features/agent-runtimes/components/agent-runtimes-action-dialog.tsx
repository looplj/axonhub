'use client';

import { useEffect, useMemo, useState } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useTranslation } from 'react-i18next';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { AutoComplete } from '@/components/auto-complete';
import { useCreateAgentRuntime, useUpdateAgentRuntime } from '../data/agent-runtimes';
import {
  AgentRuntime,
  AgentRuntimeType,
  CreateAgentRuntimeInput,
  UpdateAgentRuntimeInput,
  createAgentRuntimeInputSchema,
  updateAgentRuntimeInputSchema,
} from '../data/schema';

interface Props {
  currentRow?: AgentRuntime;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function AgentRuntimesActionDialog({ currentRow, open, onOpenChange }: Props) {
  const { t } = useTranslation();
  const isEdit = !!currentRow;
  const createAgentRuntime = useCreateAgentRuntime();
  const updateAgentRuntime = useUpdateAgentRuntime();

  const [hostSearchValue, setHostSearchValue] = useState('');

  const formSchema = isEdit ? updateAgentRuntimeInputSchema : createAgentRuntimeInputSchema;

  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: isEdit
      ? {
          name: currentRow?.name || '',
          type: currentRow?.type || 'vm',
          status: currentRow?.status || 'active',
          host: currentRow?.host || '',
          user: currentRow?.user || '',
          password: currentRow?.password || '',
        }
      : {
          name: '',
          type: 'vm',
          status: 'active',
          host: '',
          user: '',
          password: '',
        },
  });

  // Reset form when dialog opens/closes or currentRow changes
  useEffect(() => {
    if (open) {
      const hostValue = isEdit ? (currentRow?.host || '') : '';
      setHostSearchValue(hostValue);
      form.reset(
        isEdit
          ? {
              name: currentRow?.name || '',
              type: currentRow?.type || 'vm',
              status: currentRow?.status || 'active',
              host: hostValue,
              user: currentRow?.user || '',
              password: currentRow?.password || '',
            }
          : {
              name: '',
              type: 'vm',
              status: 'active',
              host: '',
              user: '',
              password: '',
            }
      );
    }
  }, [open, currentRow, isEdit, form]);

  const onSubmit = async (values: z.infer<typeof formSchema>) => {
    try {
      if (isEdit && currentRow) {
        await updateAgentRuntime.mutateAsync({
          id: currentRow.id,
          input: values as UpdateAgentRuntimeInput,
        });
      } else {
        await createAgentRuntime.mutateAsync(values as CreateAgentRuntimeInput);
      }
      onOpenChange(false);
      form.reset();
    } catch (_error) {
      // Error is handled by the mutation
    }
  };

  const runtimeTypes: { value: AgentRuntimeType; label: string }[] = [
    { value: 'vm', label: t('agentRuntimes.types.vm') },
    { value: 'docker', label: t('agentRuntimes.types.docker') },
    { value: 'local', label: t('agentRuntimes.types.local') },
  ];

  const hostSuggestions = useMemo(() => [
    { value: 'localhost', label: 'localhost' },
    { value: '127.0.0.1', label: '127.0.0.1' },
  ], []);

  const typeValue = form.watch('type');
  const hostValue = form.watch('host');
  const isLocalType = typeValue === 'local';
  const isLocalhost = hostValue === 'localhost' || hostValue === '127.0.0.1';

  const runtimeStatuses: { value: 'active' | 'inactive' | 'error'; label: string }[] = [
    { value: 'active', label: t('agentRuntimes.status.active') },
    { value: 'inactive', label: t('agentRuntimes.status.inactive') },
    { value: 'error', label: t('agentRuntimes.status.error') },
  ];

  return (
    <Dialog
      open={open}
      onOpenChange={(state) => {
        if (!state) {
          form.reset();
        }
        onOpenChange(state);
      }}
    >
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>
            {isEdit ? t('agentRuntimes.dialogs.edit.title') : t('agentRuntimes.dialogs.create.title')}
          </DialogTitle>
          <DialogDescription>
            {isEdit
              ? t('agentRuntimes.dialogs.edit.description')
              : t('agentRuntimes.dialogs.create.description')}
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('agentRuntimes.dialogs.fields.name.label')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('agentRuntimes.dialogs.fields.name.placeholder')}
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="type"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('agentRuntimes.dialogs.fields.type.label')}</FormLabel>
                  <Select onValueChange={field.onChange} defaultValue={field.value}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder={t('agentRuntimes.dialogs.fields.type.placeholder')} />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {runtimeTypes.map((type) => (
                        <SelectItem key={type.value} value={type.value}>
                          {type.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="status"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('agentRuntimes.dialogs.fields.status.label')}</FormLabel>
                  <Select onValueChange={field.onChange} defaultValue={field.value}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder={t('agentRuntimes.dialogs.fields.status.placeholder')} />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {runtimeStatuses.map((status) => (
                        <SelectItem key={status.value} value={status.value}>
                          {status.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />

            {!isLocalType && (
              <FormField
                control={form.control}
                name="host"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('agentRuntimes.dialogs.fields.host.label')}</FormLabel>
                    <FormControl>
                      <AutoComplete
                        selectedValue={field.value}
                        onSelectedValueChange={(value) => {
                          field.onChange(value);
                        }}
                        searchValue={hostSearchValue}
                        onSearchValueChange={(value) => {
                          setHostSearchValue(value);
                          field.onChange(value);
                        }}
                        items={hostSuggestions}
                        placeholder={t('agentRuntimes.dialogs.fields.host.placeholder')}
                        emptyMessage={t('agentRuntimes.dialogs.fields.host.emptyMessage')}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}

            {!isLocalType && !isLocalhost && (
              <>
                <FormField
                  control={form.control}
                  name="user"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('agentRuntimes.dialogs.fields.user.label')}</FormLabel>
                      <FormControl>
                        <Input
                          placeholder={t('agentRuntimes.dialogs.fields.user.placeholder')}
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name="password"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('agentRuntimes.dialogs.fields.password.label')}</FormLabel>
                      <FormControl>
                        <Input
                          type="password"
                          placeholder={t('agentRuntimes.dialogs.fields.password.placeholder')}
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </>
            )}

            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => onOpenChange(false)}
              >
                {t('common.buttons.cancel')}
              </Button>
              <Button
                type="submit"
                disabled={createAgentRuntime.isPending || updateAgentRuntime.isPending}
              >
                {createAgentRuntime.isPending || updateAgentRuntime.isPending
                  ? isEdit
                    ? t('common.buttons.saving')
                    : t('common.buttons.creating')
                  : isEdit
                    ? t('common.buttons.save')
                    : t('common.buttons.create')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
