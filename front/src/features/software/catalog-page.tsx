import { useQuery } from '@tanstack/react-query';
import { getRouteApi, Link } from '@tanstack/react-router';
import { cn } from 'cn';

import { useDemoAuth } from '@/app/auth/context';
import { FilterSelect } from '@/components/shared/filter-select';
import { PageHeader } from '@/components/shared/page-header';
import { EmptyState, ErrorState, LoadingState } from '@/components/shared/states';
import { Button } from '@/components/ui/button';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
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

import type { SoftwareSearch } from './model';

import { COST_MODELS, LICENSE_TYPES, PRODUCT_STATUSES, softwareSearchSchema } from './model';
import { softwareListOptions } from './queries';

const route = getRouteApi('/_authenticated/software');

function CatalogFilters({ search }: { search: SoftwareSearch }) {
  const { snapshot } = useDemoAuth();
  const navigate = route.useNavigate();
  return (
    <form
      aria-label='Фильтры каталога'
      onSubmit={(event) => {
        event.preventDefault();
        const values = Object.fromEntries(new FormData(event.currentTarget));
        void navigate({ search: softwareSearchSchema.parse({ ...values, page: 1 }) });
      }}
    >
      <FieldGroup className='grid gap-4 sm:grid-cols-2 xl:grid-cols-3'>
        <Field>
          <FieldLabel htmlFor='software-search'>Поиск по названию</FieldLabel>
          <Input
            defaultValue={search.search ?? ''}
            id='software-search'
            name='search'
            type='search'
          />
        </Field>
        <Field>
          <FieldLabel htmlFor='software-section'>Раздел</FieldLabel>
          <Input defaultValue={search.section ?? ''} id='software-section' name='section' />
        </Field>
        <Field>
          <FieldLabel htmlFor='software-class'>Класс</FieldLabel>
          <Input defaultValue={search.class ?? ''} id='software-class' name='class' />
        </Field>
        <FilterSelect
          defaultValue={search.license_type}
          label='Тип лицензии'
          name='license_type'
          options={LICENSE_TYPES}
        />
        <FilterSelect
          defaultValue={search.cost_model}
          label='Модель оплаты'
          name='cost_model'
          options={COST_MODELS}
        />
        <FilterSelect
          defaultValue={search.status}
          label='Статус'
          name='status'
          options={PRODUCT_STATUSES}
        />
        <FilterSelect
          defaultValue={search.in_bpo?.toString()}
          label='Перечень БПО'
          name='in_bpo'
          options={{ true: 'В перечне БПО', false: 'Вне БПО' }}
        />
        {snapshot.role === 'ADMIN' && (
          <FilterSelect
            defaultValue={search.include_archived?.toString()}
            label='Архив'
            name='include_archived'
            options={{ true: 'Включая архив', false: 'Без архива' }}
          />
        )}
        <Field>
          <FieldLabel htmlFor='software-limit'>Строк на странице</FieldLabel>
          <Input
            required
            defaultValue={search.limit}
            id='software-limit'
            max={200}
            min={1}
            name='limit'
            step={1}
            type='number'
          />
        </Field>
      </FieldGroup>
      <div className='mt-4 flex flex-wrap gap-2'>
        <Button type='submit'>Применить фильтры</Button>
        <Button nativeButton={false} render={<Link search={{}} to='/software' />} variant='outline'>
          Сбросить
        </Button>
      </div>
    </form>
  );
}

