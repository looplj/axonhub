// 职责: 读取插件贡献的页面和顶栏动作，并创建插件 UI 会话。
// 关联文件:
//   frontend/src/gql/graphql.ts
//   frontend/src/sidebar.ts
//   frontend/src/components/command-menu.tsx
//   frontend/src/stores/authStore.ts
// 遵循的 Rules:
//   .agents/rules/frontend-general.md
// 注意事项:
//   没有 access token 时不能请求 /admin/graphql。命令菜单在登录完成前也会调用本查询。
import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useAuthStore } from '@/stores/authStore';

const PLUGIN_UI_ENTRIES_QUERY = `
  query PluginUIEntries {
    pluginUIEntries {
      actions {
        pluginID key slot title icon revision
        effect { type entry }
      }
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

export interface PluginUIAction {
  pluginID: string;
  key: string;
  slot: 'app.header.actions';
  title: string | Record<string, string>;
  icon: 'puzzle' | 'search' | 'activity' | 'settings';
  effect: { type: 'dialog'; entry: string };
  revision: string;
}

export interface PluginUIEntriesResult {
  pluginUIEntries: {
    pages: PluginUIPage[];
    actions: PluginUIAction[];
  };
}

export interface PluginUISession {
  token: string;
  config: Record<string, unknown>;
  canWriteKV: boolean;
}

export function usePluginUIEntries() {
  const accessToken = useAuthStore((state) => state.auth.accessToken);

  return useQuery({
    queryKey: ['pluginUIEntries'],
    queryFn: () => graphqlRequest<PluginUIEntriesResult>(PLUGIN_UI_ENTRIES_QUERY),
    // The command menu mounts this outside the authenticated layout. Without
    // a session the admin API returns 401 and the client would leave the page.
    enabled: accessToken.length > 0,
  });
}

export function createPluginUISession(pluginID: string, entry: string, signal?: AbortSignal) {
  return graphqlRequest<{ createPluginUISession: PluginUISession }>(
    CREATE_PLUGIN_UI_SESSION_MUTATION,
    {
      pluginID,
      entry,
    },
    undefined,
    { signal }
  ).then((result) => result.createPluginUISession);
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
