import { useQuery, useQueryClient } from '@tanstack/react-query';
import { getRouteApi, Link } from '@tanstack/react-router';
import { useEffect, useState } from 'react';

import type { Import } from '@/generated/api/types.gen';

import { useDemoAuth } from '@/app/auth/context';
import { AccessDenied, NotFoundState } from '@/components/shared/route-states';
import { ErrorState, LoadingState, SuccessState } from '@/components/shared/states';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
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
import { formatDataValue } from '@/utils/api/data';
import { ApiError } from '@/utils/api/errors';

import { canRetryImport, IMPORT_STATUSES, isImportTerminal } from './model';
import {
  importDetailOptions,
  importPreviewOptions,
  refreshInventory,
  runImportAction
} from './queries';

const route = getRouteApi('/_authenticated/_admin/imports_/$importId');
const COUNTERS: [keyof Import, string][] = [
  ['total_rows', 'Всего строк'],
  ['loaded_rows', 'Загружено'],
  ['duplicate_rows', 'Дубли'],
  ['excluded_rows', 'Исключено (не ПО)'],
  ['error_rows', 'Ошибок строк'],
  ['total_devices', 'Всего устройств'],
  ['processed_devices', 'Обработано устройств'],
  ['failed_devices', 'Неуспешных снимков'],
  ['pending_devices', 'Устройств в очереди'],
  ['progress_percent', 'Прогресс, %'],
  ['collected_at', 'Дата выгрузки'],
  ['created_at', 'Загружен'],
  ['completed_at', 'Завершён']
];

