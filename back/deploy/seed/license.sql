BEGIN;

INSERT INTO devices (id, serial_number, hostname, operating_system, device_type, status) VALUES
  ('20000000-0000-4000-8000-000000000001', 'PC-001', 'dev-01', 'Windows 11', 'PHYSICAL', 'ACTIVE'),
  ('20000000-0000-4000-8000-000000000002', 'PC-002', 'it-01', 'Windows 11', 'PHYSICAL', 'ACTIVE'),
  ('20000000-0000-4000-8000-000000000003', 'PC-003', 'it-02', 'Windows 11', 'PHYSICAL', 'ACTIVE'),
  ('20000000-0000-4000-8000-000000000004', 'PC-004', 'fin-01', 'Windows 10', 'PHYSICAL', 'ACTIVE'),
  ('20000000-0000-4000-8000-000000000005', 'PC-005', 'dev-02', 'Windows 11', 'PHYSICAL', 'ACTIVE'),
  ('20000000-0000-4000-8000-000000000006', 'PC-006', 'sales-01', 'Windows 10', 'PHYSICAL', 'ACTIVE'),
  ('20000000-0000-4000-8000-000000000007', 'PC-007', 'dev-03', 'Windows 11', 'PHYSICAL', 'ACTIVE'),
  ('20000000-0000-4000-8000-000000000008', 'PC-008', 'fin-02', 'Windows 10', 'PHYSICAL', 'ACTIVE'),
  ('20000000-0000-4000-8000-000000000009', 'PC-009', 'sales-02', 'Windows 11', 'PHYSICAL', 'ACTIVE'),
  ('20000000-0000-4000-8000-000000000010', 'PC-010', 'qa-01', 'Windows 11', 'PHYSICAL', 'ACTIVE')
ON CONFLICT (id) DO UPDATE SET hostname = EXCLUDED.hostname, operating_system = EXCLUDED.operating_system;

INSERT INTO license_agreements (id, name, license_type, cost_model, commercial_allowed, review_status, agreement_text) VALUES
  ('30000000-0000-4000-8000-000000000001', 'EULA Microsoft Office (демо)', 'COMMERCIAL', 'PAID', TRUE, 'REVIEWED',
   E'This software is licensed, not sold.\n\nEach license permits installation on one device.\n\nRedistribution of the software is prohibited.\n\nA valid subscription is required for updates. The subscription renews automatically each year unless cancelled.\n\nCommercial use is permitted under the volume licensing agreement.'),
  ('30000000-0000-4000-8000-000000000002', 'GNU LGPL v.2.1', 'OPEN_SOURCE', 'FREE', TRUE, 'REVIEWED',
   E'The library may be used, copied and modified free of charge.\n\nModified versions of the library must be distributed under the same license.\n\nApplications that only link to the library may use any license.'),
  ('30000000-0000-4000-8000-000000000003', 'Бесплатная лицензия производителя', 'COMMERCIAL', 'FREE', TRUE, 'REVIEWED', NULL),
  ('30000000-0000-4000-8000-000000000004', 'Visual Studio Community License (демо)', 'COMMERCIAL', 'FREE', FALSE, 'REVIEWED',
   E'Individual developers may use the software for any purpose.\n\nEnterprise organizations may not use the software for commercial development except for open source projects.'),
  ('30000000-0000-4000-8000-000000000005', 'Коммерческая лицензия поставщика (демо)', 'COMMERCIAL', 'PAID', TRUE, 'REVIEWED', NULL),
  ('30000000-0000-4000-8000-000000000006', 'Условно-бесплатная лицензия (демо)', 'TRIAL', 'SHAREWARE', NULL, 'UNREVIEWED',
   E'You may evaluate the software for 40 days.\n\nAfter the evaluation period you must purchase a license or remove the software.'),
  ('30000000-0000-4000-8000-000000000007', 'Проприетарная лицензия (демо)', 'COMMERCIAL', 'PAID', TRUE, 'REVIEWED', NULL),
  ('30000000-0000-4000-8000-000000000008', 'MIT License', 'OPEN_SOURCE', 'FREE', TRUE, 'REVIEWED',
   E'Permission is granted free of charge to use, copy, modify, merge, publish and distribute the software.\n\nThe copyright notice must be included in all copies.\n\nThe software is provided as is, without warranty of any kind.'),
  ('30000000-0000-4000-8000-000000000009', 'Apache License 2.0', 'OPEN_SOURCE', 'FREE', TRUE, 'REVIEWED',
   E'The software may be used, reproduced and distributed free of charge.\n\nA copy of the license and notices of changes must accompany redistributions.\n\nA patent license is granted by contributors.\n\nThe software is provided as is, without warranty of any kind.'),
  ('30000000-0000-4000-8000-000000000010', 'GNU GPL v.2', 'OPEN_SOURCE', 'FREE', TRUE, 'REVIEWED',
   E'The program may be used, copied and modified free of charge.\n\nDerived works must be distributed under the same license with source code.\n\nThe program is provided without warranty.')
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, license_type = EXCLUDED.license_type, cost_model = EXCLUDED.cost_model,
  commercial_allowed = EXCLUDED.commercial_allowed, agreement_text = EXCLUDED.agreement_text;

