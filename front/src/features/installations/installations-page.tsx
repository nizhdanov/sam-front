import { getRouteApi, Link } from '@tanstack/react-router';
import { useState } from 'react';

import { FilterSelect } from '@/components/shared/filter-select';
import { PageHeader } from '@/components/shared/page-header';
import { Button } from '@/components/ui/button';
import { Field, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { InventoryTable, renderInventoryCell } from '@/features/devices/inventory-table';
import { DEVICE_TYPES } from '@/features/devices/model';
import { COST_MODELS, LICENSE_TYPES, PRODUCT_STATUSES } from '@/features/software/model';
import { formatDataValue } from '@/utils/api/data';

import { installationsSearchSchema } from './model';

const route = getRouteApi('/_authenticated/_admin/installations');
const TEXT_FILTERS = [
  'product',
  'product_id',
  'raw_name',
  'publisher',
  'version',
  'serial_number',
  'match_status'
] as const;
const LABELS = {
  product: 'Продукт',
  product_id: 'ID продукта (UUID)',
  raw_name: 'Имя из ОС',
  publisher: 'Издатель',
  version: 'Версия',
  serial_number: 'Серийный номер',
  match_status: 'Статус сопоставления'
};

export function InstallationsPage() {
  const search = route.useSearch();
  const navigate = route.useNavigate();
  const [invalid, setInvalid] = useState(false);
  const filtered = Object.entries(search).some(
    ([key, value]) => key !== 'limit' && value !== undefined
  );
  return (
    <section aria-label='Установки' className='flex min-w-0 flex-col gap-6'>
      <PageHeader description='Факты установки ПО на устройствах из выгрузок.' title='Установки' />
      <form
        key={JSON.stringify(search)}
        aria-label='Фильтры установок'
        onSubmit={(event) => {
          event.preventDefault();
          const parsed = installationsSearchSchema.safeParse(
            Object.fromEntries(new FormData(event.currentTarget))
          );
          setInvalid(!parsed.success);
          if (parsed.success) void navigate({ search: parsed.data });
        }}
      >
        <FieldGroup className='grid gap-4 sm:grid-cols-2 xl:grid-cols-3'>
          {TEXT_FILTERS.map((name) => (
            <Field key={name} data-invalid={name === 'product_id' && invalid}>
              <FieldLabel htmlFor={`installation-filter-${name}`}>{LABELS[name]}</FieldLabel>
              <Input
                aria-invalid={name === 'product_id' && invalid}
                defaultValue={search[name] ?? ''}
                id={`installation-filter-${name}`}
                name={name}
                onChange={() => setInvalid(false)}
              />
              {name === 'product_id' && invalid && (
                <FieldError>Проверьте UUID продукта и параметры фильтра.</FieldError>
              )}
            </Field>
          ))}
          <FilterSelect
            defaultValue={search.device_type}
            label='Тип устройства'
            name='device_type'
            options={DEVICE_TYPES}
          />
          <FilterSelect
            defaultValue={search.recognized === undefined ? '' : String(search.recognized)}
            label='Распознавание'
            name='recognized'
            options={{ true: 'Соотнесено с каталогом', false: 'Не распознано' }}
          />
          <FilterSelect
            defaultValue={search.status}
            label='Статус продукта'
            name='status'
            options={PRODUCT_STATUSES}
          />
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
            defaultValue={search.in_bpo === undefined ? '' : String(search.in_bpo)}
            label='В перечне БПО'
            name='in_bpo'
            options={{ true: 'Да', false: 'Нет' }}
          />
          <FilterSelect
            defaultValue={search.group_by}
            label='Группировка'
            name='group_by'
            options={{ product: 'По продукту', serial_number: 'По устройству' }}
          />
          <Field>
            <FieldLabel htmlFor='installations-limit'>Лимит строк</FieldLabel>
            <Input
              required
              defaultValue={search.limit}
              id='installations-limit'
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
            render={<Link search={{}} to='/installations' />}
            variant='outline'
          >
            Сбросить
          </Button>
        </div>
      </form>
      {search.group_by && (
        <p className='text-muted-foreground'>
          Количество в группе — число устройств. Устройства разных продуктов могут совпадать; суммы
          групп не означают размер парка. Выберите группу для просмотра исходных установок.
        </p>
      )}
      <InventoryTable
        renderCell={(value, column) => {
          if (column === search.group_by && typeof value === 'string' && value)
            return (
              <Link
                search={installationsSearchSchema.parse({
                  ...search,
                  group_by: undefined,
                  [column]: value
                })}
                className='underline'
                to='/installations'
              >
                {value}
              </Link>
            );
          if (column === 'product' && search.group_by === 'product' && value == null)
            return (
              <Link
                className='underline'
                search={{ ...search, group_by: undefined, recognized: false }}
                to='/installations'
              >
                Не распознано
              </Link>
            );
          if (column === 'count') return formatDataValue(value);
          return renderInventoryCell(value, column);
        }}
        caption={search.group_by ? 'Группы установок (число устройств)' : 'Реестр установок'}
        dataset='installations'
        filters={search}
        onResetFilters={filtered ? () => void navigate({ search: {} }) : undefined}
      />
    </section>
  );
}
