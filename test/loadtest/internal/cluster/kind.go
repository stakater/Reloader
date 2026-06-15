// Package cluster provides kind cluster management functionality.
package cluster

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Config holds configuration for kind cluster operations.
type Config struct {
	Name             string
	ContainerRuntime string // "docker" or "podman"
	PortOffset       int    // Offset for host port mappings (for parallel clusters)
}

// Manager handles kind cluster operations.
type Manager struct {
	cfg Config
}

// NewManager creates a new cluster manager.
func NewManager(cfg Config) *Manager {
	return &Manager{cfg: cfg}
}

// DetectContainerRuntime finds available container runtime.
// It checks if the runtime daemon is actually running, not just if the binary exists.
func DetectContainerRuntime() (string, error) {
	if _, err := exec.LookPath("docker"); err == nil {
		cmd := exec.Command("docker", "info")
		if err := cmd.Run(); err == nil {
			return "docker", nil
		}
	}
	if _, err := exec.LookPath("podman"); err == nil {
		cmd := exec.Command("podman", "info")
		if err := cmd.Run(); err == nil {
			return "podman", nil
		}
	}
	return "", fmt.Errorf("neither docker nor podman is running")
}

// Exists checks if the cluster already exists.
func (m *Manager) Exists() bool {
	cmd := exec.Command("kind", "get", "clusters")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == m.cfg.Name {
			return true
		}
	}
	return false
}

