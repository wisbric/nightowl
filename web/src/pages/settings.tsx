import { Link } from "@tanstack/react-router";
import { Key, Settings } from "lucide-react";

export function SettingsPage() {
  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Settings</h1>
        <p className="text-sm text-muted-foreground">Manage your account settings.</p>
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <Link
          to="/settings/tokens"
          className="flex items-start gap-4 rounded-lg border p-4 transition-colors hover:bg-muted"
        >
          <Key className="mt-0.5 h-5 w-5 text-muted-foreground" />
          <div>
            <p className="font-medium text-sm">Personal Access Tokens</p>
            <p className="text-xs text-muted-foreground">
              Generate and manage tokens for API access.
            </p>
          </div>
        </Link>
        <div className="flex items-start gap-4 rounded-lg border border-dashed p-4 opacity-50">
          <Settings className="mt-0.5 h-5 w-5 text-muted-foreground" />
          <div>
            <p className="font-medium text-sm">Preferences</p>
            <p className="text-xs text-muted-foreground">Coming soon.</p>
          </div>
        </div>
      </div>
    </div>
  );
}
