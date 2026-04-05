import { useEffect, useMemo } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Rocket, Globe } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { useRedeployAxonclawInstance } from '../data/control-axonclaw-instance';

const createRedeploySchema = (t: (key: string) => string) =>
  z.object({
    axonhubBaseUrl: z.string(),
  });

type RedeployFormValues = z.infer<ReturnType<typeof createRedeploySchema>>;

interface RedeployAxonclawDialogProps {
  agentId: string;
  instanceId: string;
  instanceName: string;
  currentBaseUrl?: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function RedeployAxonclawDialog({
  agentId,
  instanceId,
  instanceName,
  currentBaseUrl,
  open,
  onOpenChange,
}: RedeployAxonclawDialogProps) {
  const { t } = useTranslation();
  const redeployInstance = useRedeployAxonclawInstance(agentId);

  const redeploySchema = useMemo(() => createRedeploySchema(t), [t]);

  const getDefaultBaseUrl = () => {
    if (currentBaseUrl) {
      return currentBaseUrl;
    }
    if (typeof window !== 'undefined') {
      return `${window.location.protocol}//${window.location.host}`;
    }
    return '';
  };

  const form = useForm<RedeployFormValues>({
    resolver: zodResolver(redeploySchema),
    mode: 'onChange',
    defaultValues: {
      axonhubBaseUrl: '',
    },
  });

  useEffect(() => {
    if (open) {
      form.reset({
        axonhubBaseUrl: getDefaultBaseUrl(),
      });
    }
  }, [open, form, currentBaseUrl]);

  const onSubmit = async (values: RedeployFormValues) => {
    try {
      await redeployInstance.mutateAsync({
        instanceID: instanceId,
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
            {t('agents.dialogs.redeploy.title')}
          </DialogTitle>
          <DialogDescription>
            {t('agents.dialogs.redeploy.description', { name: instanceName })}
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-4'>
            <FormField
              control={form.control}
              name='axonhubBaseUrl'
              render={({ field }) => (
                <FormItem>
                  <FormLabel className='flex items-center gap-2'>
                    <Globe className='h-4 w-4' />
                    {t('agents.dialogs.redeploy.fields.axonhubBaseUrl.label')}
                  </FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('agents.dialogs.redeploy.fields.axonhubBaseUrl.placeholder')}
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <DialogFooter>
              <Button type='button' variant='outline' onClick={() => onOpenChange(false)}>
                {t('common.buttons.cancel')}
              </Button>
              <Button type='submit' disabled={redeployInstance.isPending}>
                {redeployInstance.isPending
                  ? t('common.buttons.redeploying')
                  : t('common.buttons.redeploy')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
