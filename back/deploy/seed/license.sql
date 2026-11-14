BEGIN;

INSERT INTO employees (id, external_id, full_name, department, position, email) VALUES
  ('10000000-0000-4000-8000-000000000001', 'EMP-001', 'Иванов Иван', 'Разработка', 'Разработчик', 'ivanov@example.com'),
  ('10000000-0000-4000-8000-000000000002', 'EMP-002', 'Петрова Анна', 'ИТ', 'Администратор SAM', 'petrova@example.com'),
  ('10000000-0000-4000-8000-000000000003', 'EMP-003', 'Сидоров Олег', 'ИТ', 'ИТ-директор', 'sidorov@example.com'),
  ('10000000-0000-4000-8000-000000000004', 'EMP-004', 'Кузнецова Мария', 'Финансы', 'Бухгалтер', 'kuznetsova@example.com'),
  ('10000000-0000-4000-8000-000000000005', 'EMP-005', 'Смирнов Павел', 'Разработка', 'Тимлид', 'smirnov@example.com'),
  ('10000000-0000-4000-8000-000000000006', 'EMP-006', 'Попова Елена', 'Продажи', 'Менеджер', 'popova@example.com'),
  ('10000000-0000-4000-8000-000000000007', 'EMP-007', 'Волков Дмитрий', 'Разработка', 'Дизайнер', 'volkov@example.com'),
  ('10000000-0000-4000-8000-000000000008', 'EMP-008', 'Соколова Ольга', 'Финансы', 'Финансовый аналитик', 'sokolova@example.com'),
  ('10000000-0000-4000-8000-000000000009', 'EMP-009', 'Морозов Артём', 'Продажи', 'Руководитель продаж', 'morozov@example.com'),
  ('10000000-0000-4000-8000-000000000010', 'EMP-010', 'Новикова Ксения', 'Разработка', 'QA-инженер', 'novikova@example.com')
ON CONFLICT (id) DO NOTHING;

INSERT INTO devices (id, serial_number, hostname, employee_id, operating_system) VALUES
  ('20000000-0000-4000-8000-000000000001', 'PC-001', 'dev-ivanov', '10000000-0000-4000-8000-000000000001', 'Windows 11'),
  ('20000000-0000-4000-8000-000000000002', 'PC-002', 'it-petrova', '10000000-0000-4000-8000-000000000002', 'Windows 11'),
  ('20000000-0000-4000-8000-000000000003', 'PC-003', 'it-sidorov', '10000000-0000-4000-8000-000000000003', 'Windows 11'),
  ('20000000-0000-4000-8000-000000000004', 'PC-004', 'fin-kuznetsova', '10000000-0000-4000-8000-000000000004', 'Windows 10'),
  ('20000000-0000-4000-8000-000000000005', 'PC-005', 'dev-smirnov', '10000000-0000-4000-8000-000000000005', 'Windows 11'),
  ('20000000-0000-4000-8000-000000000006', 'PC-006', 'sales-popova', '10000000-0000-4000-8000-000000000006', 'Windows 10'),
  ('20000000-0000-4000-8000-000000000007', 'PC-007', 'dev-volkov', '10000000-0000-4000-8000-000000000007', 'Windows 11'),
  ('20000000-0000-4000-8000-000000000008', 'PC-008', 'fin-sokolova', '10000000-0000-4000-8000-000000000008', 'Windows 10'),
  ('20000000-0000-4000-8000-000000000009', 'PC-009', 'sales-morozov', '10000000-0000-4000-8000-000000000009', 'Windows 11'),
  ('20000000-0000-4000-8000-000000000010', 'PC-010', 'qa-novikova', '10000000-0000-4000-8000-000000000010', 'Windows 11')
ON CONFLICT (id) DO NOTHING;

INSERT INTO license_agreements (id, name, license_type, commercial_allowed, review_status, agreement_text) VALUES
  ('30000000-0000-4000-8000-000000000001', 'Microsoft Office Volume License (демо)', 'COMMERCIAL', TRUE, 'REVIEWED',
   'This software is licensed, not sold. Each license permits installation on one device. Redistribution of the software is prohibited. A valid subscription is required for updates. Commercial use is permitted under the volume licensing agreement.'),
  ('30000000-0000-4000-8000-000000000002', 'GNU LGPL 2.1', 'OPEN_SOURCE', TRUE, 'REVIEWED', NULL),
  ('30000000-0000-4000-8000-000000000003', 'Бесплатная лицензия производителя', 'FREEWARE', TRUE, 'REVIEWED', NULL),
  ('30000000-0000-4000-8000-000000000004', 'Visual Studio Community License (демо)', 'FREEWARE', FALSE, 'REVIEWED',
   'Individual developers may use the software for any purpose. Enterprise organizations may not use the software for commercial development except for open source projects.'),
  ('30000000-0000-4000-8000-000000000005', 'Коммерческая лицензия поставщика (демо)', 'COMMERCIAL', TRUE, 'REVIEWED', NULL),
  ('30000000-0000-4000-8000-000000000006', 'Пробная лицензия (демо)', 'TRIAL', NULL, 'UNREVIEWED',
   'You may evaluate the software for 40 days. After the evaluation period you must purchase a license or remove the software.'),
  ('30000000-0000-4000-8000-000000000007', 'Проприетарная лицензия (демо)', 'PROPRIETARY', TRUE, 'REVIEWED', NULL)
