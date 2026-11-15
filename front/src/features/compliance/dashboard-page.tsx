import type { ReactNode } from 'react';

import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { cn } from 'cn';

import { useDemoAuth } from '@/app/auth/context';
import { PageHeader } from '@/components/shared/page-header';
import { ErrorState, LoadingState } from '@/components/shared/states';
import { formatDataValue } from '@/utils/api/data';

import { dashboardOptions } from './queries';

const number = new Intl.NumberFormat('ru-RU');

function count(value: number | undefined) {
  return value === undefined ? formatDataValue(value) : number.format(value);
}

function Risk({
  tone,
  title,
  value,
  unit,
  detail,
  link
}: {
  tone: 'critical' | 'warning';
  title: string;
  value: number | undefined;
  unit: string;
  detail: string;
  link: ReactNode;
}) {
  const active = (value ?? 0) > 0;
  return (
    <div
      className={cn(
        'relative flex min-w-0 flex-col gap-3 bg-card p-5 before:absolute before:inset-y-5 before:left-0 before:w-1 before:rounded-r-full',
        active && tone === 'critical' && 'before:bg-destructive',
        active && tone === 'warning' && 'before:bg-warning',
        !active && 'before:bg-success'
      )}
    >
      <span className='text-sm font-medium text-muted-foreground'>{title}</span>
      <span className='flex items-baseline gap-2'>
        <span
          className={cn(
            'text-4xl leading-none font-semibold tracking-tight',
            active && tone === 'critical' && 'text-destructive'
          )}
        >
          {count(value)}
        </span>
        <span className='text-sm text-muted-foreground'>{unit}</span>
      </span>
      <span className='text-sm'>{detail}</span>
      <span className='mt-auto text-sm font-medium text-primary underline-offset-4 [&_a]:hover:underline'>
        {link}
      </span>
    </div>
  );
}

function Stat({ label, value, link }: { label: string; value: string; link: ReactNode }) {
  return (
    <div className='flex min-w-0 items-baseline justify-between gap-4 py-3'>
      <span className='text-sm'>{link ?? label}</span>
      <span className='text-lg font-semibold'>{value}</span>
    </div>
  );
}

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
  const percent = data.recognized_percent;
  const link = 'text-primary underline-offset-4 hover:underline';

  return (
    <section aria-label='Обзор' className='flex min-w-0 flex-col gap-8'>
      <PageHeader
        actions={
          query.isFetching && (
            <span className='text-sm text-muted-foreground' role='status'>
              Обновление…
            </span>
          )
        }
        description='Риски и состояние парка по данным последних инвентаризаций.'
        title='Обзор программных активов'
      />
      {query.isError && <ErrorState error={query.error} onRetry={() => void query.refetch()} />}

      <section aria-labelledby='risks-title' className='flex flex-col gap-3'>
        <h2 className='text-lg font-semibold' id='risks-title'>
          Требует внимания
        </h2>
        <div className='grid gap-px overflow-hidden rounded-xl border bg-border sm:grid-cols-2 xl:grid-cols-4'>
          <Risk
            link={
              <Link search={{ view: 'commercial', risk: 'deficit' }} to='/compliance/balance'>
                Где не хватает лицензий
              </Link>
            }
            detail={`Продуктов: ${count(data.license_deficit?.products)}`}
            title='Дефицит лицензий'
            tone='critical'
            unit='мест'
            value={data.license_deficit?.seats}
          />
          <Risk
            link={
              <Link search={{ finding_type: 'FORBIDDEN_SOFTWARE' }} to='/compliance/findings'>
                Найти установки
              </Link>
            }
            detail={`На ${count(data.forbidden_software?.devices)} устройствах`}
            title='Запрещённое ПО'
            tone='critical'
            unit='установок'
            value={data.forbidden_software?.installations}
          />
          <Risk
            link={
              <Link search={{ finding_type: 'OUTDATED_SOFTWARE' }} to='/compliance/findings'>
                Что обновить
              </Link>
            }
            detail={`На ${count(data.outdated_software?.devices)} устройствах`}
            title='Устаревшее ПО'
            tone='warning'
            unit='установок'
            value={data.outdated_software?.installations}
          />
          <Risk
            link={
              <Link search={{ view: 'commercial', risk: 'available' }} to='/compliance/balance'>
                Какие лицензии свободны
              </Link>
            }
            detail={`Продуктов: ${count(data.idle_licenses?.products)}`}
            title='Простаивают'
            tone='warning'
            unit='мест'
            value={data.idle_licenses?.seats}
          />
        </div>
      </section>

      <div className='grid min-w-0 gap-8 lg:grid-cols-[3fr_2fr]'>
        <section
          aria-labelledby='recognition-title'
          className='flex flex-col gap-4 rounded-xl border bg-card p-5'
        >
          <div className='flex flex-wrap items-baseline justify-between gap-2'>
            <h2 className='text-lg font-semibold' id='recognition-title'>
              Распознавание ПО
            </h2>
            <span className='text-3xl font-semibold'>
              {percent === undefined ? 'Не указано' : `${percent.toLocaleString('ru-RU')}%`}
            </span>
          </div>
          <div
            aria-label='Доля распознанных установок'
            aria-valuemax={100}
            aria-valuemin={0}
            aria-valuenow={percent}
            className='h-2.5 overflow-hidden rounded-full bg-muted'
            role='progressbar'
          >
            <div className='h-full rounded-full bg-primary' style={{ width: `${percent ?? 0}%` }} />
          </div>
          <p className='text-sm text-muted-foreground'>
            {count(data.recognized_installations)} из {count(data.installations)} установок
            сопоставлены с каталогом. Остальные не участвуют в расчёте лицензий, пока их не
            разберут.
          </p>
          <div className='flex flex-wrap gap-x-6 gap-y-2 text-sm font-medium'>
            <Link
              className={link}
              search={{ tab: 'queue', status: 'PENDING', page: 1 }}
              to='/matching'
            >
              Разобрать очередь Smart Matching ({count(data.pending_review)})
            </Link>
            <Link className={link} search={{ recognized: false }} to='/installations'>
              Нераспознанные установки
            </Link>
          </div>
        </section>

        <section
          aria-labelledby='fleet-title'
          className='flex flex-col rounded-xl border bg-card px-5 py-4'
        >
          <h2 className='pb-1 text-lg font-semibold' id='fleet-title'>
            Парк
          </h2>
          <div className='flex flex-col divide-y'>
            <Stat
              link={
                <Link className={link} search={{}} to='/devices'>
                  Устройств в учёте
                </Link>
              }
              label='Устройств'
              value={count(data.devices)}
            />
            <Stat
              link={
                <Link className={link} search={{ status: 'NEW' }} to='/devices'>
                  Новых, ждут проверки
                </Link>
              }
              label='Новых'
              value={count(data.new_devices)}
            />
            <Stat
              link={
                <Link className={link} search={{}} to='/installations'>
                  Установок ПО
                </Link>
              }
              label='Установок'
              value={count(data.installations)}
            />
            <Stat
              label={`Договоров истекает за ${count(data.expiring_within_days)} дн.`}
              link={undefined}
              value={count(data.expiring_contracts)}
            />
          </div>
        </section>
      </div>
    </section>
  );
}
