import { useCallback, useEffect, useState } from "react";
import { Download, RefreshCw } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { api, ApiError } from "@/lib/api";
import { Alert, Badge, Button, Card, CardContent, CardHeader, CardTitle } from "@/components/ui";

interface UpdateState {
  currentVersion: string;
  latestVersion: string;
  releaseUrl: string;
  releaseNotes: string;
  assetName: string;
  checksum: string;
  status: string;
  message: string;
  checkedAt: number;
  appliedAt: number;
}

function formatTime(value: number) {
  if (!value) return "—";
  return new Date(value * 1000).toLocaleString();
}

export function UpdatesPage() {
  const { t } = useI18n();
  const [state, setState] = useState<UpdateState | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    try {
      const response = await api.get<UpdateState>("/api/panel/update");
      setState(response);
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    }
  }, [t]);

  useEffect(() => {
    void load();
  }, [load]);

  async function check() {
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      const response = await api.post<UpdateState>("/api/panel/update/check");
      setState(response);
      if (response.message) setNotice(response.message);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    } finally {
      setBusy(false);
    }
  }

  async function apply() {
    if (!state?.latestVersion) return;
    if (!window.confirm(t("updates.confirm"))) return;

    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      const response = await api.post<UpdateState>("/api/panel/update/apply", {
        version: state.latestVersion,
        confirm: true,
      });
      setState(response);
      if (response.message) setNotice(response.message);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="grid gap-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">{t("updates.title")}</h1>
        <Button onClick={check} disabled={busy}>
          <RefreshCw className="h-4 w-4" />
          {t("updates.check")}
        </Button>
      </div>

      {notice ? <Alert variant="info">{notice}</Alert> : null}
      {error ? <Alert>{error}</Alert> : null}

      <Card>
        <CardHeader>
          <CardTitle>{t("updates.title")}</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-3 text-sm">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-muted-foreground">{t("updates.current")}</span>
            <Badge>{state?.currentVersion || "—"}</Badge>
            <span className="text-muted-foreground">{t("updates.latest")}</span>
            <Badge>{state?.latestVersion || "—"}</Badge>
            <Badge>{state?.status || "—"}</Badge>
          </div>

          <div className="text-muted-foreground">
            {t("updates.checked")}: {formatTime(state?.checkedAt ?? 0)}
            {state?.appliedAt ? ` · ${t("updates.applied")}: ${formatTime(state.appliedAt)}` : ""}
          </div>

          {state?.assetName ? (
            <div className="text-muted-foreground">
              {t("updates.asset")}: {state.assetName}
            </div>
          ) : null}

          {state?.message ? <div>{state.message}</div> : null}

          {state?.releaseUrl ? (
            <a className="text-primary underline underline-offset-4" href={state.releaseUrl} target="_blank" rel="noreferrer">
              {t("updates.release")}
            </a>
          ) : null}

          {state?.latestVersion && state.latestVersion !== state.currentVersion ? (
            <div>
              <Button onClick={apply} disabled={busy}>
                <Download className="h-4 w-4" />
                {t("updates.install")}
              </Button>
            </div>
          ) : null}
        </CardContent>
      </Card>

      {state?.releaseNotes ? (
        <Card>
          <CardHeader>
            <CardTitle>{t("updates.notes")}</CardTitle>
          </CardHeader>
          <CardContent>
            <pre className="whitespace-pre-wrap text-xs text-muted-foreground">{state.releaseNotes}</pre>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}
