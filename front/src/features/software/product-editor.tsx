import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { useState } from 'react';
import { z } from 'zod';

import type { Product } from '@/generated/api/types.gen';

import { useDemoAuth } from '@/app/auth/context';
import { ErrorState } from '@/components/shared/states';
import { useAction } from '@/components/shared/use-action';
import { Button } from '@/components/ui/button';
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
  SheetTrigger
} from '@/components/ui/sheet';
import { Spinner } from '@/components/ui/spinner';
import { Textarea } from '@/components/ui/textarea';
import { agreementsListOptions } from '@/features/agreements/queries';

import { PRODUCT_STATUSES } from './model';
import { saveProduct } from './queries';

const text = z
  .string()
  .trim()
  .transform((value) => value || undefined);

const productFormSchema = z
  .object({
    canonical_name: z.string().trim().min(1, 'Введите наименование продукта.'),
    publisher: text,
    edition: text,
    purpose: text,
    section: text,
    class: text,
    current_version: text,
    in_bpo: z.boolean(),
    agreement_id: z.string(),
    status: z.enum(['ACTUAL', 'OUTDATED', 'FORBIDDEN']),
    status_comment: text
  })
  .refine((value) => value.status !== 'FORBIDDEN' || value.status_comment, {
    path: ['status_comment'],
    message: 'Для запрещённого ПО укажите причину запрета.'
  });

type FormState = z.input<typeof productFormSchema>;
type Errors = Partial<Record<keyof FormState, string>>;

function initial(product?: Product & { id: string }): FormState {
  return {
    canonical_name: product?.canonical_name ?? '',
    publisher: product?.publisher ?? '',
    edition: product?.edition ?? '',
    purpose: product?.purpose ?? '',
    section: product?.section ?? '',
    class: product?.class ?? '',
    current_version: product?.current_version ?? '',
    in_bpo: product?.in_bpo ?? false,
    agreement_id: product?.agreement_id ?? '',
    status: product?.status ?? 'ACTUAL',
    status_comment: product?.status_comment ?? ''
  };
}

const TEXT_FIELDS = [
  ['canonical_name', 'Наименование'],
  ['publisher', 'Издатель'],
  ['edition', 'Редакция'],
  ['current_version', 'Актуальная версия'],
  ['section', 'Раздел классификатора'],
  ['class', 'Класс классификатора']
] as const;

const STATUS_ITEMS = Object.entries(PRODUCT_STATUSES).map(([value, label]) => ({ value, label }));

