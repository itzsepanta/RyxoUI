import { useCallback, useEffect, useState } from "react";
import { RefreshCw, ScrollText } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { api, ApiError } from "@/lib/api";
import { Alert, Button, Card, CardContent, CardHeader, CardTitle, Spinner } from "@/components/ui";

interface LogLine {
  time: string;
  level: string;
  message: string;
}

interface NodeLogCardProps {
  nodeId: string;
}

// NodeLogCard shows the log of a node and can follow it live.
export function NodeLogCard({ nodeId }: NodeLogCardProps) {
  const { t } = useI18n();
  const [lines, setLines] = useState<LogLine[]>([]);
  const [loading, setLoading] = useState(true);
  const [following, setFollowing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const response = await api.get<{ data: LogLine[] }>(`/api/nodes/${nodeId}/logs?lines=200`);
      setLines(response.data ?? []);
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    } finally {
      setLoading(false);
    }
  }, [nodeId, t]);

  useEffect(() => {
    void load();
  }, [load]);

  // The stream follows the log of the node as long as it is open.
  useEffect(() => {
    if (!following) return;

    const source = new EventSource(`/api/nodes/${nodeId}/logs/stream`);
    source.onmessage = (event) => {
      try {
        const line = JSON.parse(event.data) as LogLine;
        setLines((current) => [...current.slice(-500), line]);
      } catch {
        setLines((current) => [...current.slice(-500), { time: "", level: "", message: event.data }]);
      }
    };
    source.onerror = () => {
      setFollowing(false);
      source.close();
    };
    return () => source.close();
  }, [following, nodeId]);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <ScrollText className="h-4 w-4" />
          {t("nodes.log")}
        </CardTitle>
      </CardHeader>
      <CardContent className="grid gap-3">
        {error ? <Alert>{error}</Alert> : null}

        <div className="flex items-center gap-2">
          <Button variant="outline" onClick={load} disabled={loading}>
            <RefreshCw className="h-4 w-4" />
            {t("nodes.refresh")}
          </Button>
          <Button variant={following ? "default" : "outline"} onClick={() => setFollowing((value) => !value)}>
            {following ? t("nodes.following") : t("nodes.follow")}
          </Button>
          {following ? <span className="text-xs text-muted-foreground">{t("nodes.live")}</span> : null}
        </div>

        {loading ? (
          <div className="grid place-items-center p-6">
            <Spinner />
          </div>
        ) : lines.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("common.empty")}</p>
        ) : (
          <pre className="max-h-96 overflow-auto rounded-md bg-muted/40 p-3 text-xs leading-5">
            {lines.map((line, index) => (
              <div key={`${index}-${line.time}`} className={levelClass(line.level)}>
                {line.time ? `${line.time} ` : ""}
                {line.message}
              </div>
            ))}
          </pre>
        )}
      </CardContent>
    </Card>
  );
}

// levelClass colours a line of the log by its level.
function levelClass(level: string) {
  switch (level) {
    case "error":
      return "text-destructive";
    case "warning":
      return "text-amber-600";
    case "debug":
      return "text-muted-foreground";
    default:
      return "";
  }
}
