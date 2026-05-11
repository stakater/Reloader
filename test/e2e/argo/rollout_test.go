package argo

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stakater/Reloader/test/e2e/utils"
)

// Note: Basic Argo Rollout reload tests (ConfigMap, Secret, auto=true, volume mounts, label-only negative)
// are covered by core/workloads_test.go with Label("argo").
// This file contains only Argo-specific tests that cannot be parameterized.

var _ = Describe("Argo Rollout Strategy Tests", func() {
	var (
		rolloutName   string
		configMapName string
		adapter       *utils.ArgoRolloutAdapter
	)

	BeforeEach(func() {
		rolloutName = utils.RandName("rollout")
		configMapName = utils.RandName("cm")
		adapter = utils.NewArgoRolloutAdapter(rolloutsClient)
	})

	AfterEach(func() {
		_ = utils.DeleteRollout(ctx, rolloutsClient, testNamespace, rolloutName)
		_ = utils.DeleteConfigMap(ctx, kubeClient, testNamespace, configMapName)
	})

	Context("Rollout strategy annotation", func() {
		It("should use default rollout strategy (annotation-based reload)", func() {
			By("Creating a ConfigMap")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating an Argo Rollout with auto=true (default strategy)")
			_, err = utils.CreateRollout(ctx, rolloutsClient, testNamespace, rolloutName,
				utils.WithRolloutConfigMapEnvFrom(configMapName),
				utils.WithRolloutAnnotations(utils.BuildAutoTrueAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Rollout to be ready")
			err = adapter.WaitReady(ctx, testNamespace, rolloutName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Rollout to be reloaded with annotation")
			reloaded, err := adapter.WaitReloaded(ctx, testNamespace, rolloutName,
				utils.AnnotationLastReloadedFrom, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded).To(BeTrue(), "Argo Rollout should be reloaded with default rollout strategy")
		})

		It("should use restart strategy when specified (sets restartAt field)", func() {
			By("Creating a ConfigMap")
			_, err := utils.CreateConfigMap(ctx, kubeClient, testNamespace, configMapName,
				map[string]string{"key": "initial"}, nil)
			Expect(err).NotTo(HaveOccurred())

			By("Creating an Argo Rollout with restart strategy annotation")
			_, err = utils.CreateRollout(ctx, rolloutsClient, testNamespace, rolloutName,
				utils.WithRolloutConfigMapEnvFrom(configMapName),
				utils.WithRolloutAnnotations(utils.BuildAutoTrueAnnotation()),
				utils.WithRolloutObjectAnnotations(utils.BuildRolloutRestartStrategyAnnotation()),
			)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Rollout to be ready")
			err = adapter.WaitReady(ctx, testNamespace, rolloutName, utils.WorkloadReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			By("Updating the ConfigMap")
			err = utils.UpdateConfigMap(ctx, kubeClient, testNamespace, configMapName, map[string]string{"key": "updated"})
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Rollout to have restartAt field set")
			restarted, err := adapter.WaitRestartAt(ctx, testNamespace, rolloutName, utils.ReloadTimeout)
			Expect(err).NotTo(HaveOccurred())
			Expect(restarted).To(BeTrue(), "Argo Rollout should have restartAt field set with restart strategy")
		})
	})
})
