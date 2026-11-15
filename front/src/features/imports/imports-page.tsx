import { useQuery, useQueryClient } from '@tanstack/react-query';
import { getRouteApi, Link, useRouter } from '@tanstack/react-router';
import { useRef, useState } from 'react';

import { useDemoAuth } from '@/app/auth/context';
import { FormLeaveGuard } from '@/components/shared/form-leave-guard';
import { EmptyState, ErrorState, LoadingState } from '@/components/shared/states';
import { Button } from '@/components/ui/button';
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
  FieldSet
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Spinner } from '@/components/ui/spinner';
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

import { getCollectedAt, getCsvFileError, IMPORT_STATUSES } from './model';
import { importsListOptions, uploadCsv } from './queries';

const route = getRouteApi('/_authenticated/_admin/imports');

function CsvUpload() {
  const { auth } = useDemoAuth();
  const session = auth.requireReady();
  const queryClient = useQueryClient();
  const router = useRouter();
  const [pending, setPending] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [fileError, setFileError] = useState<string>();
  const [error, setError] = useState<unknown>();
  const completedRef = useRef(false);
  return (
    <form
      aria-label='Загрузка CSV'
      className='flex flex-col gap-4'
      onChange={() => {
        completedRef.current = false;
        setDirty(true);
        setFileError(undefined);
      }}
      onSubmit={async (event) => {
        event.preventDefault();
        if (pending || !auth.isCurrent(session.epoch)) return;
        const element = event.currentTarget;
        const data = new FormData(element);
        const value = data.get('file');
        const file = value instanceof File ? value : undefined;
        const validationError = getCsvFileError(file);
        setFileError(validationError);
        if (!file || validationError) return;
        const sourceLocation = router.state.location;
        setPending(true);
        setError(undefined);
        try {
          const result = await uploadCsv(
            auth,
            queryClient,
            file,
            getCollectedAt(String(data.get('collected_at') ?? ''))
          );
          if (!auth.isCurrent(session.epoch) || router.state.location !== sourceLocation) return;
          completedRef.current = true;
          setDirty(false);
          element.reset();
          void router.navigate({
            to: '/imports/$importId',
            params: { importId: result.import_id }
          });
        } catch (cause) {
          if (auth.isCurrent(session.epoch)) setError(cause);
        } finally {
          if (auth.isCurrent(session.epoch)) setPending(false);
        }
      }}
    >
      <h2 className='text-lg font-semibold'>Новая выгрузка</h2>
      <FormLeaveGuard
        when={() => auth.isCurrent(session.epoch) && (dirty || pending) && !completedRef.current}
      />
      <FieldSet disabled={pending}>
        <FieldGroup className='grid gap-4 sm:grid-cols-2'>
          <Field data-disabled={pending} data-invalid={!!fileError}>
            <FieldLabel htmlFor='csv-file'>CSV-файл</FieldLabel>
            <Input
              accept='.csv'
              aria-describedby='csv-description csv-error'
              aria-invalid={!!fileError}
              id='csv-file'
              name='file'
              type='file'
            />
            <FieldDescription id='csv-description'>
              UTF-8, разделитель «,» или «;», до 50 МБ. Обязательные заголовки:
              COMPUTERS.SERIAL_NUMBER и SOFTWARE.DISPLAYNAME.
            </FieldDescription>
            <FieldError id='csv-error'>{fileError}</FieldError>
          </Field>
          <Field data-disabled={pending}>
            <FieldLabel htmlFor='csv-collected-at'>Дата и время выгрузки</FieldLabel>
            <Input
              aria-describedby='csv-time-description'
              id='csv-collected-at'
              name='collected_at'
              type='datetime-local'
            />
            <FieldDescription id='csv-time-description'>
              Местное время. Если поле пустое, сервер использует время загрузки. Порядок снимков
              определяет сервер.
            </FieldDescription>
          </Field>
        </FieldGroup>
      </FieldSet>
      <p className='text-sm text-muted-foreground'>
        Сначала откроется предпросмотр. Обработка начнётся после отдельного подтверждения.
      </p>
      <Button className='self-start' disabled={pending} type='submit'>
        {pending && <Spinner data-icon='inline-start' />}
        {pending ? 'Загрузка…' : 'Загрузить для предпросмотра'}
      </Button>
      {error !== undefined && <ErrorState error={error} />}
    </form>
  );
}

export function ImportsPage() {
  const search = route.useSearch();
  const navigate = route.useNavigate();
  const { auth } = useDemoAuth();
  const query = useQuery(importsListOptions(auth, search));
  return (
    <section aria-label='Импорт CSV' className='flex min-w-0 flex-col gap-6'>
      <h1 className='text-2xl font-semibold'>Импорт CSV</h1>
      <CsvUpload />
      <h2 className='text-lg font-semibold'>История импортов</h2>
      <form
        key={search.limit}
        aria-label='Размер страницы истории'
        className='flex flex-wrap items-end gap-3'
        onSubmit={(event) => {
          event.preventDefault();
          const limit = Number(new FormData(event.currentTarget).get('limit'));
          void navigate({ search: { page: 1, limit } });
        }}
      >
        <Field>
          <FieldLabel htmlFor='imports-limit'>Импортов на странице</FieldLabel>
          <Input
            required
            defaultValue={search.limit}
            id='imports-limit'
            max={200}
            min={1}
            name='limit'
            step={1}
            type='number'
          />
        </Field>
        <Button type='submit' variant='outline'>
          Применить
        </Button>
      </form>
      {query.isFetching && query.data && <p role='status'>Обновление истории…</p>}
      {query.isError && <ErrorState error={query.error} onRetry={() => void query.refetch()} />}
      {!query.data ? (
        !query.isError && <LoadingState />
      ) : (
        <>
          {!query.data.items.length ? (
            <EmptyState
              description='Загрузите CSV или выберите другую страницу истории.'
              title='На этой странице импортов нет'
            />
          ) : (
            <Table>
              <TableCaption>История импортов: всего {query.data.pagination.total}</TableCaption>
              <TableHeader>
                <TableRow>
                  <TableHead scope='col'>Файл</TableHead>
                  <TableHead scope='col'>Статус</TableHead>
                  <TableHead scope='col'>Дата выгрузки</TableHead>
                  <TableHead scope='col'>Загружено</TableHead>
                  <TableHead scope='col'>Ошибок</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {query.data.items.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell>
                      <Link
                        className='underline'
                        params={{ importId: item.id }}
                        to='/imports/$importId'
                      >
                        {item.filename ?? item.id}
                      </Link>
                    </TableCell>
                    <TableCell>{IMPORT_STATUSES[item.status]}</TableCell>
                    <TableCell>{formatDataValue(item.collected_at)}</TableCell>
                    <TableCell>{formatDataValue(item.loaded_rows)}</TableCell>
                    <TableCell>{formatDataValue(item.error_rows)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
          <nav aria-label='Страницы истории импортов' className='flex flex-wrap items-center gap-3'>
            <Button
              render={
                <Link search={{ ...search, page: Math.max(1, search.page - 1) }} to='/imports' />
              }
              disabled={search.page <= 1}
              nativeButton={false}
              variant='outline'
            >
              Предыдущая
            </Button>
            <p role='status'>
              Страница {query.data.pagination.page} из{' '}
              {Math.max(1, query.data.pagination.total_pages)}
            </p>
            <Button
              disabled={search.page >= query.data.pagination.total_pages}
              nativeButton={false}
              render={<Link search={{ ...search, page: search.page + 1 }} to='/imports' />}
              variant='outline'
            >
              Следующая
            </Button>
          </nav>
        </>
      )}
    </section>
  );
}
