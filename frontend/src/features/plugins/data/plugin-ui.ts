import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';

const PLUGIN_UI_ENTRIES_QUERY = `
  query PluginUIEntries {
    pluginUIEntries {
      pages {
        pluginID
        key
        entry
        title
      }
    }
  }
`;

const CREATE_PLUGIN_UI_SESSION_MUTATION = `
  mutation CreatePluginUISession($pluginID: String!, $entry: String!) {
    createPluginUISession(pluginID: $pluginID, entry: $entry) {
      token
      config
      canWriteKV
    }
  }
`;

export interface PluginUIPage {
  pluginID: string;
  key: string;
  entry: string;
  title: string | Record<string, string>;
}

export interface PluginUIEntriesResult {
  pluginUIEntries: {
    pages: PluginUIPage[];
  };
}

export interface PluginUISession {
  token: string;
  config: Record<string, unknown>;
  canWriteKV: boolean;
}

export function usePluginUIEntries() {
  return useQuery({
    queryKey: ['pluginUIEntries'],
    queryFn: () => graphqlRequest<PluginUIEntriesResult>(PLUGIN_UI_ENTRIES_QUERY),
  });
}

export function createPluginUISession(pluginID: string, entry: string) {
  return graphqlRequest<{ createPluginUISession: PluginUISession }>(CREATE_PLUGIN_UI_SESSION_MUTATION, {
    pluginID,
    entry,
  }).then((result) => result.createPluginUISession);
}

export function pluginFrameSrc(token: string, entry: string) {
  const clean = entry.replace(/^\/+/, '');
  const path = clean
    .split('/')
    .filter((part) => part.length > 0)
    .map((part) => encodeURIComponent(part))
    .join('/');
  return `/admin/plugin-ui/${encodeURIComponent(token)}/${path}`;
}
