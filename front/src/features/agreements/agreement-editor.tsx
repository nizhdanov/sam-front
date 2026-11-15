import { zodResolver } from '@hookform/resolvers/zod';
import { useQueryClient } from '@tanstack/react-query';
import { useBlocker, useNavigate } from '@tanstack/react-router';
import { useEffect, useRef, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';

import type { Agreement, AgreementInput } from '@/generated/api/types.gen';

import { useDemoAuth } from '@/app/auth/context';
import { ErrorState, SuccessState } from '@/components/shared/states';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Field, FieldError, FieldGroup, FieldLabel, FieldSet } from '@/components/ui/field';
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
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger
} from '@/components/ui/sheet';
import { Spinner } from '@/components/ui/spinner';
import { Textarea } from '@/components/ui/textarea';
import { COST_MODELS, LICENSE_TYPES } from '@/features/software/model';

import type { AgreementFormValues } from './model';

import { agreementFormSchema, getAgreementFormValues } from './model';
import { saveAgreement } from './queries';

const SELECT_FIELDS = [
  { name: 'license_type', label: 'Тип лицензии', options: LICENSE_TYPES },
  { name: 'cost_model', label: 'Модель оплаты', options: COST_MODELS },
  {
    name: 'commercial_allowed',
    label: 'Коммерческое использование',
    options: { unknown: 'Не определено', true: 'Разрешено', false: 'Запрещено' }
  }
] as const;

