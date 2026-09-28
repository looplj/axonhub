const OIDC_REDIRECT_KEY = 'axonhub:oidc-redirect';

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
