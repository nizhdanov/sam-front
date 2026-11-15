import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { useDemoAuth } from '@/app/auth/context';
import { ErrorState, LoadingState, SuccessState } from '@/components/shared/states';
import { useAction } from '@/components/shared/use-action';
import { Button } from '@/components/ui/button';
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Spinner } from '@/components/ui/spinner';

import type { Settings } from './model';

import { fromPercent, settingsFormSchema, toPercent } from './model';
import { saveSettings, settingsOptions } from './queries';

const FIELDS = {
  fuzzy_auto_threshold: {
    label: 'Порог автоматического нечёткого сопоставления, %',
    hint: 'Если сходство имени с продуктом каталога не ниже порога, установка распознаётся без проверки.'
  },
  auto_apply_confidence: {
    label: 'Порог автоматического применения решения ИИ, %',
    hint: 'Предложение ИИ с уверенностью не ниже порога применяется сразу, остальные попадают в очередь.'
  }
} satisfies Record<keyof Settings, { label: string; hint: string }>;

export function SettingsForm() {
  const { auth } = useDemoAuth();
  const queryClient = useQueryClient();
  const query = useQuery(settingsOptions(auth));
  const action = useAction();
  const [errors, setErrors] = useState<Partial<Record<keyof Settings, string>>>({});
  const [saved, setSaved] = useState(false);

  if (query.isPending) return <LoadingState />;
  if (query.isError) return <ErrorState error={query.error} onRetry={() => void query.refetch()} />;

  return (
    <form
      key={JSON.stringify(query.data)}
      aria-label='Пороги сопоставления'
      className='flex max-w-xl flex-col gap-4'
      onSubmit={async (event) => {
        event.preventDefault();
        setSaved(false);
        const parsed = settingsFormSchema.safeParse(
          Object.fromEntries(new FormData(event.currentTarget))
        );
        if (!parsed.success) {
          setErrors(
            Object.fromEntries(parsed.error.issues.map((issue) => [issue.path[0], issue.message]))
          );
          return;
        }
        setErrors({});
        const ok = await action.run(() =>
          saveSettings(auth, queryClient, {
            auto_apply_confidence: fromPercent(parsed.data.auto_apply_confidence),
            fuzzy_auto_threshold: fromPercent(parsed.data.fuzzy_auto_threshold)
          })
        );
        setSaved(ok);
      }}
    >
      <FieldGroup>
        {(Object.keys(FIELDS) as (keyof Settings)[]).map((name) => (
          <Field key={name} data-invalid={!!errors[name]}>
            <FieldLabel htmlFor={`setting-${name}`}>{FIELDS[name].label}</FieldLabel>
            <Input
              required
              aria-invalid={!!errors[name]}
              defaultValue={toPercent(query.data[name])}
              id={`setting-${name}`}
              max={100}
              min={0.1}
              name={name}
              step={0.1}
              type='number'
            />
            <FieldDescription>{FIELDS[name].hint}</FieldDescription>
            <FieldError>{errors[name]}</FieldError>
          </Field>
        ))}
      </FieldGroup>
      {action.error !== undefined && <ErrorState error={action.error} />}
      {saved && <SuccessState>Пороги сохранены на сервере.</SuccessState>}
      <Button className='self-start' disabled={action.pending} type='submit'>
        {action.pending && <Spinner data-icon='inline-start' />}
        Сохранить пороги
      </Button>
    </form>
  );
}
