import { useQuery } from '@tanstack/react-query';
import { getRouteApi, Link } from '@tanstack/react-router';
import { cn } from 'cn';
import { useState } from 'react';

import { useDemoAuth } from '@/app/auth/context';
import { DataTable } from '@/components/shared/data-table';
import { FilterSelect } from '@/components/shared/filter-select';
import { ErrorState, LoadingState } from '@/components/shared/states';
import { Button } from '@/components/ui/button';
import { Field, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { fetchDataTable, formatDataValue } from '@/utils/api/data';

import type { ContractsSearch } from './model';

import {
  CONTRACT_STATUSES,
  contractsSearchSchema,
  getContractLabel,
  LICENSE_METRICS
} from './model';

const route = getRouteApi('/_authenticated/_admin/contracts');

function ContractFilters({ search }: { search: ContractsSearch }) {
  const navigate = route.useNavigate();
  const [invalidId, setInvalidId] = useState(false);
  return (
    <form
      aria-label='Фильтры договоров'
      onSubmit={(event) => {
        event.preventDefault();
        const parsed = contractsSearchSchema.safeParse(
          Object.fromEntries(new FormData(event.currentTarget))
        );
        setInvalidId(!parsed.success);
        if (parsed.success) void navigate({ search: parsed.data });
      }}
    >
      <FieldGroup className='grid gap-4 sm:grid-cols-2 xl:grid-cols-3'>
        <Field>
          <FieldLabel htmlFor='contract-product'>Продукт</FieldLabel>
          <Input defaultValue={search.product ?? ''} id='contract-product' name='product' />
        </Field>
        <Field data-invalid={invalidId}>
          <FieldLabel htmlFor='contract-product-id'>ID продукта (UUID)</FieldLabel>
          <Input
            aria-invalid={invalidId}
            defaultValue={search.product_id ?? ''}
            id='contract-product-id'
            name='product_id'
            pattern='[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}'
            onChange={() => setInvalidId(false)}
          />
          {invalidId && <FieldError>Введите корректный UUID продукта.</FieldError>}
        </Field>
        <Field>
          <FieldLabel htmlFor='contract-number'>Номер договора</FieldLabel>
          <Input
            defaultValue={search.contract_number ?? ''}
            id='contract-number'
            name='contract_number'
          />
        </Field>
        <FilterSelect
          defaultValue={search.metric}
          label='Метрика лицензии'
          name='metric'
          options={LICENSE_METRICS}
        />
        <FilterSelect
          defaultValue={search.status}
          label='Статус договора'
          name='status'
          options={CONTRACT_STATUSES}
        />
        <Field>
          <FieldLabel htmlFor='contract-limit'>Лимит строк</FieldLabel>
          <Input
            required
            defaultValue={search.limit}
            id='contract-limit'
            min={1}
            name='limit'
            step={1}
            type='number'
          />
        </Field>
      </FieldGroup>
      <div className='mt-4 flex flex-wrap gap-2'>
        <Button type='submit'>Применить фильтры</Button>
        <Button
          nativeButton={false}
          render={<Link search={{}} to='/contracts' />}
          variant='outline'
        >
          Сбросить
        </Button>
        <Button
          nativeButton={false}
          render={<Link search={{ status: 'EXPIRING' }} to='/contracts' />}
          variant='outline'
        >
          Истекающие подписки
        </Button>
      </div>
    </form>
  );
}

export function ContractsPage() {
  const search = route.useSearch();
  const navigate = route.useNavigate();
  const { auth } = useDemoAuth();
  const session = auth.requireReady();
  const query = useQuery({
    queryKey: ['contracts', session.role, session.epoch, search],
    queryFn: ({ signal }) => {
      auth.requestConfig(session);
      return fetchDataTable('contracts', session.role, search, { signal });
    }
  });
  const filtered = Object.entries(search).some(
    ([key, value]) => key !== 'limit' && value !== undefined
  );
  return (
    <section aria-label='Договоры' className='flex min-w-0 flex-col gap-6'>
      <h1 className='text-2xl font-semibold'>Договоры</h1>
      <ContractFilters key={JSON.stringify(search)} search={search} />
      {query.isPending ? (
        <LoadingState />
      ) : query.isError ? (
        <ErrorState
          error={query.error}
          onRetry={() => {
            void query.refetch();
          }}
        />
      ) : (
        <>
          {query.isFetching && <p role='status'>Обновление договоров…</p>}
          <DataTable
            renderCell={(value, column) => (
              <span
                className={cn({
                  'text-success': column === 'status' && value === 'ACTIVE',
                  'text-brand-gold-foreground': column === 'status' && value === 'EXPIRING',
                  'text-destructive': column === 'status' && value === 'EXPIRED'
                })}
              >
                {getContractLabel(column, value) ?? formatDataValue(value)}
              </span>
            )}
            caption='Реестр договоров'
            data={query.data}
            dataset='contracts'
            filters={search}
            onResetFilters={
              filtered
                ? () => {
                    void navigate({ search: {} });
                  }
                : undefined
            }
          />
        </>
      )}
    </section>
  );
}
