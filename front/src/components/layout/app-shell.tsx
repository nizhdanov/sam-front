import type { ReactNode } from 'react';

import { Link } from '@tanstack/react-router';
import {
  BotIcon,
  FileTextIcon,
  GaugeIcon,
  HardDriveIcon,
  HistoryIcon,
  LayersIcon,
  MenuIcon,
  MonitorIcon,
  PackageIcon,
  ScaleIcon,
  ShieldAlertIcon,
  SparklesIcon,
  UploadIcon
} from 'lucide-react';
import { useState } from 'react';

import { useDemoAuth } from '@/app/auth/context';
import { DEMO_ACCOUNTS } from '@/app/auth/demo-accounts';
import { getNavigation, NAVIGATION_GROUPS } from '@/app/auth/permissions';
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

const ICONS: Record<string, typeof MenuIcon> = {
  '/dashboard': GaugeIcon,
  '/software': PackageIcon,
  '/agreements': FileTextIcon,
  '/contracts': FileTextIcon,
  '/devices': MonitorIcon,
  '/installations': LayersIcon,
  '/imports': UploadIcon,
  '/matching': SparklesIcon,
  '/compliance/balance': ScaleIcon,
  '/compliance/findings': ShieldAlertIcon,
  '/me': HardDriveIcon,
  '/assistant': BotIcon,
  '/audit': HistoryIcon
};

function Navigation({ onNavigate }: { onNavigate?: () => void }) {
  const { snapshot } = useDemoAuth();
  const items = getNavigation(snapshot.status === 'ready' ? snapshot.role : null);
  return (
    <nav aria-label='Основная навигация' className='flex flex-col gap-4'>
      {NAVIGATION_GROUPS.map((group) => {
        const groupItems = items.filter((item) => item.group === group);
        if (!groupItems.length) return null;
        return (
          <div key={group || 'main'} className='flex flex-col gap-0.5'>
            {group && (
              <span className='px-3 pb-1 text-xs font-medium text-sidebar-foreground/55'>
                {group}
              </span>
            )}
            {groupItems.map((item) => {
              const Icon = ICONS[item.to] ?? PackageIcon;
              return (
                <Link
                  key={item.to}
                  activeProps={{
                    className:
                      'bg-sidebar-accent text-sidebar-accent-foreground before:bg-sidebar-primary',
                    'aria-current': 'page'
                  }}
                  className='relative flex items-center gap-3 rounded-lg px-3 py-1.5 text-sm text-sidebar-foreground transition-colors outline-none before:absolute before:inset-y-1.5 before:left-0 before:w-0.75 before:rounded-full before:bg-transparent hover:bg-sidebar-accent/60 hover:text-sidebar-accent-foreground focus-visible:ring-2 focus-visible:ring-sidebar-ring'
                  to={item.to}
                  onClick={onNavigate}
                >
                  <Icon aria-hidden='true' className='size-4 shrink-0 opacity-80' />
                  {item.label}
                </Link>
              );
            })}
          </div>
        );
      })}
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
      <SheetContent className='overflow-y-auto bg-sidebar text-sidebar-foreground' side='left'>
        <SheetHeader>
          <SheetTitle className='text-white'>Разделы SAM</SheetTitle>
          <SheetDescription className='text-sidebar-foreground/70'>
            Управление программными активами
          </SheetDescription>
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
    <div className='flex min-h-svh'>
      <a className='sr-only focus:not-sr-only focus:p-3' href='#main-content'>
        Перейти к содержимому
      </a>
      <aside className='sticky top-0 hidden h-svh w-64 shrink-0 flex-col gap-5 overflow-y-auto overscroll-contain bg-sidebar px-3 py-4 text-sidebar-foreground [scrollbar-width:none] [&::-webkit-scrollbar]:hidden md:flex'>
        <Link
          aria-label='SAM — главная'
          className='flex flex-col rounded-lg px-3 outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring'
          to='/'
        >
          <span className='text-xl font-bold tracking-tight text-white'>
            SAM<span className='text-sidebar-primary'>.</span>
          </span>
          <span className='text-xs text-sidebar-foreground/65'>Программные активы</span>
        </Link>
        <Navigation />
      </aside>
      <div className='flex min-w-0 flex-1 flex-col'>
        <header className='sticky top-0 z-10 flex items-center justify-between gap-3 border-b bg-background/90 px-4 py-2.5 backdrop-blur-sm md:justify-end md:px-8'>
          <div className='flex items-center gap-3 md:hidden'>
            <MobileNavigation key={snapshot.epoch} />
            <Link
              aria-label='SAM — главная'
              className='rounded-md text-lg font-bold text-primary outline-none focus-visible:ring-2 focus-visible:ring-ring'
              to='/'
            >
              SAM
            </Link>
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
              <SelectTrigger aria-label='Роль' id='demo-role'>
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
        <main
          className='mx-auto w-full max-w-[1400px] min-w-0 flex-1 px-4 py-6 outline-none md:p-8'
          id='main-content'
          tabIndex={-1}
        >
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
