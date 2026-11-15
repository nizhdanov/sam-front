import type { AxiosRequestConfig } from 'axios';

import { CanceledError } from 'axios';
import { z } from 'zod';

import type { DemoRole } from '@/app/auth/demo-accounts';
import type { GetApiV1DataByDatasetData } from '@/generated/api/types.gen';

import { getDataByDataset } from '@/generated/api/requests/data/getDataByDataset.gen';

import { ApiError, readApiError } from './errors';
import { downloadFile } from './files';

export type Dataset = GetApiV1DataByDatasetData['path']['dataset'];
export type DataFilters = Record<string, unknown>;

// Имена взяты из описания /data/{dataset}; не все фильтры входят в generated query.
const DATASETS = {
  installations: {
    columns:
      'product_id device_id serial_number device_type raw_name product publisher version current_version match_status status license_type cost_model first_seen_at last_seen_at license_status',
    filters:
      'product product_id raw_name publisher version serial_number device_type match_status recognized status license_type cost_model in_bpo',
    groups:
      'product publisher version serial_number device_type match_status status license_type cost_model'
  },
  devices: {
    columns:
      'device_id serial_number device_type status hostname responsible location last_inventory_at installations open_findings',
    filters: 'serial_number hostname responsible location device_type status',
    groups: 'device_type status location'
  },
  products: {
    columns:
      'product_id product publisher current_version section class in_bpo status status_comment license_type cost_model purchased devices license_status',
    filters: 'product product_id publisher section class status license_type cost_model in_bpo',
    groups: 'publisher section class status license_type cost_model in_bpo'
  },
  contracts: {
    columns:
      'contract_id product_id product contract_number metric quantity valid_from expires_at status',
    filters: 'product product_id contract_number metric status',
    groups: 'product metric status'
  },
  findings: {
    columns:
      'finding_id product_id device_id finding_type severity status serial_number raw_name product version current_version detected_at comment',
    filters: 'finding_type severity status serial_number product product_id',
    groups: 'finding_type severity status serial_number product'
  }
} satisfies Record<Dataset, { columns: string; filters: string; groups: string }>;

const ADMIN_COLUMNS = new Set([
  'license_status',
  'purchased',
  'devices',
  'responsible',
  'location',
  'comment'
]);
const booleanFilter = z.union([
  z.boolean(),
  z.enum(['true', 'false']).transform((value) => value === 'true')
]);
const positiveInteger = z
  .union([z.number(), z.string().regex(/^\d+$/).transform(Number)])
  .pipe(z.number().int().positive());

function assertDatasetAccess(dataset: Dataset, role: DemoRole) {
  if ((role !== 'ADMIN' && role !== 'USER') || (dataset === 'contracts' && role !== 'ADMIN')) {
    throw new ApiError('Доступ запрещён.', { status: 403 });
  }
}

export function getDataParams(
  dataset: Dataset,
  input: DataFilters = {},
  format: 'csv' | 'json' | 'xlsx' = 'json'
) {
  const rules = DATASETS[dataset];
  const allowed = rules.filters.split(' ');
  const params: Record<string, boolean | number | string> = {
    format,
    limit: format === 'json' ? 100 : 10_000
  };

  for (const [name, raw] of Object.entries(input)) {
    if (raw === undefined || raw === null || raw === '') continue;
    if (name === 'limit') {
      const parsed = positiveInteger.safeParse(raw);
      if (!parsed.success || (format !== 'json' && parsed.data > 10_000))
        throw new ApiError('Некорректный лимит строк.');
      params.limit = parsed.data;
      continue;
    }
    if (name === 'group_by') {
      if (typeof raw !== 'string' || !rules.groups.split(' ').includes(raw))
        throw new ApiError('Недопустимая группировка.');
      params.group_by = raw;
      continue;
    }
    if (!allowed.includes(name)) throw new ApiError('Неизвестный фильтр таблицы.');
    if (name === 'recognized' || name === 'in_bpo') {
      const parsed = booleanFilter.safeParse(raw);
      if (!parsed.success) throw new ApiError('Булевый фильтр должен быть true или false.');
      params[name] = parsed.data;
      continue;
    }
    if (typeof raw !== 'string') throw new ApiError('Некорректное значение фильтра.');
    const value = raw.trim();
    if (!value) continue;
    if (name.endsWith('_id') && !z.uuid().safeParse(value).success)
      throw new ApiError('Некорректный идентификатор фильтра.');
    params[name] = value;
  }
  return params;
}

