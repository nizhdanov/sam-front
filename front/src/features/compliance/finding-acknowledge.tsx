import type { z } from 'zod';

import { zodResolver } from '@hookform/resolvers/zod';
import { useQueryClient } from '@tanstack/react-query';
import { useRef, useState } from 'react';
import { useForm } from 'react-hook-form';

import { useDemoAuth } from '@/app/auth/context';
import { FormLeaveGuard } from '@/components/shared/form-leave-guard';
import { ErrorState } from '@/components/shared/states';
import { Button } from '@/components/ui/button';
import { Field, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle
} from '@/components/ui/sheet';
import { Spinner } from '@/components/ui/spinner';
import { Textarea } from '@/components/ui/textarea';

import { acknowledgeSchema } from './model';
import { acknowledgeFinding } from './queries';

export function FindingAcknowledge({
  id,
  trigger,
  onClose,
  onSuccess
}: {
  id: string;
  trigger: HTMLButtonElement;
  onClose: () => void;
  onSuccess: () => void;
}) {
  const { auth } = useDemoAuth();
  const session = auth.requireReady();
  const queryClient = useQueryClient();
  const [error, setError] = useState<unknown>();
  const [discarding, setDiscarding] = useState(false);
  const submittingRef = useRef(false);
  const form = useForm<z.input<typeof acknowledgeSchema>>({
    resolver: zodResolver(acknowledgeSchema),
    defaultValues: { comment: '' }
  });
  const { isDirty, isSubmitting, errors } = form.formState;
  if (session.role !== 'ADMIN') return null;

  function close() {
    if (isSubmitting) return;
    if (isDirty) {
      setDiscarding(true);
      return;
    }
    onClose();
  }

  return (
    <Sheet
      open
      onOpenChange={(open) => {
        if (!open) close();
      }}
    >
      <SheetContent
        className='overflow-y-auto'
        finalFocus={() => trigger}
        showCloseButton={!isSubmitting}
      >
        <SheetHeader>
          <SheetTitle>
            {discarding ? 'Удалить несохранённый комментарий?' : 'Отметить нарушение обработанным'}
          </SheetTitle>
          <SheetDescription>
            Комментарий будет сохранён с автором и временем. Отметка об обработке не устраняет риск.
          </SheetDescription>
        </SheetHeader>
        <FormLeaveGuard when={() => auth.isCurrent(session.epoch) && (isDirty || isSubmitting)} />
        {discarding ? (
          <div className='flex flex-col gap-3 px-6 pb-6'>
            <Button onClick={() => setDiscarding(false)}>Продолжить редактирование</Button>
            <Button variant='destructive' onClick={onClose}>
              Удалить комментарий и закрыть
            </Button>
          </div>
        ) : (
          <form
            aria-label='Обработка нарушения'
            className='flex flex-col gap-4 px-6 pb-6'
            onSubmit={(event) => {
              if (submittingRef.current || !auth.isCurrent(session.epoch)) {
                event.preventDefault();
                return;
              }
              submittingRef.current = true;
              void form
                .handleSubmit(async ({ comment }) => {
                  setError(undefined);
                  try {
                    await acknowledgeFinding(auth, queryClient, id, comment);
                    if (!auth.isCurrent(session.epoch)) return;
                    form.reset();
                    onSuccess();
                  } catch (cause) {
                    if (auth.isCurrent(session.epoch)) setError(cause);
                  }
                })(event)
                .finally(() => {
                  submittingRef.current = false;
                });
            }}
          >
            <FieldGroup>
              <Field data-disabled={isSubmitting} data-invalid={!!errors.comment}>
                <FieldLabel htmlFor='finding-comment'>Комментарий</FieldLabel>
                <Textarea
                  {...form.register('comment')}
                  required
                  aria-invalid={!!errors.comment}
                  disabled={isSubmitting}
                  id='finding-comment'
                  rows={5}
                />
                <FieldError errors={[errors.comment]} />
              </Field>
            </FieldGroup>
            {error !== undefined && <ErrorState error={error} />}
            <Button disabled={isSubmitting} type='submit'>
              {isSubmitting && <Spinner data-icon='inline-start' />}Сохранить обработку
            </Button>
            <Button disabled={isSubmitting} variant='outline' onClick={close}>
              Отмена
            </Button>
          </form>
        )}
      </SheetContent>
    </Sheet>
  );
}
