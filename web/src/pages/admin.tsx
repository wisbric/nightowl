import { useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useTitle } from "@/hooks/use-title";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useState } from "react";
import { Settings, Key, Users, FileText } from "lucide-react";

export function AdminPage() {
  useTitle("Admin");
  const [apiKeyName, setApiKeyName] = useState("");
  const queryClient = useQueryClient();

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Administration</h1>

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Card className="opacity-75">
          <CardContent className="flex items-center gap-3 p-4">
            <Users className="h-8 w-8 text-muted-foreground" />
            <div>
              <p className="font-medium">Users</p>
              <p className="text-xs text-muted-foreground">Coming soon</p>
            </div>
          </CardContent>
        </Card>
        <Card className="opacity-75">
          <CardContent className="flex items-center gap-3 p-4">
            <Key className="h-8 w-8 text-muted-foreground" />
            <div>
              <p className="font-medium">API Keys</p>
              <p className="text-xs text-muted-foreground">Coming soon</p>
            </div>
          </CardContent>
        </Card>
        <Card className="opacity-75">
          <CardContent className="flex items-center gap-3 p-4">
            <Settings className="h-8 w-8 text-muted-foreground" />
            <div>
              <p className="font-medium">Configuration</p>
              <p className="text-xs text-muted-foreground">Coming soon</p>
            </div>
          </CardContent>
        </Card>
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
          {localStorage.getItem("nightowl_api_key") && (
            <p className="text-xs text-severity-ok">API key configured</p>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
