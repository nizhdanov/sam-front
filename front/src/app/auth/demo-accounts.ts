import { TOKENS } from '@/utils/constants/tokens';

export const DEMO_ACCOUNTS = [
  { role: 'USER', label: 'Пользователь', token: TOKENS.USER },
  { role: 'ADMIN', label: 'Администратор', token: TOKENS.ADMIN }
] as const;

export type DemoRole = (typeof DEMO_ACCOUNTS)[number]['role'];

export function getDemoAccount(token: string | null) {
  return DEMO_ACCOUNTS.find((account) => account.token === token);
}

export function getLanding(role: DemoRole) {
  return role === 'ADMIN' ? '/dashboard' : '/software';
}
