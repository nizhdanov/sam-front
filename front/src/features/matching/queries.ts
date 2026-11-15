import type { QueryClient } from '@tanstack/react-query';

import { queryOptions } from '@tanstack/react-query';

import type { DemoAuth } from '@/app/auth/session';

import { deleteMatchingRuleById } from '@/generated/api/requests/matching/deleteMatchingRuleById.gen';
import { getMatchingPending } from '@/generated/api/requests/matching/getMatchingPending.gen';
import { getMatchingRules } from '@/generated/api/requests/matching/getMatchingRules.gen';
import { getMatchingSettings } from '@/generated/api/requests/matching/getMatchingSettings.gen';
import { patchMatchingRuleById } from '@/generated/api/requests/matching/patchMatchingRuleById.gen';
import { postMatchingByIdApprove } from '@/generated/api/requests/matching/postMatchingByIdApprove.gen';
import { postMatchingByIdReject } from '@/generated/api/requests/matching/postMatchingByIdReject.gen';
import { putMatchingSettings } from '@/generated/api/requests/matching/putMatchingSettings.gen';
import { pageSchema, parseResponse, requireAdmin } from '@/utils/api/page';

import type { MatchingSearch, Settings } from './model';

import { ruleSchema, settingsSchema, taskSchema } from './model';

const AFFECTED = [
  'matching',
  'installations',
  'devices',
  'findings',
  'balance',
  'dashboard',
  'audit',
  'me'
];

function refresh(queryClient: QueryClient) {
  return Promise.all(
    AFFECTED.map((resource) => queryClient.invalidateQueries({ queryKey: [resource] }))
  );
}

async function mutate<T>(
  auth: DemoAuth,
  queryClient: QueryClient,
  run: (signal: AbortSignal) => Promise<T>
) {
  const session = requireAdmin(auth);
  try {
    return await run(auth.requestConfig(session).signal);
  } finally {
    if (auth.isCurrent(session.epoch)) await refresh(queryClient);
  }
}

export function pendingOptions(auth: DemoAuth, search: MatchingSearch) {
  const session = requireAdmin(auth);
  return queryOptions({
    queryKey: ['matching', 'pending', session.role, session.epoch, search.status, search.page],
    queryFn: async ({ signal }) => {
      auth.requestConfig(session);
      const response = await getMatchingPending({
        query: { status: search.status, page: search.page, limit: 20 },
        config: { signal }
      });
      return parseResponse(
        pageSchema(taskSchema),
        response.data,
        'Сервер вернул некорректную очередь сопоставления.'
      );
    }
  });
}

export function rulesOptions(auth: DemoAuth, search: MatchingSearch) {
  const session = requireAdmin(auth);
  return queryOptions({
    queryKey: ['matching', 'rules', session.role, session.epoch, search.search, search.page],
    queryFn: async ({ signal }) => {
      auth.requestConfig(session);
      const response = await getMatchingRules({
        query: { search: search.search, page: search.page, limit: 50 },
        config: { signal }
      });
      return parseResponse(
        pageSchema(ruleSchema),
        response.data,
        'Сервер вернул некорректные правила.'
      );
    }
  });
}

export function settingsOptions(auth: DemoAuth) {
  const session = requireAdmin(auth);
  return queryOptions({
    queryKey: ['matching', 'settings', session.role, session.epoch],
    queryFn: async ({ signal }) => {
      auth.requestConfig(session);
      const response = await getMatchingSettings({ config: { signal } });
      return parseResponse(settingsSchema, response.data, 'Сервер вернул некорректные пороги.');
    }
  });
}

export function approveMatch(
  auth: DemoAuth,
  queryClient: QueryClient,
  id: string,
  productId: string
) {
  return mutate(auth, queryClient, (signal) =>
    postMatchingByIdApprove({ path: { id }, body: { product_id: productId }, config: { signal } })
  );
}

export function rejectMatch(auth: DemoAuth, queryClient: QueryClient, id: string, reason: string) {
  return mutate(auth, queryClient, (signal) =>
    postMatchingByIdReject({
      path: { id },
      body: { reason: reason || undefined },
      config: { signal }
    })
  );
}

export function retargetRule(
  auth: DemoAuth,
  queryClient: QueryClient,
  id: string,
  productId: string
) {
  return mutate(auth, queryClient, (signal) =>
    patchMatchingRuleById({ path: { id }, body: { product_id: productId }, config: { signal } })
  );
}

export function deleteRule(auth: DemoAuth, queryClient: QueryClient, id: string) {
  return mutate(auth, queryClient, (signal) =>
    deleteMatchingRuleById({ path: { id }, config: { signal } })
  );
}

export async function saveSettings(auth: DemoAuth, queryClient: QueryClient, body: Settings) {
  const session = requireAdmin(auth);
  const response = await putMatchingSettings({ body, config: auth.requestConfig(session) });
  const settings = parseResponse(
    settingsSchema,
    response.data,
    'Сервер вернул некорректные пороги.'
  );
  if (auth.isCurrent(session.epoch))
    queryClient.setQueryData(['matching', 'settings', session.role, session.epoch], settings);
  return settings;
}
