import { getBackendSrv } from '@grafana/runtime';

import { PLUGIN_RESOURCES } from '../constants';

// Plugin Go backend mounts the /resources/yf/* surface. Every endpoint
// — including the Yahoo ticker lookup that used to live in the
// reference-admin Python service — is served locally by the Go plugin.
export const YF_BASE = `${PLUGIN_RESOURCES}/yf`;

type RequestOptions = {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  body?: unknown;
};

export const RES_BASE = PLUGIN_RESOURCES;

export async function yfRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  return request<T>(`${YF_BASE}${path}`, options);
}

export async function resRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  return request<T>(`${RES_BASE}${path}`, options);
}

async function request<T>(url: string, options: RequestOptions): Promise<T> {
  const method = options.method ?? 'GET';
  const srv = getBackendSrv();
  switch (method) {
    case 'GET':
      return srv.get<T>(url);
    case 'DELETE':
      return srv.delete<T>(url);
    case 'POST':
      return srv.post<T>(url, options.body);
    case 'PUT':
      return srv.put<T>(url, options.body);
    case 'PATCH':
      return srv.patch<T>(url, options.body);
  }
}
