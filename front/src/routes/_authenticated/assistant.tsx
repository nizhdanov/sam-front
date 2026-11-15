import { createFileRoute } from '@tanstack/react-router';

import { ChatPage } from '@/features/chat/chat-page';
import { chatSearchSchema } from '@/features/chat/model';

export const Route = createFileRoute('/_authenticated/assistant')({
  validateSearch: chatSearchSchema,
  component: ChatPage
});
