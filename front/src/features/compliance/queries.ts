import type { QueryClient } from '@tanstack/react-query';

import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

import type { DemoAuth } from '@/app/auth/session';

import { getDashboard } from '@/generated/api/requests/compliance/getDashboard.gen';
import { postComplianceFindingByIdAcknowledge } from '@/generated/api/requests/compliance/postComplianceFindingByIdAcknowledge.gen';
import { ApiError } from '@/utils/api/errors';

import { acknowledgeSchema, readDashboard } from './model';

export function dashboardOptions(auth: DemoAuth) {
  const session = auth.requireReady();
  if (session.role !== 'ADMIN') throw new ApiError('Доступ запрещён.', { status: 403 });
  return queryOptions({
    queryKey: ['dashboard', session.role, session.epoch],
    queryFn: async ({ signal }) => {
      auth.requestConfig(session);
      const response = await getDashboard({ config: { signal } });
      return readDashboard(response.data);
    }
  });
}

export async function acknowledgeFinding(
  auth: DemoAuth,
  queryClient: QueryClient,
  id: string,
  comment: string
) {
  const session = auth.requireReady();
  if (session.role !== 'ADMIN') throw new ApiError('Доступ запрещён.', { status: 403 });
  if (!z.uuid().safeParse(id).success) throw new ApiError('Некорректный ID нарушения.');
  const parsed = acknowledgeSchema.safeParse({ comment });
  if (!parsed.success) throw new ApiError('Введите комментарий об обработке нарушения.');
  try {
    await postComplianceFindingByIdAcknowledge({
      path: { id },
      body: parsed.data,
      config: auth.requestConfig(session)
    });
    auth.requestConfig(session);
  } finally {
    if (auth.isCurrent(session.epoch))
      await Promise.all(
        ['findings', 'devices', 'me', 'dashboard', 'compliance', 'summary', 'audit'].map(
          (resource) => queryClient.invalidateQueries({ queryKey: [resource] })
        )
      );
  }
}
