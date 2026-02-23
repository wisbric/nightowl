import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider, createRouter, createRootRoute, createRoute, Outlet, redirect } from "@tanstack/react-router";
import { AuthProvider } from "@/contexts/auth-context";
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
import { SettingsPage } from "@/pages/settings";
import { SettingsTokensPage } from "@/pages/settings-tokens";
import { LoginPage } from "@/pages/login";
import { AuthCallbackPage } from "@/pages/auth-callback";
import "./index.css";

initTheme();

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { staleTime: 30_000, retry: 1 },
  },
});

// Auth guard: in dev mode always allow; in prod require token.
function requireAuth() {
  if (import.meta.env.DEV) return;
  const token = localStorage.getItem("nightowl_token");
  if (!token) {
    throw redirect({ to: "/login" });
  }
}

// Public layout (no sidebar) for login/callback.
const publicRootRoute = createRootRoute({
  component: () => <Outlet />,
});

// Authenticated layout with sidebar.
const appLayoutRoute = createRoute({
  getParentRoute: () => publicRootRoute,
  id: "app",
  beforeLoad: requireAuth,
  component: () => (
    <AppLayout>
      <Outlet />
    </AppLayout>
  ),
});

// Public routes (no auth required).
const loginRoute = createRoute({ getParentRoute: () => publicRootRoute, path: "/login", component: LoginPage });
const authCallbackRoute = createRoute({
  getParentRoute: () => publicRootRoute,
  path: "/auth/callback",
  component: AuthCallbackPage,
  validateSearch: (search: Record<string, unknown>) => ({
    token: (search.token as string) || "",
  }),
});

// Authenticated routes.
const indexRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/", component: DashboardPage });
const alertsRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/alerts", component: AlertListPage });
const alertDetailRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/alerts/$alertId", component: AlertDetailPage });
const incidentsRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/incidents", component: IncidentListPage });
const incidentDetailRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/incidents/$incidentId", component: IncidentDetailPage });
const rostersRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/rosters", component: RosterListPage });
const rosterDetailRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/rosters/$rosterId", component: RosterDetailPage });
const escalationRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/escalation", component: EscalationListPage });
const escalationDetailRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/escalation/$policyId", component: EscalationDetailPage });
const runbooksRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/runbooks", component: RunbookListPage });
const runbookDetailRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/runbooks/$runbookId", component: RunbookDetailPage });
const adminRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/admin", component: AdminPage });
const adminUsersRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/admin/users", component: AdminUsersPage });
const adminApiKeysRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/admin/api-keys", component: AdminApiKeysPage });
const adminConfigRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/admin/config", component: AdminConfigPage });
const auditLogRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/admin/audit-log", component: AuditLogPage });
const statusRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/status", component: StatusPage });
const aboutRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/about", component: AboutPage });
const settingsRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/settings", component: SettingsPage });
const settingsTokensRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "/settings/tokens", component: SettingsTokensPage });
const notFoundRoute = createRoute({ getParentRoute: () => appLayoutRoute, path: "$", component: NotFoundPage });

const routeTree = publicRootRoute.addChildren([
  loginRoute,
  authCallbackRoute,
  appLayoutRoute.addChildren([
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
    settingsRoute,
    settingsTokensRoute,
    adminRoute,
    adminUsersRoute,
    adminApiKeysRoute,
    adminConfigRoute,
    auditLogRoute,
    notFoundRoute,
  ]),
]);

const router = createRouter({ routeTree });

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <RouterProvider router={router} />
      </AuthProvider>
    </QueryClientProvider>
  </StrictMode>
);
