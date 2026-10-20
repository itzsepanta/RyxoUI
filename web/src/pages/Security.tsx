import { useCallback, useEffect, useState } from "react";
import QRCode from "qrcode";
import { KeyRound, ShieldCheck, ShieldOff } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { api, ApiError } from "@/lib/api";
import {
  Alert,
  Badge,
  Button,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  Input,
} from "@/components/ui";

interface TwoFactorStatus {
  enabled: boolean;
  recoveryCodes: number;
}

export function SecurityPage() {
  const { t } = useI18n();
  const [status, setStatus] = useState<TwoFactorStatus | null>(null);
  const [secret, setSecret] = useState("");
  const [uri, setUri] = useState("");
  const [code, setCode] = useState("");
  const [password, setPassword] = useState("");
  const [recoveryCodes, setRecoveryCodes] = useState<string[]>([]);
  const [qrCode, setQrCode] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!uri) {
      setQrCode("");
      return;
    }
    let active = true;
    QRCode.toDataURL(uri, { width: 168, margin: 1, errorCorrectionLevel: "M" })
      .then((dataUrl) => {
        if (active) setQrCode(dataUrl);
      })
      .catch(() => {
        if (active) setQrCode("");
      });
    return () => {
      active = false;
    };
  }, [uri]);

  const load = useCallback(async () => {
    try {
      const response = await api.get<TwoFactorStatus>("/api/me/2fa");
      setStatus(response);
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    }
  }, [t]);

  useEffect(() => {
    void load();
  }, [load]);

  async function start() {
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      const response = await api.post<{ secret: string; uri: string }>("/api/me/2fa/totp");
      setSecret(response.secret);
      setUri(response.uri);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    } finally {
      setBusy(false);
    }
  }

  async function confirm() {
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      const response = await api.post<{ recoveryCodes: string[] }>("/api/me/2fa/totp/confirm", { code });
      setRecoveryCodes(response.recoveryCodes);
      setSecret("");
      setUri("");
      setCode("");
      setNotice(t("security.enabled"));
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    } finally {
      setBusy(false);
    }
  }

  async function disable() {
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      await api.delete("/api/me/2fa", { body: JSON.stringify({ password }) });
      setPassword("");
      setNotice(t("security.disabled"));
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="grid gap-4">
      <h1 className="text-xl font-semibold">{t("security.title")}</h1>

      {notice ? <Alert variant="info">{notice}</Alert> : null}
      {error ? <Alert>{error}</Alert> : null}

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            {status?.enabled ? (
              <ShieldCheck className="h-4 w-4 text-primary" />
            ) : (
              <ShieldOff className="h-4 w-4 text-muted-foreground" />
            )}
            {t("security.secondFactor")}
          </CardTitle>
        </CardHeader>
        <CardContent className="grid gap-4 text-sm">
          <div className="flex flex-wrap items-center gap-2">
            <Badge>{status?.enabled ? t("security.on") : t("security.off")}</Badge>
            {status?.enabled ? (
              <span className="text-muted-foreground">
                {t("security.codesLeft")}: {status.recoveryCodes}
              </span>
            ) : null}
          </div>

          {status?.enabled ? (
            <div className="grid gap-2 border-t border-border pt-4">
              <p className="text-muted-foreground">{t("security.disableHint")}</p>
              <div className="flex gap-2">
                <Input
                  type="password"
                  placeholder={t("auth.password")}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  autoComplete="current-password"
                />
                <Button variant="destructive" onClick={disable} disabled={busy || !password}>
                  {t("security.disable")}
                </Button>
              </div>
            </div>
          ) : uri === "" ? (
            <div>
              <Button onClick={start} disabled={busy}>
                <KeyRound className="h-4 w-4" />
                {t("security.enable")}
              </Button>
            </div>
          ) : (
            <div className="grid gap-3">
              <p className="text-muted-foreground">{t("security.scanHint")}</p>
              <div className="flex flex-col gap-4 sm:flex-row sm:items-start">
                <div className="rounded-lg border border-border bg-white p-2">
                  {qrCode ? (
                    <img src={qrCode} alt={t("security.secondFactor")} width={168} height={168} />
                  ) : (
                    <div className="h-[168px] w-[168px] animate-pulse rounded bg-muted" />
                  )}
                </div>
                <div className="grid gap-2 text-xs">
                  <div>
                    <span className="text-muted-foreground">{t("security.secret")}:{" "}</span>
                    <code className="rounded bg-muted px-1 py-0.5">{secret}</code>
                  </div>
                  <div className="break-all text-muted-foreground">{uri}</div>
                </div>
              </div>
              <div className="flex gap-2">
                <Input
                  value={code}
                  onChange={(e) => setCode(e.target.value)}
                  inputMode="numeric"
                  placeholder={t("auth.code")}
                  className="max-w-48"
                />
                <Button onClick={confirm} disabled={busy || code.length !== 6}>
                  {t("security.confirm")}
                </Button>
              </div>
            </div>
          )}

          {recoveryCodes.length > 0 ? (
            <div className="grid gap-2 border-t border-border pt-4">
              <p className="font-medium">{t("security.recoveryTitle")}</p>
              <p className="text-muted-foreground">{t("security.recoveryHint")}</p>
              <ul className="grid grid-cols-2 gap-1 font-mono text-xs">
                {recoveryCodes.map((item) => (
                  <li key={item} className="rounded bg-muted px-2 py-1">
                    {item}
                  </li>
                ))}
              </ul>
            </div>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}
