import { useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useTitle } from "@/hooks/use-title";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useState } from "react";
import { Settings, Key, Users, FileText, Shield } from "lucide-react";

export function AdminPage() {
  useTitle("Admin");
  const [apiKeyName, setApiKeyName] = useState("");
  const queryClient = useQueryClient();

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Administration</h1>

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Link to="/admin/users">
          <Card className="cursor-pointer hover:border-accent transition-colors">
            <CardContent className="flex items-center gap-3 p-4">
              <Users className="h-8 w-8 text-muted-foreground" />
              <div>
                <p className="font-medium">Users</p>
                <p className="text-xs text-muted-foreground">Manage team members</p>
              </div>
            </CardContent>
          </Card>
        </Link>
        <Link to="/admin/api-keys">
          <Card className="cursor-pointer hover:border-accent transition-colors">
            <CardContent className="flex items-center gap-3 p-4">
              <Key className="h-8 w-8 text-muted-foreground" />
              <div>
                <p className="font-medium">API Keys</p>
                <p className="text-xs text-muted-foreground">Manage API keys</p>
              </div>
            </CardContent>
          </Card>
        </Link>
        <Link to="/admin/config">
          <Card className="cursor-pointer hover:border-accent transition-colors">
            <CardContent className="flex items-center gap-3 p-4">
              <Settings className="h-8 w-8 text-muted-foreground" />
              <div>
                <p className="font-medium">Configuration</p>
                <p className="text-xs text-muted-foreground">Tenant settings</p>
              </div>
            </CardContent>
          </Card>
        </Link>
        <Link to="/admin/auth">
          <Card className="cursor-pointer hover:border-accent transition-colors">
            <CardContent className="flex items-center gap-3 p-4">
              <Shield className="h-8 w-8 text-muted-foreground" />
              <div>
                <p className="font-medium">Authentication</p>
                <p className="text-xs text-muted-foreground">OIDC & local admin</p>
              </div>
            </CardContent>
          </Card>
        </Link>
        <Link to="/admin/audit-log">
          <Card className="cursor-pointer hover:border-accent transition-colors">
            <CardContent className="flex items-center gap-3 p-4">
              <FileText className="h-8 w-8 text-muted-foreground" />
              <div>
                <p className="font-medium">Audit Log</p>
                <p className="text-xs text-muted-foreground">View activity log</p>
              </div>
            </CardContent>
          </Card>
        </Link>
      </div>

      <Card>
        <CardHeader><CardTitle>API Key Configuration</CardTitle></CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-muted-foreground">
            Enter your API key to authenticate with the NightOwl API. This is stored locally in your browser.
          </p>
          <div className="flex gap-2">
            <Input
              type="password"
              placeholder="API key (e.g. ow_dev_seed_key_do_not_use_in_production)"
              value={apiKeyName}
              onChange={(e) => setApiKeyName(e.target.value)}
            />
            <Button
              onClick={() => {
                localStorage.setItem("nightowl_api_key", apiKeyName);
                queryClient.invalidateQueries();
              }}
            >
              Save
            </Button>
          </div>
          {localStorage.getItem("nightowl_api_key") ? (
            <p className="text-xs text-severity-ok">API key configured</p>
          ) : import.meta.env.DEV ? (
            <p className="text-xs text-muted-foreground">Using dev seed key (auto)</p>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}
