import i18next from 'i18next';
import { Trans } from 'react-i18next';

export const LANGUAGES: Array<{ code: string; name: string }> = [];

export function t(id: string, defaultMessage: string, values?: Record<string, unknown>): string {
  const translated = i18next.t(id, { defaultValue: defaultMessage, ...(values ?? {}) });
  return typeof translated === 'string' ? translated : String(translated);
}

export { Trans };
