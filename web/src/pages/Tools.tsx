import { useState } from "react";
import { KeyRound, RefreshCw, ShieldCheck, Sparkles } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { api, ApiError } from "@/lib/api";
import { Alert, Button, Card, CardContent, CardHeader, CardTitle, Input } from "@/components/ui";

interface X25519Key {
  publicKey: string;
  privateKey: string;
}

interface ECHKey {
  publicKey: string;
  privateKey: string;
  config: string;
}

interface VLESSKey {
  clientKey: string;
  serverKey: string;
}

export function ToolsPage() {
  const { t } = useI18n();
  const [x25519, setX25519] = useState<X25519Key | null>(null);
  const [ech, setECH] = useState<ECHKey | null>(null);
  const [vless, setVLESS] = useState<VLESSKey | null>(null);
  const [publicName, setPublicName] = useState("cloudflare.com");
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function makeX25519() {
    setBusy("x25519");
    setError(null);
    try {
      setX25519(await api.post<X25519Key>("/api/keys/x25519"));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    } finally {
      setBusy(null);
    }
  }

  async function makeECH() {
    setBusy("ech");
    setError(null);
    try {
      setECH(await api.post<ECHKey>("/api/keys/ech", { publicName }));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    } finally {
      setBusy(null);
    }
  }

  async function makeVLESS() {
    setBusy("vless");
    setError(null);
    try {
      setVLESS(await api.post<VLESSKey>("/api/keys/vless-encryption"));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="grid gap-4">
      <div className="flex items-center gap-2">
        <Sparkles className="h-5 w-5 text-primary" />
        <h1 className="text-xl font-semibold">{t("tools.title")}</h1>
      </div>
      <p className="text-sm text-muted-foreground">{t("tools.hint")}</p>

      {error ? <Alert>{error}</Alert> : null}

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <KeyRound className="h-4 w-4" />
            {t("tools.x25519")}
          </CardTitle>
        </CardHeader>
        <CardContent className="grid gap-3 text-sm">
          <p className="text-muted-foreground">{t("tools.x25519Hint")}</p>
          <Button className="w-fit" variant="outline" disabled={busy !== null} onClick={makeX25519}>
            <RefreshCw className="h-4 w-4" />
            {t("tools.make")}
          </Button>
          {x25519 ? (
            <div className="grid gap-2">
              <Field label={t("tools.publicKey")} value={x25519.publicKey} />
              <Field label={t("tools.privateKey")} value={x25519.privateKey} secret />
            </div>
          ) : null}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <ShieldCheck className="h-4 w-4" />
            {t("tools.ech")}
          </CardTitle>
        </CardHeader>
        <CardContent className="grid gap-3 text-sm">
          <p className="text-muted-foreground">{t("tools.echHint")}</p>
          <div className="flex flex-wrap items-center gap-2">
            <Input
              className="max-w-xs"
              placeholder={t("tools.publicName")}
              value={publicName}
              onChange={(event) => setPublicName(event.target.value)}
            />
            <Button variant="outline" disabled={busy !== null} onClick={makeECH}>
              <RefreshCw className="h-4 w-4" />
              {t("tools.make")}
            </Button>
          </div>
          {ech ? (
            <div className="grid gap-2">
              <Field label={t("tools.publicKey")} value={ech.publicKey} />
              <Field label={t("tools.config")} value={ech.config} />
              <Field label={t("tools.privateKey")} value={ech.privateKey} secret />
            </div>
          ) : null}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <KeyRound className="h-4 w-4" />
            {t("tools.vlessEncryption")}
          </CardTitle>
        </CardHeader>
        <CardContent className="grid gap-3 text-sm">
          <p className="text-muted-foreground">{t("tools.vlessEncryptionHint")}</p>
          <Button className="w-fit" variant="outline" disabled={busy !== null} onClick={makeVLESS}>
            <RefreshCw className="h-4 w-4" />
            {t("tools.make")}
          </Button>
          {vless ? (
            <div className="grid gap-2">
              <Field label={t("tools.clientKey")} value={vless.clientKey} />
              <Field label={t("tools.serverKey")} value={vless.serverKey} secret />
            </div>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}

function Field({ label, value, secret }: { label: string; value: string; secret?: boolean }) {
  return (
    <div className="grid gap-1">
      <span className="text-muted-foreground">{label}</span>
      <code className="break-all rounded-md border border-border bg-muted/40 p-2 text-xs">
        {secret && value ? "•".repeat(Math.min(value.length, 48)) : value}
      </code>
    </div>
  );
}
