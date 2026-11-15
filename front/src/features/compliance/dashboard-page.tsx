import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';

import { useDemoAuth } from '@/app/auth/context';
import { ErrorState, LoadingState } from '@/components/shared/states';
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle
} from '@/components/ui/card';
import { formatDataValue } from '@/utils/api/data';

import { dashboardOptions } from './queries';

export function DashboardPage() {
  const { auth } = useDemoAuth();
  const query = useQuery(dashboardOptions(auth));
  if (!query.data)
    return query.isError ? (
      <ErrorState error={query.error} onRetry={() => void query.refetch()} />
    ) : (
      <LoadingState />
    );
  const data = query.data;
  const cards = [
    {
      title: 'Всего устройств',
      value: formatDataValue(data.devices),
      description: 'Устройства, известные системе',
      link: (
        <Link search={{}} to='/devices'>
          Реестр устройств
        </Link>
      )
    },
    {
      title: 'Новые устройства',
      value: formatDataValue(data.new_devices),
      description: 'Поступили из выгрузок и ожидают проверки',
      link: (
        <Link search={{ status: 'NEW' }} to='/devices'>
          Новые устройства
        </Link>
      )
    },
    {
      title: 'Установки',
      value: formatDataValue(data.installations),
      description: 'Факты установки из инвентаризаций',
      link: (
        <Link search={{}} to='/installations'>
          Все установки
        </Link>
      )
    },
    {
      title: 'Распознанные установки',
      value: formatDataValue(data.recognized_installations),
      description: 'Соотнесены с каталогом',
      link: (
        <Link search={{ recognized: true }} to='/installations'>
          Распознанные установки
        </Link>
      )
    },
    {
      title: 'Доля распознанного ПО',
      value: data.recognized_percent === undefined ? 'Не указано' : `${data.recognized_percent}%`,
      description: 'Показатель, рассчитанный сервером',
      link: (
        <Link search={{ recognized: false }} to='/installations'>
          Нераспознанные установки
        </Link>
      )
    },
    {
      title: 'Ожидают проверки',
      value: formatDataValue(data.pending_review),
      description: 'Нераспознанное ПО',
      link: (
        <Link search={{ recognized: false }} to='/installations'>
          Установки для проверки
        </Link>
      )
    },
    {
      title: 'Дефицит лицензий',
      value: `${formatDataValue(data.license_deficit?.seats)} мест`,
      description: `Продуктов: ${formatDataValue(data.license_deficit?.products)}. Детализация баланса пока недоступна.`,
      link: (
        <Link search={{ finding_type: 'LICENSE_DEFICIT' }} to='/compliance/findings'>
          Нарушения покрытия лицензиями
        </Link>
      )
    },
    {
      title: 'Свободные лицензии',
      value: `${formatDataValue(data.idle_licenses?.seats)} мест`,
      description: `Продуктов: ${formatDataValue(data.idle_licenses?.products)}. Детализация баланса пока недоступна.`,
      link: undefined
    },
    {
      title: 'Запрещённое ПО',
      value: `${formatDataValue(data.forbidden_software?.installations)} установок`,
      description: `Устройств: ${formatDataValue(data.forbidden_software?.devices)}`,
      link: (
        <Link search={{ finding_type: 'FORBIDDEN_SOFTWARE' }} to='/compliance/findings'>
          Риски запрещённого ПО
        </Link>
      )
    },
    {
      title: 'Устаревшее ПО',
      value: `${formatDataValue(data.outdated_software?.installations)} установок`,
      description: `Устройств: ${formatDataValue(data.outdated_software?.devices)}`,
      link: (
        <Link search={{ finding_type: 'OUTDATED_SOFTWARE' }} to='/compliance/findings'>
          Риски устаревшего ПО
        </Link>
      )
    },
    {
      title: 'Истекающие договоры',
      value: formatDataValue(data.expiring_contracts),
      description: `Горизонт сервера: ${formatDataValue(data.expiring_within_days)} дней. Детализация этого периода пока недоступна.`,
      link: undefined
    }
  ];
  return (
    <section aria-label='Обзор' className='flex min-w-0 flex-col gap-6'>
      <h1 className='text-2xl font-semibold'>Обзор программных активов</h1>
      {query.isFetching && <p role='status'>Обновление показателей…</p>}
      {query.isError && <ErrorState error={query.error} onRetry={() => void query.refetch()} />}
      <div className='grid min-w-0 gap-4 sm:grid-cols-2 xl:grid-cols-3'>
        {cards.map((card) => (
          <Card key={card.title}>
            <CardHeader>
              <CardTitle>{card.title}</CardTitle>
              <CardDescription>{card.description}</CardDescription>
            </CardHeader>
            <CardContent>
              <p className='text-2xl font-semibold'>{card.value}</p>
            </CardContent>
            {card.link && <CardFooter className='underline'>{card.link}</CardFooter>}
          </Card>
        ))}
      </div>
      <p className='text-sm text-muted-foreground'>
        Свободные места не подтверждают фактический простой ПО. Денежные потери не рассчитываются.
        Отсутствующие показатели обозначены как «Не указано».
      </p>
    </section>
  );
}
