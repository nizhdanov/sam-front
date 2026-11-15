import { QueryClient } from '@tanstack/react-query';
import { isCancel } from 'axios';
import assert from 'node:assert/strict';
import { Buffer } from 'node:buffer';
import { once } from 'node:events';
import { createServer } from 'node:http';
import { test } from 'node:test';

import './register.mjs';

const { agreementFormSchema, getAgreementFileError, getAgreementFormValues, readAgreement } =
  await import('../src/features/agreements/model.ts');
const { agreementDetailOptions, deleteAgreement, saveAgreement, uploadAgreementFile } =
  await import('../src/features/agreements/queries.ts');
const { createDemoAuth } = await import('../src/app/auth/session.ts');
const { instance, setApiSession } = await import('../src/utils/lib/instance.ts');
const { ApiError } = await import('../src/utils/api/errors.ts');

const id = 'f5b5fc3a-f23f-4e8d-a4ba-a2b3c4d5e6f7';
const agreement = {
  id,
  name: '<script>MIT</script>',
  license_type: 'OPEN_SOURCE',
  cost_model: 'FREE',
  agreement_text: null,
  source_url: null,
  commercial_allowed: false,
  products_count: 0,
  file_name: null
};

test('карточка соглашения проверяет ID/типы, сохраняет false/null/0 и отбрасывает неизвестные поля', () => {
  assert.deepEqual(readAgreement({ ...agreement, private_field: 'secret' }, id), agreement);
  assert.deepEqual(readAgreement({ id }), { id });
  for (const value of [
    null,
    {},
    { ...agreement, id: 'bad' },
    { ...agreement, license_type: 'FUTURE' },
    { ...agreement, commercial_allowed: 'false' },
    { ...agreement, products_count: -1 },
    { ...agreement, products_count: 1.5 }
  ])
    assert.throws(() => readAgreement(value), ApiError);
  assert.throws(() => readAgreement(agreement, '00000000-0000-4000-8000-000000000000'), ApiError);
});

test('форма сохраняет три состояния коммерческого использования, не теряет текст и очищает nullable поля', () => {
  for (const commercial_allowed of [true, false, null]) {
    const values = getAgreementFormValues({ ...agreement, commercial_allowed });
    const input = agreementFormSchema.parse({
      ...values,
      name: ' MIT ',
      agreement_text: '  Текст\n\n'
    });
    assert.equal(input.commercial_allowed, commercial_allowed);
    assert.equal(input.name, 'MIT');
    assert.equal(input.agreement_text, '  Текст\n\n');
    assert.equal(input.source_url, null);
    assert.equal(Object.hasOwn(input, 'id'), false);
  }
  const empty = agreementFormSchema.parse({ ...getAgreementFormValues(), name: 'Новое' });
  assert.equal(empty.license_type, undefined);
  assert.equal(empty.cost_model, undefined);
  assert.equal(empty.agreement_text, null);
  assert.equal(empty.commercial_allowed, null);
  assert.equal(
    agreementFormSchema.parse({
      ...getAgreementFormValues(agreement),
      source_url: ' https://example.org/eula '
    }).source_url,
    'https://example.org/eula'
  );
  for (const values of [
    { name: '  ' },
    { license_type: 'UNKNOWN' },
    { source_url: 'javascript:alert(1)' },
    { source_url: 'file:///tmp/eula' },
    { commercial_allowed: '' }
  ])
    assert.equal(
      agreementFormSchema.safeParse({ ...getAgreementFormValues(agreement), ...values }).success,
      false
    );
});

test('загрузка соглашения проверяет расширение, пустой файл и границу размера до запроса', () => {
  for (const name of ['eula.TXT', 'eula.pdf', 'eula.docx'])
    assert.equal(getAgreementFileError(new File(['Текст'], name)), undefined);
  for (const file of [undefined, new File([], 'eula.txt'), new File(['x'], 'eula.txt.exe')])
    assert.equal(typeof getAgreementFileError(file), 'string');
  const contents = new Uint8Array(10_000_001);
  assert.equal(getAgreementFileError(new File([contents.subarray(1)], 'eula.txt')), undefined);
  assert.equal(typeof getAgreementFileError(new File([contents], 'eula.txt')), 'string');
});

