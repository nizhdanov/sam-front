import axios from 'axios';

import { ApiError, normalizeApiError } from '@/utils/api/errors';

export const instance = axios.create({ baseURL: '/api/v1' });

instance.interceptors.request.use((config) => {
  let token: string | undefined;
  try {
    token = localStorage.getItem('token')?.trim();
  } catch {
    throw new ApiError('Хранилище браузера недоступно. Разрешите localStorage для запросов API.');
  }

  if (token) config.headers.set('Authorization', `Bearer ${token}`);
  else config.headers.delete('Authorization');

  return config;
});

instance.interceptors.response.use(
  (response) => response,
  async (error: unknown) => Promise.reject(await normalizeApiError(error))
);
