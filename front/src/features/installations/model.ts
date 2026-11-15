import { z } from 'zod';

const optionalText = z
  .string()
  .trim()
  .transform((value) => value || undefined)
  .optional();
const optionalBoolean = z
  .union([
    z.boolean(),
    z.enum(['true', 'false']).transform((value) => value === 'true'),
    z.literal('').transform(() => undefined)
  ])
  .optional();

export const installationsSearchSchema = z.object({
  product: optionalText,
  product_id: z.union([z.uuid(), z.literal('').transform(() => undefined)]).optional(),
  raw_name: optionalText,
  publisher: optionalText,
  version: optionalText,
  serial_number: optionalText,
  device_type: z
    .union([z.enum(['PHYSICAL', 'VIRTUAL']), z.literal('').transform(() => undefined)])
    .optional(),
  match_status: optionalText,
  recognized: optionalBoolean,
  status: z
    .union([z.enum(['ACTUAL', 'OUTDATED', 'FORBIDDEN']), z.literal('').transform(() => undefined)])
    .optional(),
  license_type: z
    .union([
      z.enum(['OPEN_SOURCE', 'COMMERCIAL', 'TRIAL']),
      z.literal('').transform(() => undefined)
    ])
    .optional(),
  cost_model: z
    .union([z.enum(['FREE', 'PAID', 'SHAREWARE']), z.literal('').transform(() => undefined)])
    .optional(),
  in_bpo: optionalBoolean,
  group_by: z
    .union([z.enum(['product', 'serial_number']), z.literal('').transform(() => undefined)])
    .optional(),
  limit: z.coerce.number().int().positive().default(100)
});
