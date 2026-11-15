import { z } from 'zod';

const nullableText = z.string().nullable();

export const matchingSearchSchema = z.object({
  tab: z.enum(['queue', 'rules', 'settings']).default('queue'),
  status: z.enum(['PENDING', 'APPROVED', 'REJECTED']).default('PENDING'),
  search: z
    .string()
    .trim()
    .transform((value) => value || undefined)
    .optional(),
  page: z.coerce.number().int().positive().default(1)
});
export type MatchingSearch = z.infer<typeof matchingSearchSchema>;

export const REVIEW_STATUSES = {
  PENDING: 'Ожидают решения',
  APPROVED: 'Подтверждены',
  REJECTED: 'Отклонены'
};

export const MATCH_METHODS: Record<string, string> = {
  EXACT: 'Точное совпадение',
  FUZZY: 'Нечёткое совпадение',
  LLM: 'ИИ',
  MANUAL: 'Вручную',
  ALIAS: 'Правило'
};

export const candidateSchema = z.object({
  id: z.uuid(),
  canonical_name: z.string(),
  publisher: nullableText.optional(),
  edition: nullableText.optional(),
  current_version: nullableText.optional()
});
export type Candidate = z.infer<typeof candidateSchema>;

export const taskSchema = z.object({
  id: z.uuid(),
  source_key: z.string(),
  raw_name: z.string(),
  raw_publisher: nullableText,
  raw_version: nullableText,
  candidates: z
    .array(candidateSchema)
    .nullable()
    .transform((value) => value ?? []),
  suggested_product_id: z.uuid().nullable(),
  match_method: nullableText,
  confidence_score: z.number().nullable(),
  reason: nullableText,
  status: z.enum(['PENDING', 'APPROVED', 'REJECTED']),
  affected_devices: z.number().int().nonnegative(),
  reviewed_by: nullableText,
  reviewed_at: nullableText,
  created_at: z.string()
});
export type MatchingTask = z.infer<typeof taskSchema>;

export const ruleSchema = z.object({
  id: z.uuid(),
  source_key: z.string(),
  raw_name: z.string(),
  raw_publisher: nullableText,
  raw_version: nullableText,
  product_id: z.uuid(),
  product_name: z.string(),
  version_id: z.uuid().nullable(),
  match_method: z.string(),
  reviewed_by: nullableText,
  created_at: z.string()
});
export type MatchingRule = z.infer<typeof ruleSchema>;

const share = z.number().gt(0).max(1);
export const settingsSchema = z.object({
  auto_apply_confidence: share,
  fuzzy_auto_threshold: share
});
export type Settings = z.infer<typeof settingsSchema>;

export const settingsFormSchema = z.object({
  auto_apply_confidence: z.coerce
    .number()
    .gt(0, 'Порог должен быть больше 0%')
    .max(100, 'Порог не может превышать 100%'),
  fuzzy_auto_threshold: z.coerce
    .number()
    .gt(0, 'Порог должен быть больше 0%')
    .max(100, 'Порог не может превышать 100%')
});

export function toPercent(value: number) {
  return Math.round(value * 1000) / 10;
}

export function fromPercent(value: number) {
  return Math.round(value * 10) / 1000;
}

export function formatConfidence(value: number | null) {
  if (value === null || !Number.isFinite(value)) return 'Нет оценки';
  if (value < 0 || value > 1) return `Некорректная оценка (${value})`;
  return `${toPercent(value)}%`;
}
