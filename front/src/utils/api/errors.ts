import { isAxiosError, isCancel } from 'axios';
import { z } from 'zod';

import type { _Error } from '@/generated/api';

const errorSchema = z.object({
  error: z.object({
    code: z.string().optional(),
    message: z.string().optional(),
    request_id: z.string().optional()
  })
}) satisfies z.ZodType<_Error>;

const HTTP_MESSAGES: Record<number, string> = {
  400: 'Проверьте переданные данные.',
  401: 'Демонстрационный токен недействителен. Выберите роль повторно.',
  403: 'Доступ запрещён.',
  404: 'Объект недоступен.',
  409: 'Конфликт данных. Обновите данные перед повторной попыткой.',
  413: 'Файл превышает допустимый размер.',
  429: 'Слишком много запросов. Повторите позже.',
  503: 'Сервис временно недоступен. Повторите позже.'
};

interface ApiErrorDetails {
  code?: string;
  requestId?: string;
  status?: number;
}

export class ApiError extends Error {
  readonly status?: number;
  readonly code?: string;
  readonly requestId?: string;

  constructor(message: string, details: ApiErrorDetails = {}) {
    super(message);
    this.name = 'ApiError';
    this.status = details.status;
    this.code = details.code;
    this.requestId = details.requestId;
  }
}

function safeText(value?: string) {
  if (
    !value ||
    value.length > 500 ||
    /[\p{Cc}\p{Cf}]|<[^>]+>|bearer\s|eyJ[\w-]*\.[\w-]+\.[\w-]+/iu.test(value)
  ) {
    return undefined;
  }

  return value.trim() || undefined;
}

export async function readApiError(data: unknown, status?: number) {
  let payload: unknown = data;
  try {
    if (payload instanceof Blob) payload = await payload.text();
    if (typeof payload === 'string') payload = JSON.parse(payload);
  } catch {
    payload = undefined;
  }

  const parsed = errorSchema.safeParse(payload);
  const error = parsed.success ? parsed.data.error : undefined;
  const fallback = status
    ? (HTTP_MESSAGES[status] ?? 'Не удалось выполнить запрос. Повторите позже.')
    : 'Не удалось связаться с сервером. Проверьте подключение.';

  return new ApiError(safeText(error?.message) ?? fallback, {
    status,
    code: safeText(error?.code),
    requestId: safeText(error?.request_id)
  });
}

export async function normalizeApiError(error: unknown) {
  if (error instanceof ApiError || isCancel(error)) return error;
  if (isAxiosError(error)) return readApiError(error.response?.data, error.response?.status);

  return readApiError(undefined);
}
