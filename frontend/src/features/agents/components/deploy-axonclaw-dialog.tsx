import { useEffect, useMemo } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Rocket, Server, Globe } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useQueryAgentHosts } from '@/features/agent-hosts/data/agent-hosts';
import { useDeployAxonclaw } from '../data/deploy-axonclaw';

const createDeploySchema = (t: (key: string) => string) =>
  z.object({
    hostID: z.string().min(1, t('agents.dialogs.deploy.fields.host.required')),
    hostType: z.enum(['vm', 'docker', 'local']).optional(),
    name: z.string().min(1, t('agents.dialogs.deploy.fields.name.required')),
    axonhubBaseUrl: z.string(),
  });

type DeployFormValues = z.infer<ReturnType<typeof createDeploySchema>>;

interface DeployAxonclawDialogProps {
  agentId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function DeployAxonclawDialog({ agentId, open, onOpenChange }: DeployAxonclawDialogProps) {
  const { t } = useTranslation();
  const deployAxonclaw = useDeployAxonclaw();
  const { data: hostsData } = useQueryAgentHosts({
    first: 100,
  });

  const deploySchema = useMemo(() => createDeploySchema(t), [t]);

  const form = useForm<DeployFormValues>({
    resolver: zodResolver(deploySchema),
    mode: 'onChange',
    defaultValues: {
      hostID: '',
      hostType: undefined,
      name: '',
      axonhubBaseUrl: '',
    },
  });

  const getDefaultBaseUrl = () => {
    if (typeof window !== 'undefined') {
      return `${window.location.protocol}//${window.location.host}`;
    }
    return '';
  };

  // Reset form when dialog opens
  useEffect(() => {
    if (open) {
      form.reset({
        hostID: '',
        hostType: undefined,
        name: '',
        axonhubBaseUrl: getDefaultBaseUrl(),
      });
    }
  }, [open, form]);

  const hosts = useMemo(() => {
    return hostsData?.edges?.map((e) => e.node) ?? [];
  }, [hostsData]);

  const hostId = form.watch('hostID');
  const selectedHost = useMemo(() => {
    return hosts.find((host) => host.id === hostId);
  }, [hostId, hosts]);

  useEffect(() => {
    if (selectedHost) {
      form.setValue('hostType', selectedHost.type as 'vm' | 'docker' | 'local');
    }
  }, [selectedHost, form]);

  const onSubmit = async (values: DeployFormValues) => {
    try {
      await deployAxonclaw.mutateAsync({
        agentID: agentId,
        hostID: values.hostID,
        name: values.name,
        axonhubBaseUrl: values.axonhubBaseUrl || undefined,
      });
      onOpenChange(false);
      form.reset();
    } catch (_error) {
      // Error is handled by the mutation
    }
  };

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
      <DialogContent className='sm:max-w-[500px]'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <Rocket className='h-5 w-5' />
            {t('agents.dialogs.deploy.title')}
          </DialogTitle>
          <DialogDescription>{t('agents.dialogs.deploy.description')}</DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-4'>
            <FormField
              control={form.control}
              name='hostID'
              render={({ field }) => (
                <FormItem>
                  <FormLabel className='flex items-center gap-2'>
                    <Server className='h-4 w-4' />
                    {t('agents.dialogs.deploy.fields.host.label')}
                  </FormLabel>
                  <Select onValueChange={field.onChange} value={field.value} disabled={hosts.length === 0}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue
                          placeholder={
                            hosts.length === 0
                              ? t('agents.dialogs.deploy.noHosts')
                              : t('agents.dialogs.deploy.fields.host.placeholder')
                          }
                        />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {hosts.map((host) => (
                        <SelectItem key={host.id} value={host.id}>
                          <div className='flex items-center gap-2'>
                            <span>{host.name}</span>
                            <span className='text-muted-foreground text-xs'>
                              {host.type === 'local'
                                ? t('agentHosts.types.local')
                                : `${t('agentHosts.types.' + host.type)} - ${host.addr}`}
                            </span>
                          </div>
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />

            {selectedHost && selectedHost.type !== 'local' && (
              <div className='bg-muted/50 space-y-1 rounded-md p-3 text-sm'>
                <div className='flex justify-between'>
                  <span className='text-muted-foreground'>{t('agents.dialogs.deploy.preview.type')}:</span>
                  <span className='font-medium uppercase'>{selectedHost.type}</span>
                </div>
                <div className='flex justify-between'>
                  <span className='text-muted-foreground'>{t('agents.dialogs.deploy.preview.host')}:</span>
                  <span className='font-medium'>{selectedHost.addr}</span>
                </div>
                {selectedHost.addr !== 'localhost' && selectedHost.addr !== '127.0.0.1' && (
                  <div className='flex justify-between'>
                    <span className='text-muted-foreground'>{t('agents.dialogs.deploy.preview.user')}:</span>
                    <span className='font-medium'>{selectedHost.user}</span>
                  </div>
                )}
              </div>
            )}

            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('agents.dialogs.deploy.fields.name.label')}</FormLabel>
                  <FormControl>
                    <Input placeholder={t('agents.dialogs.deploy.fields.name.placeholder')} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='axonhubBaseUrl'
              render={({ field }) => (
                <FormItem>
                  <FormLabel className='flex items-center gap-2'>
                    <Globe className='h-4 w-4' />
                    {t('agents.dialogs.deploy.fields.axonhubBaseUrl.label')}
                  </FormLabel>
                  <FormControl>
                    <Input placeholder={t('agents.dialogs.deploy.fields.axonhubBaseUrl.placeholder')} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <DialogFooter>
              <Button type='button' variant='outline' onClick={() => onOpenChange(false)}>
                {t('common.buttons.cancel')}
              </Button>
              <Button type='submit' disabled={deployAxonclaw.isPending || hosts.length === 0 || !form.formState.isValid}>
                {deployAxonclaw.isPending ? t('common.buttons.deploying') : t('common.buttons.deploy')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
