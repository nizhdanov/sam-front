import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { getRouteApi, Link } from '@tanstack/react-router';

import type { Column } from '@/components/shared/simple-table';

import { useDemoAuth } from '@/app/auth/context';
import { PageHeader } from '@/components/shared/page-header';
import { Pager } from '@/components/shared/pager';
import { SimpleTable } from '@/components/shared/simple-table';
import { EmptyState, ErrorState, LoadingState } from '@/components/shared/states';
import { Button } from '@/components/ui/button';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { formatDataValue } from '@/utils/api/data';

import type { AuditEntry } from './model';

import { AUDIT_ACTIONS, auditOptions, auditSearchSchema } from './model';

const route = getRouteApi('/_authenticated/_admin/audit');
const dateFormat = new Intl.DateTimeFormat('ru-RU', { dateStyle: 'short', timeStyle: 'medium' });

const columns: Column<AuditEntry>[] = [
  { header: 'Дата', cell: (row) => dateFormat.format(new Date(row.created_at)) },
  { header: 'Автор', cell: (row) => row.actor },
  {
    header: 'Объект',
    cell: (row) => (
      <div className='flex flex-col'>
        <span>{row.entity}</span>
        {row.entity_id && (
          <span className='font-mono text-xs break-all text-muted-foreground'>{row.entity_id}</span>
        )}
      </div>
    )
  },
  { header: 'Действие', cell: (row) => AUDIT_ACTIONS[row.action] ?? row.action },
  { header: 'Описание', cell: (row) => row.summary },
  {
    header: 'Изменения',
    cell: (row) =>
      row.changes && Object.keys(row.changes).length ? (
        <details>
          <summary className='cursor-pointer'>Поля: {Object.keys(row.changes).length}</summary>
          <dl className='mt-2 grid gap-1 text-xs'>
            {Object.entries(row.changes).map(([key, value]) => (
              <div key={key}>
                <dt className='font-medium'>{key}</dt>
                <dd className='break-all whitespace-pre-wrap text-muted-foreground'>
                  {typeof value === 'object' && value !== null
                    ? JSON.stringify(value, null, 1)
                    : formatDataValue(value)}
                </dd>
              </div>
            ))}
          </dl>
        </details>
      ) : (
        '—'
      )
  }
];

export function AuditPage() {
  const search = route.useSearch();
  const navigate = route.useNavigate();
  const { auth } = useDemoAuth();
  const query = useQuery({ ...auditOptions(auth, search), placeholderData: keepPreviousData });

  return (
    <section aria-label='Журнал изменений' className='flex min-w-0 flex-col gap-6'>
      <PageHeader
        description='Кто, когда и что изменил в справочниках, договорах и решениях.'
        title='Журнал изменений'
      />
      <form
        key={JSON.stringify(search)}
        aria-label='Фильтры журнала'
        onSubmit={(event) => {
          event.preventDefault();
          const parsed = auditSearchSchema.safeParse(
            Object.fromEntries(new FormData(event.currentTarget))
          );
          if (parsed.success) void navigate({ search: { ...parsed.data, page: 1 } });
        }}
      >
        <FieldGroup className='grid gap-4 sm:grid-cols-2 xl:grid-cols-4'>
          <Field>
            <FieldLabel htmlFor='audit-actor'>Автор</FieldLabel>
            <Input defaultValue={search.actor} id='audit-actor' name='actor' />
          </Field>
          <Field>
            <FieldLabel htmlFor='audit-entity'>Тип объекта</FieldLabel>
            <Input defaultValue={search.entity} id='audit-entity' name='entity' />
          </Field>
          <Field>
            <FieldLabel htmlFor='audit-from'>С даты</FieldLabel>
            <Input defaultValue={search.from} id='audit-from' name='from' type='date' />
          </Field>
          <Field>
            <FieldLabel htmlFor='audit-to'>По дату</FieldLabel>
            <Input defaultValue={search.to} id='audit-to' name='to' type='date' />
          </Field>
        </FieldGroup>
        <div className='mt-4 flex flex-wrap gap-2'>
          <Button type='submit'>Применить</Button>
          <Button nativeButton={false} render={<Link search={{}} to='/audit' />} variant='outline'>
            Сбросить
          </Button>
        </div>
      </form>
      {query.isPending ? (
        <LoadingState />
      ) : query.isError ? (
        <ErrorState error={query.error} onRetry={() => void query.refetch()} />
      ) : query.data.items.length ? (
        <>
          <SimpleTable
            caption='События журнала'
            columns={columns}
            rowKey={(row) => String(row.id)}
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
        <EmptyState description='Событий за выбранный период нет.' />
      )}
    </section>
  );
}
