import { useEffect, useState, type ReactNode } from "react";
import { NavLink, useNavigate } from "react-router-dom";
import { Languages, LogOut, Moon, ShieldCheck, Sun } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { useAuth } from "@/lib/useAuth";
import { Badge, Button, cn } from "@/components/ui";

const navigation = [
  { to: "/operators", key: "nav.operators" },
  { to: "/roles", key: "nav.roles" },
  { to: "/audit", key: "nav.audit" },
  { to: "/tokens", key: "nav.tokens" },
  { to: "/settings", key: "nav.settings" },
] as const;

export function AppShell({ children }: { children: ReactNode }) {
  const { t, locale, setLocale } = useI18n();
  const { me, signOut } = useAuth();
  const navigate = useNavigate();
  const [dark, setDark] = useState(false);

  useEffect(() => {
    document.documentElement.classList.toggle("dark", dark);
  }, [dark]);

  return (
    <div className="min-h-screen bg-background">
      <header className="sticky top-0 z-20 border-b border-border bg-card/80 backdrop-blur">
        <div className="mx-auto flex h-14 max-w-7xl items-center gap-4 px-4">
          <div className="flex items-center gap-2 font-semibold">
            <ShieldCheck className="h-5 w-5 text-primary" />
            <span>{t("app.name")}</span>
          </div>

          <nav className="hidden flex-1 items-center gap-1 md:flex">
            {navigation.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                className={({ isActive }) =>
                  cn(
                    "rounded-md px-3 py-1.5 text-sm transition-colors",
                    isActive ? "bg-accent text-accent-foreground" : "text-muted-foreground hover:bg-accent/60",
                  )
                }
              >
                {t(item.key)}
              </NavLink>
            ))}
          </nav>

          <div className="ms-auto flex items-center gap-2">
            <Badge>{me?.operator.roleName ?? ""}</Badge>
            <Button
              variant="ghost"
              size="icon"
              title={t("common.theme")}
              onClick={() => setDark((value) => !value)}
            >
              {dark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
            </Button>
            <Button
              variant="ghost"
              size="sm"
              title={t("common.language")}
              onClick={() => setLocale(locale === "en" ? "fa" : "en")}
            >
              <Languages className="h-4 w-4" />
              {locale === "en" ? "فارسی" : "English"}
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={async () => {
                await signOut();
                navigate("/");
              }}
            >
              <LogOut className="h-4 w-4" />
              <span className="hidden sm:inline">{t("nav.signOut")}</span>
            </Button>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-7xl px-4 py-6">{children}</main>
    </div>
  );
}