export function AgreementEditor({
  agreement,
  disabled = false
}: {
  agreement?: Agreement & { id: string };
  disabled?: boolean;
}) {
  const { auth } = useDemoAuth();
  const session = auth.requireReady();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const [error, setError] = useState<unknown>();
  const [saved, setSaved] = useState(false);
  const [confirmClose, setConfirmClose] = useState(false);
  const completedRef = useRef(false);
  const continueRef = useRef<HTMLButtonElement>(null);
  const form = useForm<AgreementFormValues, unknown, AgreementInput>({
    resolver: zodResolver(agreementFormSchema),
    defaultValues: getAgreementFormValues(agreement)
  });
  const { errors, isDirty, isSubmitting } = form.formState;

  const blocker = useBlocker({
    shouldBlockFn: () =>
      auth.isCurrent(session.epoch) && open && !completedRef.current && (isDirty || isSubmitting),
    enableBeforeUnload: () =>
      auth.isCurrent(session.epoch) && open && !completedRef.current && (isDirty || isSubmitting),
    withResolver: true
  });
  const confirming = confirmClose || blocker.status === 'blocked';
  useEffect(() => {
    if (confirming) continueRef.current?.focus();
  }, [confirming]);

  return (
    <div className='flex flex-col items-start gap-3'>
      <Sheet
        open={open}
        onOpenChange={(nextOpen, details) => {
          if (!nextOpen && (isSubmitting || isDirty)) {
            if (!isSubmitting) setConfirmClose(true);
            details.cancel();
            return;
          }
          if (nextOpen) {
            completedRef.current = false;
            form.reset(getAgreementFormValues(agreement));
            setError(undefined);
            setSaved(false);
            setConfirmClose(false);
          }
          setOpen(nextOpen);
        }}
      >
        <SheetTrigger
          render={<Button disabled={disabled} variant={agreement ? 'outline' : 'default'} />}
        >
          {agreement ? 'Редактировать соглашение' : 'Создать соглашение'}
        </SheetTrigger>
        <SheetContent className='overflow-y-auto data-[side=right]:w-full data-[side=right]:sm:max-w-xl'>
          <SheetHeader>
            <SheetTitle>{agreement ? 'Изменение соглашения' : 'Новое соглашение'}</SheetTitle>
            <SheetDescription>
              Укажите условия использования. Файл можно загрузить в сохранённой карточке.
            </SheetDescription>
          </SheetHeader>
          <form
            noValidate
            aria-label='Форма соглашения'
            className='flex flex-col gap-6 px-6 pb-6'
            onSubmit={form.handleSubmit(async (values) => {
              if (!auth.isCurrent(session.epoch)) return;
              setError(undefined);
              try {
                const result = await saveAgreement(auth, queryClient, values, agreement?.id);
                if (!auth.isCurrent(session.epoch)) return;
                completedRef.current = true;
                blocker.reset?.();
                form.reset(getAgreementFormValues(result));
                setOpen(false);
                setSaved(true);
                if (!agreement)
                  void navigate({
                    to: '/agreements/$agreementId',
                    params: { agreementId: result.id }
                  });
              } catch (saveError) {
                if (auth.isCurrent(session.epoch)) setError(saveError);
              }
            })}
          >
            <FieldSet disabled={isSubmitting}>
              <FieldGroup>
                <Field data-disabled={isSubmitting} data-invalid={!!errors.name}>
                  <FieldLabel htmlFor='agreement-name'>Название</FieldLabel>
                  <Input
                    {...form.register('name')}
                    required
                    aria-describedby={errors.name ? 'agreement-name-error' : undefined}
                    aria-invalid={!!errors.name}
                    id='agreement-name'
                  />
                  <FieldError errors={[errors.name]} id='agreement-name-error' />
                </Field>
                {SELECT_FIELDS.map(({ name, label, options }) => {
                  const items = Object.entries(options).map(([value, text]) => ({
                    value,
                    label: text
                  }));
                  if (name !== 'commercial_allowed' && !agreement?.[name])
                    items.unshift({ value: '', label: 'Не указано' });
                  return (
                    <Controller
                      key={name}
                      render={({ field, fieldState }) => (
                        <Field data-disabled={isSubmitting} data-invalid={fieldState.invalid}>
                          <FieldLabel htmlFor={`agreement-${name}`}>{label}</FieldLabel>
                          <Select
                            disabled={isSubmitting}
                            items={items}
                            name={field.name}
                            value={field.value}
                            onValueChange={field.onChange}
                          >
                            <SelectTrigger
                              ref={field.ref}
                              aria-describedby={
                                fieldState.invalid ? `agreement-${name}-error` : undefined
                              }
                              aria-invalid={fieldState.invalid}
                              className='w-full'
                              id={`agreement-${name}`}
                              onBlur={field.onBlur}
                            >
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectGroup>
                                {items.map((item) => (
                                  <SelectItem key={item.value} value={item.value}>
                                    {item.label}
                                  </SelectItem>
                                ))}
                              </SelectGroup>
                            </SelectContent>
                          </Select>
                          <FieldError errors={[fieldState.error]} id={`agreement-${name}-error`} />
                        </Field>
                      )}
                      control={form.control}
                      name={name}
                    />
                  );
                })}
                <Field data-disabled={isSubmitting}>
                  <FieldLabel htmlFor='agreement-text'>Текст соглашения</FieldLabel>
                  <Textarea {...form.register('agreement_text')} id='agreement-text' rows={10} />
                </Field>
                <Field data-disabled={isSubmitting} data-invalid={!!errors.source_url}>
                  <FieldLabel htmlFor='agreement-source'>Ссылка на источник</FieldLabel>
                  <Input
                    {...form.register('source_url')}
                    aria-describedby={errors.source_url ? 'agreement-source-error' : undefined}
                    aria-invalid={!!errors.source_url}
                    id='agreement-source'
                    type='url'
                  />
                  <FieldError errors={[errors.source_url]} id='agreement-source-error' />
                </Field>
              </FieldGroup>
              <div className='flex flex-wrap gap-2'>
                <Button disabled={isSubmitting} type='submit'>
                  {isSubmitting && <Spinner data-icon='inline-start' />}
                  {isSubmitting ? 'Сохранение…' : 'Сохранить'}
                </Button>
                <SheetClose render={<Button disabled={isSubmitting} variant='outline' />}>
                  Отмена
                </SheetClose>
              </div>
            </FieldSet>
            {error !== undefined && <ErrorState error={error} />}
            {confirming && (
              <Alert>
                <AlertTitle>Закрыть форму без сохранения изменений?</AlertTitle>
                <AlertDescription>
                  <p>Введённые изменения будут потеряны.</p>
                  <div className='flex flex-wrap gap-2'>
                    <Button
                      ref={continueRef}
                      disabled={isSubmitting}
                      variant='outline'
                      onClick={() => {
                        setConfirmClose(false);
                        blocker.reset?.();
                        form.setFocus('name');
                      }}
                    >
                      Продолжить редактирование
                    </Button>
                    <Button
                      disabled={isSubmitting}
                      variant='destructive'
                      onClick={() => {
                        completedRef.current = true;
                        form.reset(getAgreementFormValues(agreement));
                        setConfirmClose(false);
                        setOpen(false);
                        blocker.proceed?.();
                      }}
                    >
                      Закрыть без сохранения
                    </Button>
                  </div>
                </AlertDescription>
              </Alert>
            )}
          </form>
        </SheetContent>
      </Sheet>
      {saved && <SuccessState>Соглашение сохранено.</SuccessState>}
    </div>
  );
}
