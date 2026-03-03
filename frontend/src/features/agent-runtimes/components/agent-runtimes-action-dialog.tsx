'use client';

import { useEffect, useMemo, useState } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
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
import { Textarea } from '@/components/ui/textarea';
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
  AgentRuntimeAuthMethod,
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
  const [dialogContent, setDialogContent] = useState<HTMLDivElement | null>(null);

  const formSchema = isEdit ? updateAgentRuntimeInputSchema : createAgentRuntimeInputSchema;

  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: isEdit
      ? {
          name: currentRow?.name || '',
          type: currentRow?.type || 'vm',
          host: currentRow?.host || '',
          user: currentRow?.user || '',
          password: currentRow?.password || '',
          authMethod: currentRow?.authMethod || 'password',
          sshPrivateKey: currentRow?.sshPrivateKey || '',
        }
      : {
          name: '',
          type: 'vm',
          host: '',
          user: '',
          password: '',
          authMethod: 'password',
          sshPrivateKey: '',
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
              host: hostValue,
              user: currentRow?.user || '',
              password: currentRow?.password || '',
              authMethod: currentRow?.authMethod || 'password',
              sshPrivateKey: currentRow?.sshPrivateKey || '',
            }
          : {
              name: '',
              type: 'vm',
              host: '',
              user: '',
              password: '',
              authMethod: 'password',
              sshPrivateKey: '',
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

  const runtimeTypes = useMemo(() => {
    const types: { value: AgentRuntimeType; label: string }[] = [
      { value: 'vm', label: t('agentRuntimes.types.vm') },
      { value: 'docker', label: t('agentRuntimes.types.docker') },
    ];
    if (isEdit && currentRow?.type === 'local') {
      types.push({ value: 'local', label: t('agentRuntimes.types.local') });
    }
    return types;
  }, [isEdit, currentRow?.type, t]);

  const hostSuggestions = useMemo(() => [
    { value: 'localhost', label: 'localhost' },
    { value: '127.0.0.1', label: '127.0.0.1' },
  ], []);

  const typeValue = form.watch('type');
  const hostValue = form.watch('host');
  const authMethodValue = (form.watch('authMethod') || 'password') as AgentRuntimeAuthMethod;
  const isLocalType = typeValue === 'local';
  const isLocalhost = hostValue === 'localhost' || hostValue === '127.0.0.1';

  const authMethods: { value: AgentRuntimeAuthMethod; label: string }[] = [
    { value: 'password', label: t('agentRuntimes.authMethods.password') },
    { value: 'ssh_key', label: t('agentRuntimes.authMethods.ssh_key') },
  ];

  const showAuthPanel = !isLocalType && !isLocalhost;

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
      <DialogContent
        ref={setDialogContent}
        className={cn(
          'h-[600px] max-h-[90vh] flex flex-col overflow-hidden transition-all duration-300',
          showAuthPanel ? 'sm:max-w-4xl' : 'sm:max-w-[500px]'
        )}
      >
        <DialogHeader className="flex-shrink-0">
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
          <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-1 flex-col overflow-hidden">
            <div className="flex min-h-0 flex-1 gap-6 overflow-hidden">
              {/* Left Panel - Basic Info */}
              <div className="flex-1 overflow-y-auto pr-2 space-y-4">
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
                      <p className="text-xs text-muted-foreground mt-1">
                        {t('agentRuntimes.dialogs.fields.type.help')}
                      </p>
                      {typeValue === 'vm' && (
                        <p className="text-xs text-amber-600 mt-1">
                          {t('agentRuntimes.dialogs.fields.type.vmLinuxHint')}
                        </p>
                      )}
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
                            selectedValue={field.value || ''}
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
                            portalContainer={dialogContent}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}
              </div>

              {/* Right Panel - Auth Info */}
              {showAuthPanel && (
                <div className="flex-1 overflow-y-auto border-l pl-6 space-y-4">
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
                    name="authMethod"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('agentRuntimes.dialogs.fields.authMethod.label')}</FormLabel>
                        <Select onValueChange={field.onChange} defaultValue={field.value || 'password'}>
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue placeholder={t('agentRuntimes.dialogs.fields.authMethod.placeholder')} />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent>
                            {authMethods.map((method) => (
                              <SelectItem key={method.value} value={method.value}>
                                {method.label}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  {authMethodValue === 'password' && (
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
                  )}

                  {authMethodValue === 'ssh_key' && (
                    <FormField
                      control={form.control}
                      name="sshPrivateKey"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('agentRuntimes.dialogs.fields.sshPrivateKey.label')}</FormLabel>
                          <FormControl>
                            <Textarea
                            placeholder={t('agentRuntimes.dialogs.fields.sshPrivateKey.placeholder')}
                            className="font-mono text-xs resize-none h-[300px]"
                            {...field}
                          />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  )}
                </div>
              )}
            </div>

            <DialogFooter className="flex-shrink-0 pt-4 border-t">
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
