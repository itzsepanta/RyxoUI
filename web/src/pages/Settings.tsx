import { useCallback, useEffect, useState, type FormEvent } from "react";
import { useI18n } from "@/lib/i18n";
import { api, ApiError, type ListResponse } from "@/lib/api";
import { Alert, Button, Card, CardContent, Input, Spinner, Table, Td, Th } from "@/components/ui";

interface Setting {
  key: string;
  value: string;
}

export function SettingsPage() {
  const { t } = useI18n();
  const [settings, setSettings] = useState<Setting[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [key, setKey] = useState("");
  const [value, setValue] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const response = await api.get<ListResponse<Setting>>("/api/settings");
      setSettings(response.data);
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
