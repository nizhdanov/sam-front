import { z } from 'zod';

import type { ContractInput } from '@/generated/api/types.gen';

export const CONTRACT_STATUSES = {
  ACTIVE: 'Действует',
  EXPIRING: 'Скоро истекает',
  EXPIRED: 'Истёк',
  INACTIVE: 'Деактивирован',
  NOT_STARTED: 'Ещё не действует'
};

export const LICENSE_METRICS = {
  PER_DEVICE: 'На устройство',
  UNLIMITED: 'Без ограничений',
  CONCURRENT: 'Одновременные подключения',
  UNKNOWN: 'Не определена'
} satisfies Record<NonNullable<ContractInput['license_metric']>, string>;

const optionalText = z
  .string()
  .trim()
  .transform((value) => value || undefined)
  .optional();

export const contractsSearchSchema = z.object({
  product: optionalText,
  product_id: z.union([z.uuid(), z.literal('').transform(() => undefined)]).optional(),
  contract_number: optionalText,
  metric: z
    .union([
      z.enum(['PER_DEVICE', 'UNLIMITED', 'CONCURRENT', 'UNKNOWN']),
      z.literal('').transform(() => undefined)
    ])
    .optional(),
  status: z
    .union([
      z.enum(['ACTIVE', 'EXPIRING', 'EXPIRED', 'INACTIVE', 'NOT_STARTED']),
      z.literal('').transform(() => undefined)
    ])
    .optional(),
  limit: z.coerce.number().int().positive().default(100)
});

export type ContractsSearch = z.infer<typeof contractsSearchSchema>;

export function getContractLabel(column: string, value: unknown) {
  if (typeof value !== 'string') return undefined;
  const labels: Record<string, string> | undefined =
    column === 'status' ? CONTRACT_STATUSES : column === 'metric' ? LICENSE_METRICS : undefined;
  return labels && Object.hasOwn(labels, value) ? labels[value] : undefined;
}
