import { useQuery } from '@tanstack/react-query';
import { getRouteApi, Link } from '@tanstack/react-router';

import type { Column } from '@/components/shared/simple-table';

import { useDemoAuth } from '@/app/auth/context';
import { SimpleTable } from '@/components/shared/simple-table';
import { EmptyState, ErrorState, LoadingState } from '@/components/shared/states';
import { Button } from '@/components/ui/button';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { COST_MODELS } from '@/features/software/model';

import type { CommercialBalance, FreeBalance } from './balance';

import { balanceOptions, CALCULATION_STATUSES, filterByRisk, METRICS } from './balance';

const route = getRouteApi('/_authenticated/_admin/compliance/balance');

function label(map: Record<string, string>, value: string | null | undefined) {
  if (!value) return '—';
  return map[value] ?? value;
}

function installationsLink(productId: string, text: string) {
  return (
    <Link
      className='font-medium underline-offset-4 hover:underline'
      search={{ product_id: productId }}
      to='/installations'
    >
      {text}
    </Link>
  );
}

const commercialColumns: Column<CommercialBalance>[] = [
  { header: 'Продукт', cell: (row) => installationsLink(row.product_id, row.product_name) },
  { header: 'Модель оплаты', cell: (row) => label(COST_MODELS, row.cost_model) },
  { header: 'Метрика', cell: (row) => label(METRICS, row.metric) },
  { header: 'Куплено', cell: (row) => row.purchased ?? 'Без лимита' },
  { header: 'Установлено', cell: (row) => row.consumed },
  {
    header: 'Дефицит',
    cell: (row) =>
      row.deficit > 0 ? (
        <span className='font-semibold text-destructive'>Не хватает {row.deficit}</span>
      ) : (
        '—'
      )
  },
  {
    header: 'Свободно',
    cell: (row) =>
      row.available === null ? (
        '—'
      ) : row.available > 0 ? (
        <span className='font-semibold text-amber-600 dark:text-amber-400'>
          Простаивает {row.available}
        </span>
      ) : (
        0
      )
  },
  {
    header: 'Расчёт',
    cell: (row) => (
      <div className='flex flex-col gap-1'>
        <span>{label(CALCULATION_STATUSES, row.calculation_status)}</span>
        {row.note && <span className='text-xs text-muted-foreground'>{row.note}</span>}
      </div>
    )
  }
];

const freeColumns: Column<FreeBalance>[] = [
  { header: 'Продукт', cell: (row) => installationsLink(row.product_id, row.product_name) },
  { header: 'Издатель', cell: (row) => row.publisher ?? '—' },
  { header: 'В перечне БПО', cell: (row) => (row.in_bpo ? 'Да' : 'Нет') },
  { header: 'Актуальная версия', cell: (row) => row.current_version ?? '—' },
  { header: 'Устройств', cell: (row) => row.devices },
  {
    header: 'С устаревшей версией',
    cell: (row) =>
      row.outdated_devices > 0 ? (
        <span className='font-semibold text-amber-600 dark:text-amber-400'>
          {row.outdated_devices}
        </span>
      ) : (
        0
      )
  }
];

export function BalancePage() {
  const search = route.useSearch();
  const navigate = route.useNavigate();
  const { auth } = useDemoAuth();
  const query = useQuery(balanceOptions(auth, search.view));

  let content;
  if (query.isPending) content = <LoadingState />;
  else if (query.isError)
    content = <ErrorState error={query.error} onRetry={() => void query.refetch()} />;
  else if (query.data.view === 'commercial') {
    const rows = filterByRisk(query.data.items, search.risk);
    const deficit = query.data.items.filter((item) => item.deficit > 0);
    content = (
      <div className='flex flex-col gap-4'>
        <p className='text-sm text-muted-foreground'>
          Купленные места берутся только из действующих договоров. Продуктов с дефицитом:{' '}
          {deficit.length}, мест не хватает: {deficit.reduce((sum, item) => sum + item.deficit, 0)}.
        </p>
        <div className='flex flex-wrap gap-2'>
          {(
            [
              [undefined, 'Все'],
              ['deficit', 'Дефицит'],
              ['available', 'Свободные места']
            ] as const
          ).map(([risk, text]) => (
            <Button
              key={text}
              aria-pressed={search.risk === risk}
              variant={search.risk === risk ? 'default' : 'outline'}
              onClick={() => void navigate({ search: { ...search, risk } })}
            >
              {text}
            </Button>
          ))}
        </div>
        {rows.length ? (
          <SimpleTable
            caption='Баланс коммерческих лицензий'
            columns={commercialColumns}
            rowKey={(row) => `${row.product_id}:${row.metric}`}
            rows={rows}
          />
        ) : (
          <EmptyState description='Нет продуктов, подходящих под выбранный фильтр.' />
        )}
      </div>
    );
  } else {
    content = query.data.items.length ? (
      <SimpleTable
        caption='Бесплатное ПО'
        columns={freeColumns}
        rowKey={(row) => row.product_id}
        rows={query.data.items}
      />
    ) : (
      <EmptyState description='Бесплатное ПО на устройствах не обнаружено.' />
    );
  }

  return (
    <section aria-label='Баланс лицензий' className='flex min-w-0 flex-col gap-6'>
      <h1 className='text-2xl font-semibold'>Баланс лицензий</h1>
      <Tabs
        value={search.view}
        onValueChange={(view) => void navigate({ search: { view: view as typeof search.view } })}
      >
        <TabsList aria-label='Вид баланса'>
          <TabsTrigger value='commercial'>Коммерческое ПО</TabsTrigger>
          <TabsTrigger value='free'>Бесплатное ПО</TabsTrigger>
        </TabsList>
      </Tabs>
      {content}
    </section>
  );
}
