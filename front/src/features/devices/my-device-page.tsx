import { useQuery } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';

import { useDemoAuth } from '@/app/auth/context';
import { EmptyState, ErrorState, LoadingState } from '@/components/shared/states';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { FindingsTable } from '@/features/compliance/findings-table';
import { myDeviceSearchSchema } from '@/features/compliance/model';
import { ApiError } from '@/utils/api/errors';

import { DeviceDetails } from './device-details';
import { InventoryTable } from './inventory-table';
import { myDeviceOptions } from './queries';

const route = getRouteApi('/_authenticated/me');
const TYPES = { FORBIDDEN_SOFTWARE: 'Запрещённое ПО', OUTDATED_SOFTWARE: 'Устаревшие версии' };

export function MyDevicePage() {
  const { auth } = useDemoAuth();
  const search = route.useSearch();
  const navigate = route.useNavigate();
  const query = useQuery(myDeviceOptions(auth));
  if (query.isError && query.error instanceof ApiError) {
    if (query.error.status === 403)
      return (
        <EmptyState
          description='В текущем доступе не задано собственное устройство или просмотр запрещён. Обратитесь к администратору.'
          title={<h1>Компьютер не привязан</h1>}
        />
      );
    if (query.error.status === 404)
      return (
        <EmptyState
          description='Данные привязанного устройства ещё не поступили в систему. Обратитесь к администратору.'
          title={<h1>Мой компьютер пока не найден</h1>}
        />
      );
  }
  if (!query.data)
    return query.isError ? (
      <ErrorState error={query.error} onRetry={() => void query.refetch()} />
    ) : (
      <LoadingState />
    );
  const device = query.data;
  const filters = { serial_number: device.serial_number };
  return (
    <section aria-label='Мой компьютер' className='flex min-w-0 flex-col gap-6'>
      <h1 className='text-2xl font-semibold'>Мой компьютер</h1>
      {query.isFetching && <p role='status'>Обновление карточки…</p>}
      {query.isError && <ErrorState error={query.error} onRetry={() => void query.refetch()} />}
      <DeviceDetails device={device} />
      <h2 className='text-lg font-semibold'>Установленное ПО</h2>
      <InventoryTable
        caption='Установки моего компьютера'
        dataset='installations'
        filters={filters}
      />
      <h2 className='text-lg font-semibold'>Риски и актуальные версии</h2>
      <Tabs
        className='min-w-0'
        value={search.finding_type}
        onValueChange={(value) => {
          const parsed = myDeviceSearchSchema.safeParse({ finding_type: value });
          if (parsed.success) void navigate({ search: parsed.data });
        }}
      >
        <TabsList
          aria-label='Риски моего компьютера'
          className='max-w-full flex-wrap justify-start group-data-horizontal/tabs:h-auto'
        >
          {Object.entries(TYPES).map(([value, label]) => (
            <TabsTrigger key={value} value={value}>
              {label}
            </TabsTrigger>
          ))}
        </TabsList>
        {Object.keys(TYPES).map((value) => (
          <TabsContent key={value} value={value}>
            <FindingsTable filters={{ ...filters, finding_type: value }} />
          </TabsContent>
        ))}
      </Tabs>
    </section>
  );
}