test('generated HTTP: создание/чтение/PATCH/multipart/409/204, инвалидирование кеша и смена роли', async (t) => {
  const requests = [];
  let stored = { ...agreement };
  let conflict = false;
  let malformed = false;
  let started;
  let finish;
  const server = createServer(async (request, response) => {
    try {
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const data = Buffer.concat(chunks);
      const entry = { method: request.method, path: request.url };
      requests.push(entry);
      assert.ok(request.headers.authorization?.startsWith('Bearer '));
      response.setHeader('content-type', 'application/json');
      if (request.method === 'DELETE') {
        if (conflict) {
          response.writeHead(409);
          response.end(
            JSON.stringify({
              error: { message: 'Соглашение используется продуктами.', request_id: 'agreement-409' }
            })
          );
          return;
        }
        response.writeHead(204);
        response.end();
        return;
      }
      if (request.url.endsWith('/file')) {
        const form = await new Request('http://localhost', {
          method: 'POST',
          headers: request.headers,
          body: data
        }).formData();
        const file = form.get('file');
        entry.file = { name: file.name, text: await file.text() };
        assert.match(request.headers['content-type'], /^multipart\/form-data; boundary=/);
        stored = { ...stored, file_name: file.name, agreement_text: entry.file.text };
      } else if (request.method !== 'GET') {
        entry.body = JSON.parse(data.toString());
        stored = { ...stored, ...entry.body };
        if (stored.name === 'Задержка') {
          finish = () => response.end(JSON.stringify(stored));
          started();
          return;
        }
      }
      response.writeHead(request.method === 'POST' && request.url === '/agreements' ? 201 : 200);
      response.end(JSON.stringify(malformed ? {} : stored));
    } catch (error) {
      response.writeHead(500);
      response.end(JSON.stringify({ error: { message: error.message } }));
    }
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  t.after(
    () =>
      new Promise((resolve) => {
        server.closeAllConnections();
        server.close(resolve);
      })
  );

  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  let token = null;
  const storage = {
    getItem: () => token,
    setItem: (_key, value) => {
      token = value;
    }
  };
  const descriptor = Object.getOwnPropertyDescriptor(globalThis, 'localStorage');
  Object.defineProperty(globalThis, 'localStorage', { configurable: true, value: storage });
  const baseURL = instance.defaults.baseURL;
  const adapter = instance.defaults.adapter;
  instance.defaults.baseURL = `http://127.0.0.1:${server.address().port}`;
  instance.defaults.adapter = 'http';
  const auth = createDemoAuth(client, () => storage);
  setApiSession(auth);
  t.after(() => {
    setApiSession();
    client.clear();
    instance.defaults.baseURL = baseURL;
    instance.defaults.adapter = adapter;
    if (descriptor) Object.defineProperty(globalThis, 'localStorage', descriptor);
    else delete globalThis.localStorage;
  });
  await auth.initialize();
  const body = agreementFormSchema.parse(getAgreementFormValues(agreement));
  const file = new File(['Извлечённый текст'], 'eula.txt');
  assert.throws(
    () => agreementDetailOptions(auth, id),
    (error) => error.status === 403
  );
  for (const action of [
    () => saveAgreement(auth, client, body),
    () => uploadAgreementFile(auth, client, id, file),
    () => deleteAgreement(auth, client, id)
  ])
    await assert.rejects(action, (error) => error.status === 403);
  assert.equal(requests.length, 0);

  await auth.selectRole('ADMIN');
  const options = agreementDetailOptions(auth, id);
  const epoch = auth.requireReady().epoch;
  const softwareKey = ['software', 'ADMIN', epoch];
  client.setQueryData(softwareKey, 'cached');
  assert.deepEqual(await saveAgreement(auth, client, body), agreement);
  assert.deepEqual(requests[0], { method: 'POST', path: '/agreements', body });
  assert.equal(client.getQueryState(softwareKey).isInvalidated, true);
  assert.deepEqual(await client.fetchQuery(options), agreement);
  assert.equal(requests.at(-1).path, `/agreements/${id}`);
  const updated = await saveAgreement(
    auth,
    client,
    { ...body, commercial_allowed: null, agreement_text: 'Новый текст' },
    id
  );
  assert.equal(updated.commercial_allowed, null);
  assert.equal(updated.agreement_text, 'Новый текст');
  assert.equal(requests.at(-1).method, 'PATCH');
  assert.equal(requests.at(-1).body.commercial_allowed, null);

  const count = requests.length;
  await assert.rejects(
    uploadAgreementFile(auth, client, id, new File(['x'], 'wrong.exe')),
    ApiError
  );
  assert.equal(requests.length, count);
  const uploaded = await uploadAgreementFile(auth, client, id, file);
  assert.equal(uploaded.file_name, 'eula.txt');
  assert.equal(uploaded.agreement_text, 'Извлечённый текст');
  assert.deepEqual(requests.at(-1).file, { name: 'eula.txt', text: 'Извлечённый текст' });

  conflict = true;
  await assert.rejects(
    deleteAgreement(auth, client, id),
    (error) => error.status === 409 && error.requestId === 'agreement-409'
  );
  assert.deepEqual(client.getQueryData(options.queryKey), uploaded);
  assert.deepEqual(await client.fetchQuery(options), uploaded);
  conflict = false;
  malformed = true;
  await assert.rejects(
    saveAgreement(auth, client, { name: 'Сохранено без корректного ответа' }, id),
    ApiError
  );
  assert.deepEqual(client.getQueryData(options.queryKey), uploaded);
  malformed = false;
  assert.equal((await client.fetchQuery(options)).name, 'Сохранено без корректного ответа');
  await deleteAgreement(auth, client, id);
  assert.equal(client.getQueryData(options.queryKey), undefined);

  const received = new Promise((resolve) => {
    started = resolve;
  });
  const delayed = saveAgreement(auth, client, { name: 'Задержка' }, id).catch((error) => error);
  await received;
  await auth.selectRole('USER');
  assert.equal(isCancel(await delayed), true);
  finish();
  assert.equal(client.getQueryCache().getAll().length, 0);
  const before = requests.length;
  await assert.rejects(options.queryFn({ signal: new AbortController().signal }), ApiError);
  assert.equal(requests.length, before);
});
