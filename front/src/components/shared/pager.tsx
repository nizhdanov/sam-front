import { Button } from '@/components/ui/button';

export function Pager({
  page,
  totalPages,
  total,
  onChange
}: {
  page: number;
  totalPages: number;
  total: number;
  onChange: (page: number) => void;
}) {
  if (totalPages <= 1) return <p className='text-sm text-muted-foreground'>Всего: {total}</p>;
  return (
    <nav aria-label='Страницы' className='flex flex-wrap items-center gap-3'>
      <Button disabled={page <= 1} variant='outline' onClick={() => onChange(page - 1)}>
        Назад
      </Button>
      <span className='text-sm'>
        Страница {page} из {totalPages} · всего {total}
      </span>
      <Button disabled={page >= totalPages} variant='outline' onClick={() => onChange(page + 1)}>
        Вперёд
      </Button>
    </nav>
  );
}
