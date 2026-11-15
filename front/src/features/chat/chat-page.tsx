import { useQuery, useQueryClient } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { cn } from 'cn';
import { ArrowUpIcon, PlusIcon, SquareIcon, Trash2Icon } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';

import { useDemoAuth } from '@/app/auth/context';
import { ErrorState, LoadingState } from '@/components/shared/states';
import { Bubble, BubbleContent } from '@/components/ui/bubble';
import { Button } from '@/components/ui/button';
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupText,
  InputGroupTextarea
} from '@/components/ui/input-group';
import { Message, MessageContent } from '@/components/ui/message';
import {
  MessageScroller,
  MessageScrollerButton,
  MessageScrollerContent,
  MessageScrollerProvider,
  MessageScrollerViewport
} from '@/components/ui/message-scroller';
import { Spinner } from '@/components/ui/spinner';
import { ApiError } from '@/utils/api/errors';

import type { ChatMessage } from './model';

import { Markdown } from './markdown';
import { MAX_QUESTION, questionSchema } from './model';
import {
  conversationsOptions,
  createConversation,
  deleteConversation,
  messagesOptions,
  streamAnswer,
  suggestionsOptions
} from './queries';

const route = getRouteApi('/_authenticated/assistant');

interface Live {
  answer: string;
  question: string;
  status: 'error' | 'streaming';
}

function Conversations({ active, disabled }: { active?: string; disabled: boolean }) {
  const { auth } = useDemoAuth();
  const queryClient = useQueryClient();
  const navigate = route.useNavigate();
  const query = useQuery(conversationsOptions(auth));
  const [error, setError] = useState<unknown>();
  const [confirming, setConfirming] = useState<string>();
  return (
    <aside aria-label='Диалоги' className='flex min-w-0 flex-col gap-2 lg:w-64 lg:shrink-0'>
      <Button disabled={disabled} variant='outline' onClick={() => void navigate({ search: {} })}>
        <PlusIcon data-icon='inline-start' />
        Новый диалог
      </Button>
      {query.isPending && <Spinner />}
      {query.isError && <ErrorState error={query.error} onRetry={() => void query.refetch()} />}
      {error !== undefined && <ErrorState error={error} />}
      <ul className='flex max-h-48 flex-col gap-1 overflow-y-auto lg:max-h-[70svh]'>
        {query.data?.map((item) => (
          <li key={item.id} className='group flex items-center gap-1'>
            <button
              className={cn(
                'min-w-0 flex-1 truncate rounded-lg px-3 py-2 text-start text-sm hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
                item.id === active && 'bg-muted font-medium'
              )}
              disabled={disabled}
              type='button'
              onClick={() => void navigate({ search: { c: item.id } })}
            >
              {item.title || 'Без названия'}
            </button>
            <Button
              aria-label={
                confirming === item.id
                  ? 'Подтвердить удаление диалога'
                  : `Удалить диалог «${item.title || 'Без названия'}»`
              }
              disabled={disabled}
              size={confirming === item.id ? 'sm' : 'icon-sm'}
              variant={confirming === item.id ? 'destructive' : 'ghost'}
              onBlur={() => setConfirming(undefined)}
              onClick={async () => {
                if (confirming !== item.id) {
                  setConfirming(item.id);
                  return;
                }
                setConfirming(undefined);
                setError(undefined);
                try {
                  await deleteConversation(auth, queryClient, item.id);
                  if (item.id === active) void navigate({ search: {} });
                } catch (deleteError) {
                  setError(deleteError);
                }
              }}
            >
              {confirming === item.id ? 'Удалить?' : <Trash2Icon />}
            </Button>
          </li>
        ))}
      </ul>
    </aside>
  );
}

function Bubbles({
  message,
  streaming
}: {
  message: Pick<ChatMessage, 'content' | 'role' | 'status'>;
  streaming?: boolean;
}) {
  if (message.role === 'user')
    return (
      <Message align='end'>
        <Bubble align='end'>
          <BubbleContent className='wrap-break-word whitespace-pre-wrap'>
            {message.content}
          </BubbleContent>
        </Bubble>
      </Message>
    );
  return (
    <Message>
      <MessageContent className='min-w-0'>
        {message.content ? (
          <Markdown content={message.content} streaming={!!streaming} />
        ) : (
          streaming && <Spinner aria-label='Ассистент печатает' />
        )}
        {message.status !== 'COMPLETED' && !streaming && (
          <p className='text-xs text-destructive'>
            {message.status === 'FAILED'
              ? 'Ответ не получен.'
              : 'Ответ прерван и может быть неполным.'}
          </p>
        )}
      </MessageContent>
    </Message>
  );
}

