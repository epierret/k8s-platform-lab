package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WebAppSpec : ce que le développeur DEMANDE.
type WebAppSpec struct {
	// Image du conteneur, idéalement épinglée par digest. Obligatoire.
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// Nombre de pods voulus, de 0 à 10.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:default=1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Port d'écoute : pas de port privilégié (< 1024), car on tourne en non-root.
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=8080
	// +optional
	Port int32 `json:"port,omitempty"`

	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// WebAppStatus : ce que le robot OBSERVE.
type WebAppStatus struct {
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
	// "J'ai traité la version n° X de ta demande."
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Une seule condition par type (Available...).
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// WebApp : une application web gérée par l'opérateur.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=wa
// +kubebuilder:printcolumn:name="Desired",type=integer,JSONPath=`.spec.replicas`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyReplicas`
// +kubebuilder:printcolumn:name="Available",type=string,JSONPath=`.status.conditions[?(@.type=="Available")].status`
type WebApp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WebAppSpec   `json:"spec"`
	Status WebAppStatus `json:"status,omitempty"`
}

// WebAppList : la liste renvoyée par "kubectl get webapps" (obligatoire pour tout type).
// +kubebuilder:object:root=true
type WebAppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WebApp `json:"items"`
}
