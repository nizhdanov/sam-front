import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate } from '@tanstack/react-router';
import { useState } from 'react';

import type { Product } from '@/generated/api/types.gen';

import { useDemoAuth } from '@/app/auth/context';
import { DataTable } from '@/components/shared/data-table';
import { ErrorState, LoadingState } from '@/components/shared/states';
import { useAction } from '@/components/shared/use-action';
import { Button } from '@/components/ui/button';
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
  SheetTrigger
} from '@/components/ui/sheet';
import { Spinner } from '@/components/ui/spinner';
import { CONTRACT_STATUSES, getContractLabel } from '@/features/contracts/model';
import { fetchDataTable, formatDataValue } from '@/utils/api/data';

import type { ProductVersion } from './queries';

import { ProductEditor } from './product-editor';
import { addVersion, deleteProduct, saveProduct, updateVersion, versionsOptions } from './queries';

type AdminProduct = Product & { id: string };

const LIFECYCLE: Record<ProductVersion['lifecycle_status'], string> = {
  CURRENT: 'Актуальная',
  OUTDATED: 'Устаревшая',
  UNSUPPORTED: 'Не поддерживается',
  UNKNOWN: 'Не определено'
};

export function ProductActions({ product }: { product: AdminProduct }) {
  const { auth } = useDemoAuth();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const action = useAction();
  const [confirm, setConfirm] = useState(false);
  const linked = (product.active_installations ?? 0) > 0 || (product.purchased_seats ?? 0) > 0;

  return (
    <div className='flex flex-col items-end gap-2'>
      <div className='flex flex-wrap gap-2'>
        <ProductEditor product={product} />
        <Button
          disabled={action.pending}
          variant='outline'
          onClick={() =>
            void action.run(() =>
              saveProduct(auth, queryClient, { archived: !product.archived_at }, product.id)
            )
          }
        >
          {product.archived_at ? 'Вернуть из архива' : 'В архив'}
        </Button>
        <Sheet
          open={confirm}
          onOpenChange={(next, details) => {
            if (action.pending) details.cancel();
            else setConfirm(next);
          }}
        >
          <SheetTrigger
            render={<Button disabled={action.pending || linked} variant='destructive' />}
          >
            Удалить
          </SheetTrigger>
          <SheetContent>
            <SheetHeader>
              <SheetTitle>Удалить продукт?</SheetTitle>
              <SheetDescription>
                «{product.canonical_name}» исчезнет из каталога. Действие нельзя отменить.
              </SheetDescription>
            </SheetHeader>
            {action.error !== undefined && (
              <div className='px-4'>
                <ErrorState error={action.error} />
              </div>
            )}
            <SheetFooter>
              <Button
                disabled={action.pending}
                variant='destructive'
                onClick={async () => {
                  if (await action.run(() => deleteProduct(auth, queryClient, product.id)))
                    void navigate({ to: '/software', search: {} });
                }}
              >
                {action.pending && <Spinner data-icon='inline-start' />}
                Удалить продукт
              </Button>
            </SheetFooter>
          </SheetContent>
        </Sheet>
      </div>
      {linked && (
        <p className='text-sm text-muted-foreground'>
          Удаление недоступно: есть установки или договоры. Продукт можно отправить в архив.
        </p>
      )}
      {!confirm && action.error !== undefined && <ErrorState error={action.error} />}
    </div>
  );
}

