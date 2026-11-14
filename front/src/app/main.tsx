import { QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider } from '@tanstack/react-router';
import ReactDOM from 'react-dom/client';

import { queryClient } from '@/utils/lib';
import { setApiSession } from '@/utils/lib/instance';

import { DemoAuthContext } from './auth/context';
import { getLanding } from './auth/demo-accounts';
import { createDemoAuth } from './auth/session';
import { getRouter } from './router';

async function bootstrap() {
  const auth = createDemoAuth(queryClient);
  setApiSession(auth);
  await auth.initialize();
  const router = getRouter(auth);

  auth.subscribe(() => {
    router.clearCache();
    const snapshot = auth.getSnapshot();
    if (snapshot.status !== 'ready' || !snapshot.role) return;
    const landing = getLanding(snapshot.role);
    void router.navigate({ to: landing, replace: true }).then(() => {
      if (auth.isCurrent(snapshot.epoch)) void router.invalidate();
    });
  });
  window.addEventListener('storage', (event) => {
    if (event.key === 'token' || event.key === null) void auth.syncStorage();
  });

  const rootElement = document.getElementById('app')!;
  if (rootElement.innerHTML) return;
  ReactDOM.createRoot(rootElement).render(
    <QueryClientProvider client={queryClient}>
      <DemoAuthContext value={auth}>
        <RouterProvider router={router} />
      </DemoAuthContext>
    </QueryClientProvider>
  );
}

void bootstrap();
