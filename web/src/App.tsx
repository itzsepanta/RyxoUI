import { Navigate, Route, Routes } from "react-router-dom";
import { useEffect, useState } from "react";
import { useAuth } from "@/lib/auth";
import { AppShell } from "@/components/AppShell";
import { LoginPage } from "@/pages/Login";
import { SetupPage } from "@/pages/Setup";
import { OperatorsPage } from "@/pages/Operators";
import { RolesPage } from "@/pages/Roles";
import { AuditPage } from "@/pages/Audit";
import { TokensPage } from "@/pages/Tokens";
import { SettingsPage } from "@/pages/Settings";
import { UpdatesPage } from "@/pages/Updates";
import { SecurityPage } from "@/pages/Security";
import { NodesPage } from "@/pages/Nodes";
import { NodeDetailPage } from "@/pages/NodeDetail";
import { ToolsPage } from "@/pages/Tools";
import { AlertsPage } from "@/pages/Alerts";
import { DocsPage } from "@/pages/Docs";
import { ForbiddenPage, NotFoundPage } from "@/pages/Errors";
import { Spinner } from "@/components/ui";

export function App() {
  const auth = useAuth();
  const [forbidden, setForbidden] = useState(false);

  // A request that is refused shows the page of the refusal.
  useEffect(() => {
    const refuse = () => setForbidden(true);
    window.addEventListener("ryxoui:forbidden", refuse);
    return () => window.removeEventListener("ryxoui:forbidden", refuse);
  }, []);

  if (auth.loading) {
    return (
      <div className="grid min-h-screen place-items-center">
        <Spinner />
      </div>
    );
  }

  if (!auth.setup?.initialized) {
    return <SetupPage onDone={() => void auth.refresh()} />;
  }

  if (!auth.me) {
    return <LoginPage onSignIn={() => void auth.refresh()} />;
  }

  if (forbidden) {
    return <ForbiddenPage />;
  }

  return (
    <AppShell>
      <Routes>
        <Route path="/" element={<Navigate to="/operators" replace />} />
        <Route path="/operators" element={<OperatorsPage />} />
        <Route path="/nodes" element={<NodesPage />} />
        <Route path="/nodes/:id" element={<NodeDetailPage />} />
        <Route path="/roles" element={<RolesPage />} />
        <Route path="/audit" element={<AuditPage />} />
        <Route path="/tokens" element={<TokensPage />} />
        <Route path="/settings" element={<SettingsPage />} />
        <Route path="/tools" element={<ToolsPage />} />
        <Route path="/alerts" element={<AlertsPage />} />
        <Route path="/docs" element={<DocsPage />} />
        <Route path="/updates" element={<UpdatesPage />} />
        <Route path="/security" element={<SecurityPage />} />
        <Route path="*" element={<NotFoundPage />} />
      </Routes>
    </AppShell>
  );
}
