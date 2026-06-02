const API_BASE = import.meta.env.VITE_API_BASE || '';
const swrRequests = new Map<string, Promise<unknown>>();

function etagStorageKey(path: string) {
  return `dbback-etag:${path}`;
}

function readEtag(path: string) {
  return window.sessionStorage.getItem(etagStorageKey(path)) || undefined;
}

function storeEtag(path: string, etag: string) {
  window.sessionStorage.setItem(etagStorageKey(path), etag);
}

export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = jsonHeaders(init);
  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    credentials: 'include',
    headers,
  });
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(payload.error || 'Nao foi possivel concluir a operacao.');
  }
  return payload as T;
}

export async function apiSWR<T>(path: string, init?: RequestInit): Promise<T | undefined> {
  const requestKey = swrRequestKey(path, init);
  if (requestKey && swrRequests.has(requestKey)) {
    return swrRequests.get(requestKey) as Promise<T | undefined>;
  }

  const request = fetchSWR<T>(path, init);
  if (requestKey) {
    swrRequests.set(requestKey, request);
    request.then(
      () => swrRequests.delete(requestKey),
      () => swrRequests.delete(requestKey),
    );
  }
  return request;
}

async function fetchSWR<T>(path: string, init?: RequestInit): Promise<T | undefined> {
  const headers = new Headers(init?.headers || {});
  const previousEtag = readEtag(path);
  if (previousEtag) {
    headers.set('If-None-Match', previousEtag);
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    cache: init?.cache ?? 'default',
    credentials: 'include',
    headers: jsonHeaders({ ...init, headers }),
  });

  if (response.status === 304) {
    return undefined;
  }

  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(payload.error || 'Nao foi possivel concluir a operacao.');
  }

  const etag = response.headers.get('ETag');
  if (etag) {
    storeEtag(path, etag);
  }
  return payload as T;
}

function jsonHeaders(init?: RequestInit) {
  const headers = new Headers(init?.headers || {});
  if (!headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }
  return headers;
}

function swrRequestKey(path: string, init?: RequestInit) {
  const method = (init?.method || 'GET').toUpperCase();
  if (method !== 'GET' || init?.signal) return '';
  return path;
}
