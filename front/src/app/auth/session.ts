import type { QueryClient } from '@tanstack/react-query';

import { ApiError } from '@/utils/api/errors';

import type { DemoRole } from './demo-accounts';

import { DEMO_ACCOUNTS, getDemoAccount } from './demo-accounts';

interface AuthSnapshot {
  epoch: number;
  error?: ApiError;
  role: DemoRole | null;
  status: 'error' | 'ready' | 'switching';
}

export interface DemoSession {
  epoch: number;
  role: DemoRole;
  signal: AbortSignal;
}

export function createDemoAuth(
  queryClient: QueryClient,
  getStorage: () => Pick<Storage, 'getItem' | 'setItem'> = () => localStorage
) {
  let snapshot: AuthSnapshot = { role: null, status: 'switching', epoch: 0 };
  let controller = new AbortController();
  const listeners = new Set<() => void>();

  function publish(next: AuthSnapshot) {
    snapshot = next;
    for (const listener of listeners) listener();
  }

  function clearContext() {
    controller.abort();
    controller = new AbortController();
    const cancelled = queryClient.cancelQueries();
    queryClient.clear();
    return cancelled;
  }

  function fail(epoch: number, error: ApiError) {
    if (snapshot.epoch !== epoch) return;
    publish({ ...snapshot, status: 'error', epoch: epoch + 1, error });
    void clearContext();
  }

  async function change(role: DemoRole, write: boolean) {
    const epoch = snapshot.epoch + 1;
    publish({ role: snapshot.role, status: 'switching', epoch });
    await clearContext();
    if (snapshot.epoch !== epoch) return;

    const account = DEMO_ACCOUNTS.find((item) => item.role === role);
    if (!account?.token) {
      fail(epoch, new ApiError('Демонстрационный токен роли не задан.'));
      return;
    }

    try {
      if (write) getStorage().setItem('token', account.token);
      if (getStorage().getItem('token') !== account.token) {
        fail(epoch, new ApiError('Токен изменён в другой вкладке. Выберите роль повторно.'));
        return;
      }
      publish({ role, status: 'ready', epoch });
    } catch {
      fail(
        epoch,
        new ApiError('Хранилище браузера недоступно. Разрешите localStorage и выберите роль.')
      );
    }
  }

  async function initialize() {
    try {
      const account = getDemoAccount(getStorage().getItem('token'));
      await change(account?.role ?? 'USER', !account);
    } catch {
      fail(
        snapshot.epoch,
        new ApiError('Хранилище браузера недоступно. Разрешите localStorage и выберите роль.')
      );
    }
  }

  async function syncStorage() {
    try {
      const account = getDemoAccount(getStorage().getItem('token'));
      if (!account) {
        fail(
          snapshot.epoch,
          new ApiError('В другой вкладке удалён или изменён токен. Выберите роль.')
        );
        return;
      }
      if (snapshot.status === 'ready' && snapshot.role === account.role) return;
      await change(account.role, false);
    } catch {
      fail(
        snapshot.epoch,
        new ApiError('Не удалось прочитать localStorage. Выберите роль повторно.')
      );
    }
  }

  function requireReady(): DemoSession {
    if (snapshot.status !== 'ready' || !snapshot.role) {
      throw snapshot.error ?? new ApiError('Дождитесь переключения роли.');
    }
    return { role: snapshot.role, epoch: snapshot.epoch, signal: controller.signal };
  }

  function isCurrent(epoch: number) {
    return snapshot.status === 'ready' && snapshot.epoch === epoch;
  }

  return {
    getSnapshot: () => snapshot,
    subscribe(listener: () => void) {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    initialize,
    selectRole: (role: DemoRole) => change(role, true),
    syncStorage,
    requireReady,
    isCurrent,
    fail,
    requestConfig(session: DemoSession) {
      if (!isCurrent(session.epoch))
        throw new ApiError('Контекст роли изменился. Повторите действие.');
      return { signal: session.signal };
    }
  };
}

export type DemoAuth = ReturnType<typeof createDemoAuth>;
