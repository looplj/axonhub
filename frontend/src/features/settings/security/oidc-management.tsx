import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';

import { Button } from '@/components/ui/button';
import { useAuthStore } from '@/stores/authStore';
import { graphqlRequest } from '@/gql/graphql';
import { UNLINK_OIDC_IDENTITY_MUTATION } from '@/gql/users';
import { authApi } from '@/lib/api-client';
import { Link as LinkIcon, Unlink } from 'lucide-react';

interface OidcManagementProps {
  providers: any[];
}

export default function OidcManagement({ providers }: OidcManagementProps) {
  const { t } = useTranslation();
  const user = useAuthStore((state) => state.auth.user);
  const setUser = useAuthStore((state) => state.auth.setUser);

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
    <div className='space-y-4'>
      <div>
        <h3 className='text-lg font-medium'>{t('security.oidc.title', 'Linked OIDC Providers')}</h3>
        <p className='text-muted-foreground text-sm'>
          {t('security.oidc.description', 'Manage your linked OIDC providers below. You can link new providers or unlink existing ones to enable SSO login.')}
        </p>
      </div>

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
    </div>
  );
}
