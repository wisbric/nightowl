package runbook

// TemplateRunbook defines a pre-seeded runbook template.
type TemplateRunbook struct {
	Title    string
	Content  string
	Category *string
	Tags     []string
}

func strPtr(s string) *string { return &s }

// TemplateRunbooks returns the pre-seeded Kubernetes runbook templates.
func TemplateRunbooks() []TemplateRunbook {
	return []TemplateRunbook{
		{
			Title:    "Pod CrashLoopBackOff",
			Category: strPtr("kubernetes"),
			Tags:     []string{"k8s", "pod", "crashloop"},
			Content: `# Pod CrashLoopBackOff

## Symptoms
- Pod status shows CrashLoopBackOff
- Container restarts with increasing back-off delay
- Events show "Back-off restarting failed container"

## Diagnosis

### 1. Check pod events and status
` + "```bash" + `
kubectl describe pod <pod-name> -n <namespace>
kubectl get events -n <namespace> --sort-by='.lastTimestamp' | grep <pod-name>
` + "```" + `

### 2. Check container logs
` + "```bash" + `
kubectl logs <pod-name> -n <namespace> --previous
kubectl logs <pod-name> -n <namespace> -c <container-name> --previous
` + "```" + `

### 3. Check resource limits
` + "```bash" + `
kubectl top pod <pod-name> -n <namespace>
kubectl get pod <pod-name> -n <namespace> -o jsonpath='{.spec.containers[*].resources}'
` + "```" + `

## Common Causes
1. **Application error** — crash on startup due to missing config, bad env vars, or dependency unavailable
2. **OOMKilled** — container exceeds memory limits (check exit code 137)
3. **Liveness probe failure** — probe misconfigured or app not ready in time
4. **Missing dependencies** — database, config map, or secret not available

## Resolution
1. Fix the application error based on log output
2. Increase memory limits if OOMKilled
3. Adjust liveness probe initialDelaySeconds and timeoutSeconds
4. Ensure all dependencies (ConfigMaps, Secrets, Services) exist
`,
		},
		{
			Title:    "OOMKilled — Out of Memory",
			Category: strPtr("kubernetes"),
			Tags:     []string{"k8s", "oom", "memory"},
			Content: `# OOMKilled — Out of Memory

## Symptoms
- Container terminated with exit code 137
- Pod events show "OOMKilled"
- Container restarts repeatedly

## Diagnosis

### 1. Confirm OOM
` + "```bash" + `
kubectl describe pod <pod-name> -n <namespace> | grep -A5 "Last State"
kubectl get pod <pod-name> -n <namespace> -o jsonpath='{.status.containerStatuses[*].lastState}'
` + "```" + `

### 2. Check current memory usage
` + "```bash" + `
kubectl top pod <pod-name> -n <namespace>
kubectl top pod <pod-name> -n <namespace> --containers
` + "```" + `

### 3. Check memory limits
` + "```bash" + `
kubectl get pod <pod-name> -n <namespace> -o jsonpath='{.spec.containers[*].resources}'
` + "```" + `

## Common Causes
1. **Memory leak** — application allocates memory without releasing it
2. **Undersized limits** — workload needs more memory than allocated
3. **JVM heap misconfiguration** — heap size exceeds container limit
4. **Large data processing** — loading large datasets into memory

## Resolution
1. Profile the application for memory leaks (pprof for Go, heap dump for JVM)
2. Increase memory limits in the deployment manifest
3. For JVM: set -Xmx to 75% of container memory limit
4. Implement pagination or streaming for large data processing
`,
		},
		{
			Title:    "TLS Certificate Expiry",
			Category: strPtr("kubernetes"),
			Tags:     []string{"k8s", "tls", "certificate", "cert-manager"},
			Content: `# TLS Certificate Expiry

## Symptoms
- Browser shows certificate warning
- Clients receive TLS handshake errors
- cert-manager Certificate resource shows "not ready"

## Diagnosis

### 1. Check certificate status
` + "```bash" + `
kubectl get certificates -A
kubectl describe certificate <cert-name> -n <namespace>
` + "```" + `

### 2. Check cert-manager logs
` + "```bash" + `
kubectl logs -n cert-manager deploy/cert-manager
kubectl get certificaterequests -n <namespace>
kubectl get orders -n cert-manager
kubectl get challenges -n cert-manager
` + "```" + `

### 3. Inspect the actual certificate
` + "```bash" + `
kubectl get secret <tls-secret> -n <namespace> -o jsonpath='{.data.tls\.crt}' | base64 -d | openssl x509 -text -noout
` + "```" + `

## Common Causes
1. **cert-manager issue** — pod not running, RBAC misconfigured, or rate limited
2. **DNS challenge failure** — DNS provider credentials expired or misconfigured
3. **HTTP challenge failure** — ingress not routing .well-known/acme-challenge
4. **Rate limiting** — Let's Encrypt rate limits exceeded

## Resolution
1. Restart cert-manager if pods are unhealthy
2. Verify DNS provider credentials in the ClusterIssuer secret
3. Check ingress rules allow HTTP-01 challenge path
4. Wait for rate limit reset or use a different domain
5. Manual renewal: delete the Certificate resource and recreate it
`,
		},
		{
			Title:    "etcd Cluster Degraded",
			Category: strPtr("kubernetes"),
			Tags:     []string{"k8s", "etcd", "control-plane"},
			Content: `# etcd Cluster Degraded

## Symptoms
- API server responds slowly or intermittently
- etcd health checks fail
- "etcdserver: request timed out" in API server logs
- Cluster operations (kubectl) are slow

## Diagnosis

### 1. Check etcd member health
` + "```bash" + `
kubectl -n kube-system exec -it etcd-<node> -- etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key \
  endpoint health --cluster
` + "```" + `

### 2. Check etcd member list
` + "```bash" + `
etcdctl member list --write-out=table
` + "```" + `

### 3. Check disk performance
` + "```bash" + `
# etcd requires low-latency disk I/O (< 10ms for fsync)
kubectl -n kube-system exec -it etcd-<node> -- etcdctl \
  --endpoints=https://127.0.0.1:2379 \
  endpoint status --write-out=table
` + "```" + `

## Common Causes
1. **Disk I/O latency** — etcd requires fast fsync (SSD recommended)
2. **Network partition** — etcd members cannot communicate
3. **Disk space exhaustion** — etcd data directory is full
4. **Too many revisions** — compaction not running, DB size growing

## Resolution
1. Move etcd to SSD-backed storage
2. Fix network connectivity between control plane nodes
3. Compact and defragment etcd: ` + "`etcdctl compact` + `etcdctl defrag`" + `
4. Increase etcd quota if DB size exceeded: --quota-backend-bytes
`,
		},
		{
			Title:    "Node NotReady",
			Category: strPtr("kubernetes"),
			Tags:     []string{"k8s", "node", "not-ready"},
			Content: `# Node NotReady

## Symptoms
- Node status shows "NotReady"
- Pods on the node are evicted or stuck in "Terminating"
- kubectl get nodes shows the node as unavailable

## Diagnosis

### 1. Check node status and conditions
` + "```bash" + `
kubectl describe node <node-name>
kubectl get node <node-name> -o jsonpath='{.status.conditions}' | python3 -m json.tool
` + "```" + `

### 2. Check kubelet status
` + "```bash" + `
# SSH to the node
systemctl status kubelet
journalctl -u kubelet --since "10 minutes ago" --no-pager
` + "```" + `

### 3. Check system resources
` + "```bash" + `
# On the node
df -h          # disk space
free -m        # memory
top            # CPU and process list
` + "```" + `

### 4. Check container runtime
` + "```bash" + `
systemctl status containerd  # or docker
crictl ps
crictl info
` + "```" + `

## Common Causes
1. **Kubelet crashed** — OOM, certificate expired, or config error
2. **Disk pressure** — node disk usage exceeded eviction threshold (85%)
3. **Memory pressure** — node memory usage exceeded threshold
4. **Network issue** — node cannot reach API server
5. **Container runtime failure** — containerd/docker daemon crashed

## Resolution
1. Restart kubelet: ` + "`systemctl restart kubelet`" + `
2. Clean up disk space: remove unused images, logs, temp files
3. Increase node resources or add more nodes
4. Fix network connectivity to control plane
5. Restart container runtime: ` + "`systemctl restart containerd`" + `
`,
		},
		{
			Title:    "PVC Stuck in Pending",
			Category: strPtr("kubernetes"),
			Tags:     []string{"k8s", "pvc", "storage"},
			Content: `# PVC Stuck in Pending

## Symptoms
- PersistentVolumeClaim status is "Pending"
- Pods using the PVC are stuck in "Pending"
- Events show "waiting for a volume to be created"

## Diagnosis

### 1. Check PVC status and events
` + "```bash" + `
kubectl describe pvc <pvc-name> -n <namespace>
kubectl get events -n <namespace> --sort-by='.lastTimestamp' | grep <pvc-name>
` + "```" + `

### 2. Check StorageClass
` + "```bash" + `
kubectl get storageclass
kubectl describe storageclass <storage-class-name>
` + "```" + `

### 3. Check CSI driver
` + "```bash" + `
kubectl get csidrivers
kubectl get pods -n kube-system | grep csi
kubectl logs <csi-controller-pod> -n kube-system
` + "```" + `

### 4. Check available PVs (for static provisioning)
` + "```bash" + `
kubectl get pv
kubectl get pv -o wide
` + "```" + `

## Common Causes
1. **No StorageClass** — default StorageClass not set or does not exist
2. **CSI driver not running** — provisioner pods are crashed or missing
3. **Quota exceeded** — cloud provider storage quota exhausted
4. **Zone mismatch** — PV in different zone than the requesting pod
5. **Access mode mismatch** — PV does not support required access mode (RWO vs RWX)

## Resolution
1. Create or set a default StorageClass
2. Fix CSI driver deployment (check RBAC, image pull, node affinity)
3. Request quota increase from cloud provider
4. Add topology constraints or provision storage in the correct zone
5. Use a storage solution that supports the required access mode
`,
		},
		{
			Title:    "DNS Resolution Failure",
			Category: strPtr("kubernetes"),
			Tags:     []string{"k8s", "dns", "coredns"},
			Content: `# DNS Resolution Failure

## Symptoms
- Pods cannot resolve service names or external domains
- "Could not resolve host" errors in application logs
- nslookup/dig commands fail from within pods

## Diagnosis

### 1. Test DNS from a debug pod
` + "```bash" + `
kubectl run dns-test --image=busybox:1.36 --rm -it --restart=Never -- nslookup kubernetes.default
kubectl run dns-test --image=busybox:1.36 --rm -it --restart=Never -- nslookup google.com
` + "```" + `

### 2. Check CoreDNS pods
` + "```bash" + `
kubectl get pods -n kube-system -l k8s-app=kube-dns
kubectl logs -n kube-system -l k8s-app=kube-dns --tail=50
` + "```" + `

### 3. Check CoreDNS ConfigMap
` + "```bash" + `
kubectl get configmap coredns -n kube-system -o yaml
` + "```" + `

### 4. Check DNS service
` + "```bash" + `
kubectl get svc kube-dns -n kube-system
kubectl get endpoints kube-dns -n kube-system
` + "```" + `

### 5. Check pod DNS config
` + "```bash" + `
kubectl exec <pod-name> -n <namespace> -- cat /etc/resolv.conf
` + "```" + `

## Common Causes
1. **CoreDNS pods crashed** — OOM, config error, or resource starvation
2. **CoreDNS ConfigMap error** — syntax error in Corefile
3. **Network policy blocking DNS** — port 53 UDP/TCP not allowed
4. **Upstream DNS unreachable** — node DNS configuration broken
5. **ndots too high** — excessive DNS lookups before resolving

## Resolution
1. Restart CoreDNS: ` + "`kubectl rollout restart deployment coredns -n kube-system`" + `
2. Fix CoreDNS ConfigMap syntax errors
3. Add NetworkPolicy allowing egress to kube-dns service on port 53
4. Fix upstream DNS in node /etc/resolv.conf
5. Set ndots:2 in pod dnsConfig or use FQDN with trailing dot
`,
		},
	}
}
