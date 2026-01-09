package reloader

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Config holds configuration for a Reloader deployment.
type Config struct {
	Version        string
	Image          string
	Namespace      string
	ReloadStrategy string
}

// Manager handles Reloader deployment operations.
type Manager struct {
	config      Config
	kubeContext string
}

// NewManager creates a new Reloader manager.
func NewManager(config Config) *Manager {
	return &Manager{
		config: config,
	}
}

// SetKubeContext sets the kubeconfig context to use.
func (m *Manager) SetKubeContext(kubeContext string) {
	m.kubeContext = kubeContext
}

// kubectl returns kubectl command with optional context.
func (m *Manager) kubectl(ctx context.Context, args ...string) *exec.Cmd {
	if m.kubeContext != "" {
		args = append([]string{"--context", m.kubeContext}, args...)
	}
	return exec.CommandContext(ctx, "kubectl", args...)
}

// namespace returns the namespace for this reloader instance.
func (m *Manager) namespace() string {
	if m.config.Namespace != "" {
		return m.config.Namespace
	}
	return fmt.Sprintf("reloader-%s", m.config.Version)
}

// releaseName returns the release name for this instance.
func (m *Manager) releaseName() string {
	return fmt.Sprintf("reloader-%s", m.config.Version)
}

// Job returns the Prometheus job name for this Reloader instance.
func (m *Manager) Job() string {
	return fmt.Sprintf("reloader-%s", m.config.Version)
}

// Deploy deploys Reloader to the cluster using raw manifests.
func (m *Manager) Deploy(ctx context.Context) error {
	ns := m.namespace()
	name := m.releaseName()

	fmt.Printf("Deploying Reloader (%s) with image %s...\n", m.config.Version, m.config.Image)

	manifest := m.buildManifest(ns, name)

	applyCmd := m.kubectl(ctx, "apply", "-f", "-")
	applyCmd.Stdin = strings.NewReader(manifest)
	applyCmd.Stdout = os.Stdout
	applyCmd.Stderr = os.Stderr
	if err := applyCmd.Run(); err != nil {
		return fmt.Errorf("applying manifest: %w", err)
	}

	fmt.Printf("Waiting for Reloader deployment to be ready...\n")
	waitCmd := m.kubectl(ctx, "rollout", "status", "deployment", name,
		"-n", ns,
		"--timeout=120s")
	waitCmd.Stdout = os.Stdout
	waitCmd.Stderr = os.Stderr
	if err := waitCmd.Run(); err != nil {
		return fmt.Errorf("waiting for deployment: %w", err)
	}

	time.Sleep(2 * time.Second)

	fmt.Printf("Reloader (%s) deployed successfully\n", m.config.Version)
	return nil
}

// buildManifest creates the raw Kubernetes manifest for Reloader.
func (m *Manager) buildManifest(ns, name string) string {
	var args []string
	args = append(args, "--log-format=json")
	if m.config.ReloadStrategy != "" && m.config.ReloadStrategy != "default" {
		args = append(args, fmt.Sprintf("--reload-strategy=%s", m.config.ReloadStrategy))
	}

	argsYAML := ""
	if len(args) > 0 {
		argsYAML = "        args:\n"
		for _, arg := range args {
			argsYAML += fmt.Sprintf("        - %q\n", arg)
		}
	}

	return fmt.Sprintf(`---
apiVersion: v1
kind: Namespace
metadata:
  name: %[1]s
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: %[2]s
  namespace: %[1]s
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: %[2]s
rules:
- apiGroups: ["*"]
  resources: ["*"]
  verbs: ["*"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: %[2]s
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: %[2]s
subjects:
- kind: ServiceAccount
  name: %[2]s
  namespace: %[1]s
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: %[2]s
  namespace: %[1]s
  labels:
    app: %[2]s
    app.kubernetes.io/name: reloader
    loadtest-version: %[3]s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: %[2]s
  template:
    metadata:
      labels:
        app: %[2]s
        app.kubernetes.io/name: reloader
        loadtest-version: %[3]s
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9090"
        prometheus.io/path: "/metrics"
    spec:
      serviceAccountName: %[2]s
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
      containers:
      - name: reloader
        image: %[4]s
        imagePullPolicy: IfNotPresent
        ports:
        - name: http
          containerPort: 9090
%[5]s        resources:
          requests:
            cpu: 10m
            memory: 64Mi
          limits:
            cpu: 500m
            memory: 256Mi
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
`, ns, name, m.config.Version, m.config.Image, argsYAML)
}

// Cleanup removes all Reloader resources from the cluster.
func (m *Manager) Cleanup(ctx context.Context) error {
	ns := m.namespace()
	name := m.releaseName()

	delDeploy := m.kubectl(ctx, "delete", "deployment", name, "-n", ns, "--ignore-not-found")
	delDeploy.Run()

	delCRB := m.kubectl(ctx, "delete", "clusterrolebinding", name, "--ignore-not-found")
	delCRB.Run()

	delCR := m.kubectl(ctx, "delete", "clusterrole", name, "--ignore-not-found")
	delCR.Run()

	delNS := m.kubectl(ctx, "delete", "namespace", ns, "--wait=false", "--ignore-not-found")
	if err := delNS.Run(); err != nil {
		return fmt.Errorf("deleting namespace: %w", err)
	}

	return nil
}

// CleanupByVersion removes Reloader resources for a specific version without needing a Manager instance.
// This is useful for cleaning up from previous runs before creating a new Manager.
func CleanupByVersion(ctx context.Context, version, kubeContext string) {
	ns := fmt.Sprintf("reloader-%s", version)
	name := fmt.Sprintf("reloader-%s", version)

	nsArgs := []string{"delete", "namespace", ns, "--wait=false", "--ignore-not-found"}
	crArgs := []string{"delete", "clusterrole", name, "--ignore-not-found"}
	crbArgs := []string{"delete", "clusterrolebinding", name, "--ignore-not-found"}

	if kubeContext != "" {
		nsArgs = append([]string{"--context", kubeContext}, nsArgs...)
		crArgs = append([]string{"--context", kubeContext}, crArgs...)
		crbArgs = append([]string{"--context", kubeContext}, crbArgs...)
	}

	exec.CommandContext(ctx, "kubectl", nsArgs...).Run()
	exec.CommandContext(ctx, "kubectl", crArgs...).Run()
	exec.CommandContext(ctx, "kubectl", crbArgs...).Run()
}

// CollectLogs collects logs from the Reloader pod and writes them to the specified file.
func (m *Manager) CollectLogs(ctx context.Context, logPath string) error {
	ns := m.namespace()
	name := m.releaseName()

	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return fmt.Errorf("creating log directory: %w", err)
	}

	cmd := m.kubectl(ctx, "logs",
		"-n", ns,
		"-l", fmt.Sprintf("app=%s", name),
		"--tail=-1")

	out, err := cmd.Output()
	if err != nil {
		cmd = m.kubectl(ctx, "logs",
			"-n", ns,
			"-l", "app.kubernetes.io/name=reloader",
			"--tail=-1")
		out, err = cmd.Output()
		if err != nil {
			return fmt.Errorf("collecting logs: %w", err)
		}
	}

	if err := os.WriteFile(logPath, out, 0644); err != nil {
		return fmt.Errorf("writing logs: %w", err)
	}

	return nil
}
