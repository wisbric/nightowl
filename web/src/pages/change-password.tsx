import { useState, type FormEvent } from "react";
import { useNavigate } from "@tanstack/react-router";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/contexts/auth-context";
import { Check, X } from "lucide-react";

function validatePassword(pw: string) {
  return {
    minLength: pw.length >= 12,
    hasUpper: /[A-Z]/.test(pw),
    hasLower: /[a-z]/.test(pw),
    hasDigitOrSymbol: /[0-9]/.test(pw) || /[^A-Za-z0-9]/.test(pw),
  };
}

export function ChangePasswordPage() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const validation = validatePassword(newPassword);
  const allValid =
    validation.minLength &&
    validation.hasUpper &&
    validation.hasLower &&
    validation.hasDigitOrSymbol;
  const passwordsMatch = newPassword === confirmPassword && confirmPassword !== "";

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);

    if (!allValid) {
      setError("Password does not meet requirements");
      return;
    }
    if (!passwordsMatch) {
      setError("Passwords do not match");
      return;
    }

    setLoading(true);

    try {
      const res = await fetch("/auth/change-password", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "same-origin",
        body: JSON.stringify({
          current_password: currentPassword,
          new_password: newPassword,
        }),
      });

      if (!res.ok) {
        const body = await res.json().catch(() => ({ message: "Failed to change password" }));
        throw new Error(body.message || "Failed to change password");
      }

      // Cookie is refreshed by the server; update frontend auth state.
      login({
        id: "local-admin",
        email: "admin@local",
        display_name: "Local Admin",
        role: "admin",
      });

      navigate({ to: "/" });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to change password");
    } finally {
      setLoading(false);
    }
  }

  function Req({ met, label }: { met: boolean; label: string }) {
    return (
      <div className="flex items-center gap-2 text-xs">
        {met ? (
          <Check className="h-3 w-3 text-severity-ok" />
        ) : (
          <X className="h-3 w-3 text-muted-foreground" />
        )}
        <span className={met ? "text-severity-ok" : "text-muted-foreground"}>{label}</span>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <div className="w-full max-w-sm">
        <div className="mb-8 flex flex-col items-center gap-3">
          <img src="/owl-logo.png" alt="NightOwl" className="h-16 w-auto" />
          <h1 className="text-2xl font-bold tracking-tight">Change Password</h1>
          <p className="text-sm text-muted-foreground">
            You must change your password before continuing.
          </p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle className="text-center text-lg">Set New Password</CardTitle>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit} className="space-y-4">
              <div>
                <label htmlFor="current" className="block text-sm font-medium mb-1">
                  Current Password
                </label>
                <Input
                  id="current"
                  type="password"
                  value={currentPassword}
                  onChange={(e) => setCurrentPassword(e.target.value)}
                  required
                  autoComplete="current-password"
                />
              </div>

              <div>
                <label htmlFor="new" className="block text-sm font-medium mb-1">
                  New Password
                </label>
                <Input
                  id="new"
                  type="password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  required
                  autoComplete="new-password"
                />
              </div>

              <div>
                <label htmlFor="confirm" className="block text-sm font-medium mb-1">
                  Confirm New Password
                </label>
                <Input
                  id="confirm"
                  type="password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  required
                  autoComplete="new-password"
                />
              </div>

              {/* Requirements checklist */}
              <div className="space-y-1 rounded-md border p-3">
                <p className="text-xs font-medium mb-2">Password Requirements</p>
                <Req met={validation.minLength} label="At least 12 characters" />
                <Req met={validation.hasUpper} label="At least one uppercase letter" />
                <Req met={validation.hasLower} label="At least one lowercase letter" />
                <Req met={validation.hasDigitOrSymbol} label="At least one number or symbol" />
                <Req met={passwordsMatch} label="Passwords match" />
              </div>

              {error && <p className="text-sm text-destructive">{error}</p>}

              <Button
                type="submit"
                className="w-full"
                disabled={loading || !allValid || !passwordsMatch}
              >
                {loading ? "Changing..." : "Change Password"}
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
