import type { ErrorComponentProps } from '@tanstack/react-router';

import { Link, useRouter } from '@tanstack/react-router';

import { useDemoAuth } from '@/app/auth/context';
import { getLanding } from '@/app/auth/demo-accounts';
import { Button } from '@/components/ui/button';
import { ApiError } from '@/utils/api/errors';

import { EmptyState, ErrorState } from './states';

export function AccessDenied() {
  const { snapshot } = useDemoAuth();
  return (
    <EmptyState
      action={
        <Button nativeButton={false} render={<Link to={getLanding(snapshot.role ?? 'USER')} />}>
          Открыть доступный раздел
        </Button>
      }
      description='У выбранной роли нет доступа к этому разделу.'
      title={<h1>Доступ запрещён</h1>}
    />
  );
}

export function NotFoundState() {
  return (
    <EmptyState
      action={
        <Button nativeButton={false} render={<Link to='/' />}>
          На главную
        </Button>
      }
      description='Проверьте адрес или выберите раздел в меню.'
      title={<h1>Страница не найдена</h1>}
    />
  );
}

export function RouteError({ error, reset }: ErrorComponentProps) {
  const router = useRouter();
  if (error instanceof ApiError && error.status === 403) return <AccessDenied />;
  if (error instanceof ApiError && error.status === 404) return <NotFoundState />;
  return (
    <ErrorState
      error={error}
      onRetry={() => {
        void router.invalidate().then(reset);
      }}
    />
  );
}