// Delete deletes the kind cluster.
func (m *Manager) Delete(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "kind", "delete", "cluster", "--name", m.cfg.Name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Create creates a new kind cluster with optimized settings.
func (m *Manager) Create(ctx context.Context) error {
	if m.cfg.ContainerRuntime == "podman" {
		os.Setenv("KIND_EXPERIMENTAL_PROVIDER", "podman")
	}

	if m.Exists() {
		fmt.Printf("Cluster %s already exists, deleting...\n", m.cfg.Name)
		if err := m.Delete(ctx); err != nil {
			return fmt.Errorf("deleting existing cluster: %w", err)
		}
	}

	httpPort := 8080 + m.cfg.PortOffset
	httpsPort := 8443 + m.cfg.PortOffset

	config := fmt.Sprintf(`kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  podSubnet: "10.244.0.0/16"
  serviceSubnet: "10.96.0.0/16"
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
        kube-api-qps: "50"
        kube-api-burst: "100"
        serialize-image-pulls: "false"
        event-qps: "50"
        event-burst: "100"
  - |
    kind: ClusterConfiguration
    apiServer:
      extraArgs:
        max-requests-inflight: "800"
        max-mutating-requests-inflight: "400"
        watch-cache-sizes: "configmaps#1000,secrets#1000,pods#1000"
    controllerManager:
      extraArgs:
        kube-api-qps: "200"
        kube-api-burst: "200"
    scheduler:
      extraArgs:
        kube-api-qps: "200"
        kube-api-burst: "200"
  extraPortMappings:
  - containerPort: 80
    hostPort: %d
    protocol: TCP
  - containerPort: 443
    hostPort: %d
    protocol: TCP
- role: worker
  kubeadmConfigPatches:
  - |
    kind: JoinConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        max-pods: "250"
        kube-api-qps: "50"
        kube-api-burst: "100"
        serialize-image-pulls: "false"
        event-qps: "50"
        event-burst: "100"
- role: worker
  kubeadmConfigPatches:
  - |
    kind: JoinConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        max-pods: "250"
        kube-api-qps: "50"
        kube-api-burst: "100"
        serialize-image-pulls: "false"
        event-qps: "50"
        event-burst: "100"
- role: worker
  kubeadmConfigPatches:
  - |
    kind: JoinConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        max-pods: "250"
        kube-api-qps: "50"
        kube-api-burst: "100"
        serialize-image-pulls: "false"
        event-qps: "50"
        event-burst: "100"
- role: worker
  kubeadmConfigPatches:
  - |
    kind: JoinConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        max-pods: "250"
        kube-api-qps: "50"
        kube-api-burst: "100"
        serialize-image-pulls: "false"
        event-qps: "50"
        event-burst: "100"
- role: worker
  kubeadmConfigPatches:
  - |
    kind: JoinConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        max-pods: "250"
        kube-api-qps: "50"
        kube-api-burst: "100"
        serialize-image-pulls: "false"
        event-qps: "50"
        event-burst: "100"
- role: worker
  kubeadmConfigPatches:
  - |
    kind: JoinConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        max-pods: "250"
        kube-api-qps: "50"
        kube-api-burst: "100"
        serialize-image-pulls: "false"
        event-qps: "50"
        event-burst: "100"
`, httpPort, httpsPort)
	cmd := exec.CommandContext(ctx, "kind", "create", "cluster", "--name", m.cfg.Name, "--config=-")
	cmd.Stdin = strings.NewReader(config)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// GetKubeconfig returns the kubeconfig for the cluster.
func (m *Manager) GetKubeconfig() (string, error) {
	cmd := exec.Command("kind", "get", "kubeconfig", "--name", m.cfg.Name)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("getting kubeconfig: %w", err)
	}
	return string(out), nil
}

// Context returns the kubectl context name for this cluster.
func (m *Manager) Context() string {
	return "kind-" + m.cfg.Name
}

// Name returns the cluster name.
func (m *Manager) Name() string {
	return m.cfg.Name
}

// LoadImage loads a container image into the kind cluster.
func (m *Manager) LoadImage(ctx context.Context, image string) error {
	if !m.imageExistsLocally(image) {
		fmt.Printf("  Image not found locally, pulling: %s\n", image)
		pullCmd := exec.CommandContext(ctx, m.cfg.ContainerRuntime, "pull", image)
		pullCmd.Stdout = os.Stdout
		pullCmd.Stderr = os.Stderr
		if err := pullCmd.Run(); err != nil {
			return fmt.Errorf("pulling image %s: %w", image, err)
		}
	} else {
		fmt.Printf("  Image found locally: %s\n", image)
	}

	fmt.Printf("  Copying image to kind cluster...\n")

	if m.cfg.ContainerRuntime == "podman" {
		tmpFile := fmt.Sprintf("/tmp/kind-image-%d.tar", time.Now().UnixNano())
		defer os.Remove(tmpFile)

		saveCmd := exec.CommandContext(ctx, m.cfg.ContainerRuntime, "save", image, "-o", tmpFile)
		if err := saveCmd.Run(); err != nil {
			return fmt.Errorf("saving image %s: %w", image, err)
		}

		loadCmd := exec.CommandContext(ctx, "kind", "load", "image-archive", tmpFile, "--name", m.cfg.Name)
		loadCmd.Stdout = os.Stdout
		loadCmd.Stderr = os.Stderr
		if err := loadCmd.Run(); err != nil {
			return fmt.Errorf("loading image archive: %w", err)
		}
	} else {
		loadCmd := exec.CommandContext(ctx, "kind", "load", "docker-image", image, "--name", m.cfg.Name)
		loadCmd.Stdout = os.Stdout
		loadCmd.Stderr = os.Stderr
		if err := loadCmd.Run(); err != nil {
			return fmt.Errorf("loading image %s: %w", image, err)
		}
	}

	return nil
}

// imageExistsLocally checks if an image exists in the local container runtime.
func (m *Manager) imageExistsLocally(image string) bool {
	cmd := exec.Command(m.cfg.ContainerRuntime, "image", "exists", image)
	if err := cmd.Run(); err == nil {
		return true
	}

	cmd = exec.Command(m.cfg.ContainerRuntime, "image", "inspect", image)
	if err := cmd.Run(); err == nil {
		return true
	}

	cmd = exec.Command(m.cfg.ContainerRuntime, "images", "--format", "{{.Repository}}:{{.Tag}}")
	out, err := cmd.Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.TrimSpace(line) == image {
				return true
			}
		}
	}

	return false
}

// PullImage pulls an image using the container runtime.
func (m *Manager) PullImage(ctx context.Context, image string) error {
	cmd := exec.CommandContext(ctx, m.cfg.ContainerRuntime, "pull", image)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ExecKubectl runs a kubectl command against the cluster.
func (m *Manager) ExecKubectl(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w: %s", err, stderr.String())
	}
	return stdout.Bytes(), nil
}
