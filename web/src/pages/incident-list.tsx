import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { api } from "@/lib/api";
import { useTitle } from "@/hooks/use-title";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { SeverityBadge } from "@/components/ui/severity-badge";
import { Badge } from "@/components/ui/badge";
import { formatRelativeTime } from "@/lib/utils";
import type { Incident } from "@/types/api";
import { Search } from "lucide-react";

export function IncidentListPage() {
  useTitle("Knowledge Base");
  const [search, setSearch] = useState("");
  const [severityFilter, setSeverityFilter] = useState("");

  const isSearch = search.length >= 2;

  const { data: incidents, isLoading } = useQuery({
    queryKey: ["incidents", search, severityFilter],
    queryFn: () => {
      if (isSearch) {
        return api.get<Incident[]>(`/incidents/search?q=${encodeURIComponent(search)}&limit=50`);
      }
      const params = new URLSearchParams();
      if (severityFilter) params.set("severity", severityFilter);
      params.set("limit", "50");
      return api.get<Incident[]>(`/incidents?${params}`);
    },
  });

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Knowledge Base</h1>
        <Link to="/incidents/$incidentId" params={{ incidentId: "new" }}>
          <Button>New Incident</Button>
        </Link>
      </div>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-4">
            <div className="relative flex-1">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                placeholder="Search incidents..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="pl-9"
              />
            </div>
            <Select value={severityFilter} onChange={(e) => setSeverityFilter(e.target.value)} className="w-40">
              <option value="">All severities</option>
              <option value="critical">Critical</option>
              <option value="major">Major</option>
              <option value="warning">Warning</option>
              <option value="info">Info</option>
            </Select>
          </div>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <p className="text-sm text-muted-foreground">Loading...</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Severity</TableHead>
                  <TableHead>Title</TableHead>
                  <TableHead>Category</TableHead>
                  <TableHead>Services</TableHead>
                  <TableHead>Occurrences</TableHead>
                  <TableHead>Updated</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(incidents ?? []).map((inc) => (
                  <TableRow key={inc.id}>
                    <TableCell><SeverityBadge severity={inc.severity} /></TableCell>
                    <TableCell>
                      <Link to="/incidents/$incidentId" params={{ incidentId: inc.id }} className="text-sm hover:text-accent transition-colors">
                        {inc.title}
                      </Link>
                    </TableCell>
                    <TableCell className="text-muted-foreground text-sm">{inc.category || "—"}</TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {(inc.services ?? []).slice(0, 3).map((s) => (
                          <Badge key={s} variant="outline" className="text-xs">{s}</Badge>
                        ))}
                      </div>
                    </TableCell>
                    <TableCell className="text-sm">{inc.occurrence_count}</TableCell>
                    <TableCell className="text-muted-foreground text-sm whitespace-nowrap">{formatRelativeTime(inc.updated_at)}</TableCell>
                  </TableRow>
                ))}
                {(incidents ?? []).length === 0 && (
                  <TableRow>
                    <TableCell colSpan={6} className="text-center text-muted-foreground py-8">
                      {isSearch ? "No matching incidents" : "No incidents found"}
                    </TableCell>
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
