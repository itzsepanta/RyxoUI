import { Link } from "react-router-dom";
import { Compass, ShieldAlert } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { Button, Card, CardContent } from "@/components/ui";

// NotFoundPage is shown when the address of the browser leads nowhere.
export function NotFoundPage() {
  const { t } = useI18n();

  return (
    <div className="grid place-items-center py-20">
      <Card className="w-full max-w-md">
        <CardContent className="grid justify-items-center gap-3 py-10 text-center">
          <Compass className="h-8 w-8 text-primary" />
          <h1 className="text-lg font-semibold">{t("errors.notFound")}</h1>
          <p className="text-sm text-muted-foreground">{t("errors.notFoundHint")}</p>
          <Button variant="outline" className="mt-2">
            <Link to="/operators">{t("errors.goHome")}</Link>
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}

// ForbiddenPage is shown when the operator may not do what the screen asks.
export function ForbiddenPage() {
  const { t } = useI18n();

  return (
    <div className="grid place-items-center py-20">
      <Card className="w-full max-w-md">
        <CardContent className="grid justify-items-center gap-3 py-10 text-center">
          <ShieldAlert className="h-8 w-8 text-destructive" />
          <h1 className="text-lg font-semibold">{t("errors.forbidden")}</h1>
          <p className="text-sm text-muted-foreground">{t("errors.forbiddenHint")}</p>
          <Button variant="outline" className="mt-2">
            <Link to="/operators">{t("errors.goHome")}</Link>
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
