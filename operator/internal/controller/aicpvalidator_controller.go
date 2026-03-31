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
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	aicpv1alpha1 "github.com/aicp-protocol/aicp/operator/api/v1alpha1"
)

// AICPValidatorReconciler reconciles an AICPValidator object.
type AICPValidatorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=aicp.aicp.dev,resources=aicpvalidators,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=aicp.aicp.dev,resources=aicpvalidators/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=aicp.aicp.dev,resources=aicpvalidators/finalizers,verbs=update
// +kubebuilder:rbac:groups=aicp.aicp.dev,resources=aicpclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete

func (r *AICPValidatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	var validator aicpv1alpha1.AICPValidator
	if err := r.Get(ctx, req.NamespacedName, &validator); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling AICPValidator", "name", validator.Name)

	// Look up the parent cluster.
	var cluster aicpv1alpha1.AICPCluster
	if err := r.Get(ctx, types.NamespacedName{
		Name:      validator.Spec.ClusterRef,
		Namespace: validator.Namespace,
	}, &cluster); err != nil {
		return ctrl.Result{}, fmt.Errorf("looking up cluster %s: %w", validator.Spec.ClusterRef, err)
	}

	// Reconcile the StatefulSet.
	if err := r.reconcileStatefulSet(ctx, &validator, &cluster); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconciling statefulset: %w", err)
	}

	// Reconcile the headless Service.
	if err := r.reconcileHeadlessService(ctx, &validator); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconciling headless service: %w", err)
	}

	// Update status.
	validator.Status.Phase = "Active"
	validator.Status.Ready = true
	if err := r.Status().Update(ctx, &validator); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating status: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *AICPValidatorReconciler) reconcileStatefulSet(
	ctx context.Context,
	validator *aicpv1alpha1.AICPValidator,
	cluster *aicpv1alpha1.AICPCluster,
) error {
	name := validator.Name
	labels := map[string]string{
		"app.kubernetes.io/name":       "aicp-validator",
		"app.kubernetes.io/instance":   validator.Name,
		"app.kubernetes.io/managed-by": "aicp-operator",
		"aicp.dev/cluster":             validator.Spec.ClusterRef,
	}

	image := validator.Spec.Image
	if image == "" {
		image = cluster.Spec.Image
	}

	grpcPort := validator.Spec.GRPCPort
	if grpcPort == 0 {
		grpcPort = 9091
	}

	var replicas int32 = 1

	container := corev1.Container{
		Name:  "aicp-validator",
		Image: image,
		Args:  []string{"--mode=validator"},
		Ports: []corev1.ContainerPort{
			{Name: "grpc", ContainerPort: grpcPort, Protocol: corev1.ProtocolTCP},
			{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
		},
		Env: []corev1.EnvVar{
			{Name: "AICP_VALIDATOR_MODE", Value: "true"},
			{Name: "AICP_GRPC_PORT", Value: fmt.Sprintf("%d", grpcPort)},
			{Name: "AICP_VALIDATOR_KEY", ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: validator.Spec.KeySecretRef.Name},
					Key:                  "private-key",
				},
			}},
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/health",
					Port: intstr.FromString("http"),
				},
			},
			InitialDelaySeconds: 10,
			PeriodSeconds:       10,
		},
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: validator.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: name + "-headless",
			Replicas:    &replicas,
			Selector:    &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{container},
				},
			},
		},
	}

	// Add PVC for data storage if configured.
	if validator.Spec.Storage != nil {
		storageSize := validator.Spec.Storage.Size
		if storageSize == "" {
			storageSize = "10Gi"
		}

		pvcTemplate := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "data"},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse(storageSize),
					},
				},
			},
		}

		if validator.Spec.Storage.StorageClassName != "" {
			pvcTemplate.Spec.StorageClassName = &validator.Spec.Storage.StorageClassName
		}

		sts.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{pvcTemplate}

		container.VolumeMounts = []corev1.VolumeMount{
			{Name: "data", MountPath: "/data"},
		}
		sts.Spec.Template.Spec.Containers = []corev1.Container{container}
	}

	if err := ctrl.SetControllerReference(validator, sts, r.Scheme); err != nil {
		return err
	}

	var existing appsv1.StatefulSet
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: validator.Namespace}, &existing)
	if errors.IsNotFound(err) {
		return r.Create(ctx, sts)
	}
	if err != nil {
		return err
	}
	return r.Update(ctx, sts)
}

func (r *AICPValidatorReconciler) reconcileHeadlessService(
	ctx context.Context,
	validator *aicpv1alpha1.AICPValidator,
) error {
	name := validator.Name + "-headless"
	labels := map[string]string{
		"app.kubernetes.io/name":     "aicp-validator",
		"app.kubernetes.io/instance": validator.Name,
	}

	grpcPort := validator.Spec.GRPCPort
	if grpcPort == 0 {
		grpcPort = 9091
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: validator.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Selector:  labels,
			Ports: []corev1.ServicePort{
				{Name: "grpc", Port: grpcPort, TargetPort: intstr.FromString("grpc"), Protocol: corev1.ProtocolTCP},
			},
		},
	}

	if err := ctrl.SetControllerReference(validator, svc, r.Scheme); err != nil {
		return err
	}

	var existing corev1.Service
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: validator.Namespace}, &existing)
	if errors.IsNotFound(err) {
		return r.Create(ctx, svc)
	}
	if err != nil {
		return err
	}
	return r.Update(ctx, svc)
}

// SetupWithManager sets up the controller with the Manager.
func (r *AICPValidatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&aicpv1alpha1.AICPValidator{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Named("aicpvalidator").
		Complete(r)
}
