import { useState, useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { zodResolver } from '@hookform/resolvers/zod';
import { toast } from 'sonner';
import { z } from 'zod';

import { Button } from '@/components/ui/button';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { useAuthStore } from '@/stores/authStore';
import { graphqlRequest } from '@/gql/graphql';
import { UPDATE_MY_PASSWORD_MUTATION, UNLINK_OIDC_IDENTITY_MUTATION } from '@/gql/users';
import { authApi } from '@/lib/api-client';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { ShieldAlert, Link as LinkIcon, Unlink } from 'lucide-react';

const getFormSchema = (hasPassword: boolean) => {
  const schema = z.object({
    oldPassword: hasPassword ? z.string().min(1, 'Current password is required') : z.string().optional(),
    newPassword: z.string().min(8, 'Password must be at least 8 characters'),
    confirmPassword: z.string().min(1, 'Confirm new password is required'),
  }).refine((data) => data.newPassword === data.confirmPassword, {
    message: "Passwords don't match",
    path: ['confirmPassword'],
  });
  return schema;
};

type FormValues = {
    oldPassword?: string;
    newPassword: string;
    confirmPassword: string;
};

export default function SecurityForm() {
  const { t } = useTranslation();
  const [isUpdating, setIsUpdating] = useState(false);
  const [providers, setProviders] = useState<any[]>([]);
  const user = useAuthStore((state) => state.auth.user);
  const setUser = useAuthStore((state) => state.auth.setUser); 

  useEffect(() => {
    const fetchProviders = async () => {
      try {
        const res = await authApi.getOIDCProviders();
        setProviders(res.data || []);
      } catch (err) {
        console.error('Failed to fetch OIDC providers:', err);
      }
    };
    fetchProviders();
  }, []);

  const hasPassword = user?.hasPassword ?? true;
  const formSchema = getFormSchema(hasPassword);

  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema) as any,
    defaultValues: {
      oldPassword: '',
      newPassword: '',
      confirmPassword: '',
    },
  });

  const onSubmit = async (data: FormValues) => {
    setIsUpdating(true);
    try {
      if (!user) {
        throw new Error('User not loaded');
      }

      await graphqlRequest(UPDATE_MY_PASSWORD_MUTATION, {
        input: {
          oldPassword: hasPassword ? data.oldPassword : null,
          newPassword: data.newPassword,
        },
      });

      toast.success(t('security.messages.passwordChangeSuccess', 'Password changed successfully'));
      form.reset();
    } catch (error: any) {
      console.error('Failed to change password:', error);
      toast.error(
        t('security.messages.passwordChangeError', 'Failed to change password: ') +
          (error.response?.errors?.[0]?.message || error.message || t('common.errors.unknown')),
      );
    } finally {
      setIsUpdating(false);
    }
  };

  const handleLink = async (providerName: string) => {
    try {
      const res = await authApi.getOIDCLinkAuthorizeURL(providerName);
      if (res.data && res.data.url) {
        window.location.href = res.data.url;
      }
    } catch (error: any) {
      toast.error(t('security.oidc.linkError', 'Failed to initiate linking: ') + error.message);
    }
  };

  const handleUnlink = async (identityId: string) => {
    if (!window.confirm(t('security.oidc.confirmUnlink', 'Are you sure you want to unlink this provider?'))) return;
    
    try {
      await graphqlRequest(UNLINK_OIDC_IDENTITY_MUTATION, { id: identityId });
      toast.success(t('security.oidc.unlinkSuccess', 'Successfully unlinked provider.'));
      // Remove the unlinked identity from the local user state
      if (user) {
        setUser({
          ...user,
          oidcIdentities: (user.oidcIdentities || []).filter(oidc => oidc.id !== identityId)
        });
      }
    } catch (error: any) {
      toast.error(t('security.oidc.unlinkError', 'Failed to unlink provider: ') + error.message);
    }
  };

  const linkedProviders = user?.oidcIdentities || [];
  const linkedProviderNames = new Set(linkedProviders.map((oidc: any) => oidc.idpName));
  const unlinkedProviders = providers.filter(p => !linkedProviderNames.has(p.name));

  return (
    <div className='space-y-8'>
      {providers.length > 0 && (
        <Alert>
          <ShieldAlert className='h-4 w-4' />
          <AlertTitle>{t('security.oidc.title', 'OIDC Identity Linked')}</AlertTitle>
          <AlertDescription>
            {t('security.oidc.description', 'Manage your linked OIDC providers below. You can link new providers or unlink existing ones to enable SSO login.')}
            
            {(linkedProviders.length > 0 || unlinkedProviders.length > 0) && (
              <div className='mt-4 space-y-4'>
                {linkedProviders.length > 0 && (
                  <div className='space-y-2'>
                    <h4 className='font-medium text-sm text-foreground'>{t('common.linked', 'Linked Providers')}</h4>
                    <div className='grid gap-2 grid-cols-1 md:grid-cols-2'>
                      {linkedProviders.map((oidc: any) => (
                        <div key={oidc.id} className='flex items-center justify-between p-3 rounded-md border bg-muted/50'>
                          <div className='flex items-center gap-2'>
                            <span className='font-semibold'>{oidc.idpName}</span>
                            <span className='text-xs text-muted-foreground'>({oidc.email})</span>
                          </div>
                          <Button 
                            variant='ghost' 
                            size='sm' 
                            className='h-8 text-destructive hover:text-destructive'
                            onClick={() => handleUnlink(oidc.id)}
                            type='button'
                          >
                            <Unlink className='w-4 h-4 mr-2' />
                            {t('common.unlink', 'Unlink')}
                          </Button>
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {unlinkedProviders.length > 0 && (
                  <div className='space-y-2'>
                    <h4 className='font-medium text-sm text-foreground'>{t('common.available', 'Available Providers')}</h4>
                    <div className='grid gap-2 grid-cols-1 md:grid-cols-2'>
                      {unlinkedProviders.map((p: any) => (
                        <div key={p.name} className='flex items-center justify-between p-3 rounded-md border text-muted-foreground'>
                          <div className='flex items-center gap-2'>
                            <span className='font-semibold'>{p.display_name}</span>
                          </div>
                          <Button 
                            variant='outline' 
                            size='sm' 
                            className='h-8'
                            onClick={() => handleLink(p.name)}
                            type='button'
                          >
                            <LinkIcon className='w-4 h-4 mr-2' />
                            {t('common.link', 'Link')}
                          </Button>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            )}
          </AlertDescription>
        </Alert>
      )}

      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-6'>
          <h3 className='text-lg font-medium'>
            {hasPassword ? t('security.password.changeTitle', 'Change Password') : t('security.password.setTitle', 'Set Initial Password')}
          </h3>
          
          {hasPassword && (
            <FormField
              control={form.control}
              name='oldPassword'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('security.password.oldPassword', 'Current Password')}</FormLabel>
                  <FormControl>
                    <Input type='password' placeholder='********' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          )}

          <FormField
            control={form.control}
            name='newPassword'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('security.password.newPassword', 'New Password')}</FormLabel>
                <FormControl>
                  <Input type='password' placeholder='********' {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='confirmPassword'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('security.password.confirmPassword', 'Confirm New Password')}</FormLabel>
                <FormControl>
                  <Input type='password' placeholder='********' {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <div>
            <Button type='submit' disabled={isUpdating}>
              {isUpdating 
                ? t('common.saving', 'Saving...') 
                : (hasPassword 
                    ? t('security.password.changeButton', 'Change Password') 
                    : t('security.password.setButton', 'Set Password'))}
            </Button>
          </div>
        </form>
      </Form>
    </div>
  );
}
