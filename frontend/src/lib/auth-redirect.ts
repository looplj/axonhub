// 职责: 校验登录回跳地址，并判断 401 是否应该把浏览器送去登录页。
// 关联文件:
//   frontend/src/gql/graphql.ts
//   frontend/src/main.tsx
//   frontend/src/components/command-menu.tsx
//   frontend/src/routes/oauth/oidc/idp-callback.tsx
//   frontend/src/features/auth/data/auth.ts
// 注意事项:
//   未携带凭证的 401 不是过期会话，不能跳转 /sign-in。
//   路径以 /oauth/ 开头时，401 必须留在当前页。离开页面会取消 /oauth/oidc/exchange。
const OIDC_REDIRECT_KEY = 'axonhub:oidc-redirect';

export function isAuthExchangePath(pathname: string): boolean {
  return pathname.startsWith('/oauth/');
}

// Only a request that presented a credential, on a page that is not itself
// finishing login, should abandon the session and open the sign-in page.
export function shouldRedirectUnauthorizedToSignIn(token: string, pathname: string): boolean {
  return token.length > 0 && !isAuthExchangePath(pathname);
}

// Resolve browser URL normalization before allowing a local return destination.
export function getSafeRedirect(redirect?: string): string | undefined {
  if (!redirect || !redirect.startsWith('/') || redirect.startsWith('//')) {
    return undefined;
  }

  try {
    const url = new URL(redirect, window.location.origin);
    if (url.origin !== window.location.origin || decodeURIComponent(url.pathname).startsWith('/sign-in')) {
      return undefined;
    }
    // A normalized path beginning with // would be reinterpreted as a host.
    if (url.pathname.startsWith('//')) {
      return undefined;
    }
    return `${url.pathname}${url.search}${url.hash}`;
  } catch {
    return undefined;
  }
}

export function storeOIDCRedirect(redirect?: string): void {
  try {
    const safeRedirect = getSafeRedirect(redirect);
    if (safeRedirect) {
      window.sessionStorage.setItem(OIDC_REDIRECT_KEY, safeRedirect);
    } else {
      window.sessionStorage.removeItem(OIDC_REDIRECT_KEY);
    }
  } catch {
    // Storage may be unavailable; authentication can still use the default page.
  }
}

export function consumeOIDCRedirect(): string | undefined {
  try {
    const redirect = window.sessionStorage.getItem(OIDC_REDIRECT_KEY);
    window.sessionStorage.removeItem(OIDC_REDIRECT_KEY);
    return getSafeRedirect(redirect ?? undefined);
  } catch {
    return undefined;
  }
}
