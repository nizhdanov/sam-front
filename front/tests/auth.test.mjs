import { QueryClient } from '@tanstack/react-query';
import { AxiosError, isCancel } from 'axios';
import assert from 'node:assert/strict';
import { afterEach, test } from 'node:test';

import './register.mjs';

const { createDemoAuth } = await import('../src/app/auth/session.ts');
const { DEMO_ACCOUNTS, getDemoAccount, getLanding } =
  await import('../src/app/auth/demo-accounts.ts');
const { canAccess, getNavigation } = await import('../src/app/auth/permissions.ts');
const { ApiError } = await import('../src/utils/api/errors.ts');
const { instance, setApiSession } = await import('../src/utils/lib/instance.ts');
const { queryClient: appQueryClient } = await import('../src/utils/lib/queryClient.ts');

const storageDescriptor = Object.getOwnPropertyDescriptor(globalThis, 'localStorage');
afterEach(() => {
  setApiSession();
  if (storageDescriptor) Object.defineProperty(globalThis, 'localStorage', storageDescriptor);
  else delete globalThis.localStorage;
});

function setup(saved = null) {
  let token = saved;
  const writes = [];
  const storage = {
    getItem: (key) => (key === 'token' ? token : null),
    setItem(key, value) {
      assert.equal(key, 'token');
      token = value;
      writes.push(value);
    }
  };
  Object.defineProperty(globalThis, 'localStorage', { configurable: true, value: storage });
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const auth = createDemoAuth(queryClient, () => storage);
  setApiSession(auth);
  return {
    auth,
    storage,
    queryClient,
    writes,
    setSaved: (value) => {
      token = value;
    }
  };
}

const user = DEMO_ACCOUNTS.find((account) => account.role === 'USER');
const admin = DEMO_ACCOUNTS.find((account) => account.role === 'ADMIN');

test('точное сопоставление токенов, USER по умолчанию и восстановление ADMIN', async () => {
  assert.notEqual(user.token, admin.token);
  for (const account of DEMO_ACCOUNTS) assert.equal(getDemoAccount(account.token), account);
  for (const token of [null, '', 'USER', 'ADMIN', `${admin.token} `])
    assert.equal(getDemoAccount(token), undefined);
  for (const saved of [null, 'unknown', admin.token]) {
    const { auth, storage, writes } = setup(saved);
    await auth.initialize();
    assert.equal(auth.getSnapshot().role, saved === admin.token ? 'ADMIN' : 'USER');
    assert.equal(storage.getItem('token'), saved === admin.token ? admin.token : user.token);
    assert.equal(writes.length, saved === admin.token ? 0 : 1);
  }
});

test('USER→ADMIN→USER очищает Query/мутации, отменяет старую работу и меняет Bearer', async () => {
  const { auth, queryClient, writes } = setup();
  await auth.initialize();
  for (const role of ['ADMIN', 'USER']) {
    const previous = auth.requireReady();
    queryClient.setQueryData(['private', previous.epoch], 'private');
    queryClient.getMutationCache().build(queryClient, { mutationFn: async () => null });
    const pending = queryClient.fetchQuery({
      queryKey: ['pending', previous.epoch],
      queryFn: ({ signal }) =>
        new Promise((_, reject) => {
          signal.addEventListener('abort', () => reject(new Error('aborted')), { once: true });
        })
    });
    const rejected = pending.catch(() => 'cancelled');
    const changing = auth.selectRole(role);
    assert.equal(auth.getSnapshot().status, 'switching');
    assert.equal(previous.signal.aborted, true);
    assert.throws(() => auth.requireReady());
    await changing;
    assert.equal(await rejected, 'cancelled');
    assert.equal(queryClient.getQueryCache().getAll().length, 0);
    assert.equal(queryClient.getMutationCache().getAll().length, 0);
    assert.equal(auth.isCurrent(previous.epoch), false);
    assert.throws(() => auth.requestConfig(previous));
    const response = await instance.get('/test', {
      ...auth.requestConfig(auth.requireReady()),
      adapter: async (config) => ({
        data: null,
        status: 200,
        statusText: 'OK',
        headers: {},
        config
      })
    });
    assert.equal(
      response.config.headers.get('Authorization'),
      `Bearer ${role === 'ADMIN' ? admin.token : user.token}`
    );
  }
  assert.deepEqual(writes, [user.token, admin.token, user.token]);
});

test('storage sync читает актуальное значение, не пишет его повторно и закрывает неизвестный токен', async () => {
  const { auth, setSaved, writes, queryClient } = setup();
  await auth.initialize();
  const previous = auth.requireReady();
  queryClient.setQueryData(['private'], 'private');
  setSaved(admin.token);
  await auth.syncStorage();
  assert.equal(auth.getSnapshot().role, 'ADMIN');
  assert.equal(previous.signal.aborted, true);
  assert.equal(queryClient.getQueryData(['private']), undefined);
  assert.deepEqual(writes, [user.token]);
  setSaved(null);
  await auth.syncStorage();
  assert.equal(auth.getSnapshot().status, 'error');
  assert.throws(() => auth.requireReady());
  await auth.selectRole('USER');
  assert.equal(auth.getSnapshot().status, 'ready');
});

