import { useQuery } from '@tanstack/react-query';
import { getRouteApi, Link } from '@tanstack/react-router';

import { useDemoAuth } from '@/app/auth/context';
import { AccessDenied, NotFoundState } from '@/components/shared/route-states';
import { EmptyState, ErrorState, LoadingState } from '@/components/shared/states';
import { Button } from '@/components/ui/button';
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

import { DeviceEditor } from './device-editor';
import { InventoryTable } from './inventory-table';
import { DEVICE_STATUSES, DEVICE_TYPES, INVENTORY_RESULTS } from './model';
import { deviceDetailOptions } from './queries';

const route = getRouteApi('/_authenticated/devices_/$deviceId');

export function DevicePage() {
  const { deviceId } = route.useParams();
  const { auth } = useDemoAuth();
  const session = auth.requireReady();
  const query = useQuery(deviceDetailOptions(auth, deviceId));
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
  const device = query.data;
  const details = [
    ['Серийный номер', device.serial_number],
    ['Тип устройства', device.device_type && DEVICE_TYPES[device.device_type]],
    ['Статус', device.status && DEVICE_STATUSES[device.status]],
    ['Имя компьютера', device.hostname],
    ['Операционная система', device.operating_system],
    ['Последний снимок', device.last_inventory_at],
    ['Установок', device.installations_count],
    ['Открытых нарушений', device.open_findings_count]
  ];
  const filters = { serial_number: device.serial_number };
  return (
    <section aria-label='Карточка устройства' className='flex min-w-0 flex-col gap-6'>
      {session.role === 'ADMIN' && (
        <Button
          className='self-start'
          nativeButton={false}
          render={<Link search={{}} to='/devices' />}
          variant='outline'
        >
          Все устройства
        </Button>
      )}
      <h1 className='text-2xl font-semibold wrap-break-word'>Устройство {device.serial_number}</h1>
      {query.isFetching && <p role='status'>Обновление карточки…</p>}
      {query.isError && <ErrorState error={query.error} onRetry={() => void query.refetch()} />}
      <dl className='grid min-w-0 gap-4 sm:grid-cols-2'>
        {details.map(([label, value]) => (
          <div key={label} className='min-w-0'>
            <dt className='text-sm text-muted-foreground'>{label}</dt>
            <dd className='wrap-break-word'>{formatDataValue(value)}</dd>
          </div>
        ))}
      </dl>
      {session.role === 'ADMIN' && <DeviceEditor key={device.id} device={device} />}
      <h2 className='text-lg font-semibold'>Установленное ПО</h2>
      <InventoryTable
        key={`installations-${deviceId}`}
        caption='Установки устройства'
        dataset='installations'
        filters={filters}
      />
      <h2 className='text-lg font-semibold'>Нарушения</h2>
      <InventoryTable
        key={`findings-${deviceId}`}
        caption='Нарушения устройства'
        dataset='findings'
        filters={filters}
      />
      {session.role === 'ADMIN' && (
        <>
          <h2 className='text-lg font-semibold'>История снимков</h2>
          {device.imports === undefined ? (
            <p>История не предоставлена сервером.</p>
          ) : !device.imports.length ? (
            <EmptyState title='Снимков пока нет' />
          ) : (
            <Table>
              <TableCaption>История снимков устройства</TableCaption>
              <TableHeader>
                <TableRow>
                  <TableHead scope='col'>ID снимка</TableHead>
                  <TableHead scope='col'>Дата выгрузки</TableHead>
                  <TableHead scope='col'>Результат</TableHead>
                  <TableHead scope='col'>Обработан</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {device.imports.map((item) => (
                  <TableRow key={item.inventory_id}>
                    <TableCell>{item.inventory_id}</TableCell>
                    <TableCell>{formatDataValue(item.collected_at)}</TableCell>
                    <TableCell>
                      {item.result ? INVENTORY_RESULTS[item.result] : 'Не указано'}
                    </TableCell>
                    <TableCell>{formatDataValue(item.processed_at)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </>
      )}
    </section>
  );
}
