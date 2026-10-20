import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Plus, Trash2 } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { api, ApiError, type Operator, type Role } from "@/lib/api";
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

function formatTime(value: number) {
  if (!value) return "—";
  return new Date(value * 1000).toLocaleString();
}

export function OperatorsPage() {
  const { t } = useI18n();
  const [operators, setOperators] = useState<Operator[]>([]);
  const [roles, setRoles] = useState<Role[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [roleId, setRoleId] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [operatorList, roleList] = await Promise.all([
        api.list<Operator>("/api/operators"),
        api.list<Role>("/api/roles"),
      ]);
      setOperators(operatorList.data);
      setRoles(roleList.data);
      setRoleId((current) => current || roleList.data[0]?.id || "");
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
      await api.post("/api/operators", { username, password, roleId });
      setUsername("");
      setPassword("");
      setCreating(false);
      setNotice(t("operators.created"));
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    }
  }

  async function remove(id: string) {
    setError(null);
    try {
      await api.delete(`/api/operators/${id}`);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.error"));
    }
  }

  return (
    <div className="grid gap-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">{t("operators.title")}</h1>
        <Button onClick={() => setCreating((value) => !value)}>
          <Plus className="h-4 w-4" />
          {t("operators.new")}
        </Button>
      </div>

      {notice ? <Alert variant="info">{notice}</Alert> : null}
      {error ? <Alert>{error}</Alert> : null}

      {creating ? (
        <Card>
          <CardHeader>
            <CardTitle>{t("operators.new")}</CardTitle>
          </CardHeader>
          <CardContent>
            <form className="grid gap-3 md:grid-cols-4" onSubmit={submit}>
              <Input
                placeholder={t("operators.username")}
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                required
              />
              <Input
                type="password"
                placeholder={t("auth.password")}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                minLength={8}
                required
              />
              <select
                className="h-9 rounded-md border border-input bg-transparent px-2 text-sm"
                value={roleId}
                onChange={(e) => setRoleId(e.target.value)}
              >
                {roles.map((role) => (
                  <option key={role.id} value={role.id}>
                    {role.name}
                  </option>
                ))}
              </select>
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
          ) : operators.length === 0 ? (
            <p className="p-8 text-center text-sm text-muted-foreground">{t("common.empty")}</p>
          ) : (
            <Table>
              <thead>
                <tr>
                  <Th>{t("operators.username")}</Th>
                  <Th>{t("operators.role")}</Th>
                  <Th>{t("operators.status")}</Th>
                  <Th>{t("operators.lastLogin")}</Th>
                  <Th>{t("operators.actions")}</Th>
                </tr>
              </thead>
              <tbody>
                {operators.map((operator) => (
                  <tr key={operator.id} className="border-t border-border">
                    <Td className="font-medium">{operator.username}</Td>
                    <Td>
                      <Badge>{operator.roleName}</Badge>
                    </Td>
                    <Td>{operator.status}</Td>
                    <Td className="text-muted-foreground">{formatTime(operator.lastLoginAt)}</Td>
                    <Td>
                      {operator.isOwner ? null : (
                        <Button variant="ghost" size="icon" onClick={() => remove(operator.id)}>
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
