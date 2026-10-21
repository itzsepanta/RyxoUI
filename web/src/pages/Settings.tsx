import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Trash2, Webhook } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { api, ApiError } from "@/lib/api";
import { Alert, Badge, Button, Card, CardContent, CardHeader, CardTitle, Input, Spinner, Table, Td, Th } from "@/components/ui";

interface Setting {
  key: string;
  value: string;
}

interface Webhook {
  id: string;
  name: string;
  url: string;
  events: string[];
  enabled: boolean;
}

export function SettingsPage() {
  const { t } = useI18n();
  const [settings, setSettings] = useState<Setting[]>([]);
  const [webhooks, setWebhooks] = useState<Webhook[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [key, setKey] = useState("");
  const [value, setValue] = useState("");
  const [hookName, setHookName] = useState("");
  const [hookUrl, setHookUrl] = useState("");
  const [hookSecret, setHookSecret] = useState("");
  const [hookEvents, setHookEvents] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [settingResponse, hookResponse] = await Promise.all([
        api.list<Setting>("/api/settings"),
        api.list<Webhook>("/api/webhooks"),
      ]);
      setSettings(settingResponse.data);
      setWebhooks(hookResponse.data);
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
      await api.put(`/api/settings/${encodeURIComponent(key)}`, { value });
      setNotice(t("settings.saved"));
      setKey("");
      setValue("");
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    }
  }

  async function addWebhook(event: FormEvent) {
    event.preventDefault();
    setError(null);
    try {
      await api.post("/api/webhooks", {
        name: hookName,
        url: hookUrl,
        secret: hookSecret,
        events: hookEvents
          .split(",")
          .map((item) => item.trim())
          .filter((item) => item !== ""),
      });
      setHookName("");
      setHookUrl("");
      setHookSecret("");
      setHookEvents("");
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    }
  }

  async function removeWebhook(id: string) {
    setError(null);
    try {
      await api.delete(`/api/webhooks/${id}`);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    }
  }

  return (
    <div className="grid gap-4">
      <h1 className="text-xl font-semibold">{t("settings.title")}</h1>

      {notice ? <Alert variant="info">{notice}</Alert> : null}
      {error ? <Alert>{error}</Alert> : null}

      <Card>
        <CardContent>
          <form className="grid gap-3 md:grid-cols-3" onSubmit={submit}>
            <Input
              placeholder={t("settings.key")}
              value={key}
              onChange={(e) => setKey(e.target.value)}
              required
            />
            <Input
              placeholder={t("settings.value")}
              value={value}
              onChange={(e) => setValue(e.target.value)}
              required
            />
            <Button type="submit">{t("common.save")}</Button>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Webhook className="h-4 w-4" />
            {t("settings.webhooks")}
          </CardTitle>
        </CardHeader>
        <CardContent className="grid gap-3">
          <p className="text-xs text-muted-foreground">{t("settings.webhooksHint")}</p>

          <form className="grid gap-2 md:grid-cols-4" onSubmit={addWebhook}>
            <Input
              placeholder={t("settings.key")}
              value={hookName}
              onChange={(e) => setHookName(e.target.value)}
              required
            />
            <Input
              placeholder="https://example.com/hook"
              value={hookUrl}
              onChange={(e) => setHookUrl(e.target.value)}
              required
            />
            <Input
              type="password"
              placeholder={t("settings.secret")}
              value={hookSecret}
              onChange={(e) => setHookSecret(e.target.value)}
            />
            <Input
              placeholder={t("settings.events")}
              value={hookEvents}
              onChange={(e) => setHookEvents(e.target.value)}
            />
            <Button type="submit" variant="outline" className="w-fit md:col-span-4">
              {t("common.create")}
            </Button>
          </form>

          {webhooks.length === 0 ? (
            <p className="p-4 text-center text-sm text-muted-foreground">{t("common.empty")}</p>
          ) : (
            <Table>
              <thead>
                <tr>
                  <Th>{t("settings.key")}</Th>
                  <Th>URL</Th>
                  <Th>{t("settings.events")}</Th>
                  <Th>{t("nodes.status")}</Th>
                  <Th>{t("operators.actions")}</Th>
                </tr>
              </thead>
              <tbody>
                {webhooks.map((hook) => (
                  <tr key={hook.id} className="border-t border-border">
                    <Td className="font-medium">{hook.name}</Td>
                    <Td className="font-mono text-xs">{hook.url}</Td>
                    <Td className="text-muted-foreground">
                      {hook.events.length === 0 ? t("settings.allEvents") : hook.events.join(", ")}
                    </Td>
                    <Td>
                      <Badge>{hook.enabled ? t("common.enabled") : t("common.disabled")}</Badge>
                    </Td>
                    <Td>
                      <Button variant="ghost" size="icon" onClick={() => removeWebhook(hook.id)}>
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </Td>
                  </tr>
                ))}
              </tbody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardContent className="p-0">
          {loading ? (
            <div className="grid place-items-center p-8">
              <Spinner />
            </div>
          ) : settings.length === 0 ? (
            <p className="p-8 text-center text-sm text-muted-foreground">{t("common.empty")}</p>
          ) : (
            <Table>
              <thead>
                <tr>
                  <Th>{t("settings.key")}</Th>
                  <Th>{t("settings.value")}</Th>
                </tr>
              </thead>
              <tbody>
                {settings.map((setting) => (
                  <tr key={setting.key} className="border-t border-border">
                    <Td className="font-medium">{setting.key}</Td>
                    <Td className="text-muted-foreground">{setting.value}</Td>
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
