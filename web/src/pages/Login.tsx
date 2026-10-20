import { useState, type FormEvent } from "react";
import { ShieldCheck } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { api, ApiError } from "@/lib/api";
import { Alert, Button, Card, CardContent, CardHeader, CardTitle, Input } from "@/components/ui";

export function LoginPage({ onSignIn }: { onSignIn: () => void }) {
  const { t } = useI18n();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await api.post("/api/login", { username, password });
      onSignIn();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="grid min-h-screen place-items-center px-4">
      <Card className="w-full max-w-sm">
        <CardHeader className="items-center text-center">
          <ShieldCheck className="h-8 w-8 text-primary" />
          <CardTitle>{t("app.name")}</CardTitle>
          <p className="text-sm text-muted-foreground">{t("app.subtitle")}</p>
        </CardHeader>
        <CardContent>
          <form className="grid gap-3" onSubmit={submit}>
            <label className="grid gap-1 text-sm">
              <span className="text-muted-foreground">{t("auth.username")}</span>
              <Input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" required />
            </label>
            <label className="grid gap-1 text-sm">
              <span className="text-muted-foreground">{t("auth.password")}</span>
              <Input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="current-password"
                required
              />
            </label>
            {error ? <Alert>{error}</Alert> : null}
            <Button type="submit" disabled={busy}>
              {busy ? t("common.loading") : t("auth.signIn")}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
