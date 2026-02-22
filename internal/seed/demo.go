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

	"github.com/wisbric/nightowl/internal/auth"
	"github.com/wisbric/nightowl/internal/db"
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

	info, err := prov.Provision(ctx, "Acme Corp", "acme", json.RawMessage(`{"timezone":"Europe/Berlin","slack_workspace_url":"https://acme-corp.slack.com","slack_channel":"#ops-alerts"}`))
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

	// ── Runbooks ────────────────────────────────────────────────────────
	rbCat := func(s string) *string { return &s }
	rbTrue := true

	rbCrashLoop, err := tq.CreateRunbook(ctx, db.CreateRunbookParams{
		Title:      "CrashLoopBackOff Triage",
		Content:    "## CrashLoopBackOff Triage\n\n### Immediate Actions\n1. `kubectl describe pod <pod> -n <ns>` — check Events section for reason\n2. `kubectl logs <pod> -n <ns> --previous` — check last crash logs\n3. Check if OOMKilled: `kubectl get pod <pod> -n <ns> -o jsonpath='{.status.containerStatuses[0].lastState.terminated.reason}'`\n\n### Common Causes\n- **OOMKilled**: Increase memory limits in deployment spec\n- **Error exit code 1**: Application startup failure — check config/secrets\n- **Error exit code 137**: SIGKILL — likely OOM or preemption\n- **ImagePullBackOff**: Wrong image tag or registry auth\n\n### Resolution\n1. Fix root cause (config, memory, image)\n2. `kubectl rollout restart deployment/<name> -n <ns>`\n3. Monitor: `kubectl get pods -n <ns> -w`\n4. Verify: no restarts for 5+ minutes",
		Category:   rbCat("compute"),
		IsTemplate: &rbTrue,
		Tags:       []string{"kubernetes", "crashloop", "pod", "troubleshooting"},
		CreatedBy:  pgtype.UUID{Bytes: alice.ID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("creating runbook crashloop: %w", err)
	}

	rbOOM, err := tq.CreateRunbook(ctx, db.CreateRunbookParams{
		Title:      "OOMKilled Recovery",
		Content:    "## OOMKilled Recovery\n\n### Diagnosis\n1. Confirm OOM: `kubectl describe pod <pod> -n <ns> | grep -A5 'Last State'`\n2. Check current limits: `kubectl get pod <pod> -n <ns> -o jsonpath='{.spec.containers[0].resources}'`\n3. Review metrics: check Grafana memory dashboard for the namespace\n\n### Immediate Fix\n1. Increase memory limit by 50%:\n```yaml\nresources:\n  requests:\n    memory: \"384Mi\"\n  limits:\n    memory: \"512Mi\"\n```\n2. Apply: `kubectl apply -f deployment.yaml`\n3. Monitor: `kubectl top pods -n <ns>`\n\n### Long-term\n- Profile application memory usage with pprof\n- Set up VPA (Vertical Pod Autoscaler) recommendations\n- Add memory alerts at 80% threshold\n- Check for memory leaks in application logs",
		Category:   rbCat("compute"),
		IsTemplate: &rbTrue,
		Tags:       []string{"kubernetes", "oom", "memory", "resources"},
		CreatedBy:  pgtype.UUID{Bytes: alice.ID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("creating runbook oom: %w", err)
	}

	rbCert, err := tq.CreateRunbook(ctx, db.CreateRunbookParams{
		Title:      "TLS Certificate Expiry",
		Content:    "## TLS Certificate Expiry\n\n### Check Status\n1. `kubectl get certificate -A` — find certificates managed by cert-manager\n2. `kubectl describe certificate <name> -n <ns>` — check Ready condition and renewal time\n3. `openssl s_client -connect <host>:443 -servername <host> 2>/dev/null | openssl x509 -noout -dates`\n\n### Force Renewal\n1. Delete the secret to trigger renewal:\n```bash\nkubectl delete secret <tls-secret> -n <ns>\n```\n2. cert-manager will detect the missing secret and reissue\n3. Monitor: `kubectl get certificate <name> -n <ns> -w`\n\n### Troubleshooting\n- Check cert-manager logs: `kubectl logs -n cert-manager deploy/cert-manager`\n- Verify ClusterIssuer: `kubectl get clusterissuer -o yaml`\n- Check DNS propagation for DNS01 challenges\n- Verify ACME account is not rate-limited (Let's Encrypt)",
		Category:   rbCat("network"),
		IsTemplate: &rbTrue,
		Tags:       []string{"tls", "certificate", "cert-manager", "security"},
		CreatedBy:  pgtype.UUID{Bytes: diana.ID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("creating runbook cert: %w", err)
	}
	logger.Info("seed-demo: created runbooks", "count", 3)

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

	// ── Rosters ─────────────────────────────────────────────────────────
	now := time.Now()
	startDate := pgtype.Date{Time: now.AddDate(0, -1, 0), Valid: true}
	fts := true

	rosterNZDesc := "New Zealand on-call team covering NZST business hours"
	rosterNZ, err := tq.CreateRoster(ctx, db.CreateRosterParams{
		Name: "NZ On-Call", Description: &rosterNZDesc,
		Timezone: "Pacific/Auckland", RotationType: "weekly", RotationLength: 7,
		HandoffTime:        pgtype.Time{Microseconds: 9 * 3600 * 1e6, Valid: true}, // 09:00
		IsFollowTheSun:     &fts,
		EscalationPolicyID: pgtype.UUID{Bytes: policy.ID, Valid: true},
		StartDate:          startDate,
	})
	if err != nil {
		return fmt.Errorf("creating roster NZ: %w", err)
	}

	rosterDEDesc := "Germany on-call team covering CET business hours"
	rosterDE, err := tq.CreateRoster(ctx, db.CreateRosterParams{
		Name: "DE On-Call", Description: &rosterDEDesc,
		Timezone: "Europe/Berlin", RotationType: "weekly", RotationLength: 7,
		HandoffTime:        pgtype.Time{Microseconds: 9 * 3600 * 1e6, Valid: true},
		IsFollowTheSun:     &fts,
		LinkedRosterID:     pgtype.UUID{Bytes: rosterNZ.ID, Valid: true},
		EscalationPolicyID: pgtype.UUID{Bytes: policy.ID, Valid: true},
		StartDate:          startDate,
	})
	if err != nil {
		return fmt.Errorf("creating roster DE: %w", err)
	}

	// NZ members: Chandra (pos 1), Enzo (pos 2)
	for _, m := range []struct {
		rid uuid.UUID
		uid uuid.UUID
		pos int32
	}{
		{rosterNZ.ID, chandra.ID, 1},
		{rosterNZ.ID, enzo.ID, 2},
		{rosterDE.ID, alice.ID, 1},
		{rosterDE.ID, diana.ID, 2},
		{rosterDE.ID, bob.ID, 3},
	} {
		if _, err := tq.CreateRosterMember(ctx, db.CreateRosterMemberParams{
			RosterID: m.rid, UserID: m.uid, Position: m.pos,
		}); err != nil {
			return fmt.Errorf("creating roster member: %w", err)
		}
	}
	logger.Info("seed-demo: created rosters with members", "rosters", 2, "members", 5)

	// ── Knowledge Base Incidents ────────────────────────────────────────
	type incSpec struct {
		title, severity, category string
		fps, tags, services      []string
		clusters, namespaces     []string
		symptoms, rootCause, sol string
		errorPatterns            []string
		runbookID                pgtype.UUID
		createdBy                pgtype.UUID
	}

	incSpecs := []incSpec{
		{
			title: "Pod CrashLoopBackOff — OOMKilled", severity: "critical", category: "compute",
			fps: []string{"crashloop-oom"}, tags: []string{"kubernetes", "oom", "crashloop"},
			services: []string{"payment-gateway"}, clusters: []string{"prod-eu-1"}, namespaces: []string{"payments"},
			symptoms:      "Pod enters CrashLoopBackOff with increasing backoff delays. Container last state shows OOMKilled (exit code 137).",
			rootCause:     "Memory limit set too low for peak traffic. Java heap + off-heap exceeds container memory limit during garbage collection spikes.",
			sol:           "Increase memory limit from 256Mi to 512Mi. Set -XX:MaxRAMPercentage=75 to prevent JVM from consuming entire cgroup limit. Add VPA recommendations.",
			errorPatterns: []string{"OOMKilled", "exit code 137", "CrashLoopBackOff"},
			runbookID:     pgtype.UUID{Bytes: rbOOM.ID, Valid: true},
			createdBy:     pgtype.UUID{Bytes: alice.ID, Valid: true},
		},
		{
			title: "Pod CrashLoopBackOff — Config Error", severity: "warning", category: "compute",
			fps: []string{"crashloop-config"}, tags: []string{"kubernetes", "crashloop", "configuration"},
			services: []string{"auth-service"}, clusters: []string{"prod-eu-1"}, namespaces: []string{"identity"},
			symptoms:      "Pod crashes immediately after starting. Logs show 'failed to load configuration' or 'missing required environment variable'.",
			rootCause:     "ConfigMap or Secret referenced by the deployment was updated without restarting pods, or a required env var was removed.",
			sol:           "Verify all referenced ConfigMaps and Secrets exist. Check `kubectl describe pod` for missing volume mounts. Rollout restart after fixing config.",
			errorPatterns: []string{"exit code 1", "failed to load configuration", "missing required"},
			runbookID:     pgtype.UUID{Bytes: rbCrashLoop.ID, Valid: true},
			createdBy:     pgtype.UUID{Bytes: bob.ID, Valid: true},
		},
		{
			title: "TLS Certificate Expired — Ingress 503", severity: "critical", category: "network",
			fps: []string{"cert-expired-ingress"}, tags: []string{"tls", "certificate", "ingress", "503"},
			services: []string{"ingress-controller"}, clusters: []string{"prod-eu-1"}, namespaces: []string{"ingress-nginx"},
			symptoms:      "All HTTPS endpoints return 503. Browser shows NET::ERR_CERT_DATE_INVALID. cert-manager Certificate resource shows Ready=False.",
			rootCause:     "cert-manager ClusterIssuer ACME account key was rotated without updating the secret. Renewal failed silently 30 days before expiry.",
			sol:           "Delete the TLS secret to trigger immediate re-issuance. Verify ClusterIssuer has valid ACME account. Set up certificate expiry monitoring at 14-day threshold.",
			errorPatterns: []string{"x509: certificate has expired", "503 Service Temporarily Unavailable", "NET::ERR_CERT_DATE_INVALID"},
			runbookID:     pgtype.UUID{Bytes: rbCert.ID, Valid: true},
			createdBy:     pgtype.UUID{Bytes: chandra.ID, Valid: true},
		},
		{
			title: "PostgreSQL Disk Usage Critical (>90%)", severity: "critical", category: "storage",
			fps: []string{"pg-disk-full"}, tags: []string{"postgresql", "disk", "storage", "cnpg"},
			services: []string{"customer-db"}, clusters: []string{"prod-eu-1"}, namespaces: []string{"database"},
			symptoms:      "CNPG PostgreSQL pod reports PVC usage above 90%. Write operations start failing with 'could not extend file' errors.",
			rootCause:     "WAL retention + bloated tables from missing VACUUM. autovacuum was disabled during a migration and not re-enabled.",
			sol:           "1. Run VACUUM FULL on bloated tables. 2. Ensure autovacuum is enabled. 3. Expand PVC: kubectl edit pvc data-customer-db-0 (if storage class supports expansion). 4. Set up alerts at 80% threshold.",
			errorPatterns: []string{"could not extend file", "No space left on device", "disk usage.*9[0-9]%"},
			createdBy:     pgtype.UUID{Bytes: alice.ID, Valid: true},
		},
		{
			title: "Node NotReady — Kubelet Stopped", severity: "critical", category: "compute",
			fps: []string{"node-notready"}, tags: []string{"kubernetes", "node", "kubelet"},
			services: []string{}, clusters: []string{"prod-eu-1"}, namespaces: []string{},
			symptoms:      "Node shows NotReady status. Pods on the node are evicted. kubelet is not responding to API server.",
			rootCause:     "Kernel OOM killer terminated kubelet due to memory pressure from a daemonset with no resource limits.",
			sol:           "SSH to node, restart kubelet: `systemctl restart kubelet`. Set resource limits on all daemonsets. Consider using system-reserved and kube-reserved kubelet flags.",
			errorPatterns: []string{"NodeNotReady", "node.*condition.*Ready.*False", "Kubelet stopped posting node status"},
			createdBy:     pgtype.UUID{Bytes: diana.ID, Valid: true},
		},
		{
			title: "ImagePullBackOff — Private Registry Auth Failure", severity: "warning", category: "compute",
			fps: []string{"imagepull-auth"}, tags: []string{"kubernetes", "image", "registry", "auth"},
			services: []string{"order-api"}, clusters: []string{"prod-eu-1"}, namespaces: []string{"commerce"},
			symptoms:      "New pods stuck in ImagePullBackOff. Events show 'unauthorized: authentication required' for ghcr.io images.",
			rootCause:     "Image pull secret expired. GHCR personal access token used in imagePullSecret had a 90-day expiry.",
			sol:           "Rotate the image pull secret with a new PAT. Use workload identity or managed identity instead of static tokens. Set up token expiry monitoring.",
			errorPatterns: []string{"ImagePullBackOff", "unauthorized: authentication required", "ErrImagePull"},
			createdBy:     pgtype.UUID{Bytes: bob.ID, Valid: true},
		},
		{
			title: "High Latency — DNS Resolution Failures", severity: "warning", category: "network",
			fps: []string{"dns-latency"}, tags: []string{"dns", "coredns", "latency", "network"},
			services: []string{"order-api", "payment-gateway"}, clusters: []string{"prod-eu-1"}, namespaces: []string{"kube-system"},
			symptoms:      "Multiple services report increased p99 latency. DNS lookups intermittently fail with SERVFAIL. CoreDNS pods show high CPU usage.",
			rootCause:     "CoreDNS ndots:5 default causing excessive DNS queries. Each service-to-service call generates 5+ DNS lookups before resolving.",
			sol:           "Set `dnsConfig.options: [{name: ndots, value: '2'}]` in pod specs for services that call external domains. Scale CoreDNS with HPA. Enable DNS caching in application.",
			errorPatterns: []string{"SERVFAIL", "i/o timeout.*lookup", "dns: lookup.*no such host"},
			createdBy:     pgtype.UUID{Bytes: chandra.ID, Valid: true},
		},
		{
			title: "etcd Leader Election Failure", severity: "critical", category: "compute",
			fps: []string{"etcd-no-leader"}, tags: []string{"etcd", "control-plane", "leader-election"},
			services: []string{}, clusters: []string{"prod-eu-1"}, namespaces: []string{"kube-system"},
			symptoms:      "kubectl commands hang or timeout. API server returns 'etcdserver: leader changed' errors. New pods cannot be scheduled.",
			rootCause:     "Network partition between etcd members caused by a misconfigured firewall rule during infrastructure maintenance.",
			sol:           "1. Check etcd member health: `etcdctl endpoint health`. 2. Verify network connectivity between members. 3. If quorum lost, restore from backup. 4. Add etcd health to maintenance checklist.",
			errorPatterns: []string{"etcdserver: leader changed", "context deadline exceeded", "etcdserver: no leader"},
			createdBy:     pgtype.UUID{Bytes: alice.ID, Valid: true},
		},
		{
			title: "HPA Flapping — Rapid Scale Up/Down", severity: "info", category: "compute",
			fps: []string{"hpa-flapping"}, tags: []string{"kubernetes", "hpa", "autoscaling"},
			services: []string{"order-api"}, clusters: []string{"prod-eu-1"}, namespaces: []string{"commerce"},
			symptoms:      "HPA rapidly scales between min and max replicas every 2-3 minutes. CPU oscillates around the target threshold.",
			rootCause:     "HPA target CPU utilization set to 50% with default stabilization window. Bursty traffic pattern triggers constant rescaling.",
			sol:           "Increase `behavior.scaleDown.stabilizationWindowSeconds` to 300. Set scaleDown rate limit to 1 pod per 60s. Consider using KEDA with a longer polling interval.",
			errorPatterns: []string{"ScalingActive", "DesiredReplicas.*changed", "HPA.*scaled"},
			createdBy:     pgtype.UUID{Bytes: enzo.ID, Valid: true},
		},
		{
			title: "PVC Pending — No Available Persistent Volumes", severity: "warning", category: "storage",
			fps: []string{"pvc-pending"}, tags: []string{"kubernetes", "storage", "pvc", "provisioner"},
			services: []string{"customer-db"}, clusters: []string{"prod-eu-1"}, namespaces: []string{"database"},
			symptoms:      "StatefulSet pods stuck in Pending. PVC shows status Pending. Events show 'waiting for first consumer to be created' or 'no persistent volumes available'.",
			rootCause:     "Storage class provisioner ran out of capacity in the availability zone. Longhorn storage pool exhausted.",
			sol:           "1. Check storage pool: `kubectl get nodes.longhorn.io -n longhorn-system`. 2. Add disk capacity or a new node. 3. For immediate fix: create PV manually. 4. Set up storage capacity alerting.",
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
		{alice.ID, "create", "roster", rosterNZ.ID, `{"name":"NZ On-Call"}`},
		{alice.ID, "create", "roster", rosterDE.ID, `{"name":"DE On-Call"}`},
		{alice.ID, "create", "incident", incidentIDs[0], `{"title":"Pod CrashLoopBackOff — OOMKilled"}`},
		{bob.ID, "create", "incident", incidentIDs[1], `{"title":"Pod CrashLoopBackOff — Config Error"}`},
		{chandra.ID, "create", "incident", incidentIDs[2], `{"title":"TLS Certificate Expired — Ingress 503"}`},
		{alice.ID, "acknowledge", "alert", alerts[9].ID, `{"alert":"High Error Rate — auth-service"}`},
		{chandra.ID, "resolve", "alert", alerts[10].ID, `{"alert":"Certificate Expiring"}`},
		{bob.ID, "resolve", "alert", alerts[11].ID, `{"alert":"CrashLoopBackOff — auth-service config"}`},
		{diana.ID, "update", "escalation_policy", policy.ID, `{"change":"added tier 3 phone escalation"}`},
		{alice.ID, "create", "runbook", rbCrashLoop.ID, `{"title":"CrashLoopBackOff Triage"}`},
		{alice.ID, "create", "runbook", rbOOM.ID, `{"title":"OOMKilled Recovery"}`},
		{diana.ID, "create", "runbook", rbCert.ID, `{"title":"TLS Certificate Expiry"}`},
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

	logger.Info("seed-demo: completed",
		"tenant", info.Slug,
		"users", len(users),
		"services", len(svcs),
		"incidents", len(incidentIDs),
		"alerts", len(alerts),
		"runbooks", 3,
		"rosters", 2,
		"escalation_policies", 1,
		"audit_entries", len(auditSpecs),
	)
	return nil
}
