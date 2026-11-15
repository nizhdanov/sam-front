import { useRef, useState } from 'react';
import { z } from 'zod';

import type { DataFilters } from '@/utils/api/data';

import { useDemoAuth } from '@/app/auth/context';
import { SuccessState } from '@/components/shared/states';
import { Button } from '@/components/ui/button';
import { InventoryTable, renderInventoryCell } from '@/features/devices/inventory-table';
import { formatDataValue } from '@/utils/api/data';

import { FindingAcknowledge } from './finding-acknowledge';

export function FindingsTable({
  filters,
  onResetFilters
}: {
  filters: DataFilters;
  onResetFilters?: () => void;
}) {
  const { snapshot } = useDemoAuth();
  const [selected, setSelected] = useState<{ id: string; trigger: HTMLButtonElement }>();
  const [saved, setSaved] = useState(false);
  const listRef = useRef<HTMLDivElement>(null);
  return (
    <div data-findings ref={listRef} className='flex min-w-0 flex-col gap-4' tabIndex={-1}>
      <p className='text-sm text-muted-foreground'>
        Устаревание и дефицит определяет сервер. «Обработано» означает, что риск рассмотрен;
        устранение отражается отдельным статусом «Устранено».
      </p>
      {saved && <SuccessState>Обработка сохранена. Данные запрошены повторно.</SuccessState>}
      <InventoryTable
        renderCell={(value, column, row) => {
          if (column !== 'finding_id' || snapshot.role !== 'ADMIN')
            return renderInventoryCell(value, column);
          const id = z.uuid().safeParse(value);
          return (
            <div className='flex flex-col items-start gap-2'>
              <span>{formatDataValue(value)}</span>
              {!id.success && <span className='text-destructive'>Некорректный ID нарушения</span>}
              <Button
                disabled={!id.success || row.status !== 'OPEN'}
                size='sm'
                variant='outline'
                onClick={(event) => {
                  if (!id.success) return;
                  setSaved(false);
                  setSelected({ id: id.data, trigger: event.currentTarget });
                }}
              >
                Обработано
              </Button>
            </div>
          );
        }}
        caption='Нарушения'
        dataset='findings'
        filters={filters}
        onResetFilters={onResetFilters}
      />
      {selected && (
        <FindingAcknowledge
          key={selected.id}
          id={selected.id}
          trigger={selected.trigger}
          onClose={() => setSelected(undefined)}
          onSuccess={() => {
            setSelected(undefined);
            setSaved(true);
            requestAnimationFrame(() => listRef.current?.focus());
          }}
        />
      )}
    </div>
  );
}
