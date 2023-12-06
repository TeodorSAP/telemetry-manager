//go:build e2e

package e2e

import (
	"net/http"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitlog "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"

	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Overrides", Label("logging", "custom"), Ordered, func() {
	const (
		mockBackendName = "overrides-receiver"
		mockNs          = "overrides-log-http-output"
		pipelineName    = "http-output-pipeline"
	)
	var telemetryExportURL string

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeLogs, backend.WithPersistentHostSecret(isOperational()))
		objs = append(objs, mockBackend.K8sObjects()...)
		telemetryExportURL = mockBackend.TelemetryExportURL(proxyClient)
		namespaces := []string{kitkyma.SystemNamespaceName}

		logPipeline := kitlog.NewPipeline(pipelineName).
			WithIncludeNamespaces(namespaces).
			WithSecretKeyRef(mockBackend.HostSecretRef()).
			WithHTTPOutput().
			Persistent(isOperational())
		objs = append(objs, logPipeline.K8sObject())

		return objs
	}

	Context("Before deploying a logpipeline", func() {
		It("Should have a healthy webhook", func() {
			verifiers.WebhookShouldBeHealthy(ctx, k8sClient)
		})
	})

	Context("When a logpipeline with HTTP output exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()
			// DeferCleanup(func() {
			// 	Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			// })
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running logpipeline", Label(operationalTest), func() {
			verifiers.LogPipelineShouldBeRunning(ctx, k8sClient, pipelineName)
		})

		It("Should have a log backend running", Label(operationalTest), func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: mockBackendName})
		})

		It("Should have telemetry-manager logs in the backend", Label(operationalTest), func() {
			// verifiers.LogsShouldBeDelivered(proxyClient, "telemetry-operator", telemetryExportURL)
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					ContainLd(ContainLogRecord(WithPodName(ContainSubstring("telemetry-operator")))),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})