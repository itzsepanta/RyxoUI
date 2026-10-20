import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react";

export type Locale = "en" | "fa";

const dictionaries = {
  en: {
    "app.name": "RyxoUI",
    "app.subtitle": "Self hosted control panel",
    "nav.dashboard": "Dashboard",
    "nav.operators": "Operators",
    "nav.roles": "Roles",
    "nav.audit": "Audit log",
    "nav.tokens": "API tokens",
    "nav.settings": "Settings",
    "nav.signOut": "Sign out",
    "auth.username": "Username",
    "auth.password": "Password",
    "auth.signIn": "Sign in",
    "auth.setup": "Create the owner account",
    "auth.setupHint": "The panel has no operator yet. Create the first one to continue.",
    "common.save": "Save",
    "common.cancel": "Cancel",
    "common.create": "Create",
    "common.delete": "Delete",
    "common.loading": "Loading…",
    "common.empty": "Nothing here yet",
    "common.error": "Something went wrong",
    "common.language": "Language",
    "common.theme": "Theme",
    "theme.light": "Light",
    "theme.dark": "Dark",
    "operators.title": "Operators",
    "operators.new": "New operator",
    "operators.username": "Username",
    "operators.role": "Role",
    "operators.status": "Status",
    "operators.lastLogin": "Last sign in",
    "operators.actions": "Actions",
    "operators.created": "Operator created",
    "roles.title": "Roles",
    "roles.new": "New role",
    "roles.name": "Name",
    "roles.base": "Inherits from",
    "roles.scopes": "Scopes",
    "roles.description": "Description",
    "roles.builtin": "Built in",
    "audit.title": "Audit log",
    "audit.actor": "Actor",
    "audit.action": "Action",
    "audit.entity": "Entity",
    "audit.time": "Time",
    "audit.ip": "Address",
    "tokens.title": "API tokens",
    "tokens.new": "New token",
    "tokens.name": "Name",
    "tokens.scopes": "Scopes",
    "tokens.created": "Token created. Copy it now, it is shown only once.",
    "settings.title": "Settings",
    "settings.key": "Key",
    "settings.value": "Value",
    "settings.saved": "Setting saved",
  },
  fa: {
    "app.name": "RyxoUI",
    "app.subtitle": "پنل کنترل خودمیزبان",
    "nav.dashboard": "داشبورد",
    "nav.operators": "اپراتورها",
    "nav.roles": "نقش‌ها",
    "nav.audit": "گزارش تغییرات",
    "nav.tokens": "توکن‌های API",
    "nav.settings": "تنظیمات",
    "nav.signOut": "خروج",
    "auth.username": "نام کاربری",
    "auth.password": "گذرواژه",
    "auth.signIn": "ورود",
    "auth.setup": "ساخت حساب مالک",
    "auth.setupHint": "هنوز اپراتوری ساخته نشده است. برای ادامه اولین حساب را بسازید.",
    "common.save": "ذخیره",
    "common.cancel": "انصراف",
    "common.create": "ساختن",
    "common.delete": "حذف",
    "common.loading": "در حال بارگذاری…",
    "common.empty": "هنوز چیزی اینجا نیست",
    "common.error": "خطایی رخ داد",
    "common.language": "زبان",
    "common.theme": "پوسته",
    "theme.light": "روشن",
    "theme.dark": "تیره",
    "operators.title": "اپراتورها",
    "operators.new": "اپراتور جدید",
    "operators.username": "نام کاربری",
    "operators.role": "نقش",
    "operators.status": "وضعیت",
    "operators.lastLogin": "آخرین ورود",
    "operators.actions": "کنش‌ها",
    "operators.created": "اپراتور ساخته شد",
    "roles.title": "نقش‌ها",
    "roles.new": "نقش جدید",
    "roles.name": "نام",
    "roles.base": "ارث‌بری از",
    "roles.scopes": "دامنه‌ها",
    "roles.description": "توضیح",
    "roles.builtin": "درون‌ساخت",
    "audit.title": "گزارش تغییرات",
    "audit.actor": "عامل",
    "audit.action": "کنش",
    "audit.entity": "موجودیت",
    "audit.time": "زمان",
    "audit.ip": "نشانی",
    "tokens.title": "توکن‌های API",
    "tokens.new": "توکن جدید",
    "tokens.name": "نام",
    "tokens.scopes": "دامنه‌ها",
    "tokens.created": "توکن ساخته شد. همین حالا کپی کنید، فقط یک بار نشان داده می‌شود.",
    "settings.title": "تنظیمات",
    "settings.key": "کلید",
    "settings.value": "مقدار",
    "settings.saved": "تنظیم ذخیره شد",
  },
} as const;

export type TranslationKey = keyof (typeof dictionaries)["en"];

interface I18nValue {
  locale: Locale;
  dir: "ltr" | "rtl";
  t: (key: TranslationKey) => string;
  setLocale: (locale: Locale) => void;
}

const I18nContext = createContext<I18nValue | null>(null);

const storageKey = "ryxoui.locale";

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(() => {
    const stored = typeof localStorage !== "undefined" ? localStorage.getItem(storageKey) : null;
    return stored === "fa" ? "fa" : "en";
  });

  const dir: "ltr" | "rtl" = locale === "fa" ? "rtl" : "ltr";

  useEffect(() => {
    document.documentElement.lang = locale;
    document.documentElement.dir = dir;
    localStorage.setItem(storageKey, locale);
  }, [locale, dir]);

  const value = useMemo<I18nValue>(
    () => ({
      locale,
      dir,
      t: (key: TranslationKey) => dictionaries[locale][key] ?? key,
      setLocale: setLocaleState,
    }),
    [locale, dir],
  );

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nValue {
  const value = useContext(I18nContext);
  if (!value) {
    throw new Error("useI18n must be used inside I18nProvider");
  }
  return value;
}
