import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { useDemoAuth } from '@/app/auth/context';
import { ErrorState } from '@/components/shared/states';
import { useAction } from '@/components/shared/use-action';
import { Button } from '@/components/ui/button';
import { Field, FieldLabel } from '@/components/ui/field';
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

import type { MatchingTask } from './model';

import { formatConfidence } from './model';
import { ProductPicker } from './product-picker';
import { approveMatch, rejectMatch } from './queries';

export function TaskReview({ task }: { task: MatchingTask }) {
  const { auth } = useDemoAuth();
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [productId, setProductId] = useState(task.suggested_product_id ?? undefined);
  const [reason, setReason] = useState('');
  const action = useAction();

  async function finish(run: () => Promise<unknown>) {
    if (await action.run(run)) setOpen(false);
  }

  return (
    <Sheet
      open={open}
      onOpenChange={(next, details) => {
        if (action.pending) details.cancel();
        else {
          action.reset();
          setOpen(next);
        }
      }}
    >
      <SheetTrigger render={<Button size='sm' />}>Решить</SheetTrigger>
      <SheetContent className='overflow-y-auto data-[side=right]:w-full data-[side=right]:sm:max-w-lg'>
        <SheetHeader>
          <SheetTitle className='wrap-break-word'>{task.raw_name}</SheetTitle>
          <SheetDescription>
            {[task.raw_publisher, task.raw_version].filter(Boolean).join(' · ') ||
              'Издатель и версия не указаны'}
            {' · '}устройств: {task.affected_devices}
          </SheetDescription>
        </SheetHeader>
        <div className='flex flex-col gap-4 px-4'>
          <p className='text-sm'>
            Уверенность: <strong>{formatConfidence(task.confidence_score)}</strong>
          </p>
          {task.reason && (
            <p className='rounded-xl bg-muted p-3 text-sm wrap-break-word'>{task.reason}</p>
          )}
          <ProductPicker
            candidates={task.candidates}
            suggested={task.suggested_product_id}
            value={productId}
            onChange={setProductId}
          />
          <Field>
            <FieldLabel htmlFor={`reject-${task.id}`}>Причина отказа (необязательно)</FieldLabel>
            <Textarea
              id={`reject-${task.id}`}
              maxLength={500}
              value={reason}
              onChange={(event) => setReason(event.target.value)}
            />
          </Field>
          {action.error !== undefined && <ErrorState error={action.error} />}
        </div>
        <SheetFooter>
          <Button
            disabled={!productId || action.pending}
            onClick={() =>
              productId && void finish(() => approveMatch(auth, queryClient, task.id, productId))
            }
          >
            {action.pending && <Spinner data-icon='inline-start' />}
            Подтвердить и создать правило
          </Button>
          <Button
            disabled={action.pending}
            variant='outline'
            onClick={() =>
              void finish(() => rejectMatch(auth, queryClient, task.id, reason.trim()))
            }
          >
            Отклонить
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
