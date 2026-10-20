import { useCallback, useEffect, useState } from "react";
import { useI18n } from "@/lib/i18n";
import { api, ApiError, type AuditEntry, type ListResponse } from "@/lib/api";
import { Alert, Card, CardContent, Spinner, Table, Td, Th } from "@/components/ui";

function formatTime(value: number) {
  if (!value) return "—";
  return new Date(value * 1000).toLocaleString();
}

export function AuditPage() {
  const { t } = useI18n();
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const response = await api.get<ListResponse<AuditEntry>>("/api/audit?limit=100");
      setEntries(response.data);
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
      <h1 className="text-xl font-semibold">{t("audit.title")}</h1>
      {error ? <Alert>{error}</Alert> : null}

      <Card>
        <CardContent className="p-0">
          {loading ? (
            <div className="grid place-items-center p-8">
              <Spinner />
            </div>
          ) : entries.length === 0 ? (
            <p className="p-8 text-center text-sm text-muted-foreground">{t("common.empty")}</p>
          ) : (
            <Table>
              <thead>
                <tr>
                  <Th>{t("audit.time")}</Th>
                  <Th>{t("audit.actor")}</Th>
                  <Th>{t("audit.action")}</Th>
                  <Th>{t("audit.entity")}</Th>
                  <Th>{t("audit.ip")}</Th>
                </tr>
              </thead>
              <tbody>
                {entries.map((entry) => (
                  <tr key={entry.id} className="border-t border-border">
                    <Td className="whitespace-nowrap text-muted-foreground">{formatTime(entry.createdAt)}</Td>
                    <Td>{entry.actorName || "—"}</Td>
                    <Td>{entry.action}</Td>
                    <Td className="text-muted-foreground">
                      {entry.entity}
                      {entry.entityId ? ` · ${entry.entityId}` : ""}
                    </Td>
                    <Td className="text-muted-foreground">{entry.ip || "—"}</Td>
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
