import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Plus, Trash2 } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { api, ApiError } from "@/lib/api";
import { Alert, Badge, Button, Card, CardContent, CardHeader, CardTitle, Input, Select, Table, Td, Th } from "@/components/ui";

interface EntryPoint {
  id: string;
  name: string;
  tag: string;
  type: string;
  config: Record<string, unknown>;
  enabled: boolean;
}

interface Egress {
  id: string;
  name: string;
  tag: string;
  type: string;
  config: Record<string, unknown>;
  enabled: boolean;
}

interface Endpoint {
  id: string;
  name: string;
  nodeId: string;
  type: string;
  config: Record<string, unknown>;
}

interface Preset {
  id: string;
  name: string;
  kind: string;
  type: string;
  remark: string;
  config: Record<string, unknown>;
}

interface NodeConfigCardProps {
  nodeId: string;
  onChanged?: () => void;
}

// NodeConfigCard holds the configuration of a node: the listeners, the
// outgoing connections and the destinations.
export function NodeConfigCard({ nodeId, onChanged }: NodeConfigCardProps) {
  const { t } = useI18n();
  const [entryPoints, setEntryPoints] = useState<EntryPoint[]>([]);
  const [egresses, setEgresses] = useState<Egress[]>([]);
  const [endpoints, setEndpoints] = useState<Endpoint[]>([]);
  const [presets, setPresets] = useState<Preset[]>([]);
  const [entryPreset, setEntryPreset] = useState("");
  const [egressPreset, setEgressPreset] = useState("");
  const [error, setError] = useState<string | null>(null);

  const [entryName, setEntryName] = useState("");
  const [entryTag, setEntryTag] = useState("");
  const [entryType, setEntryType] = useState("vless");
  const [entryPort, setEntryPort] = useState("443");

  const [egressName, setEgressName] = useState("");
  const [egressTag, setEgressTag] = useState("");
  const [egressType, setEgressType] = useState("freedom");

  const [endpointName, setEndpointName] = useState("");
  const [endpointTag, setEndpointTag] = useState("");
  const [endpointDomain, setEndpointDomain] = useState("");

  const load = useCallback(async () => {
    try {
      const [entry, egress, endpoint, preset] = await Promise.all([
        api.list<EntryPoint>(`/api/entrypoints?nodeId=${nodeId}`),
        api.list<Egress>(`/api/egresses?nodeId=${nodeId}`),
        api.list<Endpoint>(`/api/endpoints?nodeId=${nodeId}`),
        api.list<Preset>("/api/presets"),
      ]);
      setEntryPoints(entry.data);
      setEgresses(egress.data);
      setEndpoints(endpoint.data);
      setPresets(preset.data);
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    }
  }, [nodeId, t]);

  useEffect(() => {
    void load();
  }, [load]);

  async function createEntryPoint(event: FormEvent) {
    event.preventDefault();
    setError(null);
    try {
      const preset = presets.find((item) => item.id === entryPreset);
      const config = preset
        ? { ...preset.config, port: Number(entryPort) || preset.config.port, clients: [] }
        : { port: Number(entryPort) || 443, clients: [] };

      await api.post("/api/entrypoints", {
        name: entryName,
        tag: entryTag,
        type: preset?.type ?? entryType,
        nodeId,
        config,
      });
      setEntryName("");
      setEntryTag("");
      setEntryPreset("");
      await load();
      onChanged?.();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    }
  }

  async function createEgress(event: FormEvent) {
    event.preventDefault();
    setError(null);
    try {
      const preset = presets.find((item) => item.id === egressPreset);
      await api.post("/api/egresses", {
        name: egressName,
        tag: egressTag,
        type: preset?.type ?? egressType,
        nodeId,
        config: preset?.config ?? { protocol: egressType },
      });
      setEgressName("");
      setEgressTag("");
      setEgressPreset("");
      await load();
      onChanged?.();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    }
  }

  async function createEndpoint(event: FormEvent) {
    event.preventDefault();
    setError(null);
    try {
      await api.post("/api/endpoints", {
        name: endpointName,
        nodeId,
        type: "routing",
        config: { type: "field", domain: [endpointDomain], outboundTag: endpointTag },
      });
      setEndpointName("");
      setEndpointTag("");
      setEndpointDomain("");
      await load();
      onChanged?.();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    }
  }

  async function remove(path: string, id: string) {
    setError(null);
    try {
      await api.delete(`${path}/${id}`);
      await load();
      onChanged?.();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("nodes.config")}</CardTitle>
      </CardHeader>
      <CardContent className="grid gap-6">
        {error ? <Alert>{error}</Alert> : null}

        <section className="grid gap-2">
          <h3 className="text-sm font-semibold">{t("nodes.entrypoints")}</h3>
          {entryPoints.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("common.empty")}</p>
          ) : (
            <Table>
              <thead>
                <tr>
                  <Th>{t("nodes.name")}</Th>
                  <Th>{t("nodes.tag")}</Th>
                  <Th>{t("nodes.type")}</Th>
                  <Th>{t("nodes.status")}</Th>
                  <Th>{t("operators.actions")}</Th>
                </tr>
              </thead>
              <tbody>
                {entryPoints.map((item) => (
                  <tr key={item.id} className="border-t border-border">
                    <Td className="font-medium">{item.name}</Td>
                    <Td className="font-mono text-xs">{item.tag}</Td>
                    <Td>{item.type}</Td>
                    <Td>
                      <Badge>{item.enabled ? t("common.enabled") : t("common.disabled")}</Badge>
                    </Td>
                    <Td>
                      <Button variant="ghost" size="icon" onClick={() => remove("/api/entrypoints", item.id)}>
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </Td>
                  </tr>
                ))}
              </tbody>
            </Table>
          )}
          <form className="flex flex-wrap items-center gap-2" onSubmit={createEntryPoint}>
            <Input
              className="w-36"
              placeholder={t("nodes.name")}
              value={entryName}
              onChange={(e) => setEntryName(e.target.value)}
              required
            />
            <Input
              className="w-36 font-mono"
              placeholder={t("nodes.tag")}
              value={entryTag}
              onChange={(e) => setEntryTag(e.target.value)}
              required
            />
            <Select
              className="w-44"
              value={entryPreset}
              onChange={(e) => {
                setEntryPreset(e.target.value);
                const preset = presets.find((item) => item.id === e.target.value);
                if (preset) {
                  setEntryType(preset.type);
                  const port = preset.config?.port;
                  if (typeof port === "number") setEntryPort(String(port));
                }
              }}
            >
              <option value="">{t("nodes.preset")}</option>
              {presets
                .filter((item) => item.kind === "entrypoint")
                .map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.name}
                  </option>
                ))}
            </Select>
            <Input
              className="w-24"
              placeholder={t("nodes.port")}
              value={entryPort}
              onChange={(e) => setEntryPort(e.target.value)}
              inputMode="numeric"
            />
            <Button type="submit" variant="outline">
              <Plus className="h-4 w-4" />
              {t("common.create")}
            </Button>
          </form>
        </section>

        <section className="grid gap-2">
          <h3 className="text-sm font-semibold">{t("nodes.egresses")}</h3>
          {egresses.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("common.empty")}</p>
          ) : (
            <Table>
              <thead>
                <tr>
                  <Th>{t("nodes.name")}</Th>
                  <Th>{t("nodes.tag")}</Th>
                  <Th>{t("nodes.type")}</Th>
                  <Th>{t("operators.actions")}</Th>
                </tr>
              </thead>
              <tbody>
                {egresses.map((item) => (
                  <tr key={item.id} className="border-t border-border">
                    <Td className="font-medium">{item.name}</Td>
                    <Td className="font-mono text-xs">{item.tag}</Td>
                    <Td>{item.type}</Td>
                    <Td>
                      <Button variant="ghost" size="icon" onClick={() => remove("/api/egresses", item.id)}>
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </Td>
                  </tr>
                ))}
              </tbody>
            </Table>
          )}
          <form className="flex flex-wrap items-center gap-2" onSubmit={createEgress}>
            <Input
              className="w-36"
              placeholder={t("nodes.name")}
              value={egressName}
              onChange={(e) => setEgressName(e.target.value)}
              required
            />
            <Input
              className="w-36 font-mono"
              placeholder={t("nodes.tag")}
              value={egressTag}
              onChange={(e) => setEgressTag(e.target.value)}
              required
            />
            <Select
              className="w-40"
              value={egressPreset}
              onChange={(e) => {
                setEgressPreset(e.target.value);
                const preset = presets.find((item) => item.id === e.target.value);
                if (preset) setEgressType(preset.type);
              }}
            >
              <option value="">{t("nodes.preset")}</option>
              {presets
                .filter((item) => item.kind === "egress")
                .map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.name}
                  </option>
                ))}
            </Select>
            <Button type="submit" variant="outline">
              <Plus className="h-4 w-4" />
              {t("common.create")}
            </Button>
          </form>
        </section>

        <section className="grid gap-2">
          <h3 className="text-sm font-semibold">{t("nodes.endpoints")}</h3>
          {endpoints.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("common.empty")}</p>
          ) : (
            <Table>
              <thead>
                <tr>
                  <Th>{t("nodes.name")}</Th>
                  <Th>{t("nodes.tag")}</Th>
                  <Th>{t("nodes.type")}</Th>
                  <Th>{t("operators.actions")}</Th>
                </tr>
              </thead>
              <tbody>
                {endpoints.map((item) => (
                  <tr key={item.id} className="border-t border-border">
                    <Td className="font-medium">{item.name}</Td>
                    <Td className="font-mono text-xs">{String(item.config?.outboundTag ?? "—")}</Td>
                    <Td>{item.type}</Td>
                    <Td>
                      <Button variant="ghost" size="icon" onClick={() => remove("/api/endpoints", item.id)}>
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </Td>
                  </tr>
                ))}
              </tbody>
            </Table>
          )}
          <form className="flex flex-wrap items-center gap-2" onSubmit={createEndpoint}>
            <Input
              className="w-36"
              placeholder={t("nodes.name")}
              value={endpointName}
              onChange={(e) => setEndpointName(e.target.value)}
              required
            />
            <Input
              className="w-36 font-mono"
              placeholder={t("nodes.tag")}
              value={endpointTag}
              onChange={(e) => setEndpointTag(e.target.value)}
            />
            <Input
              className="w-48"
              placeholder={t("nodes.domain")}
              value={endpointDomain}
              onChange={(e) => setEndpointDomain(e.target.value)}
            />
            <Button type="submit" variant="outline">
              <Plus className="h-4 w-4" />
              {t("common.create")}
            </Button>
          </form>
        </section>
      </CardContent>
    </Card>
  );
}
