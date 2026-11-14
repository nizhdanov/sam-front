import { createFileRoute, Outlet } from '@tanstack/react-router';

import { useDemoAuth } from '@/app/auth/context';
import { LoadingState } from '@/components/shared/states';

export const Route = createFileRoute('/_authenticated')({
  beforeLoad: ({ context }) => ({ session: context.auth.requireReady() }),
  component: AuthenticatedLayout
});

function AuthenticatedLayout() {
  const { snapshot } = useDemoAuth();
  const { session } = Route.useRouteContext();
  if (snapshot.status !== 'ready' || snapshot.epoch !== session.epoch) return <LoadingState />;
  return <Outlet key={session.epoch} />;
}
