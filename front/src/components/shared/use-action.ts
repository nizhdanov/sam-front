import { useState } from 'react';

import { useDemoAuth } from '@/app/auth/context';

export function useAction() {
  const { auth } = useDemoAuth();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<unknown>();

  async function run(action: () => Promise<unknown>) {
    if (pending) return false;
    const session = auth.requireReady();
    setPending(true);
    setError(undefined);
    try {
      await action();
      return auth.isCurrent(session.epoch);
    } catch (actionError) {
      if (auth.isCurrent(session.epoch)) setError(actionError);
      return false;
    } finally {
      if (auth.isCurrent(session.epoch)) setPending(false);
    }
  }

  return { pending, error, run, reset: () => setError(undefined) };
}
