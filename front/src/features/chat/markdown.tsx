import type { ReactNode } from 'react';

import { Link } from '@tanstack/react-router';
import { DownloadIcon } from 'lucide-react';
import { Streamdown } from 'streamdown';

import { useDemoAuth } from '@/app/auth/context';
import { ErrorState } from '@/components/shared/states';
import { useAction } from '@/components/shared/use-action';
import { Button } from '@/components/ui/button';
import { Spinner } from '@/components/ui/spinner';
import { postExport } from '@/generated/api/requests/data/postExport.gen';
import { downloadFile } from '@/utils/api/files';

import { extractTable } from './table-data';

const SAM_LINK = /sam:\/\/(device|product|contract)\/([0-9a-f-]{36})/gi;
const ROUTES: Record<string, (id: string) => string> = {
  device: (id) => `/devices/${id}`,
  product: (id) => `/software/${id}`,
  contract: () => '/contracts'
};

export function toAppLinks(markdown: string) {
  return markdown.replace(SAM_LINK, (_, kind: string, id: string) =>
    ROUTES[kind.toLowerCase()](id)
  );
}

function SafeLink({ href, children }: { href?: unknown; children?: ReactNode }) {
  if (typeof href !== 'string') return <span>{children}</span>;
  if (href.startsWith('/') && !href.startsWith('//'))
    return (
      <Link className='text-primary underline underline-offset-2' to={href}>
        {children}
      </Link>
    );
  if (/^https?:\/\//i.test(href))
    return (
      <a
        className='text-primary underline underline-offset-2'
        href={href}
        rel='noopener noreferrer'
        target='_blank'
      >
        {children}
      </a>
    );
  return <span>{children}</span>;
}

function ExportableTable({
  node,
  children,
  exportable
}: {
  node?: unknown;
  children?: ReactNode;
  exportable: boolean;
}) {
  const { auth } = useDemoAuth();
  const action = useAction();
  const data = exportable ? extractTable(node) : undefined;
  return (
    <div className='my-3 flex min-w-0 flex-col gap-2'>
      <div className='overflow-x-auto rounded-xl border'>
        <table className='w-full text-sm [&_td]:border-t [&_td]:px-3 [&_td]:py-2 [&_th]:px-3 [&_th]:py-2 [&_th]:text-start'>
          {children}
        </table>
      </div>
      {data && (
        <Button
          className='self-start'
          disabled={action.pending}
          size='sm'
          variant='outline'
          onClick={() =>
            void action.run(async () => {
              const session = auth.requireReady();
              const response = await postExport({
                body: { title: 'Ответ ассистента', format: 'xlsx', ...data },
                config: { ...auth.requestConfig(session), responseType: 'blob' }
              });
              await downloadFile(response, 'sam-answer.xlsx');
            })
          }
        >
          {action.pending ? (
            <Spinner data-icon='inline-start' />
          ) : (
            <DownloadIcon data-icon='inline-start' />
          )}
          Скачать XLSX
        </Button>
      )}
      {action.error !== undefined && <ErrorState error={action.error} />}
    </div>
  );
}

export function Markdown({ content, streaming }: { content: string; streaming: boolean }) {
  return (
    <Streamdown
      components={{
        a: ({ href, children }) => <SafeLink href={href}>{children}</SafeLink>,
        table: ({ node, children }) => (
          <ExportableTable exportable={!streaming} node={node}>
            {children as ReactNode}
          </ExportableTable>
        )
      }}
      className='min-w-0 text-sm/relaxed [&_ol]:list-decimal [&_ol]:ps-5 [&_ul]:list-disc [&_ul]:ps-5'
      controls={false}
      isAnimating={streaming}
      mode={streaming ? 'streaming' : 'static'}
    >
      {toAppLinks(content)}
    </Streamdown>
  );
}
