const API_BASE = import.meta.env.VITE_API_BASE || '';
const ETAG_PREFIX = 'dbback-etag:';
const swrRequests = new Map<string, Promise<unknown>>();

type ApiPayload = {
  error?: string;
};

type ParsedResponse = {
  json: boolean;
  payload: ApiPayload;
};

function etagStorageKey(path: string) {
  return `${ETAG_PREFIX}${path}`;
}

function readEtag(path: string) {
  return window.sessionStorage.getItem(etagStorageKey(path)) || undefined;
}

function storeEtag(path: string, etag: string) {
  window.sessionStorage.setItem(etagStorageKey(path), etag);
}

export function clearApiValidators(paths?: string[]) {
  if (paths?.length) {
    paths.forEach((path) => window.sessionStorage.removeItem(etagStorageKey(path)));
    return;
  }
  Object.keys(window.sessionStorage)
    .filter((key) => key.startsWith(ETAG_PREFIX))
    .forEach((key) => window.sessionStorage.removeItem(key));
}

export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = jsonHeaders(init);
  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    credentials: 'include',
    headers,
  });
  const { json, payload } = await parseResponse(response);
  if (!response.ok) {
    throw new Error(responseErrorMessage(response, payload, json));
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

  const { json, payload } = await parseResponse(response);
  if (!response.ok) {
    throw new Error(responseErrorMessage(response, payload, json));
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
  if (method !== 'GET' || init?.signal || init?.cache === 'reload') return '';
  return path;
}

async function parseResponse(response: Response): Promise<ParsedResponse> {
  const contentType = response.headers.get('Content-Type') || '';
  if (!contentType.includes('application/json')) {
    return { json: false, payload: {} };
  }
  const payload = await response.json().catch(() => ({}));
  return { json: true, payload: isApiPayload(payload) ? payload : {} };
}

function responseErrorMessage(response: Response, payload: ApiPayload, json: boolean) {
  if (payload.error) return payload.error;
  if (import.meta.env.DEV && (!json || response.status === 404)) {
    return 'API local nao respondeu. Abra http://localhost:3000 ou rode npm run dev:full.';
  }
  return 'Nao foi possivel concluir a operacao.';
}

function isApiPayload(payload: unknown): payload is ApiPayload {
  return Boolean(payload) && typeof payload === 'object';
}
