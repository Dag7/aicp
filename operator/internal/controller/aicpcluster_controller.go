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

// AICPClusterReconciler reconciles an AICPCluster object.
type AICPClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=aicp.aicp.dev,resources=aicpclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=aicp.aicp.dev,resources=aicpclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=aicp.aicp.dev,resources=aicpclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps;secrets,verbs=get;list;watch;create;update;patch;delete

func (r *AICPClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	// Fetch the AICPCluster instance.
	var cluster aicpv1alpha1.AICPCluster
	if err := r.Get(ctx, req.NamespacedName, &cluster); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling AICPCluster", "name", cluster.Name)

	// Reconcile the server Deployment.
	if err := r.reconcileDeployment(ctx, &cluster); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconciling deployment: %w", err)
	}

	// Reconcile the server Service.
	if err := r.reconcileService(ctx, &cluster); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconciling service: %w", err)
	}

	// Reconcile the ConfigMap.
	if err := r.reconcileConfigMap(ctx, &cluster); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconciling configmap: %w", err)
	}

	// Update status.
	cluster.Status.Phase = "Running"
	if err := r.Status().Update(ctx, &cluster); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating status: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *AICPClusterReconciler) reconcileDeployment(ctx context.Context, cluster *aicpv1alpha1.AICPCluster) error {
	name := cluster.Name + "-server"
	labels := map[string]string{
		"app.kubernetes.io/name":       "aicp-server",
		"app.kubernetes.io/instance":   cluster.Name,
		"app.kubernetes.io/managed-by": "aicp-operator",
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cluster.Namespace,
		},
	}

	var existing appsv1.Deployment
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: cluster.Namespace}, &existing)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	pgPort := cluster.Spec.PostgreSQL.Port
	if pgPort == 0 {
		pgPort = 5432
	}
	pgDB := cluster.Spec.PostgreSQL.Database
	if pgDB == "" {
		pgDB = "aicp"
	}

	container := corev1.Container{
		Name:  "aicp-server",
		Image: cluster.Spec.Image,
		Ports: []corev1.ContainerPort{
			{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
			{Name: "metrics", ContainerPort: 9090, Protocol: corev1.ProtocolTCP},
		},
		Env: []corev1.EnvVar{
			{Name: "AICP_HTTP_ADDR", Value: ":8080"},
			{Name: "AICP_METRICS_ADDR", Value: ":9090"},
			{Name: "AICP_PG_HOST", Value: cluster.Spec.PostgreSQL.Host},
			{Name: "AICP_PG_PORT", Value: fmt.Sprintf("%d", pgPort)},
			{Name: "AICP_PG_DATABASE", Value: pgDB},
			{Name: "AICP_PG_USER", ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: cluster.Spec.PostgreSQL.SecretRef.Name},
					Key:                  "username",
				},
			}},
			{Name: "AICP_PG_PASSWORD", ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: cluster.Spec.PostgreSQL.SecretRef.Name},
					Key:                  "password",
				},
			}},
			{Name: "AICP_REDIS_ADDR", Value: cluster.Spec.Redis.Address},
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/health",
					Port: intstr.FromString("http"),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/ready",
					Port: intstr.FromString("http"),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
	}

	// Set resource limits if specified.
	if cluster.Spec.Resources != nil {
		resources := corev1.ResourceRequirements{}
		if cluster.Spec.Resources.CPURequest != "" || cluster.Spec.Resources.MemoryRequest != "" {
			resources.Requests = corev1.ResourceList{}
			if cluster.Spec.Resources.CPURequest != "" {
				resources.Requests[corev1.ResourceCPU] = resource.MustParse(cluster.Spec.Resources.CPURequest)
			}
			if cluster.Spec.Resources.MemoryRequest != "" {
				resources.Requests[corev1.ResourceMemory] = resource.MustParse(cluster.Spec.Resources.MemoryRequest)
			}
		}
		if cluster.Spec.Resources.CPULimit != "" || cluster.Spec.Resources.MemoryLimit != "" {
			resources.Limits = corev1.ResourceList{}
			if cluster.Spec.Resources.CPULimit != "" {
				resources.Limits[corev1.ResourceCPU] = resource.MustParse(cluster.Spec.Resources.CPULimit)
			}
			if cluster.Spec.Resources.MemoryLimit != "" {
				resources.Limits[corev1.ResourceMemory] = resource.MustParse(cluster.Spec.Resources.MemoryLimit)
			}
		}
		container.Resources = resources
	}

	// Add Redis password if secret is provided.
	if cluster.Spec.Redis.SecretRef != nil {
		container.Env = append(container.Env, corev1.EnvVar{
			Name: "AICP_REDIS_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: cluster.Spec.Redis.SecretRef.Name},
					Key:                  "password",
				},
			},
		})
	}

	if cluster.Spec.ServiceDID != "" {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "AICP_SERVICE_DID",
			Value: cluster.Spec.ServiceDID,
		})
	}

	deploy.ObjectMeta.Labels = labels
	deploy.Spec = appsv1.DeploymentSpec{
		Replicas: &cluster.Spec.Replicas,
		Selector: &metav1.LabelSelector{MatchLabels: labels},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{Labels: labels},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{container},
			},
		},
	}

	// Set owner reference for garbage collection.
	if err := ctrl.SetControllerReference(cluster, deploy, r.Scheme); err != nil {
		return err
	}

	if errors.IsNotFound(err) {
		return r.Create(ctx, deploy)
	}
	return r.Update(ctx, deploy)
}

func (r *AICPClusterReconciler) reconcileService(ctx context.Context, cluster *aicpv1alpha1.AICPCluster) error {
	name := cluster.Name + "-server"
	labels := map[string]string{
		"app.kubernetes.io/name":     "aicp-server",
		"app.kubernetes.io/instance": cluster.Name,
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cluster.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 8080, TargetPort: intstr.FromString("http"), Protocol: corev1.ProtocolTCP},
				{Name: "metrics", Port: 9090, TargetPort: intstr.FromString("metrics"), Protocol: corev1.ProtocolTCP},
			},
		},
	}

	if err := ctrl.SetControllerReference(cluster, svc, r.Scheme); err != nil {
		return err
	}

	var existing corev1.Service
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: cluster.Namespace}, &existing)
	if errors.IsNotFound(err) {
		return r.Create(ctx, svc)
	}
	if err != nil {
		return err
	}
	return r.Update(ctx, svc)
}

func (r *AICPClusterReconciler) reconcileConfigMap(ctx context.Context, cluster *aicpv1alpha1.AICPCluster) error {
	name := cluster.Name + "-config"

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cluster.Namespace,
		},
		Data: map[string]string{
			"AICP_LOG_LEVEL":   "info",
			"AICP_LOG_FORMAT":  "json",
			"AICP_SERVICE_DID": cluster.Spec.ServiceDID,
		},
	}

	if err := ctrl.SetControllerReference(cluster, cm, r.Scheme); err != nil {
		return err
	}

	var existing corev1.ConfigMap
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: cluster.Namespace}, &existing)
	if errors.IsNotFound(err) {
		return r.Create(ctx, cm)
	}
	if err != nil {
		return err
	}
	return r.Update(ctx, cm)
}

// SetupWithManager sets up the controller with the Manager.
func (r *AICPClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&aicpv1alpha1.AICPCluster{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Named("aicpcluster").
		Complete(r)
}
