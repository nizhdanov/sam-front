import { createFileRoute, Outlet, redirect } from '@tanstack/react-router';

import { useDemoAuth } from '@/app/auth/context';
import { canAccess } from '@/app/auth/permissions';
import { AccessDenied } from '@/components/shared/route-states';

export const Route = createFileRoute('/_authenticated/_admin')({
  beforeLoad: ({ context }) => {
    if (!canAccess(context.auth.requireReady().role, 'admin')) {
      throw redirect({ to: '/access-denied', replace: true });
    }
  },
  component: AdminLayout
});

function AdminLayout() {
  const { snapshot } = useDemoAuth();
  return canAccess(snapshot.role, 'admin') ? <Outlet /> : <AccessDenied />;
}
