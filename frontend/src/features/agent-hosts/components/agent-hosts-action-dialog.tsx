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
import { useCreateAgentHost, useUpdateAgentHost } from '../data/agent-hosts';
import {
  AgentHost,
  AgentHostAuthMethod,
  AgentHostType,
  CreateAgentHostInput,
  UpdateAgentHostInput,
  createAgentHostInputSchema,
  updateAgentHostInputSchema,
} from '../data/schema';

interface Props {
  currentRow?: AgentHost;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function AgentHostsActionDialog({ currentRow, open, onOpenChange }: Props) {
  const { t } = useTranslation();
  const isEdit = !!currentRow;
  const createAgentHost = useCreateAgentHost();
  const updateAgentHost = useUpdateAgentHost();

  const [hostSearchValue, setHostSearchValue] = useState('');
  const [dialogContent, setDialogContent] = useState<HTMLDivElement | null>(null);

  const formSchema = isEdit ? updateAgentHostInputSchema : createAgentHostInputSchema;

  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: isEdit
      ? {
          name: currentRow?.name || '',
          addr: (isEdit && currentRow?.type === 'local') ? undefined : (currentRow?.addr || ''),
          user: currentRow?.user || undefined,
          password: currentRow?.password || undefined,
          authMethod: currentRow?.authMethod || undefined,
          sshPrivateKey: currentRow?.sshPrivateKey || undefined,
          directory: currentRow?.directory || '',
        }
      : {
          name: '',
          type: 'vm',
          addr: '',
          user: '',
          password: '',
          authMethod: 'password',
          sshPrivateKey: '',
          directory: '',
        },
  });

  // Reset form when dialog opens/closes or currentRow changes
  useEffect(() => {
    if (open) {
      const isLocal = isEdit && currentRow?.type === 'local';
      const hostValue = isLocal ? undefined : (currentRow?.addr || '');
      setHostSearchValue(isLocal ? '' : (currentRow?.addr || ''));
      form.reset(
        isEdit
          ? {
              name: currentRow?.name || '',
              addr: hostValue,
              user: currentRow?.user || undefined,
              password: currentRow?.password || undefined,
              authMethod: currentRow?.authMethod || undefined,
              sshPrivateKey: currentRow?.sshPrivateKey || undefined,
              directory: currentRow?.directory || '',
            }
          : {
              name: '',
              type: 'vm',
              addr: '',
              user: '',
              password: '',
              authMethod: 'password',
              sshPrivateKey: '',
              directory: '',
            }
      );
    }
  }, [open, currentRow, isEdit, form]);

  const onSubmit = async (values: z.infer<typeof formSchema>) => {
    try {
      if (isEdit && currentRow) {
        await updateAgentHost.mutateAsync({
          id: currentRow.id,
          input: values as UpdateAgentHostInput,
        });
      } else {
        await createAgentHost.mutateAsync(values as CreateAgentHostInput);
      }
      onOpenChange(false);
      form.reset();
    } catch (_error) {
      // Error is handled by the mutation
    }
  };

  const hostTypes: { value: AgentHostType; label: string }[] = [
    { value: 'vm', label: t('agentHosts.types.vm') },
    { value: 'docker', label: t('agentHosts.types.docker') },
  ];

  const hostSuggestions = useMemo(() => [
    { value: 'localhost', label: 'localhost' },
    { value: '127.0.0.1', label: '127.0.0.1' },
  ], []);

  const typeValue = isEdit ? (currentRow?.type || 'vm') : form.watch('type');
  const hostValue = form.watch('addr');
  const authMethodValue = (form.watch('authMethod') || 'password') as AgentHostAuthMethod;
  const isLocalType = typeValue === 'local';
  const isLocalhost = hostValue === 'localhost' || hostValue === '127.0.0.1';
  const isEditingLocal = isEdit && currentRow?.type === 'local';

  const authMethods: { value: AgentHostAuthMethod; label: string }[] = [
    { value: 'password', label: t('agentHosts.authMethods.password') },
    { value: 'ssh_key', label: t('agentHosts.authMethods.ssh_key') },
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
            {isEdit ? t('agentHosts.dialogs.edit.title') : t('agentHosts.dialogs.create.title')}
          </DialogTitle>
          <DialogDescription>
            {isEdit
              ? t('agentHosts.dialogs.edit.description')
              : t('agentHosts.dialogs.create.description')}
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
                      <FormLabel>{t('agentHosts.dialogs.fields.name.label')}</FormLabel>
                      <FormControl>
                        <Input
                          placeholder={t('agentHosts.dialogs.fields.name.placeholder')}
                          disabled={isEditingLocal}
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                {!isEdit && (
                  <FormField
                    control={form.control}
                    name="type"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('agentHosts.dialogs.fields.type.label')}</FormLabel>
                        <Select onValueChange={field.onChange} defaultValue={field.value}>
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue placeholder={t('agentHosts.dialogs.fields.type.placeholder')} />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent>
                            {hostTypes.map((type) => (
                              <SelectItem key={type.value} value={type.value}>
                                {type.label}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                        <p className="text-xs text-muted-foreground mt-1">
                          {t('agentHosts.dialogs.fields.type.help')}
                        </p>
                        {typeValue === 'vm' && (
                          <p className="text-xs text-amber-600 mt-1">
                            {t('agentHosts.dialogs.fields.type.vmLinuxHint')}
                          </p>
                        )}
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}

                {isEdit && (
                  <FormItem>
                    <FormLabel>{t('agentHosts.dialogs.fields.type.label')}</FormLabel>
                    <FormControl>
                      <Input
                        value={t(`agentHosts.types.${typeValue}`)}
                        disabled
                        className="bg-muted"
                      />
                    </FormControl>
                    <p className="text-xs text-muted-foreground mt-1">
                      {t('agentHosts.dialogs.fields.type.help')}
                    </p>
                  </FormItem>
                )}

                {!isLocalType && (
                  <FormField
                    control={form.control}
                    name="addr"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('agentHosts.dialogs.fields.host.label')}</FormLabel>
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
                            placeholder={t('agentHosts.dialogs.fields.host.placeholder')}
                            emptyMessage={t('agentHosts.dialogs.fields.host.emptyMessage')}
                            portalContainer={dialogContent}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}

                {(typeValue === 'vm' || typeValue === 'local') && (
                  <FormField
                    control={form.control}
                    name="directory"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('agentHosts.dialogs.fields.directory.label')}</FormLabel>
                        <FormControl>
                          <Input
                            placeholder={t('agentHosts.dialogs.fields.directory.placeholder')}
                            {...field}
                          />
                        </FormControl>
                        <p className="text-xs text-muted-foreground mt-1">
                          {t('agentHosts.dialogs.fields.directory.help')}
                        </p>
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
                        <FormLabel>{t('agentHosts.dialogs.fields.user.label')}</FormLabel>
                        <FormControl>
                          <Input
                            placeholder={t('agentHosts.dialogs.fields.user.placeholder')}
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
                        <FormLabel>{t('agentHosts.dialogs.fields.authMethod.label')}</FormLabel>
                        <Select onValueChange={field.onChange} defaultValue={field.value || 'password'}>
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue placeholder={t('agentHosts.dialogs.fields.authMethod.placeholder')} />
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
                          <FormLabel>{t('agentHosts.dialogs.fields.password.label')}</FormLabel>
                          <FormControl>
                            <Input
                              type="password"
                              placeholder={t('agentHosts.dialogs.fields.password.placeholder')}
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
                          <FormLabel>{t('agentHosts.dialogs.fields.sshPrivateKey.label')}</FormLabel>
                          <FormControl>
                            <Textarea
                            placeholder={t('agentHosts.dialogs.fields.sshPrivateKey.placeholder')}
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
                disabled={createAgentHost.isPending || updateAgentHost.isPending}
              >
                {createAgentHost.isPending || updateAgentHost.isPending
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
