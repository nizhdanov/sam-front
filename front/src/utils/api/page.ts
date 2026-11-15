import { z } from 'zod';

import type { DemoAuth } from '@/app/auth/session';

import { ApiError } from './errors';

export function pageSchema<T extends z.ZodType>(item: T) {
  return z.object({
    items: z.array(item),
    pagination: z.object({
      page: z.number().int(),
      limit: z.number().int(),
      total: z.number().int().nonnegative(),
      total_pages: z.number().int().nonnegative()
    })
  });
}

export function parseResponse<T extends z.ZodType>(schema: T, payload: unknown, message: string) {
  const parsed = schema.safeParse(payload);
  if (!parsed.success) throw new ApiError(message);
  return parsed.data;
}

export function requireAdmin(auth: DemoAuth) {
  const session = auth.requireReady();
  if (session.role !== 'ADMIN') throw new ApiError('Доступ запрещён.', { status: 403 });
  return session;
}
