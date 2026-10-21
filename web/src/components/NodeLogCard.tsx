import { useCallback, useEffect, useState } from "react";
import { RefreshCw, ScrollText } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { api, ApiError } from "@/lib/api";
import { Alert, Button, Card, CardContent, CardHeader, CardTitle, Select, Spinner } from "@/components/ui";

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
  const [level, setLevel] = useState("");
  const [since, setSince] = useState("0");
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const response = await api.get<{ data: LogLine[] }>(
        `/api/nodes/${nodeId}/logs?lines=200&since=${since}`,
      );
      setLines(response.data ?? []);
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    } finally {
      setLoading(false);
    }
  }, [nodeId, since, t]);

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

  // The lines the operator wants to see.
  const shown = level ? lines.filter((line) => line.level === level) : lines;

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

        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline" onClick={load} disabled={loading}>
            <RefreshCw className="h-4 w-4" />
            {t("nodes.refresh")}
          </Button>
          <Select
            className="w-32"
            value={level}
            onChange={(event) => setLevel(event.target.value)}
          >
            <option value="">{t("nodes.everyLevel")}</option>
            <option value="error">{t("nodes.levelError")}</option>
            <option value="warning">{t("nodes.levelWarning")}</option>
            <option value="info">{t("nodes.levelInfo")}</option>
            <option value="debug">{t("nodes.levelDebug")}</option>
          </Select>
          <Select
            className="w-36"
            value={since}
            onChange={(event) => setSince(event.target.value)}
          >
            <option value="0">{t("nodes.everyTime")}</option>
            <option value="5">{t("nodes.lastMinutes").replace("{}", "5")}</option>
            <option value="15">{t("nodes.lastMinutes").replace("{}", "15")}</option>
            <option value="60">{t("nodes.lastMinutes").replace("{}", "60")}</option>
            <option value="1440">{t("nodes.lastMinutes").replace("{}", "1440")}</option>
          </Select>
          <Button variant={following ? "default" : "outline"} onClick={() => setFollowing((value) => !value)}>
            {following ? t("nodes.following") : t("nodes.follow")}
          </Button>
          {following ? <span className="text-xs text-muted-foreground">{t("nodes.live")}</span> : null}
        </div>

        {loading ? (
          <div className="grid place-items-center p-6">
            <Spinner />
          </div>
        ) : shown.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("common.empty")}</p>
        ) : (
          <pre className="max-h-96 overflow-auto rounded-md bg-muted/40 p-3 text-xs leading-5">
            {shown.map((line, index) => (
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
