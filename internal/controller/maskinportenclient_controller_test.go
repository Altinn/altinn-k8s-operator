package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resourcesv1alpha1 "github.com/altinn/altinn-k8s-operator/api/v1alpha1"
	"github.com/altinn/altinn-k8s-operator/internal"
)

var _ = Describe("MaskinportenClient Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		maskinportenclient := &resourcesv1alpha1.MaskinportenClient{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind MaskinportenClient")
			err := k8sClient.Get(ctx, typeNamespacedName, maskinportenclient)
			if err != nil && errors.IsNotFound(err) {
				resource := &resourcesv1alpha1.MaskinportenClient{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					// TODO(user): Specify other spec details if needed.
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &resourcesv1alpha1.MaskinportenClient{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance MaskinportenClient")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			rt, err := internal.NewRuntime(context.Background())
			Expect(err).NotTo(HaveOccurred())
			controllerReconciler := NewMaskinportenClientReconciler(
				rt,
				k8sClient,
				k8sClient.Scheme(),
			)

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).To(HaveOccurred())

			resource := &resourcesv1alpha1.MaskinportenClient{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.State).To(Equal("error"))
		})
	})
})
