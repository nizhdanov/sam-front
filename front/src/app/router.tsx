import { createRouter as createTanStackRouter } from '@tanstack/react-router';

import type { DemoAuth } from '@/app/auth/session';

import { NotFoundState, RouteError } from '@/components/shared/route-states';
import { LoadingState } from '@/components/shared/states';
import { routeTree } from '@/generated/routeTree.gen';
import { queryClient } from '@/utils/lib/queryClient';

export function getRouter(auth: DemoAuth) {
  const router = createTanStackRouter({
    routeTree,
    context: { auth, queryClient },
    scrollRestoration: true,
    defaultPreload: 'intent',
    defaultPreloadStaleTime: 0,
    defaultPendingComponent: LoadingState,
    defaultErrorComponent: RouteError,
    defaultNotFoundComponent: NotFoundState
  });

  return router;
}

declare module '@tanstack/react-router' {
  interface Register {
    router: ReturnType<typeof getRouter>;
  }
}
