import { Navigate, Route, Routes } from "react-router-dom";
import { useAuth } from "@/lib/useAuth";
import { AppShell } from "@/components/AppShell";
import { LoginPage } from "@/pages/Login";
import { SetupPage } from "@/pages/Setup";
import { OperatorsPage } from "@/pages/Operators";
import { RolesPage } from "@/pages/Roles";
import { AuditPage } from "@/pages/Audit";
import { TokensPage } from "@/pages/Tokens";
import { SettingsPage } from "@/pages/Settings";
import { UpdatesPage } from "@/pages/Updates";
import { Spinner } from "@/components/ui";

export function App() {
  const auth = useAuth();

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

  return (
    <AppShell>
      <Routes>
        <Route path="/" element={<Navigate to="/operators" replace />} />
        <Route path="/operators" element={<OperatorsPage />} />
        <Route path="/roles" element={<RolesPage />} />
        <Route path="/audit" element={<AuditPage />} />
        <Route path="/tokens" element={<TokensPage />} />
        <Route path="/settings" element={<SettingsPage />} />
        <Route path="/updates" element={<UpdatesPage />} />
        <Route path="*" element={<Navigate to="/operators" replace />} />
      </Routes>
    </AppShell>
  );
}
