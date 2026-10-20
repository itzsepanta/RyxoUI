import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Plus, Trash2 } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { api, ApiError, type Role, type ScopeInfo } from "@/lib/api";
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

export function RolesPage() {
  const { t } = useI18n();
  const [roles, setRoles] = useState<Role[]>([]);
  const [scopes, setScopes] = useState<ScopeInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [base, setBase] = useState("");
  const [selected, setSelected] = useState<string[]>([]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [roleList, scopeList] = await Promise.all([
        api.list<Role>("/api/roles"),
        api.list<ScopeInfo>("/api/scopes"),
      ]);
      setRoles(roleList.data);
      setScopes(scopeList.data);
      setBase((current) => current || roleList.data[0]?.name || "");
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
      await api.post("/api/roles", { name, base, scopes: selected });
      setName("");
      setSelected([]);
      setCreating(false);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    }
  }

  async function remove(id: string) {
    setError(null);
    try {
      await api.delete(`/api/roles/${id}`);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    }
  }

  function toggle(scope: string) {
    setSelected((current) =>
      current.includes(scope) ? current.filter((value) => value !== scope) : [...current, scope],
    );
  }

  return (
    <div className="grid gap-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">{t("roles.title")}</h1>
        <Button onClick={() => setCreating((value) => !value)}>
          <Plus className="h-4 w-4" />
          {t("roles.new")}
        </Button>
      </div>

      {error ? <Alert>{error}</Alert> : null}

      {creating ? (
        <Card>
          <CardHeader>
            <CardTitle>{t("roles.new")}</CardTitle>
          </CardHeader>
          <CardContent>
            <form className="grid gap-3" onSubmit={submit}>
              <div className="grid gap-3 md:grid-cols-2">
                <Input
                  placeholder={t("roles.name")}
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  required
                />
                <select
                  className="h-9 rounded-md border border-input bg-transparent px-2 text-sm"
                  value={base}
                  onChange={(e) => setBase(e.target.value)}
                >
                  {roles.map((role) => (
                    <option key={role.id} value={role.name}>
                      {role.name}
                    </option>
                  ))}
                </select>
              </div>

              <fieldset className="rounded-md border border-border p-3">
                <legend className="px-1 text-xs uppercase text-muted-foreground">{t("roles.scopes")}</legend>
                <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
                  {scopes.map((scope) => (
                    <label key={scope.name} className="flex items-start gap-2 text-sm">
                      <input
                        type="checkbox"
                        className="mt-1"
                        checked={selected.includes(scope.name)}
                        onChange={() => toggle(scope.name)}
                      />
                      <span>
                        <span className="font-medium">{scope.name}</span>
                        <span className="block text-xs text-muted-foreground">{scope.description}</span>
                      </span>
                    </label>
                  ))}
                </div>
              </fieldset>

              <div>
                <Button type="submit">{t("common.create")}</Button>
              </div>
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
          ) : (
            <Table>
              <thead>
                <tr>
                  <Th>{t("roles.name")}</Th>
                  <Th>{t("roles.base")}</Th>
                  <Th>{t("roles.scopes")}</Th>
                  <Th>{t("operators.actions")}</Th>
                </tr>
              </thead>
              <tbody>
                {roles.map((role) => (
                  <tr key={role.id} className="border-t border-border">
                    <Td>
                      <span className="font-medium">{role.name}</span>
                      {role.builtin ? (
                        <Badge className="ms-2">{t("roles.builtin")}</Badge>
                      ) : null}
                      {role.description ? (
                        <span className="block text-xs text-muted-foreground">{role.description}</span>
                      ) : null}
                    </Td>
                    <Td>{role.base}</Td>
                    <Td>
                      <div className="flex flex-wrap gap-1">
                        {role.scopes.map((scope) => (
                          <Badge key={scope}>{scope}</Badge>
                        ))}
                      </div>
                    </Td>
                    <Td>
                      {role.builtin ? null : (
                        <Button variant="ghost" size="icon" onClick={() => remove(role.id)}>
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      )}
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
