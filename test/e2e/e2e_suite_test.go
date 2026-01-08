package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stakater/Reloader/test/e2e/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeClient kubernetes.Interface
	projectDir string
	testImage  string
	ctx        context.Context
	cancel     context.CancelFunc
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Reloader E2E Suite")
}

var _ = BeforeSuite(func() {
	var err error
	ctx, cancel = context.WithCancel(context.Background())

	// Get project directory
	projectDir, err = utils.GetProjectDir()
	Expect(err).NotTo(HaveOccurred(), "Failed to get project directory")

	// Get test image from environment or use default
	testImage = utils.GetTestImage()

	GinkgoWriter.Printf("Using test image: %s\n", testImage)
	GinkgoWriter.Printf("Project directory: %s\n", projectDir)

	// Build image if SKIP_BUILD is not set
	if os.Getenv("SKIP_BUILD") != "true" {
		GinkgoWriter.Println("Building Docker image...")
		cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", testImage))
		output, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to build Docker image: %s", output)
		GinkgoWriter.Println("Docker image built successfully")
	} else {
		GinkgoWriter.Println("Skipping Docker build (SKIP_BUILD=true)")
	}

	// Load image to Kind cluster
	GinkgoWriter.Println("Loading image to Kind cluster...")
	err = utils.LoadImageToKindCluster(testImage)
	Expect(err).NotTo(HaveOccurred(), "Failed to load image to Kind cluster")
	GinkgoWriter.Println("Image loaded to Kind cluster successfully")

	// Setup Kubernetes client
	kubeconfig := utils.GetKubeconfig()
	GinkgoWriter.Printf("Using kubeconfig: %s\n", kubeconfig)

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	Expect(err).NotTo(HaveOccurred(), "Failed to build config from kubeconfig")

	kubeClient, err = kubernetes.NewForConfig(config)
	Expect(err).NotTo(HaveOccurred(), "Failed to create Kubernetes client")

	// Verify cluster connectivity
	GinkgoWriter.Println("Verifying cluster connectivity...")
	_, err = kubeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
	Expect(err).NotTo(HaveOccurred(), "Failed to connect to Kubernetes cluster")
	GinkgoWriter.Println("Cluster connectivity verified")
})

var _ = AfterSuite(func() {
	if cancel != nil {
		cancel()
	}
	GinkgoWriter.Println("E2E Suite cleanup complete")
})
