import { getRouteApi, Link } from '@tanstack/react-router';
import { useState } from 'react';

import { FilterSelect } from '@/components/shared/filter-select';
import { PageHeader } from '@/components/shared/page-header';
import { Button } from '@/components/ui/button';
import { Field, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';

import { FindingsTable } from './findings-table';
import { FINDING_SEVERITIES, FINDING_STATUSES, FINDING_TYPES, findingsSearchSchema } from './model';

const route = getRouteApi('/_authenticated/_admin/compliance/findings');

export function FindingsPage() {
  const search = route.useSearch();
  const navigate = route.useNavigate();
  const [invalid, setInvalid] = useState(false);
  const filtered = Object.entries(search).some(
    ([key, value]) => key !== 'limit' && value !== undefined
  );
  return (
    <section aria-label='Нарушения' className='flex min-w-0 flex-col gap-6'>
      <PageHeader
        description='Запрещённое и устаревшее ПО, дефицит лицензий — с точностью до устройства.'
        title='Нарушения'
      />
      <Tabs
        className='min-w-0'
        value={search.finding_type ?? 'all'}
        onValueChange={(value) => {
          const parsed = findingsSearchSchema.safeParse({
            ...search,
            finding_type: value === 'all' ? undefined : value
          });
          if (parsed.success) void navigate({ search: parsed.data });
        }}
      >
        <TabsList
          aria-label='Категория риска'
          className='max-w-full flex-wrap justify-start group-data-horizontal/tabs:h-auto'
        >
          <TabsTrigger value='all'>Все риски</TabsTrigger>
          {Object.entries(FINDING_TYPES).map(([value, label]) => (
            <TabsTrigger key={value} value={value}>
              {label}
            </TabsTrigger>
          ))}
        </TabsList>
        <form
          key={JSON.stringify(search)}
          aria-label='Фильтры нарушений'
          onSubmit={(event) => {
            event.preventDefault();
            const parsed = findingsSearchSchema.safeParse({
              ...Object.fromEntries(new FormData(event.currentTarget)),
              finding_type: search.finding_type
            });
            setInvalid(!parsed.success);
            if (parsed.success) void navigate({ search: parsed.data });
          }}
        >
          <FieldGroup className='grid gap-4 sm:grid-cols-2 xl:grid-cols-3'>
            {Object.entries({
              product: 'Продукт',
              product_id: 'ID продукта (UUID)',
              serial_number: 'Серийный номер'
            }).map(([name, label]) => (
              <Field key={name} data-invalid={name === 'product_id' && invalid}>
                <FieldLabel htmlFor={`finding-filter-${name}`}>{label}</FieldLabel>
                <Input
                  defaultValue={
                    name === 'product'
                      ? search.product
                      : name === 'product_id'
                        ? search.product_id
                        : search.serial_number
                  }
                  aria-invalid={name === 'product_id' && invalid}
                  id={`finding-filter-${name}`}
                  name={name}
                  onChange={() => setInvalid(false)}
                />
                {name === 'product_id' && invalid && (
                  <FieldError>Проверьте UUID продукта и лимит строк.</FieldError>
                )}
              </Field>
            ))}
            <FilterSelect
              defaultValue={search.severity}
              label='Критичность'
              name='severity'
              options={FINDING_SEVERITIES}
            />
            <FilterSelect
              defaultValue={search.status}
              label='Статус нарушения'
              name='status'
              options={FINDING_STATUSES}
            />
            <Field>
              <FieldLabel htmlFor='findings-limit'>Лимит строк</FieldLabel>
              <Input
                required
                defaultValue={search.limit}
                id='findings-limit'
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
              render={<Link search={{}} to='/compliance/findings' />}
              variant='outline'
            >
              Сбросить
            </Button>
          </div>
        </form>
        {['all', ...Object.keys(FINDING_TYPES)].map((value) => (
          <TabsContent key={value} value={value}>
            <FindingsTable
              filters={search}
              onResetFilters={filtered ? () => void navigate({ search: {} }) : undefined}
            />
          </TabsContent>
        ))}
      </Tabs>
    </section>
  );
}
