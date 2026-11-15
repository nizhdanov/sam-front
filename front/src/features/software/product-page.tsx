import { useQuery } from '@tanstack/react-query';
import { getRouteApi, Link } from '@tanstack/react-router';
import { cn } from 'cn';

import { useDemoAuth } from '@/app/auth/context';
import { AccessDenied, NotFoundState } from '@/components/shared/route-states';
import { ErrorState, LoadingState } from '@/components/shared/states';
import { Button } from '@/components/ui/button';
import { formatDataValue } from '@/utils/api/data';
import { ApiError } from '@/utils/api/errors';

import { COST_MODELS, LICENSE_TYPES, PRODUCT_STATUSES } from './model';
import { softwareDetailOptions } from './queries';

const route = getRouteApi('/_authenticated/software_/$productId');

export function ProductPage() {
  const { productId } = route.useParams();
  const { auth, snapshot } = useDemoAuth();
  const query = useQuery(softwareDetailOptions(auth, productId));
  if (query.isPending) return <LoadingState />;
  if (query.isError) {
    if (query.error instanceof ApiError && query.error.status === 404) return <NotFoundState />;
    if (query.error instanceof ApiError && query.error.status === 403) return <AccessDenied />;
    return (
      <ErrorState
        error={query.error}
        onRetry={() => {
          void query.refetch();
        }}
      />
    );
  }
  const product = query.data;
  const details: [string, unknown][] = [
    ['Версия', product.current_version],
    ['Издатель', product.publisher],
    ['Редакция', product.edition],
    ['Назначение', product.purpose],
    ['Раздел', product.section],
    ['Класс', product.class],
    ['В перечне БПО', product.in_bpo],
    ['Соглашение', product.agreement_name],
    ['Тип лицензии', product.license_type && LICENSE_TYPES[product.license_type]],
    ['Модель оплаты', product.cost_model && COST_MODELS[product.cost_model]],
    ['Комментарий к статусу', product.status_comment]
  ];
  if (snapshot.role === 'ADMIN')
    details.push(
      ['Куплено мест', product.purchased_seats],
      ['Активные установки', product.active_installations],
      ['Устройств', product.devices],
      ['Дата архивации', product.archived_at]
    );
  return (
    <section aria-label='Карточка ПО' className='flex min-w-0 flex-col gap-6'>
      <Button
        className='self-start'
        nativeButton={false}
        render={<Link search={{}} to='/software' />}
        variant='outline'
      >
        Каталог ПО
      </Button>
      <h1 className='wrap-break-word'>{formatDataValue(product.canonical_name)}</h1>
      {query.isFetching && <p role='status'>Обновление карточки…</p>}
      <dl className='grid min-w-0 gap-4 sm:grid-cols-2'>
        <div className='min-w-0'>
          <dt className='text-sm text-muted-foreground'>Статус</dt>
          <dd
            className={cn('wrap-break-word', {
              'text-success': product.status === 'ACTUAL',
              'text-brand-gold-foreground': product.status === 'OUTDATED',
              'text-destructive': product.status === 'FORBIDDEN'
            })}
          >
            {product.status ? PRODUCT_STATUSES[product.status] : 'Не указано'}
          </dd>
        </div>
        {details.map(([label, value]) => (
          <div key={label} className='min-w-0'>
            <dt className='text-sm text-muted-foreground'>{label}</dt>
            <dd className='wrap-break-word whitespace-pre-wrap'>{formatDataValue(value)}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}