const dataTableSchema = z.object({
  dataset: z.string().optional(),
  group_by: z.string().optional(),
  columns: z.array(z.string()).min(1),
  rows: z.array(z.array(z.unknown())),
  total: z.number().int().nonnegative(),
  shown: z.number().int().nonnegative()
});

export function projectDataTable(
  dataset: Dataset,
  role: DemoRole,
  payload: unknown,
  groupBy?: string
) {
  assertDatasetAccess(dataset, role);
  const parsed = dataTableSchema.safeParse(payload);
  if (!parsed.success) throw new ApiError('Сервер вернул некорректную таблицу.');
  const table = parsed.data;
  if (
    (table.dataset !== undefined && table.dataset !== dataset) ||
    (table.group_by !== undefined && table.group_by !== groupBy) ||
    table.shown !== table.rows.length ||
    table.total < table.shown ||
    new Set(table.columns).size !== table.columns.length ||
    table.rows.some((row) => row.length !== table.columns.length)
  )
    throw new ApiError('Колонки, строки или счётчики таблицы не согласованы.');

  if (groupBy && !DATASETS[dataset].groups.split(' ').includes(groupBy))
    throw new ApiError('Недопустимая группировка.');
  const permitted = groupBy ? [groupBy, 'count'] : DATASETS[dataset].columns.split(' ');
  const columns = permitted.filter(
    (name) => table.columns.includes(name) && (role === 'ADMIN' || !ADMIN_COLUMNS.has(name))
  );
  if (!columns.length || (groupBy && columns.length !== 2))
    throw new ApiError('В таблице отсутствуют ожидаемые колонки.');
  const indices = columns.map((name) => table.columns.indexOf(name));
  return {
    columns,
    rows: table.rows.map((row) => indices.map((index) => row[index])),
    shown: table.shown,
    total: table.total
  };
}

export function formatDataValue(value: unknown): string {
  if (value === null || value === undefined) return 'Не указано';
  if (typeof value === 'boolean') return value ? 'Да' : 'Нет';
  if (typeof value === 'string') return value || 'Не указано';
  if (typeof value === 'number' && Number.isFinite(value)) return String(value);
  return 'Неизвестное значение';
}

export async function fetchDataTable(
  dataset: Dataset,
  role: DemoRole,
  filters: DataFilters,
  config: AxiosRequestConfig
) {
  assertDatasetAccess(dataset, role);
  const params = getDataParams(dataset, filters);
  const response = await getDataByDataset({
    path: { dataset },
    config: { ...config, params, responseType: 'json' }
  });
  if (response.status !== 200 || !dataTableSchema.safeParse(response.data).success)
    throw await readApiError(response.data, response.status);
  const table = projectDataTable(
    dataset,
    role,
    response.data,
    typeof params.group_by === 'string' ? params.group_by : undefined
  );
  if (
    dataset === 'findings' &&
    role === 'ADMIN' &&
    !params.group_by &&
    table.rows.length &&
    !table.columns.includes('finding_id')
  )
    throw new ApiError('Сервер не вернул ID нарушений. Обработка недоступна.');
  if (role === 'USER' && typeof filters.serial_number === 'string' && !params.group_by) {
    const serialIndex = table.columns.indexOf('serial_number');
    if (table.rows.some((row) => serialIndex < 0 || row[serialIndex] !== filters.serial_number))
      throw new ApiError('Сервер вернул записи другого устройства.');
  }
  return table;
}

export async function exportDataTable(
  dataset: Dataset,
  role: DemoRole,
  filters: DataFilters,
  format: 'csv' | 'xlsx',
  config: AxiosRequestConfig
) {
  assertDatasetAccess(dataset, role);
  const params = getDataParams(dataset, filters, format);
  const response = await getDataByDataset({
    path: { dataset },
    config: { ...config, params, responseType: 'blob' }
  });
  if (config.signal?.aborted) throw new CanceledError();
  await downloadFile(response, `${dataset}.${format}`);
}
