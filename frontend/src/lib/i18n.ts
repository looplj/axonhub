import i18n from 'i18next';
import LanguageDetector from 'i18next-browser-languagedetector';
import { initReactI18next } from 'react-i18next';

function mergeTranslations(...translations: Array<Record<string, unknown>>) {
  return Object.assign({}, ...translations);
}

type LocaleModule = {
  default: Record<string, unknown>;
};

function getModuleDefaultExport(module: unknown): Record<string, unknown> {
  if (module && typeof module === 'object' && 'default' in module) {
    return (module as LocaleModule).default;
  }
  return module as Record<string, unknown>;
}

const enModules = import.meta.glob('../locales/en/*.json', { eager: true }) as Record<string, unknown>;
const zhCNModules = import.meta.glob('../locales/zh-CN/*.json', { eager: true }) as Record<string, unknown>;

const enTranslation = mergeTranslations(...Object.values(enModules).map(getModuleDefaultExport));
const zhTranslation = mergeTranslations(...Object.values(zhCNModules).map(getModuleDefaultExport));

const resources = {
  en: {
    translation: enTranslation,
  },
  zh: {
    translation: zhTranslation,
  },
  'zh-CN': {
    translation: zhTranslation,
  },
};

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    fallbackLng: 'en',
    debug: false,
    supportedLngs: ['en', 'zh', 'zh-CN'],

    interpolation: {
      escapeValue: false, // React 已经默认转义了
    },

    detection: {
      order: ['localStorage', 'navigator', 'htmlTag'],
      caches: ['localStorage'],
      convertDetectedLanguage: (lng: string) => {
        const normalized = lng.toLowerCase();
        if (normalized === 'zh-cn' || normalized.startsWith('zh-')) {
          return 'zh';
        }
        return lng;
      },
    },
  });

export default i18n;
