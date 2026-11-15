import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

import type { DemoAuth } from '@/app/auth/session';

import { getBalance } from '@/generated/api/requests/compliance/getBalance.gen';
import { parseResponse, requireAdmin } from '@/utils/api/page';

export const METRICS = {
  PER_DEVICE: 'На устройство',
  UNLIMITED: 'Без ограничений',
  CONCURRENT: 'Конкурентная',
  UNKNOWN: 'Не определена'
};

export const CALCULATION_STATUSES = {
  VERIFIED: 'Подтверждён',
  INCOMPLETE: 'Неполные данные',
  UNVERIFIED: 'Не проверен',
  UNSUPPORTED_METRIC: 'Метрика не поддерживается'
};

export const balanceSearchSchema = z.object({
  view: z.enum(['commercial', 'free']).default('commercial'),
  risk: z.enum(['deficit', 'available']).optional()
});

const commercialSchema = z.object({
  product_id: z.uuid(),
  product_name: z.string(),
  cost_model: z.enum(['FREE', 'PAID', 'SHAREWARE']).nullable(),
  metric: z.string(),
  purchased: z.number().int().nullable(),
  consumed: z.number().int().nonnegative(),
  available: z.number().int().nullable(),
  deficit: z.number().int().nonnegative(),
  calculation_status: z.string(),
  note: z.string().optional(),
  last_calculated_at: z.string()
});

const freeSchema = z.object({
  product_id: z.string(),
  product_name: z.string(),
  publisher: z.string().nullable(),
  in_bpo: z.boolean(),
  status: z.string(),
  current_version: z.string().nullable(),
  devices: z.number().int().nonnegative(),
  outdated_devices: z.number().int().nonnegative()
});

const balanceSchema = z.discriminatedUnion('view', [
  z.object({ view: z.literal('commercial'), items: z.array(commercialSchema) }),
  z.object({ view: z.literal('free'), items: z.array(freeSchema) })
]);

export type CommercialBalance = z.infer<typeof commercialSchema>;
export type FreeBalance = z.infer<typeof freeSchema>;

export function balanceOptions(auth: DemoAuth, view: 'commercial' | 'free') {
  const session = requireAdmin(auth);
  return queryOptions({
    queryKey: ['balance', session.role, session.epoch, view],
    queryFn: async ({ signal }) => {
      auth.requestConfig(session);
      const response = await getBalance({ query: { view }, config: { signal } });
      return parseResponse(
        balanceSchema,
        response.data,
        'Сервер вернул некорректный баланс лицензий.'
      );
    }
  });
}

export function filterByRisk(items: CommercialBalance[], risk?: 'available' | 'deficit') {
  if (risk === 'deficit') return items.filter((item) => item.deficit > 0);
  if (risk === 'available') return items.filter((item) => (item.available ?? 0) > 0);
  return items;
}
