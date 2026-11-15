import { getRouteApi, Link } from '@tanstack/react-router';

import { FilterSelect } from '@/components/shared/filter-select';
import { Button } from '@/components/ui/button';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';

import { InventoryTable } from './inventory-table';
import { DEVICE_STATUSES, DEVICE_TYPES, devicesSearchSchema } from './model';

const route = getRouteApi('/_authenticated/_admin/devices');
const TEXT_FILTERS = [
  { name: 'serial_number', label: 'Серийный номер' },
  { name: 'hostname', label: 'Имя компьютера' },
  { name: 'responsible', label: 'Ответственный' },
  { name: 'location', label: 'Подразделение / кабинет' }
] as const;

export function DevicesPage() {
  const search = route.useSearch();
  const navigate = route.useNavigate();
  const filtered = Object.entries(search).some(
    ([key, value]) => key !== 'limit' && value !== undefined
  );
  return (
    <section aria-label='Устройства' className='flex min-w-0 flex-col gap-6'>
      <h1 className='text-2xl font-semibold'>Устройства</h1>
      <p className='text-muted-foreground'>
        Устройства создаются из CSV. Серийный номер виртуальной машины сохраняется целиком, включая
        пробелы.
      </p>
      <form
        key={JSON.stringify(search)}
        aria-label='Фильтры устройств'
        onSubmit={(event) => {
          event.preventDefault();
          const parsed = devicesSearchSchema.parse(
            Object.fromEntries(new FormData(event.currentTarget))
          );
          void navigate({ search: parsed });
        }}
      >
        <FieldGroup className='grid gap-4 sm:grid-cols-2 xl:grid-cols-3'>
          {TEXT_FILTERS.map(({ name, label }) => (
            <Field key={name}>
              <FieldLabel htmlFor={`device-filter-${name}`}>{label}</FieldLabel>
              <Input defaultValue={search[name] ?? ''} id={`device-filter-${name}`} name={name} />
            </Field>
          ))}
          <FilterSelect
            defaultValue={search.device_type}
            label='Тип устройства'
            name='device_type'
            options={DEVICE_TYPES}
          />
          <FilterSelect
            defaultValue={search.status}
            label='Статус устройства'
            name='status'
            options={DEVICE_STATUSES}
          />
          <Field>
            <FieldLabel htmlFor='devices-limit'>Лимит строк</FieldLabel>
            <Input
              required
              defaultValue={search.limit}
              id='devices-limit'
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
            render={<Link search={{}} to='/devices' />}
            variant='outline'
          >
            Сбросить
          </Button>
        </div>
      </form>
      <InventoryTable
        caption='Реестр устройств'
        dataset='devices'
        filters={search}
        onResetFilters={filtered ? () => void navigate({ search: {} }) : undefined}
      />
    </section>
  );
}
