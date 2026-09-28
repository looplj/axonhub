import { getTokenFromStorage, useAuthStore } from '@/stores/authStore';

const SESSION_RENEWAL_SECONDS = 7 * 24 * 60 * 60;

let pendingRefresh: { token: string; promise: Promise<string | null> } | null = null;
let lastInteraction = 0;

if (typeof window !== 'undefined') {
  for (const event of ['pointerdown', 'keydown']) {
    window.addEventListener(
      event,
      () => {
        lastInteraction = Date.now();
      },
      { passive: true }
    );
  }
}

const decodeClaims = (token: string): Record<string, unknown> | null => {
  const payload = token.split('.')[1];
  if (!payload) return null;

  try {
    const normalized = payload.replace(/-/g, '+').replace(/_/g, '/');
    const decoded = atob(normalized.padEnd(Math.ceil(normalized.length / 4) * 4, '='));
    const claims: unknown = JSON.parse(decoded);
    return typeof claims === 'object' && claims !== null ? (claims as Record<string, unknown>) : null;
  } catch {
    return null;
  }
};

const readNumberClaim = (claims: Record<string, unknown>, name: string): number | null => {
  const value = claims[name];
  return typeof value === 'number' && Number.isFinite(value) ? value : null;
};

const refreshAccessToken = async (token: string): Promise<string | null> => {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 5_000);
  try {
    const response = await fetch('/admin/auth/refresh', {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}` },
      signal: controller.signal,
    });

    if (response.status === 204) return token;
    if (!response.ok) return null;

    const body: unknown = await response.json();
    if (typeof body !== 'object' || body === null || !('token' in body) || typeof body.token !== 'string') return null;

    if (getTokenFromStorage() !== token) return getTokenFromStorage() || null;
    useAuthStore.getState().auth.setAccessToken(body.token);
    return body.token;
  } catch {
    return null;
  } finally {
    clearTimeout(timeout);
  }
};

export const ensureFreshAccessToken = async (): Promise<string> => {
  const token = getTokenFromStorage();
  if (!token) return '';
  if (!lastInteraction || Date.now() - lastInteraction >= 5 * 60 * 1000) return token;

  const claims = decodeClaims(token);
  const expiresAt = claims ? readNumberClaim(claims, 'exp') : null;
  const authTime = claims ? readNumberClaim(claims, 'auth_time') : null;
  const now = Math.floor(Date.now() / 1000);
  if (expiresAt === null || authTime === null || expiresAt <= now || expiresAt - now >= SESSION_RENEWAL_SECONDS) return token;

  if (!pendingRefresh || pendingRefresh.token !== token) {
    const promise = refreshAccessToken(token).finally(() => {
      if (pendingRefresh?.promise === promise) pendingRefresh = null;
    });
    pendingRefresh = { token, promise };
  }

  await pendingRefresh.promise;
  return getTokenFromStorage();
};