INSERT INTO software_products (id, canonical_name, normalized_name, publisher, normalized_publisher, edition, agreement_id,
                               status, status_comment, purpose, section, class, in_bpo, current_version) VALUES
  ('40000000-0000-4000-8000-000000000001', 'Microsoft Office', 'microsoft office', 'Microsoft Corporation', 'microsoft', NULL, '30000000-0000-4000-8000-000000000001', 'ACTUAL', NULL, 'Офисный пакет', 'Офисное ПО', 'Офисные пакеты', FALSE, '16.0'),
  ('40000000-0000-4000-8000-000000000002', '7-Zip', '7-zip', 'Igor Pavlov', 'igor pavlov', NULL, '30000000-0000-4000-8000-000000000002', 'ACTUAL', NULL, 'Архиватор', 'Утилиты', 'Архиваторы', TRUE, '24.00'),
  ('40000000-0000-4000-8000-000000000003', 'Google Chrome', 'google chrome', 'Google LLC', 'google', NULL, '30000000-0000-4000-8000-000000000003', 'ACTUAL', NULL, 'Браузер', 'Интернет', 'Браузеры', TRUE, '142.0'),
  ('40000000-0000-4000-8000-000000000004', 'Mozilla Firefox', 'mozilla firefox', 'Mozilla', 'mozilla', NULL, '30000000-0000-4000-8000-000000000010', 'ACTUAL', NULL, 'Браузер', 'Интернет', 'Браузеры', TRUE, '145.0'),
  ('40000000-0000-4000-8000-000000000005', 'Visual Studio 2022 Community', 'visual studio 2022 community', 'Microsoft Corporation', 'microsoft', 'Community', '30000000-0000-4000-8000-000000000004', 'ACTUAL', NULL, 'Среда разработки с ограничениями для организаций', 'Разработка', 'Среды разработки', FALSE, '17.12'),
  ('40000000-0000-4000-8000-000000000006', 'Visual Studio 2022 Professional', 'visual studio 2022 professional', 'Microsoft Corporation', 'microsoft', 'Professional', '30000000-0000-4000-8000-000000000005', 'ACTUAL', NULL, 'Среда разработки', 'Разработка', 'Среды разработки', FALSE, '17.12'),
  ('40000000-0000-4000-8000-000000000007', 'Adobe Acrobat Reader', 'adobe acrobat reader', 'Adobe Inc.', 'adobe', NULL, '30000000-0000-4000-8000-000000000003', 'ACTUAL', NULL, 'Просмотр PDF', 'Офисное ПО', 'Просмотрщики', TRUE, '25.001'),
  ('40000000-0000-4000-8000-000000000008', 'Adobe Photoshop', 'adobe photoshop', 'Adobe Inc.', 'adobe', NULL, '30000000-0000-4000-8000-000000000005', 'ACTUAL', NULL, 'Графический редактор', 'Графика', 'Растровые редакторы', FALSE, NULL),
  ('40000000-0000-4000-8000-000000000009', 'Zoom', 'zoom', 'Zoom Video Communications', 'zoom video communications', NULL, '30000000-0000-4000-8000-000000000003', 'ACTUAL', NULL, 'Видеосвязь', 'Коммуникации', 'Видеосвязь', FALSE, NULL),
  ('40000000-0000-4000-8000-000000000010', 'Notepad++', 'notepad++', 'Notepad++ Team', 'notepad++ team', NULL, '30000000-0000-4000-8000-000000000010', 'ACTUAL', NULL, 'Текстовый редактор', 'Утилиты', 'Текстовые редакторы', TRUE, '8.7'),
  ('40000000-0000-4000-8000-000000000011', 'VLC media player', 'vlc media player', 'VideoLAN', 'videolan', NULL, '30000000-0000-4000-8000-000000000010', 'ACTUAL', NULL, 'Медиаплеер', 'Мультимедиа', 'Проигрыватели', TRUE, '3.0.21'),
  ('40000000-0000-4000-8000-000000000012', 'TeamViewer', 'teamviewer', 'TeamViewer', 'teamviewer', NULL, '30000000-0000-4000-8000-000000000005', 'ACTUAL', NULL, 'Удалённый доступ', 'Утилиты', 'Удалённый доступ', FALSE, NULL),
  ('40000000-0000-4000-8000-000000000013', 'Torrent Downloader Pro', 'torrent downloader pro', 'Demo Software', 'demo software', NULL, NULL, 'FORBIDDEN', 'P2P-клиент: риск утечки данных и нарушения авторских прав', 'Загрузка торрентов', 'Интернет', 'P2P-клиенты', FALSE, NULL),
  ('40000000-0000-4000-8000-000000000014', 'Legacy Accounting Suite', 'legacy accounting suite', 'Demo Software', 'demo software', NULL, '30000000-0000-4000-8000-000000000007', 'ACTUAL', NULL, 'Учётная система', 'Бизнес-приложения', 'Учёт', FALSE, '7.0'),
  ('40000000-0000-4000-8000-000000000015', 'WinRAR', 'winrar', 'win.rar GmbH', 'win.rar', NULL, '30000000-0000-4000-8000-000000000006', 'ACTUAL', NULL, 'Архиватор', 'Утилиты', 'Архиваторы', FALSE, NULL),
  ('40000000-0000-4000-8000-000000000016', 'IntelliJ IDEA Ultimate', 'intellij idea ultimate', 'JetBrains s.r.o.', 'jetbrains s.r.o', 'Ultimate', '30000000-0000-4000-8000-000000000005', 'ACTUAL', NULL, 'Среда разработки', 'Разработка', 'Среды разработки', FALSE, NULL),
  ('40000000-0000-4000-8000-000000000017', 'Python', 'python', 'Python Software Foundation', 'python software foundation', NULL, '30000000-0000-4000-8000-000000000008', 'ACTUAL', NULL, 'Язык программирования', 'Разработка', 'Языки и SDK', TRUE, '3.13.0'),
  ('40000000-0000-4000-8000-000000000018', 'Git', 'git', 'The Git Development Community', 'the git development community', NULL, '30000000-0000-4000-8000-000000000010', 'ACTUAL', NULL, 'Система контроля версий', 'Разработка', 'Контроль версий', TRUE, NULL),
  ('40000000-0000-4000-8000-000000000019', 'Docker Desktop', 'docker desktop', 'Docker Inc.', 'docker', NULL, '30000000-0000-4000-8000-000000000005', 'ACTUAL', NULL, 'Контейнеризация', 'Разработка', 'Контейнеризация', FALSE, NULL),
  ('40000000-0000-4000-8000-000000000020', 'Microsoft Teams', 'microsoft teams', 'Microsoft Corporation', 'microsoft', NULL, '30000000-0000-4000-8000-000000000003', 'ACTUAL', NULL, 'Корпоративный мессенджер', 'Коммуникации', 'Мессенджеры', TRUE, NULL),
  ('40000000-0000-4000-8000-000000000021', 'Postman', 'postman', 'Postman, Inc.', 'postman', NULL, '30000000-0000-4000-8000-000000000003', 'ACTUAL', NULL, 'Тестирование API', 'Разработка', 'Инструменты API', FALSE, NULL),
  ('40000000-0000-4000-8000-000000000022', 'Telegram Desktop', 'telegram desktop', 'Telegram FZ-LLC', 'telegram fz-llc', NULL, '30000000-0000-4000-8000-000000000003', 'OUTDATED', NULL, 'Мессенджер, выводится из использования', 'Коммуникации', 'Мессенджеры', FALSE, NULL)
