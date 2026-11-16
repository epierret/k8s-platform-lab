package controller

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	platformv1alpha1 "github.com/epierret/k8s-platform-lab/api/v1alpha1"
)

const (
	defaultPort     int32 = 8080
	defaultReplicas int32 = 1
)

// labelsFor : l'étiquette qui relie le Deployment, ses pods et le Service.
func labelsFor(wa *platformv1alpha1.WebApp) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       wa.Name,
		"app.kubernetes.io/managed-by": "webapp-operator",
	}
}

func portFor(wa *platformv1alpha1.WebApp) int32 {
	if wa.Spec.Port == 0 {
		return defaultPort
	}
	return wa.Spec.Port
}

// resourcesFor : reprend ce que le développeur a demandé et complète ce qui manque,
// pour respecter la politique "requests CPU/mémoire + limite mémoire obligatoires".
func resourcesFor(wa *platformv1alpha1.WebApp) corev1.ResourceRequirements {
	res := *wa.Spec.Resources.DeepCopy()
	if res.Requests == nil {
		res.Requests = corev1.ResourceList{}
	}
	if res.Limits == nil {
		res.Limits = corev1.ResourceList{}
	}
	if _, ok := res.Requests[corev1.ResourceCPU]; !ok {
		res.Requests[corev1.ResourceCPU] = resource.MustParse("50m")
	}
	if _, ok := res.Requests[corev1.ResourceMemory]; !ok {
		res.Requests[corev1.ResourceMemory] = resource.MustParse("32Mi")
	}
	if _, ok := res.Limits[corev1.ResourceMemory]; !ok {
		res.Limits[corev1.ResourceMemory] = resource.MustParse("64Mi")
	}
	return res
}

// httpProbe : on écrit TOUTES les valeurs (même celles par défaut),
// sinon l'API server les ajoute et on croit voir une différence à chaque passage.
func httpProbe(path string, period, failures int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{Path: path, Port: intstr.FromString("http"), Scheme: corev1.URISchemeHTTP},
		},
		PeriodSeconds:    period,
		FailureThreshold: failures,
		TimeoutSeconds:   1,
		SuccessThreshold: 1,
	}
}

// mutateDeployment : remplit UNIQUEMENT les champs qu'on possède, sur l'objet existant.
func mutateDeployment(dep *appsv1.Deployment, wa *platformv1alpha1.WebApp) {
	labels := labelsFor(wa)
	if dep.Labels == nil {
		dep.Labels = map[string]string{}
	}
	for k, v := range labels {
		dep.Labels[k] = v
	}

	// Le selector est IMMUABLE : on ne le définit qu'à la création.
	if dep.Spec.Selector == nil {
		dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}
	}

	replicas := defaultReplicas
	if wa.Spec.Replicas != nil {
		replicas = *wa.Spec.Replicas
	}
	dep.Spec.Replicas = ptr.To(replicas)

	tpl := &dep.Spec.Template
	if tpl.Labels == nil {
		tpl.Labels = map[string]string{}
	}
	for k, v := range labels {
		tpl.Labels[k] = v
	}

	pod := &tpl.Spec
	pod.AutomountServiceAccountToken = ptr.To(false)
	pod.SecurityContext = &corev1.PodSecurityContext{
		RunAsNonRoot:   ptr.To(true),
		SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
	}

	if len(pod.Containers) == 0 {
		pod.Containers = []corev1.Container{{Name: "app"}}
	}
	c := &pod.Containers[0]
	c.Name = "app"
	c.Image = wa.Spec.Image
	c.Ports = []corev1.ContainerPort{{Name: "http", ContainerPort: portFor(wa), Protocol: corev1.ProtocolTCP}}
	c.Resources = resourcesFor(wa)
	c.StartupProbe = httpProbe("/healthz", 2, 10)
	c.LivenessProbe = httpProbe("/healthz", 10, 3)
	c.ReadinessProbe = httpProbe("/readyz", 5, 3)
	c.SecurityContext = &corev1.SecurityContext{
		RunAsNonRoot:             ptr.To(true),
		AllowPrivilegeEscalation: ptr.To(false),
		ReadOnlyRootFilesystem:   ptr.To(true),
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
	}
}

// mutateService : l'adresse stable http://<nom>.<namespace> sur le port 80.
func mutateService(svc *corev1.Service, wa *platformv1alpha1.WebApp) {
	labels := labelsFor(wa)
	if svc.Labels == nil {
		svc.Labels = map[string]string{}
	}
	for k, v := range labels {
		svc.Labels[k] = v
	}
	svc.Spec.Type = corev1.ServiceTypeClusterIP
	svc.Spec.Selector = labels
	svc.Spec.Ports = []corev1.ServicePort{{
		Name:       "http",
		Protocol:   corev1.ProtocolTCP,
		Port:       80,
		TargetPort: intstr.FromString("http"),
	}}
}
