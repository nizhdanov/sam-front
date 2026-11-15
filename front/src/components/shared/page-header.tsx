import type { ReactNode } from 'react';

export function PageHeader({
  title,
  description,
  actions
}: {
  title: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <header className='flex flex-wrap items-end justify-between gap-4 border-b pb-5'>
      <div className='flex max-w-3xl min-w-0 flex-col gap-1.5'>
        <h1 className='wrap-break-word'>{title}</h1>
        {description && <p className='text-[0.9375rem] text-muted-foreground'>{description}</p>}
      </div>
      {actions && <div className='flex flex-wrap gap-2'>{actions}</div>}
    </header>
  );
}
