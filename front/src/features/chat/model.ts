import { z } from 'zod';

export const MAX_QUESTION = 4000;

export const conversationSchema = z.object({
  id: z.uuid(),
  title: z.string().nullable(),
  created_at: z.string(),
  updated_at: z.string()
});
export type Conversation = z.infer<typeof conversationSchema>;

export const messageSchema = z.object({
  id: z.uuid(),
  conversation_id: z.uuid(),
  role: z.enum(['user', 'assistant']),
  content: z.string(),
  status: z.enum(['COMPLETED', 'INCOMPLETE', 'FAILED']),
  created_at: z.string()
});
export type ChatMessage = z.infer<typeof messageSchema>;

export const chatSearchSchema = z.object({
  c: z.uuid().optional().catch(undefined)
});

export const streamEventSchema = z.discriminatedUnion('event', [
  z.object({ event: z.literal('message_start'), data: z.object({ message_id: z.uuid() }) }),
  z.object({ event: z.literal('delta'), data: z.object({ content: z.string() }) }),
  z.object({
    event: z.literal('message_end'),
    data: z.object({ message_id: z.uuid(), status: z.string() })
  }),
  z.object({
    event: z.literal('error'),
    data: z.object({ code: z.string().optional(), message: z.string().optional() })
  })
]);
export type StreamEvent = z.infer<typeof streamEventSchema>;

export const questionSchema = z
  .string()
  .trim()
  .min(1, 'Введите вопрос.')
  .max(MAX_QUESTION, `Вопрос длиннее ${MAX_QUESTION} символов.`);
