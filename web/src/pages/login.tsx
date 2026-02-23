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
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [authConfig, setAuthConfig] = useState<AuthConfig | null>(null);

  useEffect(() => {
    fetch("/auth/config")
      .then((res) => res.json())
      .then(setAuthConfig)
      .catch(() => setAuthConfig({ oidc_enabled: false, oidc_name: "", local_enabled: true }));
  }, []);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);

    try {
      const res = await fetch("/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password }),
      });

      if (!res.ok) {
        const body = await res.json().catch(() => ({ message: "Login failed" }));
        throw new Error(body.message || "Invalid email or password");
      }

      const data = await res.json();
      login(data.token, data.user);
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
                    <span className="bg-card px-2 text-muted-foreground">
                      or sign in with email
                    </span>
                  </div>
                </div>
              </>
            )}

            <form onSubmit={handleSubmit} className="space-y-3">
              <div>
                <label htmlFor="email" className="block text-sm font-medium mb-1">
                  Email
                </label>
                <Input
                  id="email"
                  type="email"
                  placeholder="alice@acme.example.com"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                  autoComplete="email"
                />
              </div>
              <div>
                <label htmlFor="password" className="block text-sm font-medium mb-1">
                  Password
                </label>
                <Input
                  id="password"
                  type="password"
                  placeholder="Enter your password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  autoComplete="current-password"
                />
              </div>

              {error && (
                <p className="text-sm text-destructive">{error}</p>
              )}

              <Button type="submit" className="w-full" disabled={loading}>
                {loading ? "Signing in..." : "Sign in"}
              </Button>
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
