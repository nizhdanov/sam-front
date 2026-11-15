import type { QueryClient } from '@tanstack/react-query';

import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

import type { DemoAuth, DemoSession } from '@/app/auth/session';

import { deleteChatConversationById } from '@/generated/api/requests/chat/deleteChatConversationById.gen';
import { getChatConversationByIdMessages } from '@/generated/api/requests/chat/getChatConversationByIdMessages.gen';
import { getChatConversations } from '@/generated/api/requests/chat/getChatConversations.gen';
import { getChatSuggestions } from '@/generated/api/requests/chat/getChatSuggestions.gen';
import { postChatConversations } from '@/generated/api/requests/chat/postChatConversations.gen';
import { ApiError, readApiError } from '@/utils/api/errors';
import { pageSchema, parseResponse } from '@/utils/api/page';

import type { StreamEvent } from './model';

import { conversationSchema, messageSchema, streamEventSchema } from './model';
import { createSseParser } from './sse';

export function suggestionsOptions(auth: DemoAuth) {
  const session = auth.requireReady();
  return queryOptions({
    queryKey: ['chat', 'suggestions', session.role, session.epoch],
    queryFn: async ({ signal }) => {
      auth.requestConfig(session);
      const response = await getChatSuggestions({ config: { signal } });
      return parseResponse(
        z.object({ items: z.array(z.string()) }),
        response.data,
        'Некорректные подсказки.'
      ).items;
    },
    staleTime: Infinity
  });
}

export function conversationsOptions(auth: DemoAuth) {
  const session = auth.requireReady();
  return queryOptions({
    queryKey: ['chat', 'conversations', session.role, session.epoch],
    queryFn: async ({ signal }) => {
      auth.requestConfig(session);
      const response = await getChatConversations({ query: { limit: 50 }, config: { signal } });
      return parseResponse(
        pageSchema(conversationSchema),
        response.data,
        'Некорректный список диалогов.'
      ).items;
    }
  });
}

export function messagesOptions(auth: DemoAuth, id: string) {
  const session = auth.requireReady();
  return queryOptions({
    queryKey: ['chat', 'messages', session.role, session.epoch, id],
    queryFn: async ({ signal }) => {
      auth.requestConfig(session);
      const items = [];
      for (let page = 1; ; page++) {
        const response = await getChatConversationByIdMessages({
          path: { id },
          query: { page, limit: 200 },
          config: { signal }
        });
        const data = parseResponse(
          pageSchema(messageSchema),
          response.data,
          'Некорректная история диалога.'
        );
        items.push(...data.items);
        if (page >= data.pagination.total_pages) return items;
      }
    }
  });
}

export async function createConversation(auth: DemoAuth, queryClient: QueryClient, title: string) {
  const session = auth.requireReady();
  const response = await postChatConversations({
    body: { title: title.slice(0, 80) },
    config: auth.requestConfig(session)
  });
  const conversation = parseResponse(conversationSchema, response.data, 'Сервер не вернул диалог.');
  if (auth.isCurrent(session.epoch))
    void queryClient.invalidateQueries({ queryKey: ['chat', 'conversations'] });
  return conversation;
}

export async function deleteConversation(auth: DemoAuth, queryClient: QueryClient, id: string) {
  const session = auth.requireReady();
  await deleteChatConversationById({ path: { id }, config: auth.requestConfig(session) });
  if (auth.isCurrent(session.epoch)) {
    queryClient.removeQueries({ queryKey: ['chat', 'messages', session.role, session.epoch, id] });
    await queryClient.invalidateQueries({ queryKey: ['chat', 'conversations'] });
  }
}

export async function streamAnswer(
  auth: DemoAuth,
  session: DemoSession,
  id: string,
  content: string,
  signal: AbortSignal,
  onEvent: (event: StreamEvent) => void
) {
  const token = localStorage.getItem('token')?.trim();
  auth.requestConfig(session);
  const response = await fetch(`/api/v1/chat/conversations/${id}/messages`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'text/event-stream',
      ...(token ? { Authorization: `Bearer ${token}` } : {})
    },
    body: JSON.stringify({ content }),
    signal: AbortSignal.any([signal, session.signal])
  });
  if (!response.ok || !response.body) {
    const error = await readApiError(await response.text().catch(() => undefined), response.status);
    if (error.status === 401) auth.fail(session.epoch, error);
    throw error;
  }
  let finished = false;
  const parser = createSseParser(({ event, data }) => {
    let payload: unknown;
    try {
      payload = JSON.parse(data);
    } catch {
      return;
    }
    const parsed = streamEventSchema.safeParse({ event, data: payload });
    if (!parsed.success || !auth.isCurrent(session.epoch)) return;
    if (parsed.data.event === 'message_end' || parsed.data.event === 'error') finished = true;
    onEvent(parsed.data);
  });
  const reader = response.body.pipeThrough(new TextDecoderStream()).getReader();
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    parser.feed(value);
  }
  parser.end();
  if (!finished) throw new ApiError('Соединение прервано до завершения ответа.');
}
