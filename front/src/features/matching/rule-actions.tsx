import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { useDemoAuth } from '@/app/auth/context';
import { ErrorState } from '@/components/shared/states';
import { useAction } from '@/components/shared/use-action';
import { Button } from '@/components/ui/button';
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

import type { MatchingRule } from './model';

import { ProductPicker } from './product-picker';
import { deleteRule, retargetRule } from './queries';

export function RuleActions({ rule }: { rule: MatchingRule }) {
  const { auth } = useDemoAuth();
  const queryClient = useQueryClient();
  const [mode, setMode] = useState<'delete' | 'edit' | null>(null);
  const [productId, setProductId] = useState<string>();
  const action = useAction();

  return (
    <Sheet
      open={mode !== null}
      onOpenChange={(next, details) => {
        if (action.pending) details.cancel();
        else if (!next) {
          action.reset();
          setMode(null);
        }
      }}
    >
      <div className='flex flex-wrap gap-2'>
        <SheetTrigger
          render={<Button size='sm' variant='outline' onClick={() => setMode('edit')} />}
        >
          Перепривязать
        </SheetTrigger>
        <SheetTrigger
          render={<Button size='sm' variant='destructive' onClick={() => setMode('delete')} />}
        >
          Удалить
        </SheetTrigger>
      </div>
      <SheetContent className='overflow-y-auto data-[side=right]:w-full data-[side=right]:sm:max-w-lg'>
        <SheetHeader>
          <SheetTitle className='wrap-break-word'>
            {mode === 'delete' ? 'Удалить правило?' : 'Перепривязать правило'}
          </SheetTitle>
          <SheetDescription className='wrap-break-word'>
            «{rule.raw_name}» → {rule.product_name}.{' '}
            {mode === 'delete'
              ? 'Установки с этим именем вернутся на повторное сопоставление.'
              : 'Имя из ОС не меняется, меняется только целевой продукт.'}
          </SheetDescription>
        </SheetHeader>
        <div className='flex flex-col gap-4 px-4'>
          {mode === 'edit' && <ProductPicker value={productId} onChange={setProductId} />}
          {action.error !== undefined && <ErrorState error={action.error} />}
        </div>
        <SheetFooter>
          <Button
            disabled={action.pending || (mode === 'edit' && !productId)}
            variant={mode === 'delete' ? 'destructive' : 'default'}
            onClick={async () => {
              const ok = await action.run(() =>
                mode === 'delete'
                  ? deleteRule(auth, queryClient, rule.id)
                  : retargetRule(auth, queryClient, rule.id, productId!)
              );
              if (ok) setMode(null);
            }}
          >
            {action.pending && <Spinner data-icon='inline-start' />}
            {mode === 'delete' ? 'Удалить' : 'Сохранить'}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
