import { z } from 'zod';

import type { Agreement, AgreementInput } from '@/generated/api/types.gen';

import { ApiError } from '@/utils/api/errors';

const licenseTypeSchema = z.enum(['OPEN_SOURCE', 'COMMERCIAL', 'TRIAL']);
const costModelSchema = z.enum(['FREE', 'PAID', 'SHAREWARE']);

const agreementSchema = z.object({
  id: z.uuid(),
  name: z.string().optional(),
  license_type: licenseTypeSchema.optional(),
  cost_model: costModelSchema.optional(),
  agreement_text: z.string().nullable().optional(),
  source_url: z.string().nullable().optional(),
  commercial_allowed: z.boolean().nullable().optional(),
  file_name: z.string().nullable().optional(),
  review_status: z.string().optional(),
  products_count: z.number().int().nonnegative().optional()
}) satisfies z.ZodType<Agreement>;

export function readAgreement(payload: unknown, expectedId?: string) {
  const parsed = agreementSchema.safeParse(payload);
  if (!parsed.success || (expectedId !== undefined && parsed.data.id !== expectedId))
    throw new ApiError('Сервер вернул некорректную карточку соглашения.');
  return parsed.data;
}

export const agreementFormSchema = z
  .object({
    name: z.string().trim().min(1, 'Введите название соглашения.'),
    license_type: z.union([licenseTypeSchema, z.literal('')]),
    cost_model: z.union([costModelSchema, z.literal('')]),
    agreement_text: z.string(),
    source_url: z
      .string()
      .trim()
      .pipe(
        z.union([
          z.url({ protocol: /^https?$/, error: 'Введите адрес с https:// или http://.' }),
          z.literal('')
        ])
      ),
    commercial_allowed: z.enum(['unknown', 'true', 'false'])
  })
  .transform(
    (values) =>
      ({
        name: values.name,
        license_type: values.license_type || undefined,
        cost_model: values.cost_model || undefined,
        agreement_text: values.agreement_text || null,
        source_url: values.source_url || null,
        commercial_allowed:
          values.commercial_allowed === 'unknown' ? null : values.commercial_allowed === 'true'
      }) satisfies AgreementInput
  );

export type AgreementFormValues = z.input<typeof agreementFormSchema>;

export function getAgreementFormValues(agreement?: Agreement): AgreementFormValues {
  return {
    name: agreement?.name ?? '',
    license_type: agreement?.license_type ?? '',
    cost_model: agreement?.cost_model ?? '',
    agreement_text: agreement?.agreement_text ?? '',
    source_url: agreement?.source_url ?? '',
    commercial_allowed:
      agreement?.commercial_allowed == null
        ? 'unknown'
        : agreement.commercial_allowed
          ? 'true'
          : 'false'
  };
}

export function getAgreementFileError(file?: File) {
  if (!file?.name || file.size === 0) return 'Выберите непустой файл соглашения.';
  if (!/\.(?:txt|pdf|docx)$/i.test(file.name)) return 'Допустимы файлы .txt, .pdf и .docx.';
  // shortcut: 10 МБ ограничены 10 000 000 байтами, уточнить при фиксации серверной границы.
  if (file.size > 10_000_000) return 'Размер файла должен быть не больше 10 МБ.';
}
