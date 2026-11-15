import { z } from 'zod';

import type { DemoRole } from '@/app/auth/demo-accounts';
import type { DeviceCard, PatchApiV1DevicesByIdData } from '@/generated/api/types.gen';

import { ApiError } from '@/utils/api/errors';

export const DEVICE_TYPES = { PHYSICAL: 'Физическое', VIRTUAL: 'Виртуальное' } satisfies Record<
  NonNullable<DeviceCard['device_type']>,
  string
>;
export const DEVICE_STATUSES = { NEW: 'Новое из выгрузки', ACTIVE: 'Активное' } satisfies Record<
  NonNullable<DeviceCard['status']>,
  string
>;
export const INVENTORY_RESULTS = {
  APPLIED: 'Применён',
  ALREADY_PROCESSED: 'Уже обработан',
  SUPERSEDED: 'Заменён более новым снимком'
};

const optionalText = z
  .string()
  .trim()
  .transform((value) => value || undefined)
  .optional();
export const devicesSearchSchema = z.object({
  serial_number: optionalText,
  hostname: optionalText,
  responsible: optionalText,
  location: optionalText,
  device_type: z
    .union([z.enum(['PHYSICAL', 'VIRTUAL']), z.literal('').transform(() => undefined)])
    .optional(),
  status: z.union([z.enum(['NEW', 'ACTIVE']), z.literal('').transform(() => undefined)]).optional(),
  limit: z.coerce.number().int().positive().default(100)
});

const publicDeviceSchema = z.object({
  id: z.uuid(),
  serial_number: z.string().min(1),
  device_type: z.enum(['PHYSICAL', 'VIRTUAL']).optional(),
  status: z.enum(['NEW', 'ACTIVE']).optional(),
  hostname: z.string().nullable().optional(),
  operating_system: z.string().nullable().optional(),
  last_inventory_at: z.string().nullable().optional(),
  installations_count: z.number().int().nonnegative().optional(),
  open_findings_count: z.number().int().nonnegative().optional()
});
const adminDeviceSchema = publicDeviceSchema.extend({
  responsible: z.string().nullable().optional(),
  location: z.string().nullable().optional(),
  imports: z
    .array(
      z.object({
        inventory_id: z.uuid(),
        collected_at: z.string().optional(),
        result: z.enum(['APPLIED', 'ALREADY_PROCESSED', 'SUPERSEDED']),
        processed_at: z.string().optional()
      })
    )
    .optional()
}) satisfies z.ZodType<DeviceCard>;

export function readDevice(
  payload: unknown,
  role: DemoRole,
  id: string
): DeviceCard & { id: string; serial_number: string } {
  const parsed = (role === 'ADMIN' ? adminDeviceSchema : publicDeviceSchema).safeParse(payload);
  if (!parsed.success || parsed.data.id !== id)
    throw new ApiError('Сервер вернул некорректную карточку устройства.');
  return parsed.data;
}

export const deviceFormSchema = z.object({
  device_type: z.union([z.enum(['PHYSICAL', 'VIRTUAL']), z.literal('').transform(() => undefined)]),
  status: z.union([z.enum(['NEW', 'ACTIVE']), z.literal('').transform(() => undefined)]),
  hostname: z
    .string()
    .trim()
    .transform((value) => value || null),
  responsible: z
    .string()
    .trim()
    .transform((value) => value || null),
  location: z
    .string()
    .trim()
    .transform((value) => value || null)
}) satisfies z.ZodType<NonNullable<PatchApiV1DevicesByIdData['body']>>;

export function getDeviceFormValues(device: DeviceCard): z.input<typeof deviceFormSchema> {
  return {
    device_type: device.device_type ?? '',
    status: device.status ?? '',
    hostname: device.hostname ?? '',
    responsible: device.responsible ?? '',
    location: device.location ?? ''
  };
}
