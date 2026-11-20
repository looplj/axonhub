// 职责: 读取部署路径前缀，在应用路径与浏览器路径之间转换。
// 关联文件:
//   frontend/src/main.tsx
//   frontend/src/lib/api-client.ts
//   frontend/src/lib/auth-redirect.ts
//   frontend/src/gql/graphql.ts
// 注意事项:
//   服务端注入的前缀不带末尾斜杠，根路径部署时为空。

export function getBasePath(): string {
  return typeof document === 'undefined'
    ? ''
    : (document.querySelector<HTMLMetaElement>('meta[name="axonhub-base-path"]')?.content ?? '');
}

export function appPath(path: string): string {
  return `${getBasePath()}${path}`;
}

export function appRoutePath(pathname: string): string | undefined {
  const basePath = getBasePath();
  if (!basePath) return pathname;
  if (pathname === basePath) return '/';
  if (pathname.startsWith(`${basePath}/`)) return pathname.slice(basePath.length);
  return undefined;
}
