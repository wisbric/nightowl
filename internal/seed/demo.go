package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wisbric/core/pkg/auth"

	"github.com/wisbric/nightowl/internal/db"
	"github.com/wisbric/nightowl/pkg/roster"
	"github.com/wisbric/nightowl/pkg/tenant"
)

// RunDemo provisions the "acme" tenant with comprehensive demo data.
// It is destructive: it drops and recreates the tenant if it exists.
func RunDemo(ctx context.Context, pool *pgxpool.Pool, databaseURL, migrationsDir string, logger *slog.Logger) error {
	q := db.New(pool)

	// Drop existing tenant so we always get fresh demo data.
	if existing, err := q.GetTenantBySlug(ctx, "acme"); err == nil {
		logger.Info("seed-demo: dropping existing tenant 'acme'")
		if _, err := pool.Exec(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS tenant_%s CASCADE", existing.Slug)); err != nil {
			return fmt.Errorf("dropping tenant schema: %w", err)
		}
		if err := q.DeleteTenant(ctx, existing.ID); err != nil {
			return fmt.Errorf("deleting tenant row: %w", err)
		}
	}

	prov := &tenant.Provisioner{
		DB:            pool,
		DatabaseURL:   databaseURL,
		MigrationsDir: migrationsDir,
		Logger:        logger,
	}

	info, err := prov.Provision(ctx, "Acme Corp", "acme", json.RawMessage(`{"timezone":"Europe/Berlin","slack_workspace_url":"https://acme-corp.slack.com","slack_channel":"#ops-alerts","bookowl_api_url":"http://localhost:8081/api/v1","bookowl_api_key":"bw_dev_seed_key_do_not_use_in_production"}`))
	if err != nil {
		return fmt.Errorf("provisioning tenant: %w", err)
	}
	logger.Info("seed-demo: provisioned tenant", "id", info.ID, "slug", info.Slug)

	// Acquire a tenant-scoped connection.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquiring connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, fmt.Sprintf("SET search_path TO %s, public", info.Schema)); err != nil {
		return fmt.Errorf("setting search_path: %w", err)
	}
	tq := db.New(conn)

	// ── Users ───────────────────────────────────────────────────────────
	type userSpec struct {
		extID, email, name, tz, phone, role string
	}
	userSpecs := []userSpec{
		{"oidc|alice", "alice@acme.example.com", "Alice Hartmann", "Europe/Berlin", "+4915112345678", "admin"},
		{"oidc|bob", "bob@acme.example.com", "Bob Mitchell", "America/New_York", "+14155551234", "engineer"},
		{"oidc|chandra", "chandra@acme.example.com", "Chandra Patel", "Pacific/Auckland", "+6421555789", "engineer"},
		{"oidc|diana", "diana@acme.example.com", "Diana Krueger", "Europe/Berlin", "+4917655598765", "manager"},
		{"oidc|enzo", "enzo@acme.example.com", "Enzo Rossi", "Pacific/Auckland", "+6422555432", "engineer"},
	}

	users := make([]db.User, len(userSpecs))
	for i, s := range userSpecs {
		phone := s.phone
		users[i], err = tq.CreateUser(ctx, db.CreateUserParams{
			ExternalID: s.extID, Email: s.email, DisplayName: s.name,
			Timezone: s.tz, Phone: &phone, Role: s.role,
		})
		if err != nil {
			return fmt.Errorf("creating user %s: %w", s.name, err)
		}
	}
	logger.Info("seed-demo: created users", "count", len(users))

	alice, bob, chandra, diana, enzo := users[0], users[1], users[2], users[3], users[4]

	// ── Services ────────────────────────────────────────────────────────
	type svcSpec struct {
		name, cluster, ns, desc, tier, meta string
		owner                               uuid.UUID
	}
	svcSpecs := []svcSpec{
		{"payment-gateway", "prod-eu-1", "payments", "Stripe payment processing service", "critical", `{"team":"payments","language":"go"}`, alice.ID},
		{"auth-service", "prod-eu-1", "identity", "OIDC/OAuth2 authentication provider", "critical", `{"team":"platform","language":"go"}`, diana.ID},
		{"order-api", "prod-eu-1", "commerce", "Order management REST API", "standard", `{"team":"commerce","language":"typescript"}`, bob.ID},
		{"customer-db", "prod-eu-1", "database", "PostgreSQL customer data cluster (CNPG)", "critical", `{"team":"data","language":"postgresql"}`, alice.ID},
		{"ingress-controller", "prod-eu-1", "ingress-nginx", "NGINX Ingress Controller", "standard", `{"team":"infra","language":"helm"}`, chandra.ID},
	}

	svcs := make([]db.Service, len(svcSpecs))
	for i, s := range svcSpecs {
		cl, ns, desc, tier := s.cluster, s.ns, s.desc, s.tier
		svcs[i], err = tq.CreateService(ctx, db.CreateServiceParams{
			Name: s.name, Cluster: &cl, Namespace: &ns, Description: &desc,
			OwnerID: pgtype.UUID{Bytes: s.owner, Valid: true}, Tier: &tier,
			Metadata: []byte(s.meta),
		})
		if err != nil {
			return fmt.Errorf("creating service %s: %w", s.name, err)
		}
	}
	logger.Info("seed-demo: created services", "count", len(svcs))

	svcPayment, svcAuth, svcOrder, svcDB, svcIngress := svcs[0], svcs[1], svcs[2], svcs[3], svcs[4]

	// ── Escalation Policy ───────────────────────────────────────────────
	policyDesc := "Production critical services — page on-call, escalate to manager, then VP Engineering"
	repeatCount := int32(1)
	policy, err := tq.CreateEscalationPolicy(ctx, db.CreateEscalationPolicyParams{
		Name:        "Production Critical",
		Description: &policyDesc,
		Tiers: json.RawMessage(`[
			{"tier":1,"timeout_minutes":5,"notify_via":["slack","sms"],"targets":["on-call"]},
			{"tier":2,"timeout_minutes":15,"notify_via":["slack","sms","phone"],"targets":["on-call","manager"]},
			{"tier":3,"timeout_minutes":30,"notify_via":["phone"],"targets":["vp-engineering"]}
		]`),
		RepeatCount: &repeatCount,
	})
	if err != nil {
		return fmt.Errorf("creating escalation policy: %w", err)
	}
	logger.Info("seed-demo: created escalation policy", "id", policy.ID)

	// ── Rosters (v2 — explicit schedule) ───────────────────────────────
	handoffTime := pgtype.Time{Microseconds: 9 * 3600 * 1e6, Valid: true} // 09:00

	var rosterNZID, rosterDEID uuid.UUID
	err = conn.QueryRow(ctx, `INSERT INTO rosters (name, description, timezone, handoff_time, handoff_day,
	      schedule_weeks_ahead, max_consecutive_weeks, is_follow_the_sun,
	      escalation_policy_id, is_active)
	    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,true) RETURNING id`,
		"NZ On-Call", "New Zealand on-call team covering NZST business hours",
		"Pacific/Auckland", handoffTime, 1, 12, 2, true,
		policy.ID,
	).Scan(&rosterNZID)
	if err != nil {
		return fmt.Errorf("creating roster NZ: %w", err)
	}

	err = conn.QueryRow(ctx, `INSERT INTO rosters (name, description, timezone, handoff_time, handoff_day,
	      schedule_weeks_ahead, max_consecutive_weeks, is_follow_the_sun,
	      linked_roster_id, escalation_policy_id, is_active)
	    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,true) RETURNING id`,
		"DE On-Call", "Germany on-call team covering CET business hours",
		"Europe/Berlin", handoffTime, 1, 12, 2, true,
		rosterNZID, policy.ID,
	).Scan(&rosterDEID)
	if err != nil {
		return fmt.Errorf("creating roster DE: %w", err)
	}

	// Add members (v2: no position, has is_active).
	for _, m := range []struct {
		rid uuid.UUID
		uid uuid.UUID
	}{
		{rosterNZID, chandra.ID},
		{rosterNZID, enzo.ID},
		{rosterDEID, alice.ID},
		{rosterDEID, diana.ID},
		{rosterDEID, bob.ID},
	} {
		if _, err := conn.Exec(ctx,
			`INSERT INTO roster_members (roster_id, user_id, is_active, joined_at) VALUES ($1,$2,true,now())`,
			m.rid, m.uid); err != nil {
			return fmt.Errorf("creating roster member: %w", err)
		}
	}

	// Generate schedules for both rosters using the service.
	rosterSvc := roster.NewService(conn, logger)
	if _, err := rosterSvc.GenerateSchedule(ctx, rosterNZID, time.Now(), 12); err != nil {
		return fmt.Errorf("generating NZ schedule: %w", err)
	}
	if _, err := rosterSvc.GenerateSchedule(ctx, rosterDEID, time.Now(), 12); err != nil {
		return fmt.Errorf("generating DE schedule: %w", err)
	}
	logger.Info("seed-demo: created rosters with members and schedule", "rosters", 2, "members", 5)

	// ── Knowledge Base Incidents ────────────────────────────────────────
	type incSpec struct {
		title, severity, category string
		fps, tags, services       []string
		clusters, namespaces      []string
		symptoms, rootCause, sol  string
		errorPatterns             []string
		runbookID                 pgtype.UUID
		createdBy                 pgtype.UUID
	}

	incSpecs := []incSpec{
		{
			title: "Pod CrashLoopBackOff — OOMKilled", severity: "critical", category: "compute",
			fps: []string{"crashloop-oom"}, tags: []string{"kubernetes", "oom", "crashloop"},
			services: []string{"payment-gateway"}, clusters: []string{"prod-eu-1"}, namespaces: []string{"payments"},
			symptoms: `Pod payment-gateway-7f8b9c-x4k2m enters CrashLoopBackOff with increasing backoff delays (10s, 20s, 40s...).
Container last state shows OOMKilled (exit code 137). The pod has restarted 14 times in the last 30 minutes.
Grafana memory dashboard shows the container hitting its 256Mi limit during peak traffic windows.
Application logs (from --previous) show the Go runtime printing "runtime: out of memory" just before the crash.
Other pods in the same deployment are healthy — this appears to affect only pods handling large batch payment requests.`,
			rootCause: `Memory limit set too low for peak traffic. The payment-gateway service processes batch payment files that require
loading the entire CSV into memory. During the 14:00-14:30 UTC window, a merchant submitted a 50MB batch file.
The Go heap grew to 230Mi, and with goroutine stacks and runtime overhead the container exceeded the 256Mi limit.
The kernel OOM killer terminated the process (SIGKILL, exit code 137), and kubelet restarted it in CrashLoopBackOff.
VPA recommendations had been suggesting 512Mi for 2 weeks but were not applied because VPA is in "recommend only" mode.`,
			sol: `1. Immediate: Increase memory limit from 256Mi to 512Mi in the deployment spec
2. Apply: kubectl set resources deployment/payment-gateway -n payments --limits=memory=512Mi --requests=memory=256Mi
3. Set -XX:MaxRAMPercentage=75 (if JVM) or GOMEMLIMIT=400MiB (Go 1.19+) to prevent runtime from using entire cgroup
4. Long-term: Implement streaming CSV parsing instead of loading entire file into memory
5. Enable VPA in "Auto" mode for this deployment to auto-adjust limits based on observed usage
6. Add alert at 80% memory threshold: container_memory_working_set_bytes / container_spec_memory_limit_bytes > 0.8`,
			errorPatterns: []string{"OOMKilled", "exit code 137", "CrashLoopBackOff"},
			runbookID:     pgtype.UUID{},
			createdBy:     pgtype.UUID{Bytes: alice.ID, Valid: true},
		},
		{
			title: "Pod CrashLoopBackOff — Config Error", severity: "warning", category: "compute",
			fps: []string{"crashloop-config"}, tags: []string{"kubernetes", "crashloop", "configuration"},
			services: []string{"auth-service"}, clusters: []string{"prod-eu-1"}, namespaces: []string{"identity"},
			symptoms: `Pod auth-service-5a4b3c-rst01 crashes immediately after starting (exit code 1, no OOMKilled).
Container logs show: "FATAL: failed to load configuration: required environment variable OIDC_CLIENT_SECRET is not set"
The pod has been in CrashLoopBackOff for 8 minutes. All 3 replicas are affected.
The deployment was last updated 12 minutes ago by CI/CD pipeline (Argo CD sync).
No code changes — only a ConfigMap update was applied.`,
			rootCause: `A developer updated the auth-service ConfigMap to add a new config key but accidentally deleted the OIDC_CLIENT_SECRET
reference. The ConfigMap was applied via kubectl without --dry-run validation. Since ConfigMap changes don't trigger
a rolling restart, existing pods continued running with the old (cached) config. When Argo CD performed its regular
sync 4 minutes later, it restarted the deployment, and all new pods picked up the broken ConfigMap.`,
			sol: `1. Check the ConfigMap diff: kubectl diff -f configmap.yaml (compare to Git)
2. Restore the missing key: kubectl edit configmap auth-service-config -n identity
3. Verify all referenced env vars exist: kubectl set env deployment/auth-service --list -n identity
4. Rollout restart: kubectl rollout restart deployment/auth-service -n identity
5. Prevention: add a pre-sync hook in Argo CD that validates required env vars before applying
6. Consider using Sealed Secrets or External Secrets Operator for sensitive values to separate them from ConfigMaps`,
			errorPatterns: []string{"exit code 1", "failed to load configuration", "missing required"},
			runbookID:     pgtype.UUID{},
			createdBy:     pgtype.UUID{Bytes: bob.ID, Valid: true},
		},
		{
			title: "TLS Certificate Expired — Ingress 503", severity: "critical", category: "network",
			fps: []string{"cert-expired-ingress"}, tags: []string{"tls", "certificate", "ingress", "503"},
			services: []string{"ingress-controller"}, clusters: []string{"prod-eu-1"}, namespaces: []string{"ingress-nginx"},
			symptoms: `All HTTPS endpoints across api.acme.example.com and app.acme.example.com return 503 Service Unavailable.
Browsers display NET::ERR_CERT_DATE_INVALID with certificate expiry date showing yesterday.
The cert-manager Certificate resource shows Ready=False with message: "Failed to create Order: ACME account key mismatch."
Approximately 12,000 users are affected. Mobile apps also failing (certificate pinning check fails).
Uptime monitoring (Pingdom) triggered a global downtime alert at 03:12 UTC.`,
			rootCause: `During a routine infrastructure update 6 weeks ago, the cert-manager ClusterIssuer ACME account key was rotated
as part of a Helm chart upgrade. The new account key was generated but the old ACME registration was not migrated.
When cert-manager attempted to renew the certificate 30 days before expiry, the ACME server rejected the request
because the account key didn't match the original registration. cert-manager retried daily but all attempts failed.
No alert was configured for certificate renewal failures — only for certificate expiry (which fired too late).`,
			sol: `1. Immediate: Delete the TLS secret to force cert-manager to re-issue with a fresh order:
   kubectl delete secret acme-tls-prod -n ingress-nginx
2. Verify the ClusterIssuer ACME registration: kubectl describe clusterissuer letsencrypt-prod
3. If registration is broken: delete and recreate the ClusterIssuer to trigger fresh ACME registration
4. Monitor cert-manager logs until the new certificate is issued: kubectl logs -n cert-manager deploy/cert-manager -f
5. Verify: openssl s_client -connect api.acme.example.com:443 | openssl x509 -noout -dates
6. Prevention: Add Prometheus alert for cert_manager_certificate_ready_status == 0 (fires immediately on renewal failure)
7. Set up 14-day and 7-day expiry warnings: x509_cert_not_after - time() < 14*24*3600`,
			errorPatterns: []string{"x509: certificate has expired", "503 Service Temporarily Unavailable", "NET::ERR_CERT_DATE_INVALID"},
			runbookID:     pgtype.UUID{},
			createdBy:     pgtype.UUID{Bytes: chandra.ID, Valid: true},
		},
		{
			title: "PostgreSQL Disk Usage Critical (>90%)", severity: "critical", category: "storage",
			fps: []string{"pg-disk-full"}, tags: []string{"postgresql", "disk", "storage", "cnpg"},
			services: []string{"customer-db"}, clusters: []string{"prod-eu-1"}, namespaces: []string{"database"},
			symptoms: `CNPG PostgreSQL primary pod (customer-db-0) reports PVC usage at 94%.
Write operations intermittently failing with: "ERROR: could not extend file base/16384/2619: No space left on device"
Replication lag on replica-1 increasing (currently 45 seconds behind primary).
pg_stat_activity shows 12 idle-in-transaction connections holding open transactions.
The PVC is 50Gi, and df inside the container shows 47Gi used with WAL files accounting for 18Gi.`,
			rootCause: `Three contributing factors:
1. autovacuum was accidentally disabled on the largest table (customer_events, 2.1M rows) during a schema migration
   3 weeks ago. The migration script ran "ALTER TABLE customer_events SET (autovacuum_enabled = false)" but the
   re-enable statement was in a commented-out section.
2. Dead tuples accumulated to 800K rows, causing table bloat from 8Gi to 14Gi.
3. WAL retention was set to 7 days (wal_keep_size = 16GB) which is too aggressive for a 50Gi volume.
   Combined with the bloat, total usage exceeded 90%.`,
			sol: `1. Immediate triage: check which tables are bloated:
   SELECT schemaname, tablename, pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename))
   FROM pg_tables WHERE schemaname = 'public' ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC LIMIT 10;
2. Re-enable autovacuum: ALTER TABLE customer_events SET (autovacuum_enabled = true);
3. Run manual vacuum: VACUUM (VERBOSE, ANALYZE) customer_events;
4. If critically full, run VACUUM FULL (requires exclusive lock — schedule maintenance window)
5. Reduce WAL retention: ALTER SYSTEM SET wal_keep_size = '4GB'; SELECT pg_reload_conf();
6. Expand PVC if StorageClass supports it: kubectl edit pvc data-customer-db-0 (increase to 100Gi)
7. Set up alerts at 75% and 85% thresholds to catch this earlier`,
			errorPatterns: []string{"could not extend file", "No space left on device", "disk usage.*9[0-9]%"},
			createdBy:     pgtype.UUID{Bytes: alice.ID, Valid: true},
		},
		{
			title: "Node NotReady — Kubelet Stopped", severity: "critical", category: "compute",
			fps: []string{"node-notready"}, tags: []string{"kubernetes", "node", "kubelet"},
			services: []string{}, clusters: []string{"prod-eu-1"}, namespaces: []string{},
			symptoms: `Node prod-worker-05 shows NotReady status for 3 minutes (and counting).
12 pods are being evicted from the node, including 2 payment-gateway replicas.
kubectl describe node shows: condition Ready=False, message "Kubelet stopped posting node status".
The node was previously healthy for 45 days since last restart.
Cloud provider console shows the instance is running (no hardware issues detected).`,
			rootCause: `The kernel OOM killer terminated the kubelet process due to memory pressure. A newly deployed log-collector
daemonset had no resource limits configured and was consuming 3.8Gi of memory (out of 8Gi node total).
Combined with system processes and other daemonsets, total memory usage hit 98%, triggering the OOM killer.
The OOM killer selected kubelet (PID 1284, oom_score_adj -999) because it was the largest non-essential process
after the log-collector. Without kubelet, the node stopped heartbeating and the API server marked it NotReady.`,
			sol: `1. Immediate: SSH to the node and restart kubelet: systemctl restart kubelet
2. Verify node recovers: kubectl get node prod-worker-05 (should return to Ready within 60s)
3. Fix the root cause: add resource limits to the log-collector daemonset:
   kubectl set resources daemonset/log-collector --limits=memory=512Mi,cpu=200m -n monitoring
4. Cordon and drain the node if it doesn't recover: kubectl cordon prod-worker-05 && kubectl drain prod-worker-05 --ignore-daemonsets
5. Prevention: Enforce resource limits via OPA/Gatekeeper policy (reject pods without limits)
6. Set system-reserved in kubelet config: --system-reserved=memory=1Gi,cpu=500m
7. Set kube-reserved: --kube-reserved=memory=512Mi,cpu=200m`,
			errorPatterns: []string{"NodeNotReady", "node.*condition.*Ready.*False", "Kubelet stopped posting node status"},
			createdBy:     pgtype.UUID{Bytes: diana.ID, Valid: true},
		},
		{
			title: "ImagePullBackOff — Private Registry Auth Failure", severity: "warning", category: "compute",
			fps: []string{"imagepull-auth"}, tags: []string{"kubernetes", "image", "registry", "auth"},
			services: []string{"order-api"}, clusters: []string{"prod-eu-1"}, namespaces: []string{"commerce"},
			symptoms: `New pods for order-api deployment stuck in ImagePullBackOff after a routine deployment.
Events show: "Failed to pull image ghcr.io/acme/order-api:v2.3.1: unauthorized: authentication required"
Existing running pods are unaffected (they already pulled the image).
The image tag v2.3.1 was pushed to GHCR 20 minutes ago and is confirmed to exist via GitHub UI.
kubectl get secret ghcr-pull -n commerce -o jsonpath='{.data.\.dockerconfigjson}' shows the secret exists.`,
			rootCause: `The GitHub Personal Access Token (PAT) used in the imagePullSecret expired 2 days ago. The PAT was created with
a 90-day expiry by a team member who has since left the company. No monitoring was configured for token expiry.
The issue only became visible during deployment because existing pods had already cached the image layers locally.
Other namespaces using the same registry (payments, identity) will also fail on their next deployment.`,
			sol: `1. Generate a new fine-grained PAT in GitHub (org admin account, packages:read scope, 180-day expiry)
2. Update the secret in all affected namespaces:
   kubectl create secret docker-registry ghcr-pull --docker-server=ghcr.io --docker-username=acme-bot \
     --docker-password=<new-pat> -n commerce --dry-run=client -o yaml | kubectl apply -f -
3. Repeat for: payments, identity, monitoring namespaces
4. Restart failed pods: kubectl rollout restart deployment/order-api -n commerce
5. Long-term: migrate to workload identity / OIDC token exchange (no static tokens)
6. Add token expiry monitoring: check PAT expiry dates weekly via GitHub API
7. Document rotation procedure in BookOwl runbooks`,
			errorPatterns: []string{"ImagePullBackOff", "unauthorized: authentication required", "ErrImagePull"},
			createdBy:     pgtype.UUID{Bytes: bob.ID, Valid: true},
		},
		{
			title: "High Latency — DNS Resolution Failures", severity: "warning", category: "network",
			fps: []string{"dns-latency"}, tags: []string{"dns", "coredns", "latency", "network"},
			services: []string{"order-api", "payment-gateway"}, clusters: []string{"prod-eu-1"}, namespaces: []string{"kube-system"},
			symptoms: `Multiple services (order-api, payment-gateway, auth-service) report p99 latency increase from 200ms to 2.4s.
DNS lookups intermittently fail with SERVFAIL — about 3% of lookups are failing.
CoreDNS pods (2 replicas) showing CPU usage at 95% of their 200m limit.
Running nslookup from debug pods takes 2-5 seconds instead of the normal <10ms.
The issue started 45 minutes ago, correlating with a traffic spike from a partner integration going live.`,
			rootCause: `CoreDNS ndots:5 default (from /etc/resolv.conf injected by kubelet) causes excessive DNS queries.
Each service-to-service call to "payment-gateway.payments.svc.cluster.local" first tries 5 search domain suffixes
before resolving the FQDN. With the partner traffic doubling request volume, CoreDNS CPU saturated at 2 replicas.
The CoreDNS HPA was not configured, so it couldn't auto-scale. Additionally, the CoreDNS cache plugin was set to
30 seconds TTL, which is too low for internal service names that rarely change.`,
			sol: `1. Immediate: Scale CoreDNS manually: kubectl scale deployment/coredns -n kube-system --replicas=4
2. Add ndots:2 to affected services to reduce unnecessary DNS queries:
   spec.template.spec.dnsConfig.options: [{name: ndots, value: "2"}]
3. Increase CoreDNS cache TTL for cluster.local zone: edit the Corefile ConfigMap to set success/denial cache to 300s
4. Configure CoreDNS HPA: min=2, max=8, target CPU=60%
5. Consider deploying NodeLocal DNSCache daemonset for large clusters
6. Application-side: enable DNS caching in HTTP clients (e.g., Go: custom Resolver with TTL cache)
7. Monitor: add alert for coredns_dns_request_duration_seconds_bucket p99 > 200ms`,
			errorPatterns: []string{"SERVFAIL", "i/o timeout.*lookup", "dns: lookup.*no such host"},
			createdBy:     pgtype.UUID{Bytes: chandra.ID, Valid: true},
		},
		{
			title: "etcd Leader Election Failure", severity: "critical", category: "compute",
			fps: []string{"etcd-no-leader"}, tags: []string{"etcd", "control-plane", "leader-election"},
			services: []string{}, clusters: []string{"prod-eu-1"}, namespaces: []string{"kube-system"},
			symptoms: `kubectl commands hang or timeout with: "error: the server doesn't have a resource type"
API server logs show: "etcdserver: leader changed" repeating every 2-3 seconds.
New pods cannot be scheduled — kube-scheduler is unable to write to etcd.
etcd metric etcd_server_has_leader is 0 on all 3 etcd members.
The cluster has been in this state for 5 minutes. Impact: no new deployments, no scaling, no pod scheduling.`,
			rootCause: `Network partition between etcd members caused by a firewall rule change during scheduled infrastructure maintenance.
The infra team added a new network policy that inadvertently blocked TCP port 2380 (etcd peer communication) between
the 3 etcd nodes. Port 2379 (client) was unaffected, so the API server could still connect to individual members,
but the members couldn't replicate to each other. Without peer communication, the leader couldn't maintain quorum
and stepped down. With no leader, all write operations fail.`,
			sol: `1. Verify etcd member connectivity: etcdctl member list && etcdctl endpoint health --cluster
2. Check network: telnet <etcd-peer-ip>:2380 from each member to verify port 2380 is open
3. If firewall issue: revert the firewall rule change (check recent changes in infra change log)
4. If quorum is lost and cannot be restored:
   a. Stop all etcd members
   b. Restore from the most recent etcd snapshot: etcdctl snapshot restore /backup/etcd-snapshot.db
   c. Restart etcd members one at a time
5. Verify cluster health: etcdctl endpoint health --cluster (all members should report "healthy")
6. Verify API server: kubectl get nodes (should respond within 1 second)
7. Prevention: add etcd peer port (2380) to the "never block" list in firewall automation
8. Add alert: etcd_server_has_leader == 0 for > 30 seconds → critical page`,
			errorPatterns: []string{"etcdserver: leader changed", "context deadline exceeded", "etcdserver: no leader"},
			createdBy:     pgtype.UUID{Bytes: alice.ID, Valid: true},
		},
		{
			title: "HPA Flapping — Rapid Scale Up/Down", severity: "info", category: "compute",
			fps: []string{"hpa-flapping"}, tags: []string{"kubernetes", "hpa", "autoscaling"},
			services: []string{"order-api"}, clusters: []string{"prod-eu-1"}, namespaces: []string{"commerce"},
			symptoms: `HPA for order-api rapidly scales between 2 (min) and 8 replicas every 2-3 minutes.
kubectl describe hpa order-api shows: "New size: 8; reason: cpu resource utilization above target" followed by
"New size: 2; reason: All metrics below target" within 90 seconds.
CPU utilization oscillates: 80% → 20% → 75% → 15% in a repeating pattern.
No impact on end users yet, but the constant pod churn is generating excessive container image pulls and DNS updates.`,
			rootCause: `HPA target CPU utilization was set to 50% with the default stabilization window (0 seconds for scale-up,
300 seconds for scale-down was overridden to 0 in a previous configuration change).
The order-api service handles webhook callbacks from payment providers. These arrive in bursts every 60 seconds.
Each burst spikes CPU to 80% for ~15 seconds, then CPU drops to 15% while waiting for the next batch.
With no stabilization window, HPA reacts to every spike and every trough, creating a perpetual scale-up/down cycle.`,
			sol: `1. Add stabilization window for scale-down to prevent rapid oscillation:
   spec.behavior.scaleDown.stabilizationWindowSeconds: 300
2. Add a scale-down rate limit to prevent aggressive downsizing:
   spec.behavior.scaleDown.policies: [{type: Pods, value: 1, periodSeconds: 60}]
3. Increase the scale-up stabilization slightly to absorb burst spikes:
   spec.behavior.scaleUp.stabilizationWindowSeconds: 30
4. Consider increasing minReplicas to 3 (to handle the baseline burst without triggering HPA)
5. Alternative: use KEDA with a Prometheus scaler and a longer polling interval (60s) instead of HPA
6. Application-side: investigate if webhook processing can be batched or rate-limited at ingestion`,
			errorPatterns: []string{"ScalingActive", "DesiredReplicas.*changed", "HPA.*scaled"},
			createdBy:     pgtype.UUID{Bytes: enzo.ID, Valid: true},
		},
		{
			title: "PVC Pending — No Available Persistent Volumes", severity: "warning", category: "storage",
			fps: []string{"pvc-pending"}, tags: []string{"kubernetes", "storage", "pvc", "provisioner"},
			services: []string{"customer-db"}, clusters: []string{"prod-eu-1"}, namespaces: []string{"database"},
			symptoms: `StatefulSet customer-db replica-2 pod stuck in Pending for 12 minutes.
PVC data-customer-db-2 shows status Pending with event: "waiting for first consumer to be created"
followed by: "persistentvolume-controller: no persistent volumes available for this claim and no storage class is set"
The StorageClass "longhorn" exists and is the default, but the Longhorn UI shows all nodes at >95% storage utilization.
Replicas 0 and 1 are running normally.`,
			rootCause: `The Longhorn storage pool is exhausted across all 3 worker nodes. Each node has a 200Gi disk allocated to Longhorn,
but existing volumes (PostgreSQL, Redis, Prometheus) consume 580Gi total. The new 50Gi PVC request for replica-2
cannot be satisfied because no single node has 50Gi free.
This happened because the team added Prometheus TSDB persistence (100Gi) last week without checking available capacity.
Longhorn's over-provisioning percentage is set to 100% (default), meaning it won't provision beyond physical capacity.`,
			sol: `1. Check current storage allocation: kubectl get nodes.longhorn.io -n longhorn-system -o wide
2. Check Longhorn volume usage: kubectl get volumes.longhorn.io -n longhorn-system
3. Quick fix: add disk capacity to an existing node (attach additional EBS/PD volume)
4. Or: add a new worker node with Longhorn storage configured
5. For immediate unblocking: manually create a PV on a node with free space:
   longhorn-cli volume create --size 50Gi --numberOfReplicas 2
6. Long-term: set up storage capacity alerting at 80% utilization
7. Review over-provisioning settings: consider enabling Longhorn over-provisioning at 125% for non-critical workloads
8. Document minimum storage requirements per environment in the capacity planning spreadsheet`,
			errorPatterns: []string{"no persistent volumes available", "waiting for first consumer", "ProvisioningFailed"},
			createdBy:     pgtype.UUID{Bytes: diana.ID, Valid: true},
		},
	}

	// Use raw SQL to avoid scanning tsvector (search_vector) column which
	// pgx v5 cannot scan into interface{}.
	const insertIncident = `INSERT INTO incidents (
		title, fingerprints, severity, category, tags,
		services, clusters, namespaces, symptoms, error_patterns,
		root_cause, solution, runbook_id, created_by
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) RETURNING id`

	incidentIDs := make([]uuid.UUID, len(incSpecs))
	for i, s := range incSpecs {
		if err := conn.QueryRow(ctx, insertIncident,
			s.title, s.fps, s.severity, s.category, s.tags,
			s.services, s.clusters, s.namespaces, s.symptoms, s.errorPatterns,
			s.rootCause, s.sol, s.runbookID, s.createdBy,
		).Scan(&incidentIDs[i]); err != nil {
			return fmt.Errorf("creating incident %q: %w", s.title, err)
		}
	}
	logger.Info("seed-demo: created incidents", "count", len(incidentIDs))

	// ── Alerts ──────────────────────────────────────────────────────────
	type alertSpec struct {
		fp, status, severity, source, title, desc string
		labels                                    string
		svcID                                     pgtype.UUID
		policyID                                  pgtype.UUID
	}

	policyRef := pgtype.UUID{Bytes: policy.ID, Valid: true}
	noPolicy := pgtype.UUID{}
	svcRef := func(s db.Service) pgtype.UUID { return pgtype.UUID{Bytes: s.ID, Valid: true} }

	alertSpecs := []alertSpec{
		// Critical — firing
		{"crashloop-payment-gw-7f8b", "firing", "critical", "alertmanager",
			"Pod CrashLoopBackOff — payment-gateway-7f8b9c-x4k2m",
			"Pod payment-gateway-7f8b9c-x4k2m in namespace payments is in CrashLoopBackOff. Last exit code: 137 (OOMKilled). Restarts: 14.",
			`{"pod":"payment-gateway-7f8b9c-x4k2m","namespace":"payments","cluster":"prod-eu-1","container":"payment-gateway","exit_code":"137"}`,
			svcRef(svcPayment), policyRef},
		{"etcd-leader-lost-prod", "firing", "critical", "alertmanager",
			"etcd Cluster Has No Leader",
			"etcd cluster prod-eu-1 has had no leader for 45 seconds. API server requests are timing out.",
			`{"cluster":"prod-eu-1","component":"etcd","job":"kube-etcd"}`,
			pgtype.UUID{}, policyRef},
		{"disk-critical-pg-0", "firing", "critical", "prometheus",
			"Disk Usage Critical — data-customer-db-0",
			"PVC data-customer-db-0 is 94% full. Write operations may fail imminently.",
			`{"pvc":"data-customer-db-0","namespace":"database","cluster":"prod-eu-1","mountpoint":"/var/lib/postgresql/data"}`,
			svcRef(svcDB), policyRef},
		{"node-notready-worker-05", "firing", "critical", "alertmanager",
			"Node NotReady — prod-worker-05",
			"Node prod-worker-05 has been in NotReady state for 3 minutes. 12 pods are being evicted.",
			`{"node":"prod-worker-05","cluster":"prod-eu-1","condition":"Ready","status":"False"}`,
			pgtype.UUID{}, policyRef},

		// Warning — firing
		{"high-memory-order-api", "firing", "warning", "prometheus",
			"High Memory Usage — order-api",
			"order-api deployment memory usage at 87% of limit (442Mi / 512Mi). OOM risk under load.",
			`{"deployment":"order-api","namespace":"commerce","cluster":"prod-eu-1","usage_percent":"87"}`,
			svcRef(svcOrder), noPolicy},
		{"hpa-flapping-order-api", "firing", "warning", "prometheus",
			"HPA Flapping — order-api",
			"HPA order-api has scaled up/down 8 times in the last 15 minutes. Current replicas: 4, desired: 2.",
			`{"hpa":"order-api","namespace":"commerce","cluster":"prod-eu-1","min_replicas":"2","max_replicas":"10"}`,
			svcRef(svcOrder), noPolicy},
		{"dns-latency-coredns", "firing", "warning", "prometheus",
			"CoreDNS High Latency — p99 > 500ms",
			"CoreDNS p99 latency is 823ms (threshold: 500ms). Multiple services reporting increased response times.",
			`{"pod":"coredns-7db6d8ff4d-abc12","namespace":"kube-system","cluster":"prod-eu-1","p99_ms":"823"}`,
			pgtype.UUID{}, noPolicy},
		{"pvc-pending-db-replica", "firing", "warning", "alertmanager",
			"PVC Pending — data-customer-db-2",
			"PVC data-customer-db-2 has been in Pending state for 12 minutes. StatefulSet replica cannot start.",
			`{"pvc":"data-customer-db-2","namespace":"database","cluster":"prod-eu-1","storageclass":"longhorn"}`,
			svcRef(svcDB), noPolicy},
		{"imagepull-order-api-new", "firing", "warning", "alertmanager",
			"ImagePullBackOff — order-api-v2.3.1",
			"Pods for order-api deployment stuck in ImagePullBackOff. Image ghcr.io/acme/order-api:v2.3.1 cannot be pulled.",
			`{"pod":"order-api-6c4d5f-mnop1","namespace":"commerce","cluster":"prod-eu-1","image":"ghcr.io/acme/order-api:v2.3.1"}`,
			svcRef(svcOrder), noPolicy},

		// Acknowledged
		{"auth-high-error-rate", "acknowledged", "warning", "prometheus",
			"High Error Rate — auth-service (5xx > 2%)",
			"auth-service is returning 5xx errors at 3.2% rate (threshold: 2%). Investigating OIDC provider connectivity.",
			`{"deployment":"auth-service","namespace":"identity","cluster":"prod-eu-1","error_rate":"3.2%"}`,
			svcRef(svcAuth), noPolicy},

		// Resolved
		{"cert-expiry-api-acme", "resolved", "warning", "cert-manager",
			"Certificate Expiring — api.acme.example.com",
			"TLS certificate for api.acme.example.com was expiring in 5 days. Certificate has been renewed.",
			`{"domain":"api.acme.example.com","namespace":"ingress-nginx","issuer":"letsencrypt-prod"}`,
			svcRef(svcIngress), noPolicy},
		{"crashloop-auth-config", "resolved", "warning", "alertmanager",
			"Pod CrashLoopBackOff — auth-service-5a4b3c-rst01",
			"auth-service pod was crashing due to missing OIDC_CLIENT_SECRET env var after ConfigMap update. Fixed by restoring secret.",
			`{"pod":"auth-service-5a4b3c-rst01","namespace":"identity","cluster":"prod-eu-1","exit_code":"1"}`,
			svcRef(svcAuth), noPolicy},
		{"high-latency-payment-gw", "resolved", "info", "prometheus",
			"Payment Gateway p99 Latency > 2s",
			"payment-gateway p99 latency was elevated at 2.4s. Resolved after upstream Stripe API recovered.",
			`{"deployment":"payment-gateway","namespace":"payments","cluster":"prod-eu-1","p99_ms":"2400"}`,
			svcRef(svcPayment), noPolicy},
		{"node-high-cpu-worker-03", "resolved", "warning", "prometheus",
			"High CPU Usage — prod-worker-03",
			"Node prod-worker-03 CPU usage was at 95% due to runaway log-collector daemonset. Resolved by setting CPU limits.",
			`{"node":"prod-worker-03","cluster":"prod-eu-1","usage_percent":"95"}`,
			pgtype.UUID{}, noPolicy},
		{"cert-expiry-internal-ca", "resolved", "info", "cert-manager",
			"Internal CA Certificate Renewed",
			"Internal CA certificate for service mesh mTLS was approaching expiry. cert-manager auto-renewed successfully.",
			`{"domain":"*.acme.internal","namespace":"istio-system","issuer":"internal-ca"}`,
			pgtype.UUID{}, noPolicy},
		{"ingress-5xx-spike", "resolved", "critical", "alertmanager",
			"Ingress 5xx Spike — ingress-nginx",
			"ingress-nginx returned 5xx errors for 2 minutes during a deployment rollout. Resolved when new pods passed readiness checks.",
			`{"deployment":"ingress-nginx-controller","namespace":"ingress-nginx","cluster":"prod-eu-1","error_count":"342"}`,
			svcRef(svcIngress), policyRef},
	}

	alerts := make([]db.Alert, len(alertSpecs))
	for i, s := range alertSpecs {
		desc := s.desc
		alerts[i], err = tq.CreateAlert(ctx, db.CreateAlertParams{
			Fingerprint: s.fp, Status: s.status, Severity: s.severity,
			Source: s.source, Title: s.title, Description: &desc,
			Labels: json.RawMessage(s.labels), Annotations: json.RawMessage(`{}`),
			ServiceID: s.svcID, EscalationPolicyID: s.policyID,
		})
		if err != nil {
			return fmt.Errorf("creating alert %q: %w", s.title, err)
		}
	}
	logger.Info("seed-demo: created alerts", "count", len(alerts))

	// ── Audit Log Entries ───────────────────────────────────────────────
	loopback := netip.MustParseAddr("10.0.1.50")
	ua := "NightOwl/0.1.0"

	type auditSpec struct {
		userID     uuid.UUID
		action     string
		resource   string
		resourceID uuid.UUID
		detail     string
	}

	auditSpecs := []auditSpec{
		{alice.ID, "create", "service", svcPayment.ID, `{"name":"payment-gateway"}`},
		{diana.ID, "create", "service", svcAuth.ID, `{"name":"auth-service"}`},
		{bob.ID, "create", "service", svcOrder.ID, `{"name":"order-api"}`},
		{alice.ID, "create", "escalation_policy", policy.ID, `{"name":"Production Critical"}`},
		{alice.ID, "create", "roster", rosterNZID, `{"name":"NZ On-Call"}`},
		{alice.ID, "create", "roster", rosterDEID, `{"name":"DE On-Call"}`},
		{alice.ID, "create", "incident", incidentIDs[0], `{"title":"Pod CrashLoopBackOff — OOMKilled"}`},
		{bob.ID, "create", "incident", incidentIDs[1], `{"title":"Pod CrashLoopBackOff — Config Error"}`},
		{chandra.ID, "create", "incident", incidentIDs[2], `{"title":"TLS Certificate Expired — Ingress 503"}`},
		{alice.ID, "acknowledge", "alert", alerts[9].ID, `{"alert":"High Error Rate — auth-service"}`},
		{chandra.ID, "resolve", "alert", alerts[10].ID, `{"alert":"Certificate Expiring"}`},
		{bob.ID, "resolve", "alert", alerts[11].ID, `{"alert":"CrashLoopBackOff — auth-service config"}`},
		{diana.ID, "update", "escalation_policy", policy.ID, `{"change":"added tier 3 phone escalation"}`},
	}

	for _, s := range auditSpecs {
		if _, err := tq.CreateAuditLogEntry(ctx, db.CreateAuditLogEntryParams{
			UserID:     pgtype.UUID{Bytes: s.userID, Valid: true},
			Action:     s.action,
			Resource:   s.resource,
			ResourceID: pgtype.UUID{Bytes: s.resourceID, Valid: true},
			Detail:     []byte(s.detail),
			IpAddress:  &loopback,
			UserAgent:  &ua,
		}); err != nil {
			return fmt.Errorf("creating audit entry: %w", err)
		}
	}
	logger.Info("seed-demo: created audit log entries", "count", len(auditSpecs))

	// ── API Key ─────────────────────────────────────────────────────────
	apiKeyHash := auth.HashAPIKey(DevAPIKey)
	if _, err := q.CreateAPIKey(ctx, db.CreateAPIKeyParams{
		TenantID:    info.ID,
		KeyHash:     apiKeyHash,
		KeyPrefix:   DevAPIKey[:16],
		Description: "Development seed API key",
		Role:        "admin",
		Scopes:      []string{"*"},
	}); err != nil {
		return fmt.Errorf("creating seed API key: %w", err)
	}

	// ── Local Admin ─────────────────────────────────────────────────────
	if err := ensureLocalAdmin(ctx, pool, info.ID, logger); err != nil {
		return err
	}

	logger.Info("seed-demo: completed",
		"tenant", info.Slug,
		"users", len(users),
		"services", len(svcs),
		"incidents", len(incidentIDs),
		"alerts", len(alerts),
		"rosters", 2,
		"escalation_policies", 1,
		"audit_entries", len(auditSpecs),
	)
	return nil
}
