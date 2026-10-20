import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { api, ApiError, type Me, type SetupState } from "@/lib/api";

interface AuthValue {
  me: Me | null;
  setup: SetupState | null;
  loading: boolean;
  error: string | null;
  refresh: () => Promise<void>;
  signIn: (username: string, password: string) => Promise<void>;
  signOut: () => Promise<void>;
}

const AuthContext = createContext<AuthValue | null>(null);

// AuthProvider keeps the session state of the panel in one place so that
// every screen sees the same state.
export function AuthProvider({ children }: { children: ReactNode }) {
  const [me, setMe] = useState<Me | null>(null);
  const [setup, setSetup] = useState<SetupState | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const state = await api.get<SetupState>("/api/setup");
      setSetup(state);
      if (!state.initialized) {
        setMe(null);
        return;
      }
      try {
        const meResponse = await api.get<Me>("/api/me");
        setMe(meResponse);
        setError(null);
      } catch {
        setMe(null);
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "request failed");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const signIn = useCallback(
    async (username: string, password: string) => {
      await api.post("/api/login", { username, password });
      await refresh();
    },
    [refresh],
  );

  const signOut = useCallback(async () => {
    try {
      await api.post("/api/logout");
    } finally {
      setMe(null);
      setSetup((current) => current);
    }
  }, []);

  const value = useMemo<AuthValue>(
    () => ({ me, setup, loading, error, refresh, signIn, signOut }),
    [me, setup, loading, error, refresh, signIn, signOut],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

// useAuth returns the session state of the panel.
export function useAuth(): AuthValue {
  const value = useContext(AuthContext);
  if (!value) {
    throw new Error("useAuth must be used inside AuthProvider");
  }
  return value;
}
