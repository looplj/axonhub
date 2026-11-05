import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { toast } from 'sonner';
import { ensureFreshAccessToken } from '@/lib/auth-session';
import i18n from '@/lib/i18n';

export const PLUGIN_SECRET_MASK = '******';

const PLUGIN_FIELDS = `
  id
  pluginID
  name
  version
  description
  enabled
  priority
  builtin
  grantedPermissions
  manifestPermissions
  configFields {
    name
    type
    title
    description
    format
    enumValues
    minimum
    maximum
    defaultJSON
  }
  config
  runtimeStatus
  runtimeError
`;

const PLUGINS_QUERY = `
  query Plugins {
    pluginCountersArePerInstance
    plugins {
      ${PLUGIN_FIELDS}
    }
  }
`;

const INSTALL_PLUGIN_MUTATION = `
  mutation InstallPlugin($file: Upload!, $grantedPermissions: [String!]) {
    installPlugin(file: $file, grantedPermissions: $grantedPermissions) {
      plugin {
        ${PLUGIN_FIELDS}
      }
      upgradePreview {
        currentVersion
        newVersion
        permissions
        events
        hooks
        pages
      }
    }
  }
`;

const ENABLE_PLUGIN_MUTATION = `
  mutation EnablePlugin($id: ID!, $grantedPermissions: [String!]!) {
    enablePlugin(id: $id, grantedPermissions: $grantedPermissions) {
      ${PLUGIN_FIELDS}
    }
  }
`;

const DISABLE_PLUGIN_MUTATION = `
  mutation DisablePlugin($id: ID!) {
    disablePlugin(id: $id) {
      ${PLUGIN_FIELDS}
    }
  }
`;

const UPDATE_PLUGIN_SETTINGS_MUTATION = `
  mutation UpdatePluginSettings($id: ID!, $priority: Int!, $config: Map!) {
    updatePluginSettings(id: $id, priority: $priority, config: $config) {
      ${PLUGIN_FIELDS}
    }
  }
`;

const DELETE_PLUGIN_MUTATION = `
  mutation DeletePlugin($id: ID!) {
    deletePlugin(id: $id)
  }
`;

export interface PluginConfigField {
  name: string;
  type: string;
  title: string;
  description: string;
  format: string;
  enumValues: string[];
  minimum?: number | null;
  maximum?: number | null;
  defaultJSON: string;
}

export interface Plugin {
  id: string;
  pluginID: string;
  name: string;
  version: string;
  description: string;
  enabled: boolean;
  priority: number;
  builtin: boolean;
  grantedPermissions: string[];
  manifestPermissions: string[];
  configFields: PluginConfigField[];
  config: Record<string, unknown>;
  runtimeStatus: string;
  runtimeError: string;
}

export interface PluginUpgradePreview {
  currentVersion: string;
  newVersion: string;
  permissions: string[];
  events: string[];
  hooks: string[];
  pages: string[];
}

export interface PluginInstallResult {
  plugin: Plugin | null;
  upgradePreview: PluginUpgradePreview | null;
}

export interface PluginsQueryResult {
  plugins: Plugin[];
  pluginCountersArePerInstance: boolean;
}

export function usePlugins() {
  return useQuery({
    queryKey: ['plugins'],
    queryFn: () => graphqlRequest<PluginsQueryResult>(PLUGINS_QUERY),
  });
}

export async function installPlugin(file: File, grantedPermissions: string[] | null): Promise<PluginInstallResult> {
  const formData = new FormData();
  formData.append(
    'operations',
    JSON.stringify({
      query: INSTALL_PLUGIN_MUTATION,
      variables: { file: null, grantedPermissions },
    })
  );
  formData.append('map', JSON.stringify({ '0': ['variables.file'] }));
  formData.append('0', file);

  const token = await ensureFreshAccessToken();
  const headers: Record<string, string> = {};
  if (token) headers.Authorization = `Bearer ${token}`;
  const response = await fetch('/admin/graphql', {
    method: 'POST',
    headers,
    body: formData,
  });

  const failure = () => new Error(i18n.t('plugins.upload.failed'));
  if (!response.headers.get('content-type')?.includes('application/json')) {
    throw failure();
  }
  const result = await response.json().catch(() => { throw failure(); });
  if (result.errors?.length) {
    throw new Error(result.errors[0].message);
  }
  if (!response.ok || !result.data?.installPlugin) throw failure();
  return result.data.installPlugin as PluginInstallResult;
}

export function useInstallPlugin() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ file, grantedPermissions }: { file: File; grantedPermissions: string[] | null }) =>
      installPlugin(file, grantedPermissions),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['plugins'] });
      queryClient.invalidateQueries({ queryKey: ['pluginUIEntries'] });
      queryClient.invalidateQueries({ queryKey: ['plugin-ui-session'] });
    },
  });
}

export function useEnablePlugin() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, grantedPermissions }: { id: string; grantedPermissions: string[] }) =>
      graphqlRequest<{ enablePlugin: Plugin }>(ENABLE_PLUGIN_MUTATION, { id, grantedPermissions }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['plugins'] });
      queryClient.invalidateQueries({ queryKey: ['pluginUIEntries'] });
      queryClient.invalidateQueries({ queryKey: ['plugin-ui-session'] });
      toast.success(i18n.t('plugins.actions.enabled'));
    },
    onError: (error: Error) => {
      toast.error(error.message || i18n.t('plugins.actions.failed'));
    },
  });
}

export function useDisablePlugin() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => graphqlRequest<{ disablePlugin: Plugin }>(DISABLE_PLUGIN_MUTATION, { id }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['plugins'] });
      queryClient.invalidateQueries({ queryKey: ['pluginUIEntries'] });
      queryClient.invalidateQueries({ queryKey: ['plugin-ui-session'] });
      toast.success(i18n.t('plugins.actions.disabled'));
    },
    onError: (error: Error) => {
      toast.error(error.message || i18n.t('plugins.actions.failed'));
    },
  });
}

export function useUpdatePluginSettings() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, priority, config }: { id: string; priority: number; config: Record<string, unknown> }) =>
      graphqlRequest<{ updatePluginSettings: Plugin }>(UPDATE_PLUGIN_SETTINGS_MUTATION, { id, priority, config }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['plugins'] });
      queryClient.invalidateQueries({ queryKey: ['pluginUIEntries'] });
      queryClient.invalidateQueries({ queryKey: ['plugin-ui-session'] });
      toast.success(i18n.t('plugins.config.saved'));
    },
    onError: (error: Error) => {
      toast.error(error.message || i18n.t('plugins.actions.failed'));
    },
  });
}

export function useDeletePlugin() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => graphqlRequest<{ deletePlugin: boolean }>(DELETE_PLUGIN_MUTATION, { id }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['plugins'] });
      queryClient.invalidateQueries({ queryKey: ['pluginUIEntries'] });
      queryClient.invalidateQueries({ queryKey: ['plugin-ui-session'] });
      toast.success(i18n.t('plugins.actions.deleted'));
    },
    onError: (error: Error) => {
      toast.error(error.message || i18n.t('plugins.actions.failed'));
    },
  });
}
