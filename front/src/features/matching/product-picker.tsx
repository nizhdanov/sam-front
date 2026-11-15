import { useQuery } from '@tanstack/react-query';
import { cn } from 'cn';
import { useState } from 'react';

import { useDemoAuth } from '@/app/auth/context';
import { ErrorState } from '@/components/shared/states';
import { Field, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Spinner } from '@/components/ui/spinner';
import { softwareListOptions } from '@/features/software/queries';

import type { Candidate } from './model';

interface Option {
  hint?: string | null;
  id: string;
  name: string;
}

function OptionList({
  options,
  value,
  onChange,
  name
}: {
  options: Option[];
  value?: string;
  onChange: (id: string) => void;
  name: string;
}) {
  return (
    <div aria-label={name} className='flex flex-col gap-2' role='radiogroup'>
      {options.map((option) => (
        <label
          key={option.id}
          className={cn(
            'flex cursor-pointer flex-col rounded-xl border p-3 text-sm',
            value === option.id && 'border-primary bg-primary/5'
          )}
        >
          <span className='flex items-center gap-2'>
            <input
              checked={value === option.id}
              name={name}
              type='radio'
              value={option.id}
              onChange={() => onChange(option.id)}
            />
            <span className='font-medium wrap-break-word'>{option.name}</span>
          </span>
          {option.hint && <span className='ps-6 text-muted-foreground'>{option.hint}</span>}
        </label>
      ))}
    </div>
  );
}

export function ProductPicker({
  candidates = [],
  suggested,
  value,
  onChange
}: {
  candidates?: Candidate[];
  suggested?: string | null;
  value?: string;
  onChange: (id: string) => void;
}) {
  const { auth } = useDemoAuth();
  const [search, setSearch] = useState('');
  const term = search.trim();
  const query = useQuery({
    ...softwareListOptions(auth, { page: 1, limit: 10, search: term }),
    enabled: term.length >= 2
  });
  const candidateOptions = candidates.map((item) => ({
    id: item.id,
    name: item.id === suggested ? `${item.canonical_name} — предложение` : item.canonical_name,
    hint: [item.publisher, item.edition].filter(Boolean).join(' · ')
  }));
  const catalogOptions = (query.data?.items ?? [])
    .filter((item) => !candidates.some((candidate) => candidate.id === item.id))
    .map((item) => ({
      id: item.id,
      name: String(item.canonical_name ?? item.id),
      hint: item.publisher
    }));

  return (
    <div className='flex flex-col gap-4'>
      {candidateOptions.length > 0 ? (
        <OptionList name='Кандидаты' options={candidateOptions} value={value} onChange={onChange} />
      ) : (
        <p className='text-sm text-muted-foreground'>Сервер не предложил кандидатов.</p>
      )}
      <Field>
        <FieldLabel htmlFor='product-picker-search'>Другой продукт из каталога</FieldLabel>
        <Input
          id='product-picker-search'
          placeholder='Название, минимум 2 символа'
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
      </Field>
      {query.isFetching && <Spinner />}
      {query.isError && <ErrorState error={query.error} />}
      {term.length >= 2 && query.isSuccess && !catalogOptions.length && (
        <p className='text-sm text-muted-foreground'>Ничего не найдено.</p>
      )}
      {catalogOptions.length > 0 && (
        <OptionList name='Каталог' options={catalogOptions} value={value} onChange={onChange} />
      )}
    </div>
  );
}
