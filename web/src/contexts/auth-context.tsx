import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from "react";

interface UserInfo {
  id: string;
  email: string;
  display_name: string;
  role: string;
  tenant_slug?: string;
}

interface AuthState {
  user: UserInfo | null;
  isAuthenticated: boolean;
  isLoading: boolean;
}

interface AuthContextValue extends AuthState {
  login: (user: UserInfo) => void;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

function getInitialState(): AuthState {
  // In dev mode, start authenticated immediately (API client uses dev API key).
  if (import.meta.env.DEV) {
    return {
      user: { id: "dev", email: "dev@localhost", display_name: "Dev User", role: "admin" },
      isAuthenticated: true,
      isLoading: true, // still loading real user data
    };
  }

  return {
    user: null,
    isAuthenticated: false,
    isLoading: true, // check cookie-based session on mount
  };
}

// eslint-disable-next-line react-refresh/only-export-components
export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>(getInitialState);

  useEffect(() => {
    if (import.meta.env.DEV) {
      // Fetch real user data from the users API so the user menu
      // shows accurate info and user-scoped features work.
      fetch("/api/v1/users", {
        headers: { "X-API-Key": "ow_dev_seed_key_do_not_use_in_production" },
      })
        .then((res) => (res.ok ? res.json() : null))
        .then((data) => {
          const admin = data?.users?.find((u: { role: string }) => u.role === "admin");
          if (admin) {
            setState({
              user: { id: admin.id, email: admin.email, display_name: admin.display_name, role: admin.role },
              isAuthenticated: true,
              isLoading: false,
            });
          } else {
            setState((s) => ({ ...s, isLoading: false }));
          }
        })
        .catch(() => {
          setState((s) => ({ ...s, isLoading: false }));
        });
      return;
    }

    // Prod: validate session cookie by calling /auth/me (cookie sent automatically).
    fetch("/auth/me", { credentials: "same-origin" })
      .then((res) => {
        if (!res.ok) throw new Error("no session");
        return res.json();
      })
      .then((user: UserInfo) => {
        setState({ user, isAuthenticated: true, isLoading: false });
      })
      .catch(() => {
        setState({ user: null, isAuthenticated: false, isLoading: false });
      });
  }, []);

  const login = useCallback((user: UserInfo) => {
    setState({ user, isAuthenticated: true, isLoading: false });
  }, []);

  const logout = useCallback(() => {
    setState({ user: null, isAuthenticated: false, isLoading: false });
    // POST to logout endpoint to clear server-side cookie.
    fetch("/auth/logout", {
      method: "POST",
      credentials: "same-origin",
    }).catch(() => {});
  }, []);

  return (
    <AuthContext.Provider value={{ ...state, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}
