import type { DemoRole } from './demo-accounts';

export const NAVIGATION = [
  { to: '/dashboard', label: 'Обзор', admin: true },
  { to: '/software', label: 'Каталог ПО', admin: false },
  { to: '/agreements', label: 'Соглашения', admin: true },
  { to: '/contracts', label: 'Договоры', admin: true },
  { to: '/devices', label: 'Устройства', admin: true },
  { to: '/installations', label: 'Установки', admin: true },
  { to: '/imports', label: 'Импорт CSV', admin: true },
  { to: '/compliance/balance', label: 'Баланс лицензий', admin: true },
  { to: '/compliance/findings', label: 'Нарушения', admin: true },
  { to: '/me', label: 'Мой компьютер', admin: false },
  { to: '/audit', label: 'Журнал', admin: true }
] as const;

export function canAccess(role: unknown, access: 'admin' | 'shared') {
  if (role !== 'USER' && role !== 'ADMIN') return false;
  return access === 'shared' || role === 'ADMIN';
}

export function getNavigation(role: DemoRole | null) {
  return NAVIGATION.filter((item) => canAccess(role, item.admin ? 'admin' : 'shared'));
}
