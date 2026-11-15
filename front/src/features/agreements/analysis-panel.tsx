import type { ReactNode } from 'react';

import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { SparklesIcon } from 'lucide-react';
import { useState } from 'react';

import { useDemoAuth } from '@/app/auth/context';
import { ErrorState, LoadingState, SuccessState } from '@/components/shared/states';
import { useAction } from '@/components/shared/use-action';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Spinner } from '@/components/ui/spinner';
import { COST_MODELS, LICENSE_TYPES } from '@/features/software/model';

import type { AgreementAnalysis } from './analysis';

import {
  analysisOptions,
  analyzeAgreement,
  approveAnalysis,
  COMMERCIAL_USE,
  paragraphIndex,
  RISK_LEVELS,
  SUBSCRIPTION_KINDS
} from './analysis';

function Sources({
  numbers,
  total,
  onOpen
}: {
  numbers: number[];
  total: number;
  onOpen: (index: number) => void;
}) {
  if (!numbers.length)
    return <span className='text-xs text-muted-foreground'>Источник не указан</span>;
  return (
    <span className='flex flex-wrap items-center gap-1 text-xs'>
      Абзацы:
      {numbers.map((number) => {
        const index = paragraphIndex(number, total);
        return index === undefined ? (
          <span key={number} className='text-muted-foreground' title='Абзаца нет в тексте'>
            [{number}?]
          </span>
        ) : (
          <button
            key={number}
            className='rounded-sm px-1 text-primary underline-offset-2 hover:underline focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none'
            type='button'
            onClick={() => onOpen(index)}
          >
            [{number}]
          </button>
        );
      })}
    </span>
  );
}

function Finding({
  title,
  description,
  quote,
  sources
}: {
  title: ReactNode;
  description?: string;
  quote?: string;
  sources: ReactNode;
}) {
  return (
    <li className='flex flex-col gap-1 rounded-xl border p-3'>
      <span className='font-medium'>{title}</span>
      {description && <span className='text-sm wrap-break-word'>{description}</span>}
      {quote && (
        <blockquote className='border-s-2 ps-3 text-sm wrap-break-word text-muted-foreground italic'>
          «{quote}»
        </blockquote>
      )}
      {sources}
    </li>
  );
}

function Group({
  title,
  empty,
  children
}: {
  title: string;
  empty: string;
  children: ReactNode[];
}) {
  return (
    <div className='flex flex-col gap-2'>
      <h3 className='font-semibold'>{title}</h3>
      {children.length ? (
        <ul className='flex flex-col gap-2'>{children}</ul>
      ) : (
        <p className='text-sm text-muted-foreground'>{empty}</p>
      )}
    </div>
  );
}

function AnalysisView({ analysis }: { analysis: AgreementAnalysis }) {
  const [active, setActive] = useState<number>();
  const { result, paragraphs } = analysis;
  const open = (index: number) => {
    setActive(index);
    document
      .getElementById(`agreement-paragraph-${index}`)
      ?.scrollIntoView({ behavior: 'smooth', block: 'center' });
  };
  const sources = (numbers: number[]) => (
    <Sources numbers={numbers} total={paragraphs.length} onOpen={open} />
  );
  const commercial = result.commercial_use?.value ?? 'UNDEFINED';

  return (
    <div className='grid min-w-0 gap-6 lg:grid-cols-2'>
      <div className='flex min-w-0 flex-col gap-5'>
        {result.summary && <p className='wrap-break-word'>{result.summary}</p>}
        <dl className='grid gap-3 sm:grid-cols-3'>
          <div>
            <dt className='text-sm text-muted-foreground'>Тип лицензии</dt>
            <dd>{result.license_type ? LICENSE_TYPES[result.license_type] : 'Не определён'}</dd>
          </div>
          <div>
            <dt className='text-sm text-muted-foreground'>Модель оплаты</dt>
            <dd>{result.cost_model ? COST_MODELS[result.cost_model] : 'Не определена'}</dd>
          </div>
          <div>
            <dt className='text-sm text-muted-foreground'>Коммерческое использование</dt>
            <dd
              className={
                commercial === 'NO'
                  ? 'font-semibold text-destructive'
                  : commercial === 'UNDEFINED'
                    ? 'text-amber-600 dark:text-amber-400'
                    : undefined
              }
            >
              {COMMERCIAL_USE[commercial]}
            </dd>
          </div>
        </dl>
        {result.license_type_evidence && (
          <ul>
            <Finding
              quote={result.license_type_evidence.quote}
              sources={sources(result.license_type_evidence.paragraphs)}
              title='Основание типа лицензии'
            />
          </ul>
        )}
        {result.commercial_use && (
          <ul>
            <Finding
              quote={result.commercial_use.quote}
              sources={sources(result.commercial_use.paragraphs)}
              title={`Коммерческое использование: ${COMMERCIAL_USE[commercial]}`}
            />
          </ul>
        )}
        <Group empty='Подписок и автопродления не найдено.' title='Подписки'>
          {result.subscriptions.map((item, index) => (
            <Finding
              key={index}
              description={item.description}
              quote={item.quote}
              sources={sources(item.paragraphs)}
              title={SUBSCRIPTION_KINDS[item.kind] ?? item.kind}
            />
          ))}
        </Group>
        <Group empty='Ограничений не найдено.' title='Ограничения'>
          {result.restrictions.map((item, index) => (
            <Finding
              key={index}
              description={item.description}
              quote={item.quote}
              sources={sources(item.paragraphs)}
              title={item.type || 'Ограничение'}
            />
          ))}
        </Group>
        <Group empty='Рисков не найдено.' title='Риски'>
          {result.risks.map((item, index) => (
            <Finding
              key={index}
              title={
                <span className={item.level === 'HIGH' ? 'text-destructive' : undefined}>
                  Риск: {RISK_LEVELS[item.level] ?? item.level}
                </span>
              }
              description={item.description}
              quote={item.quote}
              sources={sources(item.paragraphs)}
            />
          ))}
        </Group>
        <Group empty='Похожих соглашений не найдено.' title='Похожие соглашения'>
          {analysis.similar_agreements.map((item) => (
            <li
              key={item.id}
              className='flex flex-wrap items-center justify-between gap-2 rounded-xl border p-3'
            >
              <Link
                className='font-medium underline-offset-4 hover:underline'
                params={{ agreementId: item.id }}
                to='/agreements/$agreementId'
              >
                {item.name}
              </Link>
              <span className='text-sm text-muted-foreground'>
                сходство {Math.round(item.similarity * 100)}%
              </span>
            </li>
          ))}
        </Group>
      </div>
      <div className='flex min-w-0 flex-col gap-2'>
        <h3 className='font-semibold'>Текст по абзацам</h3>
        <ol className='flex max-h-[70vh] flex-col gap-2 overflow-y-auto rounded-xl border p-3 text-sm'>
          {paragraphs.map((paragraph, index) => (
            <li
              key={index}
              className={`scroll-m-4 rounded-lg p-2 wrap-break-word whitespace-pre-wrap ${active === index ? 'bg-amber-100 dark:bg-amber-900/40' : ''}`}
              id={`agreement-paragraph-${index}`}
            >
              <span className='me-2 font-mono text-xs text-muted-foreground'>[{index + 1}]</span>
              {paragraph}
            </li>
          ))}
        </ol>
      </div>
    </div>
  );
}