test('из двух одновременных переключений применяется только последнее', async () => {
  const { auth, writes } = setup();
  await auth.initialize();
  const first = auth.selectRole('ADMIN');
  const second = auth.selectRole('USER');
  await Promise.all([first, second]);
  assert.equal(auth.getSnapshot().status, 'ready');
  assert.equal(auth.getSnapshot().role, 'USER');
  assert.deepEqual(writes, [user.token, user.token]);
});

test('ошибка чтения/записи localStorage видима и не подтверждает новую роль', async (t) => {
  const { auth, storage } = setup();
  const read = t.mock.method(storage, 'getItem', () => {
    throw new Error('denied');
  });
  await auth.initialize();
  assert.equal(auth.getSnapshot().status, 'error');
  assert.equal(auth.getSnapshot().role, null);
  read.mock.restore();
  await auth.selectRole('USER');
  t.mock.method(storage, 'setItem', () => {
    throw new Error('denied');
  });
  await auth.selectRole('ADMIN');
  assert.equal(auth.getSnapshot().status, 'error');
  assert.equal(auth.getSnapshot().role, 'USER');
  assert.throws(() => auth.requireReady(), ApiError);
});

test('поздние success/401 и submit прежнего контекста не меняют новую роль', async () => {
  for (const unauthorized of [false, true]) {
    const { auth } = setup(admin.token);
    await auth.initialize();
    const oldSession = auth.requireReady();
    let finish;
    const pending = instance.get('/old', {
      ...auth.requestConfig(oldSession),
      adapter: (config) =>
        new Promise((resolve, reject) => {
          finish = () => {
            const response = {
              data: {},
              status: unauthorized ? 401 : 200,
              headers: {},
              statusText: '',
              config
            };
            if (unauthorized)
              reject(new AxiosError('old', 'ERR_BAD_REQUEST', config, null, response));
            else resolve(response);
          };
        })
    });
    const rejected = assert.rejects(pending, isCancel);
    await auth.selectRole('USER');
    finish();
    await rejected;
    assert.equal(auth.getSnapshot().role, 'USER');
    assert.equal(auth.getSnapshot().status, 'ready');
    let sent = false;
    await assert.rejects(
      instance.post(
        '/submit',
        {},
        {
          signal: oldSession.signal,
          adapter: async () => {
            sent = true;
            throw new Error('must not send');
          }
        }
      ),
      isCancel
    );
    assert.equal(sent, false);
  }
});

test('актуальный 401 останавливает данные без смены токена и допускает явный повторный выбор', async () => {
  const { auth, queryClient, writes } = setup(admin.token);
  await auth.initialize();
  queryClient.setQueryData(['private'], 'private');
  await assert.rejects(
    instance.get('/test', {
      adapter: async (config) => {
        throw new AxiosError('unauthorized', 'ERR_BAD_REQUEST', config, null, {
          data: { error: { message: 'Токен недействителен.', request_id: 'test-401' } },
          status: 401,
          statusText: '',
          headers: {},
          config
        });
      }
    }),
    (error) => error instanceof ApiError && error.status === 401
  );
  assert.equal(auth.getSnapshot().status, 'error');
  assert.equal(auth.getSnapshot().error.requestId, 'test-401');
  assert.equal(queryClient.getQueryData(['private']), undefined);
  assert.deepEqual(writes, []);
  await auth.selectRole('ADMIN');
  assert.equal(auth.getSnapshot().status, 'ready');
});

test('permissions закрыты по умолчанию, меню и landing соответствуют роли', () => {
  for (const role of [null, undefined, 'DIRECTOR', 'admin']) {
    assert.equal(canAccess(role, 'shared'), false);
    assert.equal(canAccess(role, 'admin'), false);
  }
  assert.equal(canAccess('USER', 'admin'), false);
  assert.equal(canAccess('USER', 'shared'), true);
  assert.equal(canAccess('ADMIN', 'admin'), true);
  assert.deepEqual(
    getNavigation('USER').map((item) => item.to),
    ['/software', '/me', '/assistant']
  );
  assert.ok(getNavigation('ADMIN').some((item) => item.to === '/dashboard'));
  assert.equal(getLanding('ADMIN'), '/dashboard');
  assert.equal(getLanding('USER'), '/software');
});

test('ошибки 4xx не повторяются, сетевые/5xx ограничены двумя повторами', () => {
  const retry = appQueryClient.getDefaultOptions().queries.retry;
  for (const status of [400, 401, 403, 404, 409, 413, 429]) {
    assert.equal(retry(0, new ApiError('test', { status })), false);
  }
  for (const error of [new ApiError('network'), new ApiError('server', { status: 503 })]) {
    assert.equal(retry(0, error), true);
    assert.equal(retry(2, error), false);
  }
});
