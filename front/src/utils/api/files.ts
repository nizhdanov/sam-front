import type { AxiosResponse } from 'axios';

import { ApiError, readApiError } from './errors';

function safeFilename(value: string) {
  let name = (value.split(/[\\/]/).at(-1) ?? '')
    .replace(/[\p{Cc}\p{Cf}<>:"|?*]/gu, '_')
    .slice(0, 200)
    .replace(/^[.\s]+|[.\s]+$/g, '');

  if (/^(?:con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\.|$)/i.test(name)) name = `_${name}`;

  return name;
}

export function getDownloadFilename(disposition: unknown, fallback = 'download') {
  const header = typeof disposition === 'string' ? disposition : '';
  const extended = /(?:^|;)\s*filename\*\s*=\s*UTF-8'[^']*'([^;]*)/i.exec(header);
  const basic = /(?:^|;)\s*filename\s*=\s*(?:"((?:\\.|[^"])*)"|([^;]*))/i.exec(header);
  let name = (basic?.[1] ?? basic?.[2] ?? '').replace(/\\(["\\])/g, '$1');

  if (extended) {
    try {
      name = decodeURIComponent(extended[1].trim());
    } catch {
      // Некорректный filename* не отменяет обычное имя или fallback.
    }
  }

  return safeFilename(name) || safeFilename(fallback) || 'download';
}

export async function downloadFile(
  response: Pick<AxiosResponse<unknown>, 'data' | 'headers' | 'status'>,
  fallback: string
) {
  const { data, headers, status } = response;
  const contentType = String(headers['content-type'] ?? '').toLowerCase();

  if (
    status >= 400 ||
    /json/i.test(contentType) ||
    (data instanceof Blob && /json/i.test(data.type))
  ) {
    throw await readApiError(data, status);
  }
  if (!(data instanceof Blob)) {
    throw new ApiError('Сервер вернул некорректный файл.', { status });
  }

  const url = URL.createObjectURL(data);
  const link = document.createElement('a');
  link.href = url;
  link.download = getDownloadFilename(headers['content-disposition'], fallback);
  document.body.append(link);

  try {
    link.click();
  } finally {
    link.remove();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }
}
