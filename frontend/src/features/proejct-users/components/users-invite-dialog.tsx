import { useEffect, useState } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { IconCheck, IconCopy, IconMailPlus } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { apiRequest } from '@/lib/api-client';
import { copyTextToClipboard } from '@/lib/clipboard';
import { extractNumberIDAsNumber } from '@/lib/utils';
import { useSelectedProjectId } from '@/stores/projectStore';
import { useRoles } from '@/features/project-roles/data/roles';
import { Button } from '@/components/ui/button';
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

const formSchema = z.object({
  expiresInHours: z.enum(['1', '6', '24', '168', '0']),
  maxUses: z.enum(['1', '0']),
  roleID: z.string().min(1),
});

type InviteForm = z.infer<typeof formSchema>;

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function UsersInviteDialog({ open, onOpenChange }: Props) {
  const { t } = useTranslation();
  const selectedProjectId = useSelectedProjectId();
  const [inviteLink, setInviteLink] = useState('');
  const [isCopied, setIsCopied] = useState(false);
  const { data: rolesData, isLoading: isLoadingRoles } = useRoles();
  const form = useForm<InviteForm>({
    resolver: zodResolver(formSchema),
    defaultValues: { expiresInHours: '168', maxUses: '1', roleID: '' },
  });

  const roles = rolesData?.edges.map((edge) => edge.node) || [];

  useEffect(() => {
    if (form.getValues('roleID') || roles.length === 0) {
      return;
    }
    form.setValue('roleID', roles.find((role) => role.name === 'Developer')?.id || roles[0].id);
  }, [form, roles]);

  const closeDialog = (nextOpen: boolean) => {
    if (!nextOpen) {
      form.reset();
      setInviteLink('');
      setIsCopied(false);
    }
    onOpenChange(nextOpen);
  };

  const onSubmit = async (values: InviteForm) => {
    if (!selectedProjectId) {
      return;
    }

    try {
      const response = await apiRequest<{ token: string }>('/admin/invitations', {
        method: 'POST',
        requireAuth: true,
        headers: { 'X-Project-ID': selectedProjectId },
        body: {
          expiresInHours: Number(values.expiresInHours),
          maxUses: Number(values.maxUses),
          roleID: extractNumberIDAsNumber(values.roleID),
        },
      });
      const url = new URL('/sign-up', window.location.origin);
      url.searchParams.set('invite', response.token);
      setInviteLink(url.toString());
      toast.success(t('users.messages.invitationCreated'));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('common.errors.internalServerError'));
    }
  };

  const copyInviteLink = async () => {
    try {
      await copyTextToClipboard(inviteLink);
      setIsCopied(true);
      toast.success(t('users.messages.invitationCopied'));
    } catch {
      toast.error(t('common.errors.internalServerError'));
    }
  };

  return (
    <Dialog open={open} onOpenChange={closeDialog}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader className='text-left'>
          <DialogTitle className='flex items-center gap-2'>
            <IconMailPlus />
            {t('users.dialogs.invite.title')}
          </DialogTitle>
          <DialogDescription>{t('users.dialogs.invite.description')}</DialogDescription>
        </DialogHeader>
        {inviteLink ? (
          <div className='space-y-3'>
            <Label htmlFor='invitation-link'>{t('users.form.invitationLink')}</Label>
            <div className='flex gap-2'>
              <Input id='invitation-link' value={inviteLink} readOnly />
              <Button type='button' size='icon' variant='outline' onClick={copyInviteLink} title={t('users.buttons.copyInvitationLink')}>
                {isCopied ? <IconCheck /> : <IconCopy />}
              </Button>
            </div>
            <p className='text-sm text-muted-foreground'>{t('users.messages.invitationLinkReady')}</p>
          </div>
        ) : (
          <Form {...form}>
            <form id='project-user-invite-form' onSubmit={form.handleSubmit(onSubmit)} className='space-y-4'>
              <FormField
                control={form.control}
                name='roleID'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('users.form.projectRoles')}</FormLabel>
                    <Select value={field.value} onValueChange={field.onChange} disabled={isLoadingRoles || roles.length === 0}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue placeholder={isLoadingRoles ? t('users.form.loadingRoles') : t('users.form.noProjectRoles')} />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        {roles.map((role) => (
                          <SelectItem key={role.id} value={role.id}>{role.name}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='expiresInHours'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('users.form.invitationExpiry')}</FormLabel>
                    <Select value={field.value} onValueChange={field.onChange}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value='1'>{t('users.invitation.expiry.oneHour')}</SelectItem>
                        <SelectItem value='6'>{t('users.invitation.expiry.sixHours')}</SelectItem>
                        <SelectItem value='24'>{t('users.invitation.expiry.oneDay')}</SelectItem>
                        <SelectItem value='168'>{t('users.invitation.expiry.sevenDays')}</SelectItem>
                        <SelectItem value='0'>{t('users.invitation.expiry.never')}</SelectItem>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='maxUses'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('users.form.invitationUseLimit')}</FormLabel>
                    <Select value={field.value} onValueChange={field.onChange}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value='1'>{t('users.invitation.uses.single')}</SelectItem>
                        <SelectItem value='0'>{t('users.invitation.uses.unlimited')}</SelectItem>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </form>
          </Form>
        )}
        <DialogFooter>
          {inviteLink ? (
            <Button type='button' onClick={() => closeDialog(false)}>{t('common.buttons.close')}</Button>
          ) : (
            <>
              <DialogClose asChild>
                <Button variant='outline'>{t('common.buttons.cancel')}</Button>
              </DialogClose>
              <Button type='submit' form='project-user-invite-form' disabled={form.formState.isSubmitting || isLoadingRoles || roles.length === 0}>
                {t('users.buttons.createInvitation')}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
