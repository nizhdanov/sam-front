import { useBlocker } from '@tanstack/react-router';

import { Button } from '@/components/ui/button';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle
} from '@/components/ui/sheet';

export function FormLeaveGuard({ when }: { when: () => boolean }) {
  const blocker = useBlocker({ shouldBlockFn: when, enableBeforeUnload: when, withResolver: true });
  return (
    <Sheet
      open={blocker.status === 'blocked'}
      onOpenChange={(open) => {
        if (!open) blocker.reset?.();
      }}
    >
      <SheetContent>
        <SheetHeader>
          <SheetTitle>Покинуть форму?</SheetTitle>
          <SheetDescription>
            Несохранённые изменения будут потеряны. Уже отправленный запрос может быть принят
            сервером; проверьте состояние после возвращения.
          </SheetDescription>
        </SheetHeader>
        <SheetFooter>
          <Button variant='outline' onClick={() => blocker.reset?.()}>
            Продолжить редактирование
          </Button>
          <Button variant='destructive' onClick={() => blocker.proceed?.()}>
            Уйти со страницы
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
