import { z } from 'zod';

import type { Import, ImportPreview } from '@/generated/api/types.gen';

import { ApiError } from '@/utils/api/errors';

export const IMPORT_STATUSES = {
  AWAITING_CONFIRMATION: 'Ожидает подтверждения',
  PENDING: 'В очереди',
  PARSING: 'Разбор CSV',
  PROCESSING: 'Обработка снимков',
  COMPLETED: 'Завершён',
  COMPLETED_WITH_ERRORS: 'Завершён с ошибками',
  FAILED: 'Ошибка обработки',
  CANCELLED: 'Отменён'
} satisfies Record<NonNullable<Import['status']>, string>;

const importStatusSchema = z.enum([
  'AWAITING_CONFIRMATION',
  'PENDING',
  'PARSING',
  'PROCESSING',
  'COMPLETED',
  'COMPLETED_WITH_ERRORS',
  'FAILED',
  'CANCELLED'
]);
const counter = z.number().int().nonnegative().optional();
const timestamp = z.iso.datetime({ offset: true });
const importSchema = z.object({
  id: z.uuid(),
  filename: z.string().optional(),
  status: importStatusSchema,
  file_size: counter,
  total_rows: counter,
  loaded_rows: counter,
  duplicate_rows: counter,
  excluded_rows: counter,
  error_rows: counter,
  total_devices: counter,
  processed_devices: counter,
  failed_devices: counter,
  pending_devices: counter,
  progress_percent: z.number().min(0).max(100).optional(),
  created_by: z.string().optional(),
  collected_at: timestamp.optional(),
  error_message: z.string().nullable().optional(),
  created_at: timestamp.optional(),
  completed_at: timestamp.nullable().optional()
}) satisfies z.ZodType<Import>;

const previewSchema = z.object({
  delimiter: z.enum([',', ';']),
  columns: z.array(z.string()),
  rows: z.array(
    z.object({
      row: z.number().int().positive(),
      serial_number: z.string().optional(),
      display_name: z.string().optional(),
      publisher: z.string().optional(),
      version: z.string().optional(),
      excluded: z.boolean().optional(),
      error: z.string().optional()
    })
  )
}) satisfies z.ZodType<ImportPreview>;

export const importsSearchSchema = z.object({
  page: z.coerce.number().int().positive().default(1),
  limit: z.coerce.number().int().min(1).max(200).default(50)
});

export function readImport(
  payload: unknown,
  id?: string
): Import & { id: string; status: NonNullable<Import['status']> } {
  const parsed = importSchema.safeParse(payload);
  if (!parsed.success || (id !== undefined && parsed.data.id !== id))
    throw new ApiError('Сервер вернул некорректный отчёт импорта.', { code: 'INVALID_RESPONSE' });
  return parsed.data;
}

export function readImportPreview(payload: unknown) {
  const parsed = previewSchema.safeParse(payload);
  if (!parsed.success) throw new ApiError('Сервер вернул некорректный предпросмотр CSV.');
  return parsed.data;
}

export function readImportUpload(payload: unknown) {
  const parsed = z
    .object({
      import_id: z.uuid(),
      status: z.enum(['AWAITING_CONFIRMATION', 'PENDING']),
      filename: z.string().optional(),
      preview: previewSchema
    })
    .safeParse(payload);
  if (!parsed.success) throw new ApiError('Сервер не вернул идентификатор и предпросмотр импорта.');
  return parsed.data;
}

export function readImportsPage(payload: unknown) {
  const parsed = z
    .object({
      items: z.array(z.unknown()),
      pagination: z.object({
        page: z.number().int().positive(),
        limit: z.number().int().min(1).max(200),
        total: z.number().int().nonnegative(),
        total_pages: z.number().int().nonnegative()
      })
    })
    .safeParse(payload);
  if (!parsed.success) throw new ApiError('Сервер вернул некорректную историю импортов.');
  const { items, pagination } = parsed.data;
  const imports = items.map((item) => readImport(item));
  if (
    items.length > pagination.limit ||
    items.length > pagination.total ||
    pagination.total_pages < Math.ceil(pagination.total / pagination.limit) ||
    new Set(imports.map((item) => item.id)).size !== items.length
  )
    throw new ApiError('Записи и счётчики истории импортов не согласованы.');
  return { items: imports, pagination };
}

export function isImportTerminal(status: NonNullable<Import['status']>) {
  return ['CANCELLED', 'COMPLETED', 'COMPLETED_WITH_ERRORS', 'FAILED'].includes(status);
}

export function canRetryImport(item: Import) {
  return (
    (item.status === 'FAILED' || item.status === 'COMPLETED_WITH_ERRORS') &&
    item.failed_devices !== undefined &&
    item.failed_devices > 0
  );
}

export function getCsvFileError(file?: File) {
  if (!file?.size) return 'Выберите непустой CSV-файл.';
  if (!/\.csv$/i.test(file.name)) return 'Выберите файл с расширением .csv.';
  // shortcut: консервативные 50 МБ, уточнить байтовую границу у backend перед увеличением.
  if (file.size > 50_000_000) return 'Размер CSV не должен превышать 50 МБ.';
}

export function getCollectedAt(value: string) {
  if (!value) return undefined;
  if (!z.iso.datetime({ local: true }).safeParse(value).success)
    throw new ApiError('Укажите корректную дату и время выгрузки.');
  return new Date(value).toISOString();
}
