import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { Activity, Plus, Trash2 } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { api, ApiError } from "@/lib/api";
import {
  Alert,
  Badge,
  Button,
  Card,
  CardContent,
  Input,
  Spinner,
  Table,
  Td,
  Th,
} from "@/components/ui";

export interface Node {
  id: string;
  name: string;
  address: string;
  port: number;
  status: string;
  message: string;
  remark: string;
  tags: string[];
  arch: string;
  os: string;
  coreVersion: string;
  nodeVersion: string;
  multiplier: number;
  enabled: boolean;
  limitBytes: number;
  lastSeenAt: number;
  createdAt: number;
  sshHost: string;
  sshPort: number;
  sshUser: string;
  sshKeyId: string;
  keyInstalled: boolean;
}

const statusStyles: Record<string, string> = {
  connected: "border-emerald-500/40 bg-emerald-500/10 text-emerald-600",
  connecting: "border-amber-500/40 bg-amber-500/10 text-amber-600",
  error: "border-destructive/40 bg-destructive/10 text-destructive",
  disconnected: "border-border bg-muted text-muted-foreground",
};

export function NodesPage() {
  const { t } = useI18n();
  const [nodes, setNodes] = useState<Node[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [probing, setProbing] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [address, setAddress] = useState("");
  const [sshUser, setSSHUser] = useState("root");
  const [sshPort, setSSHPort] = useState("22");

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const response = await api.list<Node>("/api/nodes");
      setNodes(response.data);
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

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    try {
      await api.post("/api/nodes", {
        name,
        address,
        sshUser,
        sshPort: Number(sshPort) || 22,
      });
      setName("");
      setAddress("");
      setCreating(false);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    }
  }

  async function remove(id: string) {
    setError(null);
    try {
      await api.delete(`/api/nodes/${id}`);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    }
  }

  async function probe(id: string) {
    setProbing(id);
    setError(null);
    try {
      await api.post(`/api/nodes/${id}/probe`);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    } finally {
      setProbing(null);
    }
  }

  return (
    <div className="grid gap-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">{t("nodes.title")}</h1>
        <Button onClick={() => setCreating((value) => !value)}>
          <Plus className="h-4 w-4" />
          {t("nodes.new")}
        </Button>
      </div>

      {error ? <Alert>{error}</Alert> : null}

      {creating ? (
        <Card>
          <CardContent>
            <form className="grid gap-3 md:grid-cols-5" onSubmit={submit}>
              <Input
                placeholder={t("nodes.name")}
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
              />
              <Input
                placeholder={t("nodes.address")}
                value={address}
                onChange={(e) => setAddress(e.target.value)}
                required
              />
              <Input
                placeholder={t("nodes.sshUser")}
                value={sshUser}
                onChange={(e) => setSSHUser(e.target.value)}
              />
              <Input
                placeholder={t("nodes.sshPort")}
                value={sshPort}
                onChange={(e) => setSSHPort(e.target.value)}
                inputMode="numeric"
              />
              <Button type="submit">{t("common.create")}</Button>
            </form>
          </CardContent>
        </Card>
      ) : null}

      <Card>
        <CardContent className="p-0">
          {loading ? (
            <div className="grid place-items-center p-8">
              <Spinner />
            </div>
          ) : nodes.length === 0 ? (
            <p className="p-8 text-center text-sm text-muted-foreground">{t("common.empty")}</p>
          ) : (
            <Table>
              <thead>
                <tr>
                  <Th>{t("nodes.name")}</Th>
                  <Th>{t("nodes.address")}</Th>
                  <Th>{t("nodes.status")}</Th>
                  <Th>{t("nodes.version")}</Th>
                  <Th>{t("nodes.traffic")}</Th>
                  <Th>{t("operators.actions")}</Th>
                </tr>
              </thead>
              <tbody>
                {nodes.map((node) => (
                  <tr key={node.id} className="border-t border-border">
                    <Td>
                      <Link className="font-medium text-primary hover:underline" to={`/nodes/${node.id}`}>
                        {node.name}
                      </Link>
                      {node.remark ? (
                        <span className="block text-xs text-muted-foreground">{node.remark}</span>
                      ) : null}
                    </Td>
                    <Td className="font-mono text-xs">
                      {node.address}:{node.port}
                    </Td>
                    <Td>
                      <Badge className={statusStyles[node.status] ?? ""}>{node.status}</Badge>
                    </Td>
                    <Td className="text-muted-foreground">{node.coreVersion || "—"}</Td>
                    <Td className="text-muted-foreground">
                      {node.limitBytes > 0 ? formatBytes(node.limitBytes) : "∞"}
                    </Td>
                    <Td>
                      <div className="flex items-center gap-1">
                        <Button
                          variant="ghost"
                          size="icon"
                          title={t("nodes.probe")}
                          disabled={probing === node.id}
                          onClick={() => probe(node.id)}
                        >
                          <Activity className="h-4 w-4" />
                        </Button>
                        <Button variant="ghost" size="icon" onClick={() => remove(node.id)}>
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </div>
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