export function AnalysisPanel({ agreementId, hasText }: { agreementId: string; hasText: boolean }) {
  const { auth } = useDemoAuth();
  const queryClient = useQueryClient();
  const query = useQuery(analysisOptions(auth, agreementId));
  const action = useAction();
  const [approved, setApproved] = useState(false);
  const analysis = query.data;
  const isApproved = !!analysis?.reviewed_at;

  return (
    <Card>
      <CardHeader>
        <CardTitle className='flex items-center gap-2'>
          <SparklesIcon aria-hidden='true' className='size-4' />
          ИИ-анализ соглашения
        </CardTitle>
        <CardDescription>
          Анализируется сохранённый на сервере текст. Сохраните изменения перед запуском. Результат
          не является юридическим заключением и применяется только после подтверждения.
        </CardDescription>
      </CardHeader>
      <CardContent className='flex flex-col gap-4'>
        <div className='flex flex-wrap gap-2'>
          <Button
            disabled={!hasText || action.pending}
            onClick={() => {
              setApproved(false);
              void action.run(() => analyzeAgreement(auth, queryClient, agreementId, !!analysis));
            }}
          >
            {action.pending && <Spinner data-icon='inline-start' />}
            {analysis ? 'Проанализировать заново' : 'Запустить анализ'}
          </Button>
          {analysis && (
            <Button
              disabled={action.pending || isApproved}
              variant='outline'
              onClick={async () => {
                setApproved(
                  await action.run(() => approveAnalysis(auth, queryClient, agreementId))
                );
              }}
            >
              {isApproved ? 'Выводы сохранены' : 'Сохранить выводы в соглашение'}
            </Button>
          )}
        </div>
        {!hasText && (
          <p className='text-sm text-muted-foreground'>Добавьте текст или файл соглашения.</p>
        )}
        {action.pending && <p role='status'>ИИ анализирует текст, это может занять до минуты…</p>}
        {action.error !== undefined && <ErrorState error={action.error} />}
        {approved && (
          <SuccessState>
            Тип лицензии, модель оплаты и коммерческое использование сохранены.
          </SuccessState>
        )}
        {analysis && (
          <p className='text-xs text-muted-foreground'>
            Модель {analysis.model} · {new Date(analysis.created_at).toLocaleString('ru-RU')}
            {isApproved && ` · подтвердил ${analysis.reviewed_by ?? '—'}`}
          </p>
        )}
        {query.isPending ? (
          <LoadingState />
        ) : query.isError ? (
          <ErrorState error={query.error} onRetry={() => void query.refetch()} />
        ) : analysis ? (
          <AnalysisView analysis={analysis} />
        ) : (
          <p className='text-sm text-muted-foreground'>Анализ ещё не выполнялся.</p>
        )}
      </CardContent>
    </Card>
  );
}
