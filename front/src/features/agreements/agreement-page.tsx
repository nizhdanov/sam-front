import { useQuery, useQueryClient } from '@tanstack/react-query';
import { getRouteApi, Link, useRouter } from '@tanstack/react-router';
import { useState } from 'react';

import { useDemoAuth } from '@/app/auth/context';
import { PageHeader } from '@/components/shared/page-header';
import { AccessDenied, NotFoundState } from '@/components/shared/route-states';
import { EmptyState, ErrorState, LoadingState, SuccessState } from '@/components/shared/states';
import { Button } from '@/components/ui/button';
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
  SheetTrigger
} from '@/components/ui/sheet';
import { Spinner } from '@/components/ui/spinner';
import { COST_MODELS, LICENSE_TYPES } from '@/features/software/model';
import { formatDataValue } from '@/utils/api/data';
import { ApiError } from '@/utils/api/errors';

import { AgreementEditor } from './agreement-editor';
import { AnalysisPanel } from './analysis-panel';
import { getAgreementFileError } from './model';
import { agreementDetailOptions, deleteAgreement, uploadAgreementFile } from './queries';

const route = getRouteApi('/_authenticated/_admin/agreements_/$agreementId');

export function AgreementsPage() {
  return (
    <section aria-label='Соглашения' className='flex min-w-0 flex-col gap-6'>
      <PageHeader title='Соглашения' />
      <AgreementEditor />
      <EmptyState
        description='Можно создать соглашение и работать с ним в отдельной карточке.'
        title='Список соглашений пока недоступен'
      />
    </section>
  );
}

