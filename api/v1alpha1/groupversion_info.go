// Package v1alpha1 contient le type WebApp du groupe platform.enrique-pierret.com.
// +kubebuilder:object:generate=true
// +groupName=platform.enrique-pierret.com
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion : le "nom de famille" de notre API
	GroupVersion = schema.GroupVersion{Group: "platform.enrique-pierret.com", Version: "v1alpha1"}
	// SchemeBuilder : enregistre nos types auprès du client Kubernetes
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion, &WebApp{}, &WebAppList{})
	metav1.AddToGroupVersion(scheme, GroupVersion)
	return nil
}