export function CatalogPage() {
  const search = route.useSearch();
  const { auth, snapshot } = useDemoAuth();
  const query = useQuery(softwareListOptions(auth, search));
  const filtered = Object.entries(search).some(
    ([key, value]) => key !== 'page' && key !== 'limit' && value !== undefined
  );
  const reset = (
    <Button nativeButton={false} render={<Link search={{}} to='/software' />} variant='outline'>
      Сбросить фильтры
    </Button>
  );
  return (
    <section aria-label='Каталог ПО' className='flex min-w-0 flex-col gap-6'>
      <PageHeader title='Каталог ПО' />
      <CatalogFilters key={JSON.stringify(search)} search={search} />
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
          {query.isFetching && <p role='status'>Обновление каталога…</p>}
          {query.data.items.length === 0 ? (
            <EmptyState
              action={
                query.data.pagination.total > 0 ? (
                  <Button
                    nativeButton={false}
                    render={<Link search={{ ...search, page: 1 }} to='/software' />}
                    variant='outline'
                  >
                    На первую страницу
                  </Button>
                ) : filtered ? (
                  reset
                ) : undefined
              }
              description={
                query.data.pagination.total > 0
                  ? 'Откройте первую страницу списка.'
                  : filtered
                    ? 'Измените или сбросьте фильтры.'
                    : 'В доступном каталоге пока нет продуктов.'
              }
              title={
                query.data.pagination.total > 0
                  ? 'На этой странице нет продуктов'
                  : filtered
                    ? 'Ничего не найдено'
                    : 'Каталог пуст'
              }
            />
          ) : (
            <Table>
              <TableCaption>Каталог ПО: найдено {query.data.pagination.total}</TableCaption>
              <TableHeader>
                <TableRow>
                  {[
                    'Наименование',
                    'Версия',
                    'Издатель',
                    'Раздел',
                    'Класс',
                    'Тип лицензии',
                    'Модель оплаты',
                    'Перечень БПО',
                    'Статус',
                    ...(snapshot.role === 'ADMIN' ? ['Куплено мест'] : [])
                  ].map((label) => (
                    <TableHead key={label} scope='col'>
                      {label}
                    </TableHead>
                  ))}
                </TableRow>
              </TableHeader>
              <TableBody>
                {query.data.items.map((product) => (
                  <TableRow key={product.id}>
                    <TableCell>
                      <Link
                        className='text-primary underline underline-offset-4'
                        params={{ productId: product.id }}
                        to='/software/$productId'
                      >
                        {formatDataValue(product.canonical_name)}
                      </Link>
                    </TableCell>
                    <TableCell>{formatDataValue(product.current_version)}</TableCell>
                    <TableCell>{formatDataValue(product.publisher)}</TableCell>
                    <TableCell>{formatDataValue(product.section)}</TableCell>
                    <TableCell>{formatDataValue(product.class)}</TableCell>
                    <TableCell>
                      {product.license_type ? LICENSE_TYPES[product.license_type] : 'Не указано'}
                    </TableCell>
                    <TableCell>
                      {product.cost_model ? COST_MODELS[product.cost_model] : 'Не указано'}
                    </TableCell>
                    <TableCell>
                      {product.in_bpo === undefined
                        ? 'Не указано'
                        : product.in_bpo
                          ? 'БПО'
                          : 'Вне БПО'}
                    </TableCell>
                    <TableCell
                      className={cn({
                        'text-success': product.status === 'ACTUAL',
                        'text-brand-gold-foreground': product.status === 'OUTDATED',
                        'text-destructive': product.status === 'FORBIDDEN'
                      })}
                    >
                      {product.status ? PRODUCT_STATUSES[product.status] : 'Не указано'}
                    </TableCell>
                    {snapshot.role === 'ADMIN' && (
                      <TableCell>{formatDataValue(product.purchased_seats)}</TableCell>
                    )}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
          <nav aria-label='Страницы каталога' className='flex flex-wrap items-center gap-3'>
            <Button
              render={
                <Link search={{ ...search, page: Math.max(1, search.page - 1) }} to='/software' />
              }
              disabled={search.page <= 1}
              nativeButton={false}
              variant='outline'
            >
              Назад
            </Button>
            <p role='status'>
              Страница {query.data.pagination.page} из{' '}
              {Math.max(1, query.data.pagination.total_pages)} · Всего {query.data.pagination.total}
            </p>
            <Button
              disabled={search.page >= query.data.pagination.total_pages}
              nativeButton={false}
              render={<Link search={{ ...search, page: search.page + 1 }} to='/software' />}
              variant='outline'
            >
              Далее
            </Button>
          </nav>
        </>
      )}
    </section>
  );
}
