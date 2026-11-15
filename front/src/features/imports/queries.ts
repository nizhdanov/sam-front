import type { QueryClient } from '@tanstack/react-query';

import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

import type { DemoAuth } from '@/app/auth/session';

import { getImportById } from '@/generated/api/requests/imports/getImportById.gen';
import { getImportByIdPreview } from '@/generated/api/requests/imports/getImportByIdPreview.gen';
import { getImports } from '@/generated/api/requests/imports/getImports.gen';
import { postImportByIdCancel } from '@/generated/api/requests/imports/postImportByIdCancel.gen';
import { postImportByIdConfirm } from '@/generated/api/requests/imports/postImportByIdConfirm.gen';
import { postImportByIdRetry } from '@/generated/api/requests/imports/postImportByIdRetry.gen';
import { postImports } from '@/generated/api/requests/imports/postImports.gen';
import { ApiError } from '@/utils/api/errors';
import { multipartConfig } from '@/utils/api/multipart';

import {
  getCsvFileError,
  isImportTerminal,
  readImport,
  readImportPreview,
  readImportsPage,
  readImportUpload
} from './model';

function requireAdmin(auth: DemoAuth) {
  const session = auth.requireReady();
  if (session.role !== 'ADMIN') throw new ApiError('Доступ запрещён.', { status: 403 });
  return session;
}

export function refreshInventory(queryClient: QueryClient) {
  return Promise.all(
    [
      'devices',
      'installations',
      'software',
      'products',
      'findings',
      'matching',
      'dashboard',
      'balance',
      'compliance',
      'audit'
    ].map((resource) => queryClient.invalidateQueries({ queryKey: [resource] }))
  );
}

export function importsListOptions(auth: DemoAuth, search: { page: number; limit: number }) {
  const session = requireAdmin(auth);
  return queryOptions({
    queryKey: ['imports', session.role, session.epoch, 'list', search],
    queryFn: async ({ signal }) => {
      auth.requestConfig(session);
      const response = await getImports({ query: search, config: { signal } });
      return readImportsPage(response.data);
    }
  });
}

export function importDetailOptions(auth: DemoAuth, id: string) {
  const session = requireAdmin(auth);
  return queryOptions({
    queryKey: ['imports', session.role, session.epoch, 'detail', id],
    queryFn: async ({ signal }) => {
      auth.requestConfig(session);
      const response = await getImportById({ path: { id }, config: { signal } });
      return readImport(response.data, id);
    },
    refetchInterval: ({ state }) => {
      if (
        !auth.isCurrent(session.epoch) ||
        !state.data ||
        state.data.status === 'AWAITING_CONFIRMATION' ||
        isImportTerminal(state.data.status) ||
        (state.error instanceof ApiError &&
          (state.error.code === 'INVALID_RESPONSE' ||
            (state.error.status !== undefined && state.error.status < 500)))
      )
        return false;
      return Math.min(30_000, 2_000 * 2 ** state.fetchFailureCount);
    },
    refetchIntervalInBackground: false
  });
}

export function importPreviewOptions(auth: DemoAuth, id: string) {
  const session = requireAdmin(auth);
  return queryOptions({
    queryKey: ['imports', session.role, session.epoch, 'preview', id],
    queryFn: async ({ signal }) => {
      auth.requestConfig(session);
      const response = await getImportByIdPreview({ path: { id }, config: { signal } });
      return readImportPreview(response.data);
    }
  });
}

export async function uploadCsv(
  auth: DemoAuth,
  queryClient: QueryClient,
  file: File,
  collectedAt?: string
) {
  const session = requireAdmin(auth);
  const error = getCsvFileError(file);
  if (error) throw new ApiError(error);
  if (collectedAt !== undefined && !z.iso.datetime({ offset: true }).safeParse(collectedAt).success)
    throw new ApiError('Укажите корректную дату и время выгрузки.');
  const body = { file };
  try {
    const response = await postImports({
      body,
      query: { confirm: false, collected_at: collectedAt },
      config: { ...multipartConfig(body), ...auth.requestConfig(session) }
    });
    const result = readImportUpload(response.data);
    auth.requestConfig(session);
    queryClient.setQueryData(
      ['imports', session.role, session.epoch, 'preview', result.import_id],
      result.preview
    );
    return result;
  } finally {
    if (auth.isCurrent(session.epoch))
      await queryClient.invalidateQueries({
        queryKey: ['imports', session.role, session.epoch, 'list']
      });
  }
}

export async function runImportAction(
  auth: DemoAuth,
  queryClient: QueryClient,
  id: string,
  action: 'cancel' | 'confirm' | 'retry'
) {
  const session = requireAdmin(auth);
  const request = { path: { id }, config: auth.requestConfig(session) };
  try {
    const response = await {
      confirm: postImportByIdConfirm,
      cancel: postImportByIdCancel,
      retry: postImportByIdRetry
    }[action](request);
    const result = readImport(response.data, id);
    auth.requestConfig(session);
    queryClient.setQueryData(['imports', session.role, session.epoch, 'detail', id], result);
    return result;
  } finally {
    if (auth.isCurrent(session.epoch))
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: ['imports', session.role, session.epoch, 'detail', id]
        }),
        queryClient.invalidateQueries({
          queryKey: ['imports', session.role, session.epoch, 'list']
        })
      ]);
  }
}
