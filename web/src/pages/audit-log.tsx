import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { formatRelativeTime } from "@/lib/utils";
import type { AuditEntry } from "@/types/api";
import { Search } from "lucide-react";

export function AuditLogPage() {
  useTitle("Audit Log");
  const [search, setSearch] = useState("");

  const { data: entries, isLoading } = useQuery({
    queryKey: ["audit-log", search],
    queryFn: () => {
      const params = new URLSearchParams();
      if (search) params.set("q", search);
      params.set("limit", "100");
      return api.get<AuditEntry[]>(`/audit-log?${params}`);
    },
  });

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/admin" className="text-muted-foreground hover:text-foreground text-sm">&larr; Admin</Link>
        <h1 className="text-2xl font-bold">Audit Log</h1>
      </div>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-4">
            <div className="relative flex-1">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                placeholder="Filter by action, resource, or user..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="pl-9"
              />
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <p className="text-sm text-muted-foreground">Loading...</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>When</TableHead>
                  <TableHead>User</TableHead>
                  <TableHead>Action</TableHead>
                  <TableHead>Resource</TableHead>
                  <TableHead>Details</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(entries ?? []).map((entry) => (
                  <TableRow key={entry.id}>
                    <TableCell className="text-sm text-muted-foreground whitespace-nowrap">{formatRelativeTime(entry.created_at)}</TableCell>
                    <TableCell className="text-sm font-mono text-xs">{entry.user_id ? entry.user_id.slice(0, 8) : entry.api_key_id?.slice(0, 8) ?? "system"}</TableCell>
                    <TableCell><Badge variant="outline" className="text-xs">{entry.action}</Badge></TableCell>
                    <TableCell className="text-sm">
                      <span className="text-muted-foreground">{entry.resource}</span>
                      <span className="font-mono text-xs ml-1">{entry.resource_id.slice(0, 8)}...</span>
                    </TableCell>
                    <TableCell className="text-xs font-mono text-muted-foreground max-w-xs truncate">
                      {entry.detail ? (() => { try { return atob(entry.detail).slice(0, 80); } catch { return entry.detail.slice(0, 80); } })() : "—"}
                    </TableCell>
                  </TableRow>
                ))}
                {(entries ?? []).length === 0 && (
                  <TableRow>
                    <TableCell colSpan={5} className="text-center text-muted-foreground py-8">No audit entries</TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