export function ProductVersions({ productId }: { productId: string }) {
  const { auth } = useDemoAuth();
  const queryClient = useQueryClient();
  const query = useQuery(versionsOptions(auth, productId));
  const action = useAction();
  const [formKey, setFormKey] = useState(0);

  return (
    <section aria-labelledby='versions-title' className='flex flex-col gap-4'>
      <h2 className='text-lg font-semibold' id='versions-title'>
        Версии
      </h2>
      {query.isPending ? (
        <LoadingState />
      ) : query.isError ? (
        <ErrorState error={query.error} onRetry={() => void query.refetch()} />
      ) : query.data.length ? (
        <ul className='divide-y rounded-xl border bg-card'>
          {query.data.map((version) => (
            <li
              key={version.id}
              className='flex flex-wrap items-center justify-between gap-3 px-4 py-3'
            >
              <div className='flex flex-col'>
                <span className='font-medium'>{version.version}</span>
                <span className='text-xs text-muted-foreground'>
                  Выпуск: {formatDataValue(version.release_date?.slice(0, 10))}, поддержка до:{' '}
                  {formatDataValue(version.end_of_support?.slice(0, 10))}
                </span>
              </div>
              <select
                aria-label={`Статус версии ${version.version}`}
                className='h-9 rounded-3xl bg-input/50 px-3 text-sm'
                disabled={action.pending}
                value={version.lifecycle_status}
                onChange={(event) =>
                  void action.run(() =>
                    updateVersion(
                      auth,
                      queryClient,
                      productId,
                      version.id,
                      event.target.value as ProductVersion['lifecycle_status']
                    )
                  )
                }
              >
                {Object.entries(LIFECYCLE).map(([value, label]) => (
                  <option key={value} value={value}>
                    {label}
                  </option>
                ))}
              </select>
            </li>
          ))}
        </ul>
      ) : (
        <p className='text-sm text-muted-foreground'>Версии ещё не добавлены.</p>
      )}
      <form
        key={formKey}
        aria-label='Новая версия'
        className='flex flex-col gap-3'
        onSubmit={async (event) => {
          event.preventDefault();
          const data = new FormData(event.currentTarget);
          const version = String(data.get('version') ?? '').trim();
          if (!version) return;
          const ok = await action.run(() =>
            addVersion(auth, queryClient, productId, {
              version,
              lifecycle_status: data.get('lifecycle_status') as ProductVersion['lifecycle_status'],
              release_date: String(data.get('release_date') || '') || undefined,
              end_of_support: String(data.get('end_of_support') || '') || undefined
            })
          );
          if (ok) setFormKey((key) => key + 1);
        }}
      >
        <FieldGroup className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
          <Field>
            <FieldLabel htmlFor='version-number'>Номер версии</FieldLabel>
            <Input required id='version-number' name='version' placeholder='например, 24.08' />
          </Field>
          <Field>
            <FieldLabel htmlFor='version-lifecycle'>Статус</FieldLabel>
            <select
              className='h-9 rounded-3xl bg-input/50 px-3 text-sm'
              defaultValue='CURRENT'
              id='version-lifecycle'
              name='lifecycle_status'
            >
              {Object.entries(LIFECYCLE).map(([value, label]) => (
                <option key={value} value={value}>
                  {label}
                </option>
              ))}
            </select>
          </Field>
          <Field>
            <FieldLabel htmlFor='version-release'>Дата выпуска</FieldLabel>
            <Input id='version-release' name='release_date' type='date' />
          </Field>
          <Field>
            <FieldLabel htmlFor='version-eos'>Конец поддержки</FieldLabel>
            <Input id='version-eos' name='end_of_support' type='date' />
          </Field>
        </FieldGroup>
        <Button className='self-start' disabled={action.pending} type='submit' variant='outline'>
          {action.pending && <Spinner data-icon='inline-start' />}
          Добавить версию
        </Button>
      </form>
      {action.error !== undefined && <ErrorState error={action.error} />}
    </section>
  );
}

export function ProductContracts({ product }: { product: AdminProduct }) {
  const { auth } = useDemoAuth();
  const session = auth.requireReady();
  const filters = { product_id: product.id, limit: 100 };
  const query = useQuery({
    queryKey: ['contracts', session.role, session.epoch, filters],
    queryFn: ({ signal }) => {
      auth.requestConfig(session);
      return fetchDataTable('contracts', session.role, filters, { signal });
    }
  });

  return (
    <section aria-labelledby='contracts-title' className='flex flex-col gap-4'>
      <div className='flex flex-wrap items-baseline justify-between gap-2'>
        <h2 className='text-lg font-semibold' id='contracts-title'>
          Договоры и закупки
        </h2>
        <span className='text-sm'>
          Мест по действующим договорам:{' '}
          <strong className='text-base'>{formatDataValue(product.purchased_seats)}</strong>
        </span>
      </div>
      {query.isPending ? (
        <LoadingState />
      ) : query.isError ? (
        <ErrorState error={query.error} onRetry={() => void query.refetch()} />
      ) : (
        <DataTable
          renderCell={(value, column) =>
            column === 'status' && typeof value === 'string' && value in CONTRACT_STATUSES
              ? CONTRACT_STATUSES[value as keyof typeof CONTRACT_STATUSES]
              : (getContractLabel(column, value) ?? formatDataValue(value))
          }
          caption='Договоры продукта'
          data={query.data}
          dataset='contracts'
          filters={filters}
        />
      )}
      <Link
        className='self-start text-sm font-medium text-primary underline-offset-4 hover:underline'
        search={{ product_id: product.id }}
        to='/contracts'
      >
        Все договоры продукта в реестре
      </Link>
    </section>
  );
}
