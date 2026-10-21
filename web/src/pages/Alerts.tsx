import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { TriangleAlert } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { api, ApiError } from "@/lib/api";
import { Alert, Badge, Card, CardContent, Spinner } from "@/components/ui";

export interface NodeAlert {
  id: string;
  nodeId: string;
  kind: string;
  severity: string;
  message: string;
  raisedAt: number;
  clearedAt: number;
}

// AlertsPage lists the warnings of every node.
export function AlertsPage() {
  const { t } = useI18n();
  const [alerts, setAlerts] = useState<NodeAlert[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const response = await api.list<NodeAlert>("/api/alerts");
      setAlerts(response.data);
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <div className="grid gap-4">
      <div className="flex items-center gap-2">
        <TriangleAlert className="h-5 w-5 text-primary" />
        <h1 className="text-xl font-semibold">{t("alerts.title")}</h1>
      </div>
      <p className="text-sm text-muted-foreground">{t("alerts.hint")}</p>

      {error ? <Alert>{error}</Alert> : null}

      <Card>
        <CardContent className="grid gap-3">
          {loading ? (
            <div className="grid place-items-center p-8">
              <Spinner />
            </div>
          ) : alerts.length === 0 ? (
            <p className="p-4 text-center text-sm text-muted-foreground">{t("common.empty")}</p>
          ) : (
            alerts.map((alert) => (
              <div key={alert.id} className="flex flex-wrap items-center gap-3 rounded-md border border-border p-3">
                <Badge
                  className={
                    alert.severity === "critical"
                      ? "border-destructive/40 bg-destructive/10 text-destructive"
                      : "border-amber-500/40 bg-amber-500/10 text-amber-600"
                  }
                >
                  {alert.kind}
                </Badge>
                <span className="text-sm">{alert.message}</span>
                <span className="ms-auto text-xs text-muted-foreground">
                  {alert.raisedAt ? new Date(alert.raisedAt * 1000).toLocaleString() : ""}
                </span>
              </div>
            ))
          )}
        </CardContent>
      </Card>
    </div>
  );
}

// NodeAlertsCard shows the warnings of one node.
export function NodeAlertsCard({ nodeId }: { nodeId: string }) {
  const { t } = useI18n();
  const [alerts, setAlerts] = useState<NodeAlert[]>([]);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const response = await api.list<NodeAlert>(`/api/nodes/${nodeId}/alerts`);
      setAlerts(response.data);
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    }
  }, [nodeId, t]);

  useEffect(() => {
    void load();
  }, [load]);

  if (error) {
    return (
      <Card>
        <CardContent className="grid gap-2 pt-4">
          <Alert>{error}</Alert>
        </CardContent>
      </Card>
    );
  }

  if (alerts.length === 0) {
    return null;
  }

  return (
    <Card>
      <CardContent className="grid gap-2 pt-4">
        {alerts.map((alert) => (
          <div key={alert.id} className="flex flex-wrap items-center gap-3 rounded-md border border-border p-3">
            <Badge
              className={
                alert.severity === "critical"
                  ? "border-destructive/40 bg-destructive/10 text-destructive"
                  : "border-amber-500/40 bg-amber-500/10 text-amber-600"
              }
            >
              {alert.kind}
            </Badge>
            <span className="text-sm">{alert.message}</span>
            <Link className="ms-auto text-xs text-muted-foreground underline" to="/alerts">
              {t("alerts.all")}
            </Link>
          </div>
        ))}
      </CardContent>
    </Card>
  );
}
