import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from "react";

interface UserInfo {
  id: string;
  email: string;
  display_name: string;
  role: string;
  tenant_slug?: string;
}

interface AuthState {
  token: string | null;
  user: UserInfo | null;
  isAuthenticated: boolean;
  isLoading: boolean;
}

interface AuthContextValue extends AuthState {
  login: (token: string, user: UserInfo) => void;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

const TOKEN_KEY = "nightowl_token";

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({
    token: null,
    user: null,
    isAuthenticated: false,
    isLoading: true,
  });

  // In dev mode, skip token-based auth entirely — the API client uses the dev API key.
  const isDevMode = import.meta.env.DEV;

  useEffect(() => {
    if (isDevMode) {
      // Fetch real user data from the users API so the user menu
      // shows accurate info and user-scoped features work.
      fetch("/api/v1/users", {
        headers: { "X-API-Key": "ow_dev_seed_key_do_not_use_in_production" },
      })
        .then((res) => (res.ok ? res.json() : null))
        .then((data) => {
          const admin = data?.users?.find((u: { role: string }) => u.role === "admin");
          setState({
            token: null,
            user: admin
              ? { id: admin.id, email: admin.email, display_name: admin.display_name, role: admin.role }
              : { id: "dev", email: "dev@localhost", display_name: "Dev User", role: "admin" },
            isAuthenticated: true,
            isLoading: false,
          });
        })
        .catch(() => {
          setState({
            token: null,
            user: { id: "dev", email: "dev@localhost", display_name: "Dev User", role: "admin" },
            isAuthenticated: true,
            isLoading: false,
          });
        });
      return;
    }

    // Try to restore session from localStorage.
    const stored = localStorage.getItem(TOKEN_KEY);
    if (!stored) {
      setState((s) => ({ ...s, isLoading: false }));
      return;
    }

    // Validate the token with the backend.
    fetch("/auth/me", {
      headers: { Authorization: `Bearer ${stored}` },
    })
      .then((res) => {
        if (!res.ok) throw new Error("invalid token");
        return res.json();
      })
      .then((user: UserInfo) => {
        setState({ token: stored, user, isAuthenticated: true, isLoading: false });
      })
      .catch(() => {
        localStorage.removeItem(TOKEN_KEY);
        setState({ token: null, user: null, isAuthenticated: false, isLoading: false });
      });
  }, [isDevMode]);

  const login = useCallback((token: string, user: UserInfo) => {
    localStorage.setItem(TOKEN_KEY, token);
    setState({ token, user, isAuthenticated: true, isLoading: false });
  }, []);

  const logout = useCallback(() => {
    localStorage.removeItem(TOKEN_KEY);
    setState({ token: null, user: null, isAuthenticated: false, isLoading: false });
    // POST to logout endpoint (fire-and-forget).
    const token = state.token;
    if (token) {
      fetch("/auth/logout", {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
      }).catch(() => {});
    }
  }, [state.token]);

  return (
    <AuthContext.Provider value={{ ...state, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
