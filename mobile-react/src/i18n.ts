import i18n from 'i18next';
import LanguageDetector from 'i18next-browser-languagedetector';
import resourcesToBackend from 'i18next-resources-to-backend';
import { initReactI18next } from 'react-i18next';

type LocaleModule = { default: Record<string, unknown> };

const localeModules = import.meta.glob<LocaleModule>('./locales/*.json');
const supportedLanguages = Object.keys(localeModules).map((path) => path.match(/\/([^/]+)\.json$/)?.[1] ?? 'en');

export async function initI18n() {
    if (i18n.isInitialized) return i18n;

    await i18n
        .use(LanguageDetector)
        .use(resourcesToBackend((language: string) => {
            const loader = localeModules[`./locales/${language}.json`] ?? localeModules['./locales/en.json'];
            return loader().then((mod) => mod.default);
        }))
        .use(initReactI18next)
        .init({
            fallbackLng: 'en',
            supportedLngs: supportedLanguages,
            nonExplicitSupportedLngs: true,
            interpolation: {
                escapeValue: false
            },
            detection: {
                order: ['localStorage', 'navigator', 'htmlTag'],
                caches: ['localStorage'],
                lookupLocalStorage: 'vrcx-mobile-language'
            }
        });

    return i18n;
}

export default i18n;
