import { isCancel } from 'axios';
import { useState } from 'react';

import type { DataFilters, Dataset } from '@/utils/api/data';

import { useDemoAuth } from '@/app/auth/context';
import { Button } from '@/components/ui/button';
import { Spinner } from '@/components/ui/spinner';
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableHead,
  TableHeader,
  TableRow
} from '@/components/ui/table';
import { exportDataTable, formatDataValue, projectDataTable } from '@/utils/api/data';

import { EmptyState, ErrorState, SuccessState } from './states';

const COLUMN_LABELS: Record<string, string> = {
  product_id: 'ID продукта',
  device_id: 'ID устройства',
  contract_id: 'ID договора',
  finding_id: 'ID нарушения',
  serial_number: 'Серийный номер',
  device_type: 'Тип устройства',
  raw_name: 'Имя из ОС',
  product: 'Продукт',
  publisher: 'Издатель',
  version: 'Версия',
  current_version: 'Актуальная версия',
  match_status: 'Распознавание',
  status: 'Статус',
  license_type: 'Тип лицензии',
  cost_model: 'Модель оплаты',
  first_seen_at: 'Обнаружено впервые',
  last_seen_at: 'Последнее обнаружение',
  license_status: 'Покрытие лицензией',
  hostname: 'Имя компьютера',
  responsible: 'Ответственный',
  location: 'Расположение',
  last_inventory_at: 'Последний снимок',
  installations: 'Установки',
  open_findings: 'Открытые нарушения',
  section: 'Раздел',
  class: 'Класс',
  in_bpo: 'В перечне БПО',
  status_comment: 'Комментарий к статусу',
  purchased: 'Куплено мест',
  devices: 'Устройств',
  contract_number: 'Номер договора',
  metric: 'Метрика',
  quantity: 'Количество мест',
  valid_from: 'Действует с',
  expires_at: 'Действует до',
  finding_type: 'Тип нарушения',
  severity: 'Критичность',
  detected_at: 'Обнаружено',
  comment: 'Комментарий',
  count: 'Количество'
};

function DataExport({ dataset, filters }: { dataset: Dataset; filters: DataFilters }) {
  const { auth, snapshot } = useDemoAuth();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<unknown>();
  const [success, setSuccess] = useState(false);

  async function handleExport(format: 'csv' | 'xlsx') {
    if (pending || snapshot.status !== 'ready' || !auth.isCurrent(snapshot.epoch)) return;
    const session = auth.requireReady();
    setPending(true);
    setError(undefined);
    setSuccess(false);
    try {
      await exportDataTable(dataset, session.role, filters, format, auth.requestConfig(session));
      if (auth.isCurrent(session.epoch)) setSuccess(true);
    } catch (cause) {
      if (auth.isCurrent(session.epoch) && !isCancel(cause)) setError(cause);
    } finally {
      if (auth.isCurrent(session.epoch)) setPending(false);
    }
  }

  return (
    <div className='flex flex-col gap-3'>
      <div className='flex flex-wrap items-center gap-2'>
        <Button
          disabled={pending || snapshot.status !== 'ready'}
          variant='outline'
          onClick={() => {
            void handleExport('csv');
          }}
        >
          {pending && <Spinner aria-label='Экспорт…' data-icon='inline-start' />} CSV
        </Button>
        <Button
          disabled={pending || snapshot.status !== 'ready'}
          variant='outline'
          onClick={() => {
            void handleExport('xlsx');
          }}
        >
          XLSX
        </Button>
        <p className='text-sm text-muted-foreground'>
          Экспорт по текущим фильтрам, до 10 000 строк.
        </p>
      </div>
      {error !== undefined && <ErrorState error={error} />}
      {success && <SuccessState>Файл передан браузеру для скачивания.</SuccessState>}
    </div>
  );
}

export function DataTable({
  dataset,
  data,
  caption,
  filters = {},
  onResetFilters
}: {
  dataset: Dataset;
  data: unknown;
  caption: string;
  filters?: DataFilters;
  onResetFilters?: () => void;
}) {
  const { snapshot } = useDemoAuth();
  if (snapshot.status !== 'ready' || !snapshot.role) return null;
  let table;
  try {
    table = projectDataTable(
      dataset,
      snapshot.role,
      data,
      typeof filters.group_by === 'string' ? filters.group_by : undefined
    );
  } catch (error) {
    return <ErrorState error={error} />;
  }

  return (
    <div className='flex min-w-0 flex-col gap-4'>
      <DataExport key={snapshot.epoch} dataset={dataset} filters={filters} />
      {table.shown === 0 ? (
        <EmptyState
          action={
            onResetFilters && (
              <Button variant='outline' onClick={onResetFilters}>
                Сбросить фильтры
              </Button>
            )
          }
          description={
            onResetFilters
              ? 'Измените или сбросьте фильтры.'
              : 'Записи появятся после добавления данных.'
          }
          title={onResetFilters ? 'Ничего не найдено' : 'Данных пока нет'}
        />
      ) : (
        <Table>
          <TableCaption>
            {caption}: показано {table.shown} из {table.total}
          </TableCaption>
          <TableHeader>
            <TableRow>
              {table.columns.map((column) => (
                <TableHead key={column} scope='col'>
                  {COLUMN_LABELS[column] ?? column}
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {table.rows.map((row, index) => (
              // У read-only DataTable нет обязательного row ID, строки не редактируются и не сортируются на клиенте.
              // eslint-disable-next-line react/no-array-index-key
              <TableRow key={index}>
                {table.columns.map((column, columnIndex) => (
                  <TableCell key={column}>{formatDataValue(row[columnIndex])}</TableCell>
                ))}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
      <p className='text-sm text-muted-foreground' role='status'>
        Показано {table.shown} из {table.total}.
        {table.shown < table.total &&
          ' Список ограничен сервером. Уточните фильтры; экспорт также ограничен 10 000 строк.'}
      </p>
    </div>
  );
}
