import { useCallback, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { Activity, ArrowLeft, KeyRound } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { api, ApiError } from "@/lib/api";
import { Alert, Badge, Button, Card, CardContent, CardHeader, CardTitle, Spinner, Table, Td, Th } from "@/components/ui";
import type { Node } from "@/pages/Nodes";

interface HealthSample {
  id: string;
  diskTotal: number;
  diskUsed: number;
  memTotal: number;
  memUsed: number;
  load1: number;
  sampledAt: number;
}

interface Usage {
  id: string;
  dateTime: number;
  up: number;
  down: number;
}

interface EntryPoint {
  id: string;
  name: string;
  tag: string;
  type: string;
  enabled: boolean;
}

interface PanelKey {
  id: string;
  name: string;
  publicKey: string;
  createdAt: number;
}

export function NodeDetailPage() {
  const { id = "" } = useParams();
  const { t } = useI18n();
  const [node, setNode] = useState<Node | null>(null);
  const [health, setHealth] = useState<HealthSample[]>([]);
  const [usage, setUsage] = useState<Usage[]>([]);
  const [entryPoints, setEntryPoints] = useState<EntryPoint[]>([]);
  const [keys, setKeys] = useState<PanelKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [probing, setProbing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [nodeResponse, healthResponse, usageResponse, entryResponse, keyResponse] = await Promise.all([
        api.get<Node>(`/api/nodes/${id}`),
        api.list<HealthSample>(`/api/nodes/${id}/health`),
        api.list<Usage>(`/api/nodes/${id}/usage`),
        api.list<EntryPoint>(`/api/entrypoints?nodeId=${id}`),
        api.list<PanelKey>("/api/ssh-keys"),
      ]);
      setNode(nodeResponse);
      setHealth(healthResponse.data);
      setUsage(usageResponse.data);
      setEntryPoints(entryResponse.data);
      setKeys(keyResponse.data);
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    } finally {
      setLoading(false);
    }
  }, [id, t]);

  useEffect(() => {
    void load();
  }, [load]);

  async function probe() {
    setProbing(true);
    setError(null);
    try {
      await api.post(`/api/nodes/${id}/probe`);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    } finally {
      setProbing(false);
    }
  }

  async function generateKey() {
    setError(null);
    try {
      const created = await api.post<PanelKey>("/api/ssh-keys", { name: `${node?.name ?? "node"}-key` });
      await api.put(`/api/nodes/${id}`, { sshKeyId: created.id });
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    }
  }

  async function markInstalled() {
    setError(null);
    try {
      await api.put(`/api/nodes/${id}`, { keyInstalled: true });
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    }
  }

  if (loading) {
    return (
      <div className="grid place-items-center py-16">
        <Spinner />
      </div>
    );
  }

  if (!node) {
    return <Alert>{error ?? t("common.error")}</Alert>;
  }

  const ownKey = keys.find((key) => key.id === node.sshKeyId);

  return (
    <div className="grid gap-4">
      <div className="flex items-center gap-3">
        <Link className="rounded-md p-1 text-muted-foreground hover:bg-accent" to="/nodes">
          <ArrowLeft className="h-4 w-4" />
        </Link>
        <h1 className="text-xl font-semibold">{node.name}</h1>
        <Badge>{node.status}</Badge>
        <Button className="ms-auto" variant="outline" disabled={probing} onClick={probe}>
          <Activity className="h-4 w-4" />
          {probing ? t("nodes.probing") : t("nodes.probe")}
        </Button>
      </div>

      {error ? <Alert>{error}</Alert> : null}

      <div className="grid gap-4 lg:grid-cols-3">
        <Card>
          <CardHeader>
            <CardTitle>{t("nodes.overview")}</CardTitle>
          </CardHeader>
          <CardContent className="grid gap-2 text-sm">
            <Row label={t("nodes.address")} value={`${node.address}:${node.port}`} />
            <Row label={t("nodes.ssh")} value={`${node.sshUser}@${node.sshHost || node.address}:${node.sshPort}`} />
            <Row label={t("nodes.version")} value={node.coreVersion || "—"} />
            <Row label={t("nodes.platform")} value={[node.os, node.arch].filter(Boolean).join(" ") || "—"} />
            <Row label={t("nodes.multiplier")} value={String(node.multiplier)} />
            <Row label={t("nodes.remark")} value={node.remark || "—"} />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t("nodes.health")}</CardTitle>
          </CardHeader>
          <CardContent className="grid gap-2 text-sm">
            {health.length === 0 ? (
              <p className="text-muted-foreground">{t("common.empty")}</p>
            ) : (
              <>
                <Row label={t("nodes.disk")} value={`${formatBytes(health[0].diskUsed)} / ${formatBytes(health[0].diskTotal)}`} />
                <Row label={t("nodes.memory")} value={`${formatBytes(health[0].memUsed)} / ${formatBytes(health[0].memTotal)}`} />
                <Row label={t("nodes.load")} value={health[0].load1.toFixed(2)} />
                <Row label={t("nodes.sampled")} value={new Date(health[0].sampledAt * 1000).toLocaleString()} />
              </>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t("nodes.traffic")}</CardTitle>
          </CardHeader>
          <CardContent className="grid gap-2 text-sm">
            <Row label="↑" value={formatBytes(usage.reduce((sum, item) => sum + item.up, 0))} />
            <Row label="↓" value={formatBytes(usage.reduce((sum, item) => sum + item.down, 0))} />
            <Row label={t("nodes.quota")} value={node.limitBytes > 0 ? formatBytes(node.limitBytes) : "∞"} />
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <KeyRound className="h-4 w-4" />
            {t("nodes.sshKeys")}
          </CardTitle>
        </CardHeader>
        <CardContent className="grid gap-3 text-sm">
          <div className="flex flex-wrap items-center gap-2">
            <Badge className={node.keyInstalled ? "" : "border-amber-500/40 bg-amber-500/10 text-amber-600"}>
              {node.keyInstalled ? t("nodes.keyInstalled") : t("nodes.keyNotInstalled")}
            </Badge>
            <Button variant="outline" onClick={generateKey}>
              {t("nodes.generateKey")}
            </Button>
            {!node.keyInstalled && ownKey ? (
              <Button variant="outline" onClick={markInstalled}>
                {t("nodes.markInstalled")}
              </Button>
            ) : null}
          </div>

          {ownKey ? (
            <div className="grid gap-1">
              <span className="text-muted-foreground">{t("nodes.publicKey")}</span>
              <code className="break-all rounded-md border border-border bg-muted/40 p-2 text-xs">
                {ownKey.publicKey.trim()}
              </code>
              <span className="text-xs text-muted-foreground">{t("nodes.publicKeyHint")}</span>
            </div>
          ) : (
            <p className="text-muted-foreground">{t("nodes.noKey")}</p>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("nodes.entrypoints")}</CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          {entryPoints.length === 0 ? (
            <p className="p-8 text-center text-sm text-muted-foreground">{t("common.empty")}</p>
          ) : (
            <Table>
              <thead>
                <tr>
                  <Th>{t("nodes.name")}</Th>
                  <Th>{t("nodes.tag")}</Th>
                  <Th>{t("nodes.type")}</Th>
                  <Th>{t("nodes.status")}</Th>
                </tr>
              </thead>
              <tbody>
                {entryPoints.map((entry) => (
                  <tr key={entry.id} className="border-t border-border">
                    <Td className="font-medium">{entry.name}</Td>
                    <Td className="font-mono text-xs">{entry.tag}</Td>
                    <Td>{entry.type}</Td>
                    <Td>
                      <Badge>{entry.enabled ? "enabled" : "disabled"}</Badge>
                    </Td>
                  </tr>
                ))}
              </tbody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-4">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-medium">{value}</span>
    </div>
  );
}

function formatBytes(value: number) {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let size = value;
  let unit = 0;
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024;
    unit += 1;
  }
  return `${size.toFixed(size >= 10 || unit === 0 ? 0 : 1)} ${units[unit]}`;
}