ON CONFLICT (id) DO NOTHING;

INSERT INTO software_products (id, canonical_name, normalized_name, publisher, normalized_publisher, edition, agreement_id, policy_status, description) VALUES
  ('40000000-0000-4000-8000-000000000001', 'Microsoft Office', 'microsoft office', 'Microsoft Corporation', 'microsoft', NULL, '30000000-0000-4000-8000-000000000001', 'ALLOWED', 'Офисный пакет'),
  ('40000000-0000-4000-8000-000000000002', '7-Zip', '7-zip', 'Igor Pavlov', 'igor pavlov', NULL, '30000000-0000-4000-8000-000000000002', 'ALLOWED', 'Архиватор'),
  ('40000000-0000-4000-8000-000000000003', 'Google Chrome', 'google chrome', 'Google LLC', 'google', NULL, '30000000-0000-4000-8000-000000000003', 'ALLOWED', 'Браузер'),
  ('40000000-0000-4000-8000-000000000004', 'Mozilla Firefox', 'mozilla firefox', 'Mozilla', 'mozilla', NULL, '30000000-0000-4000-8000-000000000002', 'ALLOWED', 'Браузер'),
  ('40000000-0000-4000-8000-000000000005', 'Visual Studio 2022 Community', 'visual studio 2022 community', 'Microsoft Corporation', 'microsoft', 'Community', '30000000-0000-4000-8000-000000000004', 'RESTRICTED', 'Бесплатная редакция с ограничениями для организаций'),
  ('40000000-0000-4000-8000-000000000006', 'Visual Studio 2022 Professional', 'visual studio 2022 professional', 'Microsoft Corporation', 'microsoft', 'Professional', '30000000-0000-4000-8000-000000000005', 'ALLOWED', 'Среда разработки'),
  ('40000000-0000-4000-8000-000000000007', 'Adobe Acrobat Reader', 'adobe acrobat reader', 'Adobe Inc.', 'adobe', NULL, '30000000-0000-4000-8000-000000000003', 'ALLOWED', 'Просмотр PDF'),
  ('40000000-0000-4000-8000-000000000008', 'Adobe Photoshop', 'adobe photoshop', 'Adobe Inc.', 'adobe', NULL, '30000000-0000-4000-8000-000000000005', 'ALLOWED', 'Графический редактор'),
  ('40000000-0000-4000-8000-000000000009', 'Zoom', 'zoom', 'Zoom Video Communications', 'zoom video communications', NULL, '30000000-0000-4000-8000-000000000003', 'ALLOWED', 'Видеосвязь'),
  ('40000000-0000-4000-8000-000000000010', 'Notepad++', 'notepad++', 'Notepad++ Team', 'notepad++ team', NULL, '30000000-0000-4000-8000-000000000002', 'ALLOWED', 'Текстовый редактор'),
  ('40000000-0000-4000-8000-000000000011', 'VLC media player', 'vlc media player', 'VideoLAN', 'videolan', NULL, '30000000-0000-4000-8000-000000000002', 'ALLOWED', 'Медиаплеер'),
  ('40000000-0000-4000-8000-000000000012', 'TeamViewer', 'teamviewer', 'TeamViewer', 'teamviewer', NULL, '30000000-0000-4000-8000-000000000005', 'ALLOWED', 'Удалённый доступ'),
  ('40000000-0000-4000-8000-000000000013', 'Torrent Downloader Pro', 'torrent downloader pro', 'Demo Software', 'demo software', NULL, NULL, 'FORBIDDEN', 'Демо: запрещено политикой безопасности'),
  ('40000000-0000-4000-8000-000000000014', 'Legacy Accounting Suite', 'legacy accounting suite', 'Demo Software', 'demo software', NULL, '30000000-0000-4000-8000-000000000007', 'ALLOWED', 'Демо: учётная система'),
  ('40000000-0000-4000-8000-000000000015', 'WinRAR', 'winrar', 'win.rar GmbH', 'win.rar', NULL, '30000000-0000-4000-8000-000000000006', 'UNKNOWN', 'Архиватор'),
  ('40000000-0000-4000-8000-000000000016', 'IntelliJ IDEA Ultimate', 'intellij idea ultimate', 'JetBrains s.r.o.', 'jetbrains s.r.o', 'Ultimate', '30000000-0000-4000-8000-000000000005', 'ALLOWED', 'Среда разработки'),
  ('40000000-0000-4000-8000-000000000017', 'Python', 'python', 'Python Software Foundation', 'python software foundation', NULL, '30000000-0000-4000-8000-000000000002', 'ALLOWED', 'Язык программирования'),
  ('40000000-0000-4000-8000-000000000018', 'Git', 'git', 'The Git Development Community', 'the git development community', NULL, '30000000-0000-4000-8000-000000000002', 'ALLOWED', 'Система контроля версий'),
  ('40000000-0000-4000-8000-000000000019', 'Docker Desktop', 'docker desktop', 'Docker Inc.', 'docker', NULL, '30000000-0000-4000-8000-000000000005', 'ALLOWED', 'Контейнеризация'),
  ('40000000-0000-4000-8000-000000000020', 'Microsoft Teams', 'microsoft teams', 'Microsoft Corporation', 'microsoft', NULL, '30000000-0000-4000-8000-000000000003', 'ALLOWED', 'Корпоративный мессенджер'),
  ('40000000-0000-4000-8000-000000000021', 'Postman', 'postman', 'Postman, Inc.', 'postman', NULL, '30000000-0000-4000-8000-000000000003', 'ALLOWED', 'Тестирование API'),
  ('40000000-0000-4000-8000-000000000022', 'Telegram Desktop', 'telegram desktop', 'Telegram FZ-LLC', 'telegram fz-llc', NULL, '30000000-0000-4000-8000-000000000003', 'RESTRICTED', 'Разрешён по согласованию')
