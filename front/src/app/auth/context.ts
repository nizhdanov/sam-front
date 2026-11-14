import { createContext, use, useSyncExternalStore } from 'react';

import type { DemoAuth } from './session';

export const DemoAuthContext = createContext<DemoAuth | null>(null);

export function useDemoAuth() {
  const auth = use(DemoAuthContext);
  if (!auth) throw new Error('DemoAuthContext is missing');
  const snapshot = useSyncExternalStore(auth.subscribe, auth.getSnapshot);
  return { auth, snapshot };
}
