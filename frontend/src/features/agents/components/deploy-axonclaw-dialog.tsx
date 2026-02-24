import { useEffect, useMemo } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useTranslation } from 'react-i18next';
import { Rocket, Server, FolderOpen } from 'lucide-react';
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
import { useDeployAxonclaw } from '../data/deploy-axonclaw';
import { useQueryAgentRuntimes } from '@/features/agent-runtimes/data/agent-runtimes';

const deploySchema = z.object({
  runtimeID: z.string().min(1, 'Runtime is required'),
  name: z.string().min(1, 'Name is required'),
  directory: z.string().optional(),
});

type DeployFormValues = z.infer<typeof deploySchema>;

interface DeployAxonclawDialogProps {
  agentId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function DeployAxonclawDialog({ agentId, open, onOpenChange }: DeployAxonclawDialogProps) {
  const { t } = useTranslation();
  const deployAxonclaw = useDeployAxonclaw();
  const { data: runtimesData } = useQueryAgentRuntimes({
    first: 100,
  });

  const form = useForm<DeployFormValues>({
    resolver: zodResolver(deploySchema),
    mode: 'onChange',
    defaultValues: {
      runtimeID: '',
      name: '',
      directory: '',
    },
  });

  // Reset form when dialog opens
  useEffect(() => {
    if (open) {
      form.reset({
        runtimeID: '',
        name: '',
        directory: '',
      });
    }
  }, [open, form]);

  const runtimes = useMemo(() => {
    return runtimesData?.edges?.map((e) => e.node) ?? [];
  }, [runtimesData]);

  const runtimeId = form.watch('runtimeID');
  const selectedRuntime = useMemo(() => {
    return runtimes.find((r) => r.id === runtimeId);
  }, [runtimeId, runtimes]);

  const onSubmit = async (values: DeployFormValues) => {
    try {
      await deployAxonclaw.mutateAsync({
        agentID: agentId,
        runtimeID: values.runtimeID,
        name: values.name,
        directory: values.directory || undefined,
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
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Rocket className="h-5 w-5" />
            {t('agents.dialogs.deploy.title')}
          </DialogTitle>
          <DialogDescription>{t('agents.dialogs.deploy.description')}</DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
            <FormField
              control={form.control}
              name="runtimeID"
              render={({ field }) => (
                <FormItem>
                  <FormLabel className="flex items-center gap-2">
                    <Server className="h-4 w-4" />
                    {t('agents.dialogs.deploy.fields.runtime.label')}
                  </FormLabel>
                  <Select onValueChange={field.onChange} value={field.value} disabled={runtimes.length === 0}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue
                          placeholder={runtimes.length === 0 ? t('agents.dialogs.deploy.noRuntimes') : t('agents.dialogs.deploy.fields.runtime.placeholder')}
                        />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {runtimes.map((runtime) => (
                        <SelectItem key={runtime.id} value={runtime.id}>
                          <div className="flex items-center gap-2">
                            <span>{runtime.name}</span>
                            <span className="text-muted-foreground text-xs">
                              ({runtime.type} - {runtime.host})
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

            {selectedRuntime && (
              <div className="bg-muted/50 rounded-md p-3 text-sm space-y-1">
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Type:</span>
                  <span className="font-medium uppercase">{selectedRuntime.type}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Host:</span>
                  <span className="font-medium">{selectedRuntime.host}</span>
                </div>
                {selectedRuntime.host !== 'localhost' && selectedRuntime.host !== '127.0.0.1' && (
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">User:</span>
                    <span className="font-medium">{selectedRuntime.user}</span>
                  </div>
                )}
              </div>
            )}

            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('agents.dialogs.deploy.fields.name.label')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('agents.dialogs.deploy.fields.name.placeholder')}
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            {selectedRuntime?.type === 'vm' && (
              <FormField
                control={form.control}
                name="directory"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel className="flex items-center gap-2">
                      <FolderOpen className="h-4 w-4" />
                      {t('agents.dialogs.deploy.fields.directory.label')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('agents.dialogs.deploy.fields.directory.placeholder')}
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
                {t('common.buttons.cancel')}
              </Button>
              <Button
                type="submit"
                disabled={deployAxonclaw.isPending || runtimes.length === 0 || !form.formState.isValid}
              >
                {deployAxonclaw.isPending
                  ? t('common.buttons.deploying')
                  : t('common.buttons.deploy')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
