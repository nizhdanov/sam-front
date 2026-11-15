import type { ReactNode } from 'react';

import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableHead,
  TableHeader,
  TableRow
} from '@/components/ui/table';

export interface Column<T> {
  className?: string;
  header: string;
  cell: (row: T) => ReactNode;
}

export function SimpleTable<T>({
  caption,
  columns,
  rows,
  rowKey,
  rowClassName
}: {
  caption: string;
  columns: Column<T>[];
  rows: T[];
  rowKey: (row: T) => string;
  rowClassName?: (row: T) => string | undefined;
}) {
  return (
    <div className='min-w-0 overflow-x-auto rounded-xl border bg-card'>
      <Table>
        <TableCaption className='sr-only'>{caption}</TableCaption>
        <TableHeader className='bg-muted/60 [&_th]:text-muted-foreground'>
          <TableRow>
            {columns.map((column) => (
              <TableHead key={column.header} className={column.className}>
                {column.header}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row) => (
            <TableRow key={rowKey(row)} className={rowClassName?.(row)}>
              {columns.map((column) => (
                <TableCell
                  key={column.header}
                  className={column.className ?? 'max-w-80 wrap-break-word whitespace-normal'}
                >
                  {column.cell(row)}
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
