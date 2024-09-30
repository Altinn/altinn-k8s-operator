package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resourcesv1alpha1 "github.com/altinn/altinn-k8s-operator/api/v1alpha1"
	"github.com/altinn/altinn-k8s-operator/internal"
)

var _ = Describe("MaskinportenClient Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "local-testapp"
		const secretName = "local-testapp-deployment-secrets"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		typeNamespacedSecretName := types.NamespacedName{
			Name:      secretName,
			Namespace: "default",
		}
		maskinportenclient := &resourcesv1alpha1.MaskinportenClient{}
		secret := &corev1.Secret{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind MaskinportenClient")
			err := k8sClient.Get(ctx, typeNamespacedName, maskinportenclient)
			if err != nil && errors.IsNotFound(err) {
				resource := &resourcesv1alpha1.MaskinportenClient{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
						Labels: map[string]string{
							"app": "local-testapp-deployment",
						},
					},
					Spec: resourcesv1alpha1.MaskinportenClientSpec{
						Scopes: []string{"altinn:resourceregistry/resource.read"},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			err = k8sClient.Get(ctx, typeNamespacedSecretName, secret)
			if err != nil && errors.IsNotFound(err) {
				f := false
				resource := &corev1.Secret{
					Immutable: &f,
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretName,
						Namespace: "default",
						Labels: map[string]string{
							"app": "local-testapp-deployment",
						},
					},
					Type: corev1.SecretTypeOpaque,
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			{
				resource := &resourcesv1alpha1.MaskinportenClient{}
				err := k8sClient.Get(ctx, typeNamespacedName, resource)
				Expect(err).NotTo(HaveOccurred())
				By("Cleanup the specific resource instance MaskinportenClient")
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
			{
				resource := &corev1.Secret{}
				err := k8sClient.Get(ctx, typeNamespacedSecretName, resource)
				Expect(err).NotTo(HaveOccurred())
				By("Cleanup the specific resource instance secret")
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			rt, err := internal.NewRuntime(context.Background(), "")
			Expect(err).NotTo(HaveOccurred())
			controllerReconciler := NewMaskinportenClientReconciler(
				rt,
				k8sClient,
				k8sClient.Scheme(),
				nil,
			)

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			resource := &resourcesv1alpha1.MaskinportenClient{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.State).To(Equal("reconciled"))
		})
	})
})