export function AgreementPage() {
  const { agreementId } = route.useParams();
  const navigate = route.useNavigate();
  const router = useRouter();
  const { auth } = useDemoAuth();
  const session = auth.requireReady();
  const queryClient = useQueryClient();
  const query = useQuery(agreementDetailOptions(auth, agreementId));
  const [pending, setPending] = useState<'delete' | 'upload' | null>(null);
  const [error, setError] = useState<unknown>();
  const [fileError, setFileError] = useState<string>();
  const [uploaded, setUploaded] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  if (query.isError && query.error instanceof ApiError) {
    if (query.error.status === 404) return <NotFoundState />;
    if (query.error.status === 403) return <AccessDenied />;
  }
  if (!query.data) {
    if (query.isError) {
      return <ErrorState error={query.error} onRetry={() => void query.refetch()} />;
    }
    return <LoadingState />;
  }
  const agreement = query.data;
  const details: [string, unknown][] = [
    ['Тип лицензии', agreement.license_type && LICENSE_TYPES[agreement.license_type]],
    ['Модель оплаты', agreement.cost_model && COST_MODELS[agreement.cost_model]],
    ['Коммерческое использование', agreement.commercial_allowed],
    ['Использующих продуктов', agreement.products_count],
    ['Файл', agreement.file_name],
    ['Источник', agreement.source_url],
    ['Статус проверки', agreement.review_status]
  ];

  return (
    <section aria-label='Карточка соглашения' className='flex min-w-0 flex-col gap-6'>
      <Button
        className='self-start'
        nativeButton={false}
        render={<Link to='/agreements' />}
        variant='outline'
      >
        Соглашения
      </Button>
      <h1 className='wrap-break-word'>{formatDataValue(agreement.name)}</h1>
      {query.isFetching && <p role='status'>Обновление карточки…</p>}
      {query.isError && <ErrorState error={query.error} onRetry={() => void query.refetch()} />}
      <AgreementEditor agreement={agreement} disabled={pending !== null} />
      <dl className='grid min-w-0 gap-4 sm:grid-cols-2'>
        {details.map(([label, value]) => (
          <div key={label} className='min-w-0'>
            <dt className='text-sm text-muted-foreground'>{label}</dt>
            <dd className='wrap-break-word whitespace-pre-wrap'>{formatDataValue(value)}</dd>
          </div>
        ))}
      </dl>
      <div className='flex min-w-0 flex-col gap-2'>
        <h2 className='text-lg font-semibold'>Текст соглашения</h2>
        <p className='wrap-break-word whitespace-pre-wrap'>
          {formatDataValue(agreement.agreement_text)}
        </p>
      </div>
      <AnalysisPanel agreementId={agreementId} hasText={!!agreement.agreement_text?.trim()} />
      <form
        aria-label='Загрузка файла соглашения'
        className='flex flex-col items-start gap-4'
        onSubmit={async (event) => {
          event.preventDefault();
          if (pending || !auth.isCurrent(session.epoch)) return;
          const element = event.currentTarget;
          const value = new FormData(element).get('file');
          const file = value instanceof File ? value : undefined;
          const validationError = getAgreementFileError(file);
          setFileError(validationError);
          if (!file || validationError) return;
          setPending('upload');
          setError(undefined);
          setUploaded(false);
          try {
            await uploadAgreementFile(auth, queryClient, agreementId, file);
            if (!auth.isCurrent(session.epoch)) return;
            element.reset();
            setUploaded(true);
          } catch (uploadError) {
            if (auth.isCurrent(session.epoch)) setError(uploadError);
          } finally {
            if (auth.isCurrent(session.epoch)) setPending(null);
          }
        }}
      >
        <FieldGroup>
          <Field data-disabled={pending !== null} data-invalid={!!fileError}>
            <FieldLabel htmlFor='agreement-file'>Файл соглашения</FieldLabel>
            <Input
              accept='.txt,.pdf,.docx'
              aria-describedby='agreement-file-description agreement-file-error'
              aria-invalid={!!fileError}
              disabled={pending !== null}
              id='agreement-file'
              name='file'
              type='file'
              onChange={() => {
                setFileError(undefined);
                setUploaded(false);
              }}
            />
            <FieldDescription id='agreement-file-description'>
              .txt, .pdf или .docx до 10 МБ. Загрузка заменит текст соглашения извлечённым из файла.
            </FieldDescription>
            <FieldError id='agreement-file-error'>{fileError}</FieldError>
          </Field>
        </FieldGroup>
        <Button disabled={pending !== null} type='submit'>
          {pending === 'upload' && <Spinner data-icon='inline-start' />}
          {pending === 'upload' ? 'Загрузка…' : 'Загрузить файл'}
        </Button>
      </form>
      {uploaded && <SuccessState>Файл загружен, текст соглашения обновлён.</SuccessState>}
      {error !== undefined && !deleteOpen && <ErrorState error={error} />}
      <Sheet
        open={deleteOpen}
        onOpenChange={(nextOpen, details) => {
          if (pending === 'delete') details.cancel();
          else {
            if (nextOpen) setError(undefined);
            setDeleteOpen(nextOpen);
          }
        }}
      >
        <SheetTrigger
          render={
            <Button className='self-start' disabled={pending !== null} variant='destructive' />
          }
        >
          Удалить соглашение
        </SheetTrigger>
        <SheetContent className='overflow-y-auto data-[side=right]:w-full'>
          <SheetHeader>
            <SheetTitle>Удалить соглашение?</SheetTitle>
            <SheetDescription>
              Это действие нельзя отменить. Соглашение, используемое продуктами, удалить нельзя.
            </SheetDescription>
          </SheetHeader>
          {error !== undefined && (
            <div className='px-6'>
              <ErrorState error={error} />
            </div>
          )}
          <SheetFooter>
            <Button
              disabled={pending !== null}
              variant='destructive'
              onClick={async () => {
                if (pending || !auth.isCurrent(session.epoch)) return;
                setPending('delete');
                setError(undefined);
                try {
                  await deleteAgreement(auth, queryClient, agreementId);
                  if (
                    auth.isCurrent(session.epoch) &&
                    router.state.location.pathname === `/agreements/${agreementId}`
                  )
                    void navigate({ to: '/agreements' });
                } catch (deleteError) {
                  if (auth.isCurrent(session.epoch)) setError(deleteError);
                } finally {
                  if (auth.isCurrent(session.epoch)) setPending(null);
                }
              }}
            >
              {pending === 'delete' && <Spinner data-icon='inline-start' />}
              {pending === 'delete' ? 'Удаление…' : 'Удалить окончательно'}
            </Button>
            <SheetClose render={<Button disabled={pending !== null} variant='outline' />}>
              Отмена
            </SheetClose>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </section>
  );
}