ON CONFLICT (id) DO UPDATE SET agreement_id = EXCLUDED.agreement_id, status = EXCLUDED.status, status_comment = EXCLUDED.status_comment,
  purpose = EXCLUDED.purpose, section = EXCLUDED.section, class = EXCLUDED.class, in_bpo = EXCLUDED.in_bpo,
  current_version = EXCLUDED.current_version;

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

INSERT INTO license_entitlements (id, product_id, agreement_id, license_metric, quantity, valid_from, expires_at, purchase_reference) VALUES
  ('60000000-0000-4000-8000-000000000001', '40000000-0000-4000-8000-000000000001', '30000000-0000-4000-8000-000000000001', 'PER_DEVICE', 6, '2026-01-01', '2027-01-01', 'Договор № 2026-001'),
  ('60000000-0000-4000-8000-000000000002', '40000000-0000-4000-8000-000000000008', '30000000-0000-4000-8000-000000000005', 'PER_DEVICE', 2, '2026-01-01', '2027-01-01', 'Договор № 2026-002'),
  ('60000000-0000-4000-8000-000000000003', '40000000-0000-4000-8000-000000000006', '30000000-0000-4000-8000-000000000005', 'PER_DEVICE', 5, '2026-01-01', '2027-01-01', 'Договор № 2026-003'),
  ('60000000-0000-4000-8000-000000000004', '40000000-0000-4000-8000-000000000012', '30000000-0000-4000-8000-000000000005', 'PER_DEVICE', 5, '2025-01-01', '2026-01-01', 'Договор № 2025-017'),
  ('60000000-0000-4000-8000-000000000005', '40000000-0000-4000-8000-000000000014', '30000000-0000-4000-8000-000000000007', 'PER_DEVICE', 10, '2026-01-01', NULL, 'Договор № 2026-004'),
  ('60000000-0000-4000-8000-000000000006', '40000000-0000-4000-8000-000000000016', '30000000-0000-4000-8000-000000000005', 'CONCURRENT', 3, '2026-01-01', '2027-01-01', 'Договор № 2026-005'),
  ('60000000-0000-4000-8000-000000000007', '40000000-0000-4000-8000-000000000019', '30000000-0000-4000-8000-000000000005', 'PER_DEVICE', 5, '2026-01-01', '2026-12-05', 'Договор № 2026-006')
ON CONFLICT (id) DO UPDATE SET license_metric = EXCLUDED.license_metric, quantity = EXCLUDED.quantity,
  valid_from = EXCLUDED.valid_from, expires_at = EXCLUDED.expires_at, purchase_reference = EXCLUDED.purchase_reference;

COMMIT;
