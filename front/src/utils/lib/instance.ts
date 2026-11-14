import type { InternalAxiosRequestConfig } from 'axios';

import axios, { CanceledError } from 'axios';

import type { DemoAuth, DemoSession } from '@/app/auth/session';

import { DEMO_ACCOUNTS } from '@/app/auth/demo-accounts';
import { ApiError, normalizeApiError } from '@/utils/api/errors';

export const instance = axios.create({ baseURL: '/api/v1' });

let auth: DemoAuth | undefined;
const contexts = new WeakMap<InternalAxiosRequestConfig, DemoSession>();

export function setApiSession(session?: DemoAuth) {
  auth = session;
}

instance.interceptors.request.use(
  (config) => {
    if (config.signal?.aborted) throw new CanceledError();
    const session = auth?.requireReady();
    if (session) contexts.set(config, session);
    let token: string | undefined;
    try {
      token = localStorage.getItem('token')?.trim();
    } catch {
      const error = new ApiError(
        'Хранилище браузера недоступно. Разрешите localStorage для запросов API.'
      );
      if (session) auth?.fail(session.epoch, error);
      throw error;
    }

    if (session) {
      if (token !== DEMO_ACCOUNTS.find((account) => account.role === session.role)?.token) {
        void auth?.syncStorage();
        throw new CanceledError();
      }
      config.signal =
        config.signal instanceof AbortSignal
          ? AbortSignal.any([config.signal, session.signal])
          : session.signal;
    }

    if (token) config.headers.set('Authorization', `Bearer ${token}`);
    else config.headers.delete('Authorization');

    return config;
  },
  (error: unknown) => {
    throw error;
  },
  { synchronous: true }
);

function isStale(config?: InternalAxiosRequestConfig) {
  const session = config && contexts.get(config);
  return session && (!auth?.isCurrent(session.epoch) || session.signal.aborted);
}

instance.interceptors.response.use(
  (response) => {
    if (isStale(response.config)) throw new CanceledError();
    return response;
  },
  async (error: unknown) => {
    const config = axios.isAxiosError(error) ? error.config : undefined;
    const normalized = await normalizeApiError(error);
    if (isStale(config)) throw new CanceledError();
    const session = config && contexts.get(config);
    if (session && normalized instanceof ApiError && normalized.status === 401) {
      auth?.fail(session.epoch, normalized);
    }
    throw normalized;
  }
);
