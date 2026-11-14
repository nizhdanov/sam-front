import type { QueryClient } from '@tanstack/react-query';

import { TanStackDevtools } from '@tanstack/react-devtools';
import { ReactQueryDevtoolsPanel } from '@tanstack/react-query-devtools';
import { createRootRouteWithContext, HeadContent, Outlet } from '@tanstack/react-router';
import { TanStackRouterDevtoolsPanel } from '@tanstack/react-router-devtools';

import type { DemoAuth } from '@/app/auth/session';

import { useDemoAuth } from '@/app/auth/context';
import { AppShell } from '@/components/layout/app-shell';

import './styles.css';

export const Route = createRootRouteWithContext<{ auth: DemoAuth; queryClient: QueryClient }>()({
  component: RootComponent
});

function RootComponent() {
  const { snapshot } = useDemoAuth();
  return (
    <>
      <HeadContent />
      <AppShell>
        <Outlet />
      </AppShell>
      {snapshot.status === 'ready' && (
        <TanStackDevtools
          config={{
            position: 'bottom-right'
          }}
          plugins={[
            {
              name: 'TanStack Router',
              render: <TanStackRouterDevtoolsPanel />
            },
            {
              name: 'React Query',
              render: <ReactQueryDevtoolsPanel />
            }
          ]}
        />
      )}
    </>
  );
}
