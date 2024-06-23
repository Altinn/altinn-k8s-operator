package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	resourcesv1alpha1 "github.com/altinn/altinn-k8s-operator/api/v1alpha1"
	"github.com/altinn/altinn-k8s-operator/test/utils"
)

const namespace = "altinn-k8s-operator-system"

var _ = Describe("controller", Ordered, func() {
	BeforeAll(func() {
		By("installing kind cluster")
		Expect(utils.StartKindCluster()).To(Succeed())

		By("installing test app")
		Expect(utils.StartTestApp()).To(Succeed())

		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, err := utils.Run(cmd, "")
		Expect(err).To(Succeed())
	})

	// AfterAll(func() {
	// 	By("uninstalling kind cluster")
	// 	Expect(utils.Destroy()).To(Succeed())
	// })

	Context("Operator", func() {
		It("should run successfully", func() {
			var controllerPodName string
			var err error

			// projectimage stores the name of the image used in the example
			var projectimage = "example.com/altinn-k8s-operator:v0.0.1"

			By("building the manager(Operator) image")
			cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", projectimage))
			_, err = utils.Run(cmd, "")
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("loading the the manager(Operator) image on Kind")
			err = utils.LoadImageToKindClusterWithName(projectimage)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("deploying the controller-manager")
			cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectimage))
			_, err = utils.Run(cmd, "")
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func() error {
				// Get pod name

				cmd = exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd, "")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				podNames := utils.GetNonEmptyLines(string(podOutput))
				if len(podNames) != 1 {
					return fmt.Errorf("expect 1 controller pods running, but got %d", len(podNames))
				}
				controllerPodName = podNames[0]
				ExpectWithOffset(2, controllerPodName).Should(ContainSubstring("controller-manager"))

				// Validate pod status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				status, err := utils.Run(cmd, "")
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				if string(status) != "Running" {
					return fmt.Errorf("controller pod in %s status", status)
				}
				return nil
			}
			EventuallyWithOffset(1, verifyControllerUp, time.Minute, time.Second).Should(Succeed())
		})

		It("should respond to MaskinportenClient", func() {

			By("creating MaskinportenClient resource")
			Expect(utils.ApplyMaskinportenClient()).To(Succeed())

			By("constructing k8s client")
			k8sClient, err := utils.GetK8sClient()
			Expect(err).To(Succeed())

			By("validating that the corresponding status is updated after reconcile")
			verifyStatusUpdated := func() error {
				maskinportenClient := &resourcesv1alpha1.MaskinportenClient{}
				err := k8sClient.Get().
					Namespace("").
					Resource("local-testapp").
					Do(context.Background()).
					Into(maskinportenClient)

				if err != nil {
					return err
				}

				state := maskinportenClient.Status.State
				if state != "reconciled" {
					return fmt.Errorf("MaskinportenClient resource in %s status", state)
				}

				return nil
			}
			EventuallyWithOffset(
				1,
				verifyStatusUpdated,
			).WithTimeout(time.Second * 5).
				WithPolling(time.Second).
				Should(Succeed())
		})
	})
})
