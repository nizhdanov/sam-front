import type { QueryClient } from '@tanstack/react-query';

import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

import type { DemoAuth } from '@/app/auth/session';

import { getAgreementByIdAnalysis } from '@/generated/api/requests/agreements/getAgreementByIdAnalysis.gen';
import { postAgreementByIdAnalysisApprove } from '@/generated/api/requests/agreements/postAgreementByIdAnalysisApprove.gen';
import { postAgreementByIdAnalyze } from '@/generated/api/requests/agreements/postAgreementByIdAnalyze.gen';
import { ApiError } from '@/utils/api/errors';
import { parseResponse, requireAdmin } from '@/utils/api/page';

const paragraphs = z
  .array(z.number().int())
  .nullish()
  .transform((value) => value ?? []);
const text = z
  .string()
  .nullish()
  .transform((value) => value ?? '');
const items = <T extends z.ZodType>(item: T) =>
  z
    .array(item)
    .nullish()
    .transform((value) => value ?? []);

const resultSchema = z.object({
  license_type: z.enum(['OPEN_SOURCE', 'COMMERCIAL', 'TRIAL']).nullish(),
  cost_model: z.enum(['FREE', 'PAID', 'SHAREWARE']).nullish(),
  license_type_evidence: z.object({ paragraphs, quote: text }).nullish(),
  commercial_use: z
    .object({
      value: z.enum(['YES', 'NO', 'UNDEFINED']).catch('UNDEFINED'),
      paragraphs,
      quote: text
    })
    .nullish(),
  subscriptions: items(z.object({ kind: z.string(), description: text, paragraphs, quote: text })),
  restrictions: items(z.object({ type: text, description: text, paragraphs, quote: text })),
  risks: items(z.object({ level: z.string(), description: text, paragraphs, quote: text })),
  summary: text
});

const analysisSchema = z.object({
  id: z.uuid(),
  agreement_id: z.uuid(),
  model: z.string(),
  status: z.string(),
  reviewed_by: z.string().nullable(),
  reviewed_at: z.string().nullable(),
  created_at: z.string(),
  result: resultSchema,
  paragraphs: z.array(z.string()),
  similar_agreements: z
    .array(
      z.object({
        id: z.uuid(),
        name: z.string(),
        license_type: z.string(),
        cost_model: z.string(),
        similarity: z.number()
      })
    )
    .nullish()
    .transform((value) => (value ?? []).slice(0, 3))
});

export type AgreementAnalysis = z.infer<typeof analysisSchema>;
export type AnalysisResult = AgreementAnalysis['result'];

export const COMMERCIAL_USE = {
  YES: 'Разрешено',
  NO: 'Запрещено',
  UNDEFINED: 'Не определено'
};
export const SUBSCRIPTION_KINDS: Record<string, string> = {
  HIDDEN_SUBSCRIPTION: 'Скрытая подписка',
  AUTO_RENEWAL: 'Автопродление'
};
export const RISK_LEVELS: Record<string, string> = {
  LOW: 'Низкий',
  MEDIUM: 'Средний',
  HIGH: 'Высокий'
};

export function paragraphIndex(number: number, total: number) {
  return Number.isInteger(number) && number >= 1 && number <= total ? number - 1 : undefined;
}

function read(payload: unknown) {
  return parseResponse(analysisSchema, payload, 'Сервер вернул некорректный результат анализа.');
}

export function analysisOptions(auth: DemoAuth, id: string) {
  const session = requireAdmin(auth);
  return queryOptions({
    queryKey: ['agreements', 'analysis', session.role, session.epoch, id],
    queryFn: async ({ signal }) => {
      auth.requestConfig(session);
      try {
        const response = await getAgreementByIdAnalysis({ path: { id }, config: { signal } });
        return read(response.data);
      } catch (error) {
        if (error instanceof ApiError && error.status === 404) return null;
        throw error;
      }
    },
    retry: false
  });
}

export async function analyzeAgreement(
  auth: DemoAuth,
  queryClient: QueryClient,
  id: string,
  force: boolean
) {
  const session = requireAdmin(auth);
  const response = await postAgreementByIdAnalyze({
    path: { id },
    query: { force },
    config: { ...auth.requestConfig(session), timeout: 180_000 }
  });
  const analysis = read(response.data);
  if (auth.isCurrent(session.epoch)) {
    queryClient.setQueryData(['agreements', 'analysis', session.role, session.epoch, id], analysis);
    void queryClient.invalidateQueries({ queryKey: ['audit'] });
  }
  return analysis;
}

export async function approveAnalysis(auth: DemoAuth, queryClient: QueryClient, id: string) {
  const session = requireAdmin(auth);
  try {
    const response = await postAgreementByIdAnalysisApprove({
      path: { id },
      config: auth.requestConfig(session)
    });
    return read(response.data);
  } finally {
    if (auth.isCurrent(session.epoch))
      await Promise.all(
        ['agreements', 'software', 'balance', 'findings', 'dashboard', 'audit'].map((resource) =>
          queryClient.invalidateQueries({ queryKey: [resource] })
        )
      );
  }
}