ON CONFLICT (id) DO NOTHING;

INSERT INTO software_versions (id, product_id, version, lifecycle_status, end_of_support) VALUES
  ('50000000-0000-4000-8000-000000000001', '40000000-0000-4000-8000-000000000001', '16.0', 'CURRENT', NULL),
  ('50000000-0000-4000-8000-000000000002', '40000000-0000-4000-8000-000000000001', '15.0', 'OUTDATED', '2023-04-11'),
  ('50000000-0000-4000-8000-000000000003', '40000000-0000-4000-8000-000000000001', '14.0', 'UNSUPPORTED', '2020-10-13'),
  ('50000000-0000-4000-8000-000000000004', '40000000-0000-4000-8000-000000000014', '7.0', 'CURRENT', NULL),
  ('50000000-0000-4000-8000-000000000005', '40000000-0000-4000-8000-000000000014', '5.0', 'UNSUPPORTED', '2021-12-31'),
  ('50000000-0000-4000-8000-000000000006', '40000000-0000-4000-8000-000000000003', '142.0', 'CURRENT', NULL),
  ('50000000-0000-4000-8000-000000000007', '40000000-0000-4000-8000-000000000003', '120.0', 'OUTDATED', NULL),
  ('50000000-0000-4000-8000-000000000008', '40000000-0000-4000-8000-000000000017', '3.13', 'CURRENT', NULL),
  ('50000000-0000-4000-8000-000000000009', '40000000-0000-4000-8000-000000000017', '3.8', 'UNSUPPORTED', '2024-10-07')
ON CONFLICT (id) DO NOTHING;

INSERT INTO license_entitlements (id, product_id, agreement_id, license_metric, quantity, unit_price, currency, valid_from, expires_at, purchase_reference) VALUES
  ('60000000-0000-4000-8000-000000000001', '40000000-0000-4000-8000-000000000001', '30000000-0000-4000-8000-000000000001', 'PER_DEVICE', 6, 25000, 'RUB', '2026-01-01', '2027-01-01', 'PO-2026-001'),
  ('60000000-0000-4000-8000-000000000002', '40000000-0000-4000-8000-000000000008', '30000000-0000-4000-8000-000000000005', 'PER_USER', 2, 32000, 'RUB', '2026-01-01', '2027-01-01', 'PO-2026-002'),
  ('60000000-0000-4000-8000-000000000003', '40000000-0000-4000-8000-000000000006', '30000000-0000-4000-8000-000000000005', 'PER_DEVICE', 5, 45000, 'RUB', '2026-01-01', '2027-01-01', 'PO-2026-003'),
  ('60000000-0000-4000-8000-000000000004', '40000000-0000-4000-8000-000000000012', '30000000-0000-4000-8000-000000000005', 'PER_DEVICE', 5, 18000, 'RUB', '2025-01-01', '2026-01-01', 'PO-2025-017'),
  ('60000000-0000-4000-8000-000000000005', '40000000-0000-4000-8000-000000000014', '30000000-0000-4000-8000-000000000007', 'PER_DEVICE', 10, 12000, 'RUB', '2026-01-01', NULL, 'PO-2026-004'),
  ('60000000-0000-4000-8000-000000000006', '40000000-0000-4000-8000-000000000016', '30000000-0000-4000-8000-000000000005', 'CONCURRENT', 3, 60000, 'RUB', '2026-01-01', '2027-01-01', 'PO-2026-005'),
  ('60000000-0000-4000-8000-000000000007', '40000000-0000-4000-8000-000000000019', '30000000-0000-4000-8000-000000000005', 'PER_USER', 5, 9000, 'RUB', '2026-01-01', '2027-01-01', 'PO-2026-006')
ON CONFLICT (id) DO NOTHING;

COMMIT;
