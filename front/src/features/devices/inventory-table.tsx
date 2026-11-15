import type { ReactNode } from 'react';

import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { cn } from 'cn';
import { z } from 'zod';

import type { DataFilters } from '@/utils/api/data';

import { useDemoAuth } from '@/app/auth/context';
import { DataTable } from '@/components/shared/data-table';
import { ErrorState, LoadingState } from '@/components/shared/states';
import { COST_MODELS, LICENSE_TYPES, PRODUCT_STATUSES } from '@/features/software/model';
import { formatDataValue } from '@/utils/api/data';

import { DEVICE_STATUSES, DEVICE_TYPES } from './model';
import { inventoryTableOptions } from './queries';

export function renderInventoryCell(value: unknown, column: string) {
  if (typeof value === 'string' && z.uuid().safeParse(value).success) {
    if (column === 'device_id')
      return (
        <Link className='underline' params={{ deviceId: value }} to='/devices/$deviceId'>
          {value}
        </Link>
      );
    if (column === 'product_id')
      return (
        <Link className='underline' params={{ productId: value }} to='/software/$productId'>
          {value}
        </Link>
      );
  }
  const labels: Record<string, Record<string, string>> = {
    device_type: DEVICE_TYPES,
    status: {
      ...DEVICE_STATUSES,
      ...PRODUCT_STATUSES,
      OPEN: 'Открыто',
      ACKNOWLEDGED: 'Обработано, не устранено',
      RESOLVED: 'Устранено'
    },
    license_type: LICENSE_TYPES,
    cost_model: COST_MODELS,
    finding_type: {
      FORBIDDEN_SOFTWARE: 'Запрещённое ПО',
      OUTDATED_SOFTWARE: 'Устаревшее ПО',
      LICENSE_DEFICIT: 'Дефицит лицензий'
    }
  };
  const options = Object.hasOwn(labels, column) ? labels[column] : undefined;
  const label =
    typeof value === 'string' && options && Object.hasOwn(options, value)
      ? options[value]
      : undefined;
  return (
    <span
      className={cn({
        'text-brand-gold-foreground': column === 'status' && value === 'NEW',
        'text-destructive': column === 'status' && value === 'FORBIDDEN'
      })}
    >
      {label ?? formatDataValue(value)}
    </span>
  );
}

export function InventoryTable({
  dataset,
  filters,
  caption,
  onResetFilters,
  renderCell = renderInventoryCell
}: {
  dataset: 'devices' | 'findings' | 'installations';
  filters: DataFilters;
  caption: string;
  onResetFilters?: () => void;
  renderCell?: (value: unknown, column: string) => ReactNode;
}) {
  const { auth } = useDemoAuth();
  const query = useQuery(inventoryTableOptions(auth, dataset, filters));
  if (!query.data)
    return query.isError ? (
      <ErrorState error={query.error} onRetry={() => void query.refetch()} />
    ) : (
      <LoadingState />
    );
  return (
    <div className='flex min-w-0 flex-col gap-3'>
      {query.isFetching && <p role='status'>Обновление данных…</p>}
      {query.isError && <ErrorState error={query.error} onRetry={() => void query.refetch()} />}
      <DataTable
        caption={caption}
        data={query.data}
        dataset={dataset}
        filters={filters}
        renderCell={renderCell}
        onResetFilters={onResetFilters}
      />
    </div>
  );
}
