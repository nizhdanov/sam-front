import type { QueryClient } from '@tanstack/react-query';

import { queryOptions } from '@tanstack/react-query';

import type { DemoAuth } from '@/app/auth/session';
import type { AgreementInput } from '@/generated/api/types.gen';

import { deleteAgreementById } from '@/generated/api/requests/agreements/deleteAgreementById.gen';
import { getAgreementById } from '@/generated/api/requests/agreements/getAgreementById.gen';
import { getAgreements } from '@/generated/api/requests/agreements/getAgreements.gen';
import { patchAgreementById } from '@/generated/api/requests/agreements/patchAgreementById.gen';
import { postAgreementByIdFile } from '@/generated/api/requests/agreements/postAgreementByIdFile.gen';
import { postAgreements } from '@/generated/api/requests/agreements/postAgreements.gen';
import { ApiError } from '@/utils/api/errors';
import { multipartConfig } from '@/utils/api/multipart';
import { pageSchema, parseResponse } from '@/utils/api/page';

import { agreementSchema, getAgreementFileError, readAgreement } from './model';

function requireAdmin(auth: DemoAuth) {
  const session = auth.requireReady();
  if (session.role !== 'ADMIN') throw new ApiError('Доступ запрещён.', { status: 403 });
  return session;
}

function refreshAgreements(queryClient: QueryClient) {
  return Promise.all(
    ['agreements', 'software'].map((resource) =>
      queryClient.invalidateQueries({ queryKey: [resource] })
    )
  );
}

export function agreementDetailOptions(auth: DemoAuth, id: string) {
  const session = requireAdmin(auth);
  return queryOptions({
    queryKey: ['agreements', session.role, session.epoch, id],
    queryFn: async ({ signal }) => {
      auth.requestConfig(session);
      const response = await getAgreementById({ path: { id }, config: { signal } });
      return readAgreement(response.data, id);
    }
  });
}

export async function saveAgreement(
  auth: DemoAuth,
  queryClient: QueryClient,
  body: AgreementInput,
  id?: string
) {
  const session = requireAdmin(auth);
  try {
    const config = auth.requestConfig(session);
    const response = id
      ? await patchAgreementById({ path: { id }, body, config })
      : await postAgreements({ body, config });
    const agreement = readAgreement(response.data, id);
    auth.requestConfig(session);
    queryClient.setQueryData(['agreements', session.role, session.epoch, agreement.id], agreement);
    return agreement;
  } finally {
    if (auth.isCurrent(session.epoch)) await refreshAgreements(queryClient);
  }
}

export async function uploadAgreementFile(
  auth: DemoAuth,
  queryClient: QueryClient,
  id: string,
  file: File
) {
  const session = requireAdmin(auth);
  const error = getAgreementFileError(file);
  if (error) throw new ApiError(error);
  try {
    const body = { file };
    const response = await postAgreementByIdFile({
      path: { id },
      body,
      config: { ...multipartConfig(body), ...auth.requestConfig(session) }
    });
    const agreement = readAgreement(response.data, id);
    auth.requestConfig(session);
    queryClient.setQueryData(['agreements', session.role, session.epoch, id], agreement);
    return agreement;
  } finally {
    if (auth.isCurrent(session.epoch)) await refreshAgreements(queryClient);
  }
}

export async function deleteAgreement(auth: DemoAuth, queryClient: QueryClient, id: string) {
  const session = requireAdmin(auth);
  try {
    const response = await deleteAgreementById({
      path: { id },
      config: auth.requestConfig(session)
    });
    if (response.status !== 204) throw new ApiError('Сервер не подтвердил удаление соглашения.');
    auth.requestConfig(session);
    queryClient.removeQueries({ queryKey: ['agreements', session.role, session.epoch, id] });
  } finally {
    if (auth.isCurrent(session.epoch)) await refreshAgreements(queryClient);
  }
}

export function agreementsListOptions(auth: DemoAuth, page: number, limit = 50) {
  const session = requireAdmin(auth);
  return queryOptions({
    queryKey: ['agreements', 'list', session.role, session.epoch, page, limit],
    queryFn: async ({ signal }) => {
      auth.requestConfig(session);
      const response = await getAgreements({ query: { page, limit }, config: { signal } });
      return parseResponse(
        pageSchema(agreementSchema),
        response.data,
        'Сервер вернул некорректный список соглашений.'
      );
    }
  });
}