export function ImportPage() {
  const { importId } = route.useParams();
  const { auth } = useDemoAuth();
  const session = auth.requireReady();
  const queryClient = useQueryClient();
  const query = useQuery(importDetailOptions(auth, importId));
  const status = query.data?.status;
  const preview = useQuery({
    ...importPreviewOptions(auth, importId),
    enabled: status === 'AWAITING_CONFIRMATION'
  });
  const [pending, setPending] = useState<string | null>(null);
  const [error, setError] = useState<unknown>();

  useEffect(() => {
    if (status && isImportTerminal(status) && auth.isCurrent(session.epoch)) {
      void refreshInventory(queryClient);
      void queryClient.invalidateQueries({
        queryKey: ['imports', session.role, session.epoch, 'list']
      });
    }
  }, [auth, importId, queryClient, session.epoch, session.role, status]);

  async function handleAction(action: 'cancel' | 'confirm' | 'retry') {
    if (pending || !auth.isCurrent(session.epoch) || !query.data) return;
    if (action === 'retry' ? !canRetryImport(query.data) : status !== 'AWAITING_CONFIRMATION')
      return;
    setPending(action);
    setError(undefined);
    try {
      await runImportAction(auth, queryClient, importId, action);
    } catch (cause) {
      if (auth.isCurrent(session.epoch)) setError(cause);
    } finally {
      if (auth.isCurrent(session.epoch)) setPending(null);
    }
  }

  if (query.isError && query.error instanceof ApiError) {
    if (query.error.status === 404) return <NotFoundState />;
    if (query.error.status === 403) return <AccessDenied />;
  }
  if (!query.data)
    return query.isError ? (
      <ErrorState error={query.error} onRetry={() => void query.refetch()} />
    ) : (
      <LoadingState />
    );
  const item = query.data;
  return (
    <section aria-label='Карточка импорта' className='flex min-w-0 flex-col gap-6'>
      <Button
        className='self-start'
        nativeButton={false}
        render={<Link search={{}} to='/imports' />}
        variant='outline'
      >
        История импортов
      </Button>
      <h1 className='wrap-break-word'>{item.filename ?? 'Импорт CSV'}</h1>
      <p className='wrap-break-word'>ID импорта: {item.id}</p>
      <p role='status'>Статус: {IMPORT_STATUSES[item.status]}</p>
      {query.isFetching && <p role='status'>Обновление отчёта…</p>}
      {query.isError && <ErrorState error={query.error} onRetry={() => void query.refetch()} />}
      {error !== undefined && <ErrorState error={error} />}
      {pending && <Spinner aria-label='Выполнение действия…' />}
      {item.status === 'AWAITING_CONFIRMATION' && (
        <>
          <Alert>
            <AlertTitle>Обработка ещё не началась</AlertTitle>
            <AlertDescription>
              Проверьте первые строки и подтвердите импорт. KB-обновления и другие исключённые
              строки не попадут в учёт.
            </AlertDescription>
          </Alert>
          <h2 className='text-lg font-semibold'>Предпросмотр CSV</h2>
          {preview.isError ? (
            <ErrorState error={preview.error} onRetry={() => void preview.refetch()} />
          ) : !preview.data ? (
            <LoadingState />
          ) : (
            <>
              <p>
                Разделитель: «{preview.data.delimiter}». Колонки: {preview.data.columns.join(', ')}.
              </p>
              <Table>
                <TableCaption>
                  Первые {preview.data.rows.length} строк CSV; это не полный отчёт
                </TableCaption>
                <TableHeader>
                  <TableRow>
                    {[
                      'Строка',
                      'Серийный номер',
                      'Имя ПО',
                      'Издатель',
                      'Версия',
                      'Исключена',
                      'Ошибка'
                    ].map((label) => (
                      <TableHead key={label} scope='col'>
                        {label}
                      </TableHead>
                    ))}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {preview.data.rows.map((row) => (
                    <TableRow key={row.row}>
                      {Object.entries({
                        row: row.row,
                        serial_number: row.serial_number,
                        display_name: row.display_name,
                        publisher: row.publisher,
                        version: row.version,
                        excluded:
                          row.excluded === true
                            ? 'Да — не ПО'
                            : row.excluded === false
                              ? 'Нет'
                              : undefined,
                        error: row.error
                      }).map(([name, value]) => (
                        <TableCell key={name}>{formatDataValue(value)}</TableCell>
                      ))}
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </>
          )}
          <div className='flex flex-wrap gap-3'>
            <Button
              disabled={
                pending !== null ||
                query.isFetching ||
                query.isError ||
                !preview.data ||
                preview.isError
              }
              onClick={() => void handleAction('confirm')}
            >
              Подтвердить обработку
            </Button>
            <Button
              disabled={pending !== null || query.isFetching || query.isError}
              variant='outline'
              onClick={() => void handleAction('cancel')}
            >
              Отменить импорт
            </Button>
          </div>
        </>
      )}
      {item.status === 'COMPLETED' && <SuccessState>Импорт завершён.</SuccessState>}
      {(item.status === 'COMPLETED_WITH_ERRORS' || item.status === 'FAILED') && (
        <Alert variant='destructive'>
          <AlertTitle>{IMPORT_STATUSES[item.status]}</AlertTitle>
          <AlertDescription>
            {item.error_message ??
              'Часть данных не обработана. Проверьте счётчики и исходную выгрузку.'}
          </AlertDescription>
        </Alert>
      )}
      {item.status === 'CANCELLED' && <p>Импорт отменён, файл удалён сервером.</p>}
      <h2 className='text-lg font-semibold'>Отчёт</h2>
      <dl className='grid min-w-0 gap-4 sm:grid-cols-2 xl:grid-cols-3'>
        {COUNTERS.map(([name, label]) => (
          <div key={name} className='min-w-0'>
            <dt className='text-sm text-muted-foreground'>{label}</dt>
            <dd className='wrap-break-word'>{formatDataValue(item[name])}</dd>
          </div>
        ))}
      </dl>
      {(item.error_rows ?? 0) > 0 && (
        <Alert>
          <AlertTitle>Есть ошибки строк</AlertTitle>
          <AlertDescription>
            Всего ошибок: {item.error_rows}. Подробный список ошибок временно недоступен. Ошибки в
            предпросмотре относятся только к первым строкам CSV.
          </AlertDescription>
        </Alert>
      )}
      {canRetryImport(item) && (
        <div className='flex flex-col items-start gap-3'>
          <p>
            Повтор обрабатывает только неуспешные снимки. Он не загружает файл заново и не меняет
            дату исходной выгрузки.
          </p>
          <Button
            disabled={pending !== null || query.isFetching || query.isError}
            variant='outline'
            onClick={() => void handleAction('retry')}
          >
            Повторить неуспешные снимки
          </Button>
        </div>
      )}
      <Button
        className='self-start'
        disabled={query.isFetching}
        variant='outline'
        onClick={() => void query.refetch()}
      >
        Обновить отчёт
      </Button>
    </section>
  );
}
