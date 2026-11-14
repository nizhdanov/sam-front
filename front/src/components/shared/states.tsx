import type { ReactNode } from 'react';

import { CircleCheckIcon, CircleXIcon, InboxIcon } from 'lucide-react';

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle
} from '@/components/ui/empty';
import { Spinner } from '@/components/ui/spinner';
import { ApiError } from '@/utils/api/errors';

export function LoadingState({ label = 'Загрузка…' }: { label?: string }) {
  return (
    <div aria-live='polite' className='flex items-center justify-center gap-3 p-8' role='status'>
      <Spinner aria-hidden='true' />
      <span>{label}</span>
    </div>
  );
}

export function EmptyState({
  title = 'Данных пока нет',
  description,
  action
}: {
  title?: ReactNode;
  description?: string;
  action?: ReactNode;
}) {
  return (
    <Empty>
      <EmptyHeader>
        <EmptyMedia variant='icon'>
          <InboxIcon aria-hidden='true' />
        </EmptyMedia>
        <EmptyTitle>{title}</EmptyTitle>
        {description && <EmptyDescription>{description}</EmptyDescription>}
      </EmptyHeader>
      {action && <EmptyContent>{action}</EmptyContent>}
    </Empty>
  );
}

export function ErrorState({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  const message =
    error instanceof ApiError ? error.message : 'Не удалось загрузить данные. Повторите попытку.';
  return (
    <Alert variant='destructive'>
      <CircleXIcon aria-hidden='true' />
      <AlertTitle>Не удалось выполнить действие</AlertTitle>
      <AlertDescription className='flex flex-col items-start gap-3 wrap-break-word'>
        <p>{message}</p>
        {error instanceof ApiError && error.requestId && <p>Код запроса: {error.requestId}</p>}
        {onRetry && (
          <Button variant='outline' onClick={onRetry}>
            Повторить
          </Button>
        )}
      </AlertDescription>
    </Alert>
  );
}

export function SuccessState({ children }: { children: ReactNode }) {
  return (
    <Alert role='status'>
      <CircleCheckIcon aria-hidden='true' />
      <AlertTitle>Готово</AlertTitle>
      <AlertDescription>{children}</AlertDescription>
    </Alert>
  );
}

export function SectionPlaceholder({ title }: { title: string }) {
  return (
    <section aria-label={title} className='flex flex-col gap-6'>
      <h1 className='text-2xl font-semibold'>{title}</h1>
      <EmptyState
        description='Данные и действия появятся на следующем этапе разработки этого раздела.'
        title='Раздел готовится'
      />
    </section>
  );
}
