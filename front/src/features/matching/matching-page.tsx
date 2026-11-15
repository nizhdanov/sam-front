import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { getRouteApi, Link } from '@tanstack/react-router';

import type { Column } from '@/components/shared/simple-table';

import { useDemoAuth } from '@/app/auth/context';
import { PageHeader } from '@/components/shared/page-header';
import { Pager } from '@/components/shared/pager';
import { SimpleTable } from '@/components/shared/simple-table';
import { EmptyState, ErrorState, LoadingState } from '@/components/shared/states';
import { Button } from '@/components/ui/button';
import { Field, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';

import type { MatchingRule, MatchingSearch, MatchingTask } from './model';

import { formatConfidence, MATCH_METHODS, REVIEW_STATUSES } from './model';
import { pendingOptions, rulesOptions } from './queries';
import { RuleActions } from './rule-actions';
import { SettingsForm } from './settings-form';
import { TaskReview } from './task-review';

const route = getRouteApi('/_authenticated/_admin/matching');

function raw(row: { raw_name: string; raw_publisher: string | null; raw_version: string | null }) {
  return (
    <div className='flex flex-col'>
      <span className='font-medium'>{row.raw_name}</span>
      <span className='text-xs text-muted-foreground'>
        {[row.raw_publisher, row.raw_version].filter(Boolean).join(' · ') || '—'}
      </span>
    </div>
  );
}

function taskColumns(status: MatchingSearch['status']): Column<MatchingTask>[] {
  return [
    { header: 'Имя из ОС', cell: raw },
    {
      header: 'Предложение',
      cell: (row) =>
        row.candidates.find((item) => item.id === row.suggested_product_id)?.canonical_name ??
        (row.candidates.length ? `Кандидатов: ${row.candidates.length}` : 'Нет кандидата')
    },
    { header: 'Уверенность', cell: (row) => formatConfidence(row.confidence_score) },
    {
      header: 'Метод',
      cell: (row) =>
        row.match_method ? (MATCH_METHODS[row.match_method] ?? row.match_method) : '—'
    },
    { header: 'Устройств', cell: (row) => row.affected_devices },
    status === 'PENDING'
      ? { header: 'Действие', cell: (row) => <TaskReview key={row.id} task={row} /> }
      : { header: 'Решение', cell: (row) => row.reviewed_by ?? '—' }
  ];
}

const ruleColumns: Column<MatchingRule>[] = [
  { header: 'Имя из ОС', cell: raw },
  {
    header: 'Продукт',
    cell: (row) => (
      <Link
        className='underline-offset-4 hover:underline'
        params={{ productId: row.product_id }}
        to='/software/$productId'
      >
        {row.product_name}
      </Link>
    )
  },
  { header: 'Метод', cell: (row) => MATCH_METHODS[row.match_method] ?? row.match_method },
  { header: 'Автор', cell: (row) => row.reviewed_by ?? 'Система' },
  { header: 'Действия', cell: (row) => <RuleActions key={row.id} rule={row} /> }
];

function Queue({ search }: { search: MatchingSearch }) {
  const navigate = route.useNavigate();
  const { auth } = useDemoAuth();
  const query = useQuery({ ...pendingOptions(auth, search), placeholderData: keepPreviousData });
  return (
    <div className='flex flex-col gap-4'>
      <div className='flex flex-wrap gap-2'>
        {(Object.keys(REVIEW_STATUSES) as MatchingSearch['status'][]).map((status) => (
          <Button
            key={status}
            aria-pressed={search.status === status}
            variant={search.status === status ? 'default' : 'outline'}
            onClick={() => void navigate({ search: { ...search, status, page: 1 } })}
          >
            {REVIEW_STATUSES[status]}
          </Button>
        ))}
      </div>
      {query.isPending ? (
        <LoadingState />
      ) : query.isError ? (
        <ErrorState error={query.error} onRetry={() => void query.refetch()} />
      ) : query.data.items.length ? (
        <>
          <SimpleTable
            caption='Очередь сопоставления'
            columns={taskColumns(search.status)}
            rowKey={(row) => row.id}
            rows={query.data.items}
          />
          <Pager
            page={query.data.pagination.page}
            total={query.data.pagination.total}
            totalPages={query.data.pagination.total_pages}
            onChange={(page) => void navigate({ search: { ...search, page } })}
          />
        </>
      ) : (
        <EmptyState description='Записей с выбранным статусом нет.' title='Очередь пуста' />
      )}
    </div>
  );
}

function Rules({ search }: { search: MatchingSearch }) {
  const navigate = route.useNavigate();
  const { auth } = useDemoAuth();
  const query = useQuery({ ...rulesOptions(auth, search), placeholderData: keepPreviousData });
  return (
    <div className='flex flex-col gap-4'>
      <form
        key={search.search}
        className='flex flex-wrap items-end gap-2'
        onSubmit={(event) => {
          event.preventDefault();
          const value = String(new FormData(event.currentTarget).get('search') ?? '').trim();
          void navigate({ search: { ...search, search: value || undefined, page: 1 } });
        }}
      >
        <Field className='w-72 max-w-full'>
          <FieldLabel htmlFor='rules-search'>Поиск по имени</FieldLabel>
          <Input defaultValue={search.search} id='rules-search' name='search' />
        </Field>
        <Button type='submit'>Найти</Button>
      </form>
      {query.isPending ? (
        <LoadingState />
      ) : query.isError ? (
        <ErrorState error={query.error} onRetry={() => void query.refetch()} />
      ) : query.data.items.length ? (
        <>
          <SimpleTable
            caption='Правила соответствия'
            columns={ruleColumns}
            rowKey={(row) => row.id}
            rows={query.data.items}
          />
          <Pager
            page={query.data.pagination.page}
            total={query.data.pagination.total}
            totalPages={query.data.pagination.total_pages}
            onChange={(page) => void navigate({ search: { ...search, page } })}
          />
        </>
      ) : (
        <EmptyState description='Правила появятся после подтверждения сопоставлений.' />
      )}
    </div>
  );
}

export function MatchingPage() {
  const search = route.useSearch();
  const navigate = route.useNavigate();
  return (
    <section aria-label='Smart Matching' className='flex min-w-0 flex-col gap-6'>
      <PageHeader
        description='ИИ предлагает продукт каталога для нераспознанных имён ПО. Подтверждённое решение становится правилом для следующих выгрузок.'
        title='Smart Matching'
      />
      <Tabs
        value={search.tab}
        onValueChange={(tab) =>
          void navigate({
            search: { tab: tab as MatchingSearch['tab'], status: 'PENDING', page: 1 }
          })
        }
      >
        <TabsList aria-label='Раздел сопоставления'>
          <TabsTrigger value='queue'>Очередь</TabsTrigger>
          <TabsTrigger value='rules'>Правила</TabsTrigger>
          <TabsTrigger value='settings'>Пороги</TabsTrigger>
        </TabsList>
        <TabsContent value='queue'>
          {search.tab === 'queue' && <Queue search={search} />}
        </TabsContent>
        <TabsContent value='rules'>
          {search.tab === 'rules' && <Rules search={search} />}
        </TabsContent>
        <TabsContent value='settings'>{search.tab === 'settings' && <SettingsForm />}</TabsContent>
      </Tabs>
    </section>
  );
}
