import i18next from 'i18next';
import { Trans } from 'react-i18next';

export const LANGUAGES: Array<{ code: string; name: string }> = [
  { code: 'en-US', name: 'English' },
  { code: 'fr-FR', name: 'Français' },
  { code: 'es-ES', name: 'Español' },
  { code: 'de-DE', name: 'Deutsch' },
  { code: 'zh-Hans', name: '中文（简体）' },
  { code: 'pt-BR', name: 'Português Brasileiro' },
  { code: 'zh-Hant', name: '中文（繁體）' },
  { code: 'it-IT', name: 'Italiano' },
  { code: 'ja-JP', name: '日本語' },
  { code: 'id-ID', name: 'Bahasa Indonesia' },
  { code: 'ko-KR', name: '한국어' },
  { code: 'ru-RU', name: 'Русский' },
  { code: 'cs-CZ', name: 'Čeština' },
  { code: 'nl-NL', name: 'Nederlands' },
  { code: 'hu-HU', name: 'Magyar' },
  { code: 'pt-PT', name: 'Português' },
  { code: 'pl-PL', name: 'Polski' },
  { code: 'sv-SE', name: 'Svenska' },
  { code: 'tr-TR', name: 'Türkçe' },
];

export function t(id: string, defaultMessage: string, values?: Record<string, unknown>): string {
  const translated = i18next.t(id, { defaultValue: defaultMessage, ...(values ?? {}) });
  return typeof translated === 'string' ? translated : String(translated);
}

export { Trans };
