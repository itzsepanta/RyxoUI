import { useCallback, useEffect, useState } from "react";
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

export function useAuth(): AuthValue {
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

  const signIn = useCallback(async (username: string, password: string) => {
    await api.post("/api/login", { username, password });
    await refresh();
  }, [refresh]);

  const signOut = useCallback(async () => {
    await api.post("/api/logout");
    setMe(null);
  }, []);

  return { me, setup, loading, error, refresh, signIn, signOut };
}
