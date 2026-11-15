import type { DemoRole } from './demo-accounts';

export const NAVIGATION = [
  { to: '/dashboard', group: '', label: 'Обзор', admin: true },
  { to: '/software', group: 'Учёт', label: 'Каталог ПО', admin: false },
  // { to: '/agreements', group: 'Учёт', label: 'Соглашения', admin: true },
  { to: '/contracts', group: 'Учёт', label: 'Договоры', admin: true },
  { to: '/devices', group: 'Учёт', label: 'Устройства', admin: true },
  { to: '/installations', group: 'Учёт', label: 'Установки', admin: true },
  { to: '/imports', group: 'Учёт', label: 'Импорт CSV', admin: true },
  { to: '/matching', group: 'ИИ', label: 'Smart Matching', admin: true },
  { to: '/compliance/balance', group: 'Комплаенс', label: 'Баланс лицензий', admin: true },
  { to: '/compliance/findings', group: 'Комплаенс', label: 'Нарушения', admin: true },
  // { to: '/me', group: 'Учёт', label: 'Мой компьютер', admin: false },
  { to: '/assistant', group: 'ИИ', label: 'ИИ-ассистент', admin: false },
  { to: '/audit', group: 'Комплаенс', label: 'Журнал', admin: true }
] as const;

export const NAVIGATION_GROUPS = ['', 'Учёт', 'Комплаенс', 'ИИ'] as const;

export function canAccess(role: unknown, access: 'admin' | 'shared') {
  if (role !== 'USER' && role !== 'ADMIN') return false;
  return access === 'shared' || role === 'ADMIN';
}

export function getNavigation(role: DemoRole | null) {
  return NAVIGATION.filter((item) => canAccess(role, item.admin ? 'admin' : 'shared'));
}
