import { z } from 'zod';

import type { DemoRole } from '@/app/auth/demo-accounts';
import type {
  CostModel,
  GetApiV1SoftwareData,
  LicenseType,
  Product,
  ProductStatus
} from '@/generated/api/types.gen';

import { ApiError } from '@/utils/api/errors';

export const PRODUCT_STATUSES = {
  ACTUAL: 'Актуальное',
  OUTDATED: 'Устаревшее',
  FORBIDDEN: 'Запрещённое'
} satisfies Record<ProductStatus, string>;

export const LICENSE_TYPES = {
  OPEN_SOURCE: 'Открытое ПО',
  COMMERCIAL: 'Коммерческое',
  TRIAL: 'Пробное'
} satisfies Record<LicenseType, string>;

export const COST_MODELS = {
  FREE: 'Бесплатно',
  PAID: 'Платно',
  SHAREWARE: 'Условно-бесплатно'
} satisfies Record<CostModel, string>;

const productStatusSchema = z.enum(['ACTUAL', 'OUTDATED', 'FORBIDDEN']);
const licenseTypeSchema = z.enum(['OPEN_SOURCE', 'COMMERCIAL', 'TRIAL']);
const costModelSchema = z.enum(['FREE', 'PAID', 'SHAREWARE']);

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

export const softwareSearchSchema = z.object({
  page: z.coerce.number().int().positive().default(1),
  limit: z.coerce.number().int().min(1).max(200).default(50),
  search: optionalText,
  section: optionalText,
  class: optionalText,
  license_type: z.union([licenseTypeSchema, z.literal('').transform(() => undefined)]).optional(),
  cost_model: z.union([costModelSchema, z.literal('').transform(() => undefined)]).optional(),
  status: z.union([productStatusSchema, z.literal('').transform(() => undefined)]).optional(),
  in_bpo: optionalBoolean,
  include_archived: optionalBoolean
});

export type SoftwareSearch = z.infer<typeof softwareSearchSchema>;

export function getSoftwareParams(search: SoftwareSearch, role: DemoRole) {
  return {
    ...search,
    include_archived: role === 'ADMIN' ? search.include_archived : undefined
  } satisfies GetApiV1SoftwareData['query'];
}

// Backend возвращает null для незаполненных строк, хотя OpenAPI описывает optional string.
const productText = z
  .string()
  .nullish()
  .transform((value) => value ?? undefined);

const publicProductSchema = z.object({
  id: z.uuid(),
  canonical_name: productText,
  publisher: productText,
  edition: productText,
  purpose: productText,
  section: productText,
  class: productText,
  in_bpo: z.boolean().optional(),
  current_version: z.string().nullable().optional(),
  agreement_name: z.string().nullable().optional(),
  license_type: licenseTypeSchema.optional(),
  cost_model: costModelSchema.optional(),
  status: productStatusSchema.optional(),
  status_comment: productText
});

const adminProductSchema = publicProductSchema.extend({
  agreement_id: z.uuid().nullable().optional(),
  purchased_seats: z.number().int().nonnegative().optional(),
  active_installations: z.number().int().nonnegative().optional(),
  devices: z.number().int().nonnegative().optional(),
  archived_at: z.string().nullable().optional()
}) satisfies z.ZodType<Product>;

export function readProduct(payload: unknown, role: DemoRole): Product & { id: string } {
  const parsed = (role === 'ADMIN' ? adminProductSchema : publicProductSchema).safeParse(payload);
  if (!parsed.success) throw new ApiError('Сервер вернул некорректную карточку ПО.');
  return parsed.data;
}

const softwarePageSchema = z.object({
  items: z.array(z.unknown()),
  pagination: z.object({
    page: z.number().int().positive(),
    limit: z.number().int().min(1).max(200),
    total: z.number().int().nonnegative(),
    total_pages: z.number().int().nonnegative()
  })
});

export function readSoftwarePage(payload: unknown, role: DemoRole) {
  const parsed = softwarePageSchema.safeParse(payload);
  if (!parsed.success) throw new ApiError('Сервер вернул некорректный список ПО.');
  const { items, pagination } = parsed.data;
  const products = items.map((item) => readProduct(item, role));
  if (
    items.length > pagination.limit ||
    items.length > pagination.total ||
    pagination.total_pages < Math.ceil(pagination.total / pagination.limit) ||
    new Set(products.map((item) => item.id)).size !== items.length
  )
    throw new ApiError('Записи и счётчики каталога не согласованы.');
  return { items: products, pagination };
}
