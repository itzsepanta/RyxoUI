import { useEffect, useState } from "react";
import { BookOpen } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { Alert, Badge, Card, CardContent, Spinner } from "@/components/ui";

interface OpenAPIOperation {
  summary?: string;
  operationId?: string;
  "x-required-scope"?: string;
}

interface OpenAPIDocument {
  info?: { title?: string; version?: string; description?: string };
  paths?: Record<string, Record<string, OpenAPIOperation>>;
}

// DocsPage shows the description of the API that the panel serves.
export function DocsPage() {
  const { t } = useI18n();
  const [document, setDocument] = useState<OpenAPIDocument | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetch("/api/openapi.json")
      .then((response) => {
        if (!response.ok) throw new Error("the description could not be read");
        return response.json();
      })
      .then((body: OpenAPIDocument) => setDocument(body))
      .catch((err) => setError(err instanceof Error ? err.message : String(err)));
  }, []);

  const groups = new Map<string, { method: string; path: string; operation: OpenAPIOperation }[]>();
  for (const [path, methods] of Object.entries(document?.paths ?? {})) {
    const tag = path.split("/").filter(Boolean)[0] ?? "panel";
    const items = groups.get(tag) ?? [];
    for (const [method, operation] of Object.entries(methods)) {
      items.push({ method: method.toUpperCase(), path, operation });
    }
    groups.set(tag, items);
  }

  return (
    <div className="grid gap-4">
      <div className="flex items-center gap-2">
        <BookOpen className="h-5 w-5 text-primary" />
        <h1 className="text-xl font-semibold">{t("docs.title")}</h1>
      </div>
      <p className="text-sm text-muted-foreground">{t("docs.hint")}</p>

      {error ? <Alert>{error}</Alert> : null}

      {!document && !error ? (
        <div className="grid place-items-center p-8">
          <Spinner />
        </div>
      ) : null}

      {[...groups.entries()].map(([tag, items]) => (
        <Card key={tag}>
          <CardContent className="grid gap-2 py-4">
            <h2 className="text-sm font-semibold">/{tag}</h2>
            {items.map((item) => (
              <div key={`${item.method}-${item.path}`} className="flex flex-wrap items-center gap-2 text-sm">
                <Badge
                  className={
                    item.method === "GET"
                      ? "border-emerald-500/40 bg-emerald-500/10 text-emerald-600"
                      : item.method === "DELETE"
                        ? "border-destructive/40 bg-destructive/10 text-destructive"
                        : "border-sky-500/40 bg-sky-500/10 text-sky-600"
                  }
                >
                  {item.method}
                </Badge>
                <code className="font-mono text-xs">{item.path}</code>
                <span className="text-muted-foreground">{item.operation.summary ?? ""}</span>
                {item.operation["x-required-scope"] ? (
                  <span className="ms-auto font-mono text-xs text-muted-foreground">
                    {item.operation["x-required-scope"]}
                  </span>
                ) : null}
              </div>
            ))}
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
