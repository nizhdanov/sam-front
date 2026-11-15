import type { ReactNode } from 'react';

import { Link } from '@tanstack/react-router';
import { MenuIcon } from 'lucide-react';
import { useState } from 'react';

import { useDemoAuth } from '@/app/auth/context';
import { DEMO_ACCOUNTS } from '@/app/auth/demo-accounts';
import { getNavigation } from '@/app/auth/permissions';
import { ErrorState, LoadingState } from '@/components/shared/states';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
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
  SheetHeader,
  SheetTitle,
  SheetTrigger
} from '@/components/ui/sheet';

const ROLE_ITEMS = DEMO_ACCOUNTS.map((account) => ({ value: account.role, label: account.label }));

function Navigation({ onNavigate }: { onNavigate?: () => void }) {
  const { snapshot } = useDemoAuth();
  return (
    <nav aria-label='Основная навигация' className='flex flex-col gap-1'>
      {getNavigation(snapshot.status === 'ready' ? snapshot.role : null).map((item) => (
        <Link
          key={item.to}
          activeProps={{
            className: 'bg-sidebar-accent font-semibold text-sidebar-accent-foreground',
            'aria-current': 'page'
          }}
          className='rounded-xl px-3 py-2 text-sm outline-none hover:bg-sidebar-accent focus-visible:ring-2 focus-visible:ring-ring'
          to={item.to}
          onClick={onNavigate}
        >
          {item.label}
        </Link>
      ))}
    </nav>
  );
}

function MobileNavigation() {
  const [open, setOpen] = useState(false);
  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger render={<Button aria-label='Открыть меню' size='icon' variant='outline' />}>
        <MenuIcon />
      </SheetTrigger>
      <SheetContent className='overflow-y-auto' side='left'>
        <SheetHeader>
          <SheetTitle>Разделы SAM</SheetTitle>
          <SheetDescription>Управление программными активами</SheetDescription>
        </SheetHeader>
        <div className='px-3 pb-6'>
          <Navigation onNavigate={() => setOpen(false)} />
        </div>
      </SheetContent>
    </Sheet>
  );
}

export function AppShell({ children }: { children: ReactNode }) {
  const { auth, snapshot } = useDemoAuth();
  return (
    <div className='flex min-h-svh flex-col'>
      <a className='sr-only focus:not-sr-only focus:p-3' href='#main-content'>
        Перейти к содержимому
      </a>
      <header className='flex flex-wrap items-center justify-between gap-3 border-b bg-card px-4 py-3 md:px-6'>
        <div className='flex items-center gap-3'>
          <div className='md:hidden'>
            <MobileNavigation key={snapshot.epoch} />
          </div>
          <Link
            aria-label='SAM — главная'
            className='rounded-md text-xl font-bold text-primary outline-none focus-visible:ring-2 focus-visible:ring-ring'
            to='/'
          >
            SAM
          </Link>
          <span className='hidden text-sm text-muted-foreground lg:block'>
            Управление программными активами
          </span>
        </div>
        <Field className='w-auto gap-1'>
          <Select
            disabled={snapshot.status === 'switching'}
            items={ROLE_ITEMS}
            value={snapshot.status === 'error' ? null : snapshot.role}
            onValueChange={(role) => {
              if (role) void auth.selectRole(role);
            }}
          >
            <SelectTrigger id='demo-role'>
              <SelectValue placeholder='Выберите роль' />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {ROLE_ITEMS.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>
      </header>
      <div className='flex flex-1'>
        <aside className='hidden w-60 shrink-0 border-r bg-sidebar p-3 text-sidebar-foreground md:block'>
          <Navigation />
        </aside>
        <main className='min-w-0 flex-1 p-4 outline-none md:p-6' id='main-content' tabIndex={-1}>
          {snapshot.status === 'switching' ? (
            <LoadingState label='Переключение роли…' />
          ) : snapshot.status === 'error' ? (
            <ErrorState error={snapshot.error} />
          ) : (
            children
          )}
        </main>
      </div>
    </div>
  );
}