export function ProductEditor({ product }: { product?: Product & { id: string } }) {
  const { auth } = useDemoAuth();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState(() => initial(product));
  const [errors, setErrors] = useState<Errors>({});
  const action = useAction();
  const agreements = useQuery({ ...agreementsListOptions(auth, 1, 200), enabled: open });
  const agreementItems = [
    { value: '', label: 'Не выбрано' },
    ...(agreements.data?.items ?? []).map((item) => ({
      value: item.id,
      label: item.name || 'Без названия'
    }))
  ];

  const set = <K extends keyof FormState>(key: K, value: FormState[K]) => {
    setForm((current) => ({ ...current, [key]: value }));
    setErrors((current) => ({ ...current, [key]: undefined }));
  };

  async function submit() {
    const parsed = productFormSchema.safeParse(form);
    if (!parsed.success) {
      setErrors(
        Object.fromEntries(parsed.error.issues.map((issue) => [issue.path[0], issue.message]))
      );
      return;
    }
    const { agreement_id, current_version, status_comment, ...rest } = parsed.data;
    const body = {
      ...rest,
      agreement_id: agreement_id || null,
      current_version: current_version ?? null,
      status_comment: rest.status === 'FORBIDDEN' ? status_comment : (status_comment ?? '')
    };
    let saved: { id: string } | undefined;
    const ok = await action.run(async () => {
      saved = await saveProduct(auth, queryClient, body, product?.id);
    });
    if (!ok || !saved) return;
    setOpen(false);
    if (!product) void navigate({ to: '/software/$productId', params: { productId: saved.id } });
  }

  return (
    <Sheet
      open={open}
      onOpenChange={(next, details) => {
        if (action.pending) return details.cancel();
        if (next) {
          setForm(initial(product));
          setErrors({});
          action.reset();
        }
        setOpen(next);
      }}
    >
      <SheetTrigger render={<Button variant={product ? 'outline' : 'default'} />}>
        {product ? 'Редактировать' : 'Добавить продукт'}
      </SheetTrigger>
      <SheetContent className='overflow-y-auto data-[side=right]:w-full data-[side=right]:sm:max-w-xl'>
        <SheetHeader>
          <SheetTitle>{product ? 'Карточка продукта' : 'Новый продукт'}</SheetTitle>
          <SheetDescription>
            Эталонная запись каталога, с которой сопоставляются установки.
          </SheetDescription>
        </SheetHeader>
        <form
          noValidate
          className='flex flex-col gap-5 px-4'
          id='product-form'
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          <FieldGroup className='grid gap-4 sm:grid-cols-2'>
            {TEXT_FIELDS.map(([name, label]) => (
              <Field
                key={name}
                className={
                  ['canonical_name', 'class', 'section'].includes(name)
                    ? 'sm:col-span-2'
                    : undefined
                }
                data-invalid={!!errors[name]}
              >
                <FieldLabel htmlFor={`product-${name}`}>{label}</FieldLabel>
                <Input
                  aria-invalid={!!errors[name]}
                  id={`product-${name}`}
                  required={name === 'canonical_name'}
                  value={form[name] ?? ''}
                  onChange={(event) => set(name, event.target.value)}
                />
                <FieldError>{errors[name]}</FieldError>
              </Field>
            ))}
            <Field className='sm:col-span-2'>
              <FieldLabel htmlFor='product-purpose'>Назначение</FieldLabel>
              <Textarea
                id='product-purpose'
                rows={2}
                value={form.purpose ?? ''}
                onChange={(event) => set('purpose', event.target.value)}
              />
            </Field>
            <Field className='sm:col-span-2'>
              <FieldLabel htmlFor='product-agreement'>Лицензионное соглашение</FieldLabel>
              <Select
                items={agreementItems}
                value={form.agreement_id}
                onValueChange={(value) => set('agreement_id', value ?? '')}
              >
                <SelectTrigger className='w-full' id='product-agreement'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    {agreementItems.map((item) => (
                      <SelectItem key={item.value} value={item.value}>
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
              {agreements.isError && (
                <FieldDescription>Не удалось загрузить справочник соглашений.</FieldDescription>
              )}
            </Field>
            <Field>
              <FieldLabel htmlFor='product-status'>Статус</FieldLabel>
              <Select
                items={STATUS_ITEMS}
                value={form.status}
                onValueChange={(value) => value && set('status', value as FormState['status'])}
              >
                <SelectTrigger className='w-full' id='product-status'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    {STATUS_ITEMS.map((item) => (
                      <SelectItem key={item.value} value={item.value}>
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
            <Field className='items-center self-end pb-2' orientation='horizontal'>
              <input
                checked={form.in_bpo}
                className='size-4 accent-primary'
                id='product-in-bpo'
                type='checkbox'
                onChange={(event) => set('in_bpo', event.target.checked)}
              />
              <FieldLabel htmlFor='product-in-bpo'>В перечне БПО</FieldLabel>
            </Field>
            {form.status === 'FORBIDDEN' && (
              <Field className='sm:col-span-2' data-invalid={!!errors.status_comment}>
                <FieldLabel htmlFor='product-status-comment'>Причина запрета</FieldLabel>
                <Textarea
                  aria-invalid={!!errors.status_comment}
                  id='product-status-comment'
                  rows={2}
                  value={form.status_comment ?? ''}
                  onChange={(event) => set('status_comment', event.target.value)}
                />
                <FieldError>{errors.status_comment}</FieldError>
              </Field>
            )}
          </FieldGroup>
          {action.error !== undefined && <ErrorState error={action.error} />}
        </form>
        <SheetFooter>
          <Button disabled={action.pending} form='product-form' type='submit'>
            {action.pending && <Spinner data-icon='inline-start' />}
            {product ? 'Сохранить изменения' : 'Создать продукт'}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