export function ChatPage() {
  const { c: conversationId } = route.useSearch();
  const navigate = route.useNavigate();
  const { auth, snapshot } = useDemoAuth();
  const queryClient = useQueryClient();
  const suggestions = useQuery(suggestionsOptions(auth));
  const history = useQuery({
    ...messagesOptions(auth, conversationId ?? ''),
    enabled: !!conversationId
  });
  const [draft, setDraft] = useState('');
  const [live, setLive] = useState<Live>();
  const [error, setError] = useState<unknown>();
  const controllerRef = useRef<AbortController>(null);
  const busy = live?.status === 'streaming';

  useEffect(() => () => controllerRef.current?.abort(), [snapshot.epoch]);

  async function ask(text: string) {
    const parsed = questionSchema.safeParse(text);
    if (!parsed.success) {
      setError(new ApiError(parsed.error.issues[0].message));
      return;
    }
    if (busy) return;
    const session = auth.requireReady();
    const abort = new AbortController();
    controllerRef.current = abort;
    setError(undefined);
    setDraft('');
    setLive({ question: parsed.data, answer: '', status: 'streaming' });
    let id = conversationId;
    try {
      if (!id) {
        id = (await createConversation(auth, queryClient, parsed.data)).id;
        if (!auth.isCurrent(session.epoch)) return;
        queryClient.setQueryData(messagesOptions(auth, id).queryKey, []);
        void navigate({ search: { c: id }, replace: true });
      }
      await streamAnswer(auth, session, id, parsed.data, abort.signal, (event) => {
        if (event.event === 'delta')
          setLive(
            (current) => current && { ...current, answer: current.answer + event.data.content }
          );
        if (event.event === 'error')
          throw new ApiError(event.data.message ?? 'Ассистент не смог ответить.');
      });
      if (auth.isCurrent(session.epoch)) setLive(undefined);
    } catch (streamError) {
      if (!auth.isCurrent(session.epoch)) return;
      if (abort.signal.aborted) setLive(undefined);
      else {
        setLive((current) => current && { ...current, status: 'error' });
        setDraft((current) => current || parsed.data);
        setError(streamError);
      }
    } finally {
      controllerRef.current = null;
      if (auth.isCurrent(session.epoch) && id) {
        await queryClient.invalidateQueries({
          queryKey: ['chat', 'messages', session.role, session.epoch, id]
        });
        void queryClient.invalidateQueries({ queryKey: ['chat', 'conversations'] });
        setLive((current) => (current?.status === 'streaming' ? current : undefined));
      }
    }
  }

  const messages = conversationId ? (history.data ?? []) : [];
  const empty = !messages.length && !live;

  return (
    <section aria-label='ИИ-ассистент' className='flex min-h-0 min-w-0 flex-1 flex-col gap-4'>
      <div className='flex flex-col gap-1'>
        <h1 className='text-2xl font-semibold'>ИИ-ассистент</h1>
        <p className='text-sm text-muted-foreground'>
          {snapshot.role === 'ADMIN'
            ? 'Вопросы о лицензиях, устройствах, договорах и нарушениях. Таблицы ответа можно выгрузить в XLSX.'
            : 'Вопросы о разрешённом ПО и вашем компьютере.'}
        </p>
      </div>
      <div className='flex min-h-0 flex-1 flex-col gap-4 lg:flex-row'>
        <Conversations key={snapshot.epoch} active={conversationId} disabled={busy} />
        <div className='flex h-[70svh] min-h-96 min-w-0 flex-1 flex-col gap-3 rounded-2xl border p-3'>
          <MessageScrollerProvider>
            <MessageScroller className='min-h-0 flex-1'>
              <MessageScrollerViewport>
                <MessageScrollerContent className='gap-4 p-2'>
                  {conversationId && history.isPending && <LoadingState />}
                  {conversationId && history.isError && (
                    <ErrorState error={history.error} onRetry={() => void history.refetch()} />
                  )}
                  {empty && !(conversationId && history.isPending) && (
                    <div className='m-auto flex max-w-xl flex-col items-center gap-3 text-center'>
                      <p className='text-muted-foreground'>Задайте вопрос или выберите пример:</p>
                      <div className='flex flex-wrap justify-center gap-2'>
                        {suggestions.data?.map((item) => (
                          <Button
                            key={item}
                            size='sm'
                            variant='outline'
                            onClick={() => void ask(item)}
                          >
                            {item}
                          </Button>
                        ))}
                      </div>
                    </div>
                  )}
                  {messages.map((message) => (
                    <Bubbles key={message.id} message={message} />
                  ))}
                  {live &&
                    !messages.some(
                      (message) =>
                        message.role === 'user' &&
                        message.content === live.question &&
                        live.status !== 'streaming'
                    ) && (
                      <>
                        <Bubbles
                          message={{ role: 'user', content: live.question, status: 'COMPLETED' }}
                        />
                        {(live.status === 'streaming' || live.answer) && (
                          <Bubbles
                            message={{
                              role: 'assistant',
                              content: live.answer,
                              status: live.status === 'error' ? 'INCOMPLETE' : 'COMPLETED'
                            }}
                            streaming={live.status === 'streaming'}
                          />
                        )}
                      </>
                    )}
                </MessageScrollerContent>
              </MessageScrollerViewport>
              <MessageScrollerButton />
            </MessageScroller>
          </MessageScrollerProvider>
          {error !== undefined && <ErrorState error={error} />}
          <form
            onSubmit={(event) => {
              event.preventDefault();
              void ask(draft);
            }}
          >
            <InputGroup>
              <InputGroupTextarea
                aria-label='Вопрос ассистенту'
                className='max-h-40'
                disabled={busy}
                maxLength={MAX_QUESTION}
                placeholder='Спросите о ПО, лицензиях или устройствах…'
                value={draft}
                onChange={(event) => setDraft(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) {
                    event.preventDefault();
                    void ask(draft);
                  }
                }}
              />
              <InputGroupAddon align='block-end'>
                <InputGroupText className='text-xs'>
                  {draft.length}/{MAX_QUESTION}
                </InputGroupText>
                {busy ? (
                  <InputGroupButton
                    aria-label='Остановить ответ'
                    className='ms-auto'
                    size='icon-sm'
                    variant='outline'
                    onClick={() => controllerRef.current?.abort()}
                  >
                    <SquareIcon />
                  </InputGroupButton>
                ) : (
                  <InputGroupButton
                    aria-label='Отправить'
                    className='ms-auto'
                    disabled={!draft.trim()}
                    size='icon-sm'
                    type='submit'
                    variant='default'
                  >
                    <ArrowUpIcon />
                  </InputGroupButton>
                )}
              </InputGroupAddon>
            </InputGroup>
          </form>
        </div>
      </div>
    </section>
  );
}
