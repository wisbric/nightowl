import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider, createRouter, createRootRoute, createRoute, Outlet } from "@tanstack/react-router";
import { AppLayout } from "@/components/layout/app-layout";
import { initTheme } from "@/hooks/use-theme";
import { DashboardPage } from "@/pages/dashboard";
import { AlertListPage } from "@/pages/alert-list";
import { AlertDetailPage } from "@/pages/alert-detail";
import { IncidentListPage } from "@/pages/incident-list";
import { IncidentDetailPage } from "@/pages/incident-detail";
import { RosterListPage } from "@/pages/roster-list";
import { RosterDetailPage } from "@/pages/roster-detail";
import { EscalationListPage } from "@/pages/escalation-list";
import { EscalationDetailPage } from "@/pages/escalation-detail";
import { AdminPage } from "@/pages/admin";
import { AdminUsersPage } from "@/pages/admin-users";
import { AdminApiKeysPage } from "@/pages/admin-api-keys";
import { AdminConfigPage } from "@/pages/admin-config";
import { AuditLogPage } from "@/pages/audit-log";
import { RunbookListPage } from "@/pages/runbook-list";
import { RunbookDetailPage } from "@/pages/runbook-detail";
import { StatusPage } from "@/pages/status";
import { AboutPage } from "@/pages/about";
import { NotFoundPage } from "@/pages/not-found";
import "./index.css";

initTheme();

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { staleTime: 30_000, retry: 1 },
  },
});

const rootRoute = createRootRoute({
  component: () => (
    <AppLayout>
      <Outlet />
    </AppLayout>
  ),
});

const indexRoute = createRoute({ getParentRoute: () => rootRoute, path: "/", component: DashboardPage });
const alertsRoute = createRoute({ getParentRoute: () => rootRoute, path: "/alerts", component: AlertListPage });
const alertDetailRoute = createRoute({ getParentRoute: () => rootRoute, path: "/alerts/$alertId", component: AlertDetailPage });
const incidentsRoute = createRoute({ getParentRoute: () => rootRoute, path: "/incidents", component: IncidentListPage });
const incidentDetailRoute = createRoute({ getParentRoute: () => rootRoute, path: "/incidents/$incidentId", component: IncidentDetailPage });
const rostersRoute = createRoute({ getParentRoute: () => rootRoute, path: "/rosters", component: RosterListPage });
const rosterDetailRoute = createRoute({ getParentRoute: () => rootRoute, path: "/rosters/$rosterId", component: RosterDetailPage });
const escalationRoute = createRoute({ getParentRoute: () => rootRoute, path: "/escalation", component: EscalationListPage });
const escalationDetailRoute = createRoute({ getParentRoute: () => rootRoute, path: "/escalation/$policyId", component: EscalationDetailPage });
const runbooksRoute = createRoute({ getParentRoute: () => rootRoute, path: "/runbooks", component: RunbookListPage });
const runbookDetailRoute = createRoute({ getParentRoute: () => rootRoute, path: "/runbooks/$runbookId", component: RunbookDetailPage });
const adminRoute = createRoute({ getParentRoute: () => rootRoute, path: "/admin", component: AdminPage });
const adminUsersRoute = createRoute({ getParentRoute: () => rootRoute, path: "/admin/users", component: AdminUsersPage });
const adminApiKeysRoute = createRoute({ getParentRoute: () => rootRoute, path: "/admin/api-keys", component: AdminApiKeysPage });
const adminConfigRoute = createRoute({ getParentRoute: () => rootRoute, path: "/admin/config", component: AdminConfigPage });
const auditLogRoute = createRoute({ getParentRoute: () => rootRoute, path: "/admin/audit-log", component: AuditLogPage });
const statusRoute = createRoute({ getParentRoute: () => rootRoute, path: "/status", component: StatusPage });
const aboutRoute = createRoute({ getParentRoute: () => rootRoute, path: "/about", component: AboutPage });
const notFoundRoute = createRoute({ getParentRoute: () => rootRoute, path: "$", component: NotFoundPage });

const routeTree = rootRoute.addChildren([
  indexRoute,
  alertsRoute,
  alertDetailRoute,
  incidentsRoute,
  incidentDetailRoute,
  rostersRoute,
  rosterDetailRoute,
  runbooksRoute,
  runbookDetailRoute,
  escalationRoute,
  escalationDetailRoute,
  statusRoute,
  aboutRoute,
  adminRoute,
  adminUsersRoute,
  adminApiKeysRoute,
  adminConfigRoute,
  auditLogRoute,
  notFoundRoute,
]);

const router = createRouter({ routeTree });

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>
);
