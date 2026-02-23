import { useEffect } from "react";
import { useNavigate, useSearch } from "@tanstack/react-router";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import { useAuth } from "@/contexts/auth-context";

export function AuthCallbackPage() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const search = useSearch({ strict: false }) as Record<string, string>;

  useEffect(() => {
    const token = search.token;
    if (!token) {
      navigate({ to: "/login" });
      return;
    }

    // Validate the token and get user info.
    fetch("/auth/me", {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((res) => {
        if (!res.ok) throw new Error("invalid token");
        return res.json();
      })
      .then((user) => {
        login(token, user);
        navigate({ to: "/" });
      })
      .catch(() => {
        navigate({ to: "/login" });
      });
  }, [search.token, login, navigate]);

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <LoadingSpinner label="Completing sign in..." />
    </div>
  );
}
