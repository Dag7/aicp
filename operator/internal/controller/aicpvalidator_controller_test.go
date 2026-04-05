/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	aicpv1alpha1 "github.com/aicp-protocol/aicp/operator/api/v1alpha1"
)

var _ = Describe("AICPValidator Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"
		const clusterName = "test-validator-cluster"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		clusterNamespacedName := types.NamespacedName{
			Name:      clusterName,
			Namespace: "default",
		}
		aicpvalidator := &aicpv1alpha1.AICPValidator{}

		BeforeEach(func() {
			By("creating the parent AICPCluster")
			cluster := &aicpv1alpha1.AICPCluster{}
			err := k8sClient.Get(ctx, clusterNamespacedName, cluster)
			if err != nil && errors.IsNotFound(err) {
				cluster = &aicpv1alpha1.AICPCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      clusterName,
						Namespace: "default",
					},
					Spec: aicpv1alpha1.AICPClusterSpec{
						Replicas:       1,
						ValidatorCount: 1,
						Image:          "ghcr.io/aicp-protocol/aicp-server:test",
						PostgreSQL: aicpv1alpha1.PostgreSQLConfig{
							Host: "postgres.default.svc",
							Port: 5432,
							SecretRef: aicpv1alpha1.SecretReference{
								Name: "test-pg-secret",
							},
						},
						Redis: aicpv1alpha1.RedisConfig{
							Address: "redis.default.svc:6379",
						},
					},
				}
				Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
			}

			By("creating the custom resource for the Kind AICPValidator")
			err = k8sClient.Get(ctx, typeNamespacedName, aicpvalidator)
			if err != nil && errors.IsNotFound(err) {
				resource := &aicpv1alpha1.AICPValidator{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: aicpv1alpha1.AICPValidatorSpec{
						ClusterRef: clusterName,
						KeySecretRef: aicpv1alpha1.SecretReference{
							Name: "test-validator-key",
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &aicpv1alpha1.AICPValidator{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance AICPValidator")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			cluster := &aicpv1alpha1.AICPCluster{}
			err = k8sClient.Get(ctx, clusterNamespacedName, cluster)
			if err == nil {
				By("Cleanup the parent AICPCluster")
				Expect(k8sClient.Delete(ctx, cluster)).To(Succeed())
			}
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &AICPValidatorReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
