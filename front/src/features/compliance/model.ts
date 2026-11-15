import { z } from 'zod';

import type { Dashboard, FindingType } from '@/generated/api/types.gen';

import { ApiError } from '@/utils/api/errors';

export const FINDING_TYPES = {
  FORBIDDEN_SOFTWARE: 'Запрещённое ПО',
  OUTDATED_SOFTWARE: 'Устаревшее ПО',
  LICENSE_DEFICIT: 'Дефицит лицензий'
} satisfies Record<FindingType, string>;
export const FINDING_STATUSES = {
  OPEN: 'Открыто',
  ACKNOWLEDGED: 'Обработано, не устранено',
  RESOLVED: 'Устранено'
};
export const FINDING_SEVERITIES = {
  LOW: 'Низкая',
  MEDIUM: 'Средняя',
  HIGH: 'Высокая',
  CRITICAL: 'Критическая'
};

const optionalText = z
  .string()
  .trim()
  .transform((value) => value || undefined)
  .optional();
export const findingTypeSchema = z.enum([
  'FORBIDDEN_SOFTWARE',
  'OUTDATED_SOFTWARE',
  'LICENSE_DEFICIT'
]);
export const findingsSearchSchema = z.object({
  finding_type: z.union([findingTypeSchema, z.literal('').transform(() => undefined)]).optional(),
  severity: z
    .union([
      z.enum(['LOW', 'MEDIUM', 'HIGH', 'CRITICAL']),
      z.literal('').transform(() => undefined)
    ])
    .optional(),
  status: z
    .union([z.enum(['OPEN', 'ACKNOWLEDGED', 'RESOLVED']), z.literal('').transform(() => undefined)])
    .optional(),
  product: optionalText,
  product_id: z.union([z.uuid(), z.literal('').transform(() => undefined)]).optional(),
  serial_number: optionalText,
  limit: z.coerce.number().int().positive().default(100)
});
export const myDeviceSearchSchema = z.object({
  finding_type: z.enum(['FORBIDDEN_SOFTWARE', 'OUTDATED_SOFTWARE']).default('FORBIDDEN_SOFTWARE')
});
export const acknowledgeSchema = z.object({
  comment: z.string().trim().min(1, 'Введите комментарий об обработке нарушения.')
});

const count = z.number().int().nonnegative().optional();
const dashboardSchema = z.object({
  devices: count,
  new_devices: count,
  installations: count,
  recognized_installations: count,
  recognized_percent: z.number().min(0).max(100).optional(),
  pending_review: count,
  license_deficit: z.object({ products: count, seats: count }).optional(),
  idle_licenses: z.object({ products: count, seats: count }).optional(),
  forbidden_software: z.object({ installations: count, devices: count }).optional(),
  outdated_software: z.object({ installations: count, devices: count }).optional(),
  expiring_contracts: count,
  expiring_within_days: count
}) satisfies z.ZodType<Dashboard>;

export function readDashboard(payload: unknown) {
  const parsed = dashboardSchema.safeParse(payload);
  if (!parsed.success) throw new ApiError('Сервер вернул некорректные показатели парка.');
  return parsed.data;
}
