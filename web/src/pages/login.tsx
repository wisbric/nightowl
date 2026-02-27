import { useState, useEffect, type FormEvent } from "react";
import { useNavigate } from "@tanstack/react-router";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import { useAuth } from "@/contexts/auth-context";

interface AuthConfig {
  oidc_enabled: boolean;
  oidc_name: string;
  local_enabled: boolean;
}

export function LoginPage() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [authConfig, setAuthConfig] = useState<AuthConfig | null>(null);
  const [retryAfter, setRetryAfter] = useState(0);

  useEffect(() => {
    fetch("/auth/config")
      .then((res) => res.json())
      .then(setAuthConfig)
      .catch(() => setAuthConfig({ oidc_enabled: false, oidc_name: "", local_enabled: true }));
  }, []);

  // Countdown timer for rate limiting.
  useEffect(() => {
    if (retryAfter <= 0) return;
    const timer = setInterval(() => {
      setRetryAfter((prev) => {
        if (prev <= 1) {
          clearInterval(timer);
          return 0;
        }
        return prev - 1;
      });
    }, 1000);
    return () => clearInterval(timer);
  }, [retryAfter]);

  async function handleLocalAdminSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);

    try {
      const res = await fetch("/auth/local", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, password }),
        credentials: "same-origin",
      });

      if (res.status === 429) {
        const body = await res.json().catch(() => ({ retry_after: 60 }));
        setRetryAfter(body.retry_after || 60);
        setError("Too many login attempts. Please wait.");
        return;
      }

      if (!res.ok) {
        const body = await res.json().catch(() => ({ message: "Login failed" }));
        throw new Error(body.message || "Invalid username or password");
      }

      const data = await res.json();

      if (data.must_change) {
        // Cookie is set by the server; redirect to change-password page.
        login(data.user);
        navigate({ to: "/change-password" });
        return;
      }

      // Cookie is set by the server; update frontend auth state.
      login(data.user);
      navigate({ to: "/" });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login failed");
    } finally {
      setLoading(false);
    }
  }

  function handleOIDCLogin() {
    window.location.href = "/auth/oidc/login";
  }

  if (!authConfig) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <LoadingSpinner label="Loading..." />
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <div className="w-full max-w-sm">
        <div className="mb-8 flex flex-col items-center gap-3">
          <img src="/owl-logo.png" alt="NightOwl" className="h-16 w-auto" />
          <h1 className="text-2xl font-bold tracking-tight">NightOwl</h1>
          <p className="text-sm text-muted-foreground">Sign in to continue</p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle className="text-center text-lg">Sign in</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {authConfig.oidc_enabled && (
              <>
                <Button
                  variant="outline"
                  className="w-full"
                  onClick={handleOIDCLogin}
                >
                  {authConfig.oidc_name || "Sign in with SSO"}
                </Button>
                <div className="relative">
                  <div className="absolute inset-0 flex items-center">
                    <span className="w-full border-t" />
                  </div>
                  <div className="relative flex justify-center text-xs uppercase">
                    <span className="bg-card px-2 text-muted-foreground">or</span>
                  </div>
                </div>
              </>
            )}

            {!authConfig.oidc_enabled && (
              <div className="relative">
                <div className="absolute inset-0 flex items-center">
                  <span className="w-full border-t" />
                </div>
                <div className="relative flex justify-center text-xs uppercase">
                  <span className="bg-card px-2 text-muted-foreground">
                    OIDC not configured
                  </span>
                </div>
              </div>
            )}

            <form onSubmit={handleLocalAdminSubmit} className="space-y-3">
              <div>
                <label htmlFor="username" className="block text-sm font-medium mb-1">
                  Username
                </label>
                <Input
                  id="username"
                  type="text"
                  placeholder="admin"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  required
                  autoComplete="username"
                />
              </div>
              <div>
                <label htmlFor="local-password" className="block text-sm font-medium mb-1">
                  Password
                </label>
                <Input
                  id="local-password"
                  type="password"
                  placeholder="Enter your password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  autoComplete="current-password"
                />
              </div>

              {error && <p className="text-sm text-destructive">{error}</p>}

              {retryAfter > 0 && (
                <p className="text-sm text-muted-foreground">
                  Try again in {retryAfter}s
                </p>
              )}

              <Button type="submit" className="w-full" disabled={loading || retryAfter > 0}>
                {loading ? "Signing in..." : "Sign in"}
              </Button>

              <p className="text-xs text-muted-foreground text-center">
                Rate limit: 10 attempts / 15 min
              </p>
            </form>
          </CardContent>
        </Card>

        <p className="mt-6 text-center text-xs text-muted-foreground">
          NightOwl — A Wisbric product
        </p>
      </div>
    </div>
  );
}
