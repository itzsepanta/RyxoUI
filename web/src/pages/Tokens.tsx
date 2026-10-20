import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Plus, Trash2 } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { api, ApiError, type ApiToken, type ListResponse } from "@/lib/api";
import {
  Alert,
  Badge,
  Button,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  Input,
  Spinner,
  Table,
  Td,
  Th,
} from "@/components/ui";

export function TokensPage() {
  const { t } = useI18n();
  const [tokens, setTokens] = useState<ApiToken[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const response = await api.get<ListResponse<ApiToken>>("/api/tokens");
      setTokens(response.data);
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
      const token = await api.post<ApiToken>("/api/tokens", { name });
      setNotice(`${t("tokens.created")}\n${token.plaintext ?? ""}`);
      setName("");
      setCreating(false);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    }
  }

  async function remove(id: string) {
    setError(null);
    try {
      await api.delete(`/api/tokens/${id}`);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    }
  }

  return (
    <div className="grid gap-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">{t("tokens.title")}</h1>
        <Button onClick={() => setCreating((value) => !value)}>
          <Plus className="h-4 w-4" />
          {t("tokens.new")}
        </Button>
      </div>

      {notice ? <Alert variant="info">{notice}</Alert> : null}
      {error ? <Alert>{error}</Alert> : null}

      {creating ? (
        <Card>
          <CardHeader>
            <CardTitle>{t("tokens.new")}</CardTitle>
          </CardHeader>
          <CardContent>
            <form className="flex gap-3" onSubmit={submit}>
              <Input
                placeholder={t("tokens.name")}
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
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
          ) : tokens.length === 0 ? (
            <p className="p-8 text-center text-sm text-muted-foreground">{t("common.empty")}</p>
          ) : (
            <Table>
              <thead>
                <tr>
                  <Th>{t("tokens.name")}</Th>
                  <Th>{t("tokens.scopes")}</Th>
                  <Th>{t("operators.actions")}</Th>
                </tr>
              </thead>
              <tbody>
                {tokens.map((token) => (
                  <tr key={token.id} className="border-t border-border">
                    <Td className="font-medium">{token.name}</Td>
                    <Td>
                      <div className="flex flex-wrap gap-1">
                        {token.scopes.map((scope) => (
                          <Badge key={scope}>{scope}</Badge>
                        ))}
                      </div>
                    </Td>
                    <Td>
                      <Button variant="ghost" size="icon" onClick={() => remove(token.id)}>
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
    </div>
  );
}
