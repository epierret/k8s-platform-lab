package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	platformv1alpha1 "github.com/epierret/k8s-platform-lab/api/v1alpha1"
)

const (
	// FinalizerName : le "check-out de l'hôtel" à faire avant la suppression.
	FinalizerName = "platform.enrique-pierret.com/registry"
	// RegistryName : le ConfigMap qui liste toutes les WebApps (dans le namespace de l'opérateur).
	RegistryName = "webapp-registry"
	// ConditionAvailable : la condition affichée dans "kubectl get wa".
	ConditionAvailable = "Available"
)

// WebAppReconciler : le robot. RegistryNamespace = où vit le registre (webapp-system).
type WebAppReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	RegistryNamespace string
}

// +kubebuilder:rbac:groups=platform.enrique-pierret.com,resources=webapps,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=platform.enrique-pierret.com,resources=webapps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=platform.enrique-pierret.com,resources=webapps/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch

// Reconcile : le thermostat. Compare le désiré (WebApp) à l'observé (cluster) et corrige.
func (r *WebAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// 0. LIRE le formulaire. S'il n'existe plus, il n'y a rien à faire.
	var wa platformv1alpha1.WebApp
	if err := r.Get(ctx, req.NamespacedName, &wa); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 1. SUPPRESSION en cours : rendre la clé (registre), puis libérer le finalizer.
	if !wa.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&wa, FinalizerName) {
			if err := r.updateRegistry(ctx, &wa, false); err != nil {
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(&wa, FinalizerName)
			if err := r.Update(ctx, &wa); err != nil {
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}
			log.Info("registry entry removed, finalizer released")
		}
		return ctrl.Result{}, nil
	}

	// 2. FINALIZER posé AVANT de créer quoi que ce soit.
	if controllerutil.AddFinalizer(&wa, FinalizerName) {
		if err := r.Update(ctx, &wa); err != nil {
			return ctrl.Result{}, err
		}
	}

	// 3. ENFANTS : "crée-le s'il n'existe pas, sinon remets-le conforme".
	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: wa.Name, Namespace: wa.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, dep, func() error {
		mutateDeployment(dep, &wa)
		return controllerutil.SetControllerReference(&wa, dep, r.Scheme)
	}); err != nil {
		return ctrl.Result{}, err
	}

	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: wa.Name, Namespace: wa.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		mutateService(svc, &wa)
		return controllerutil.SetControllerReference(&wa, svc, r.Scheme)
	}); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.updateRegistry(ctx, &wa, true); err != nil {
		return ctrl.Result{}, err
	}

	// 4. STATUT honnête : ce qu'on OBSERVE, pas ce qu'on espère.
	desired := *dep.Spec.Replicas
	ready := dep.Status.ReadyReplicas
	cond := metav1.Condition{Type: ConditionAvailable, ObservedGeneration: wa.Generation}
	if ready >= desired {
		cond.Status = metav1.ConditionTrue
		cond.Reason = "AllReplicasReady"
		cond.Message = fmt.Sprintf("%d/%d replicas ready", ready, desired)
	} else {
		cond.Status = metav1.ConditionFalse
		cond.Reason = "ReplicasNotReady"
		cond.Message = fmt.Sprintf("%d/%d replicas ready", ready, desired)
	}
	wa.Status.ReadyReplicas = ready
	wa.Status.ObservedGeneration = wa.Generation
	meta.SetStatusCondition(&wa.Status.Conditions, cond)

	if err := r.Status().Update(ctx, &wa); err != nil {
		if apierrors.IsConflict(err) {
			// Quelqu'un a modifié l'objet entre-temps : on réessaie dans 1 s.
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// updateRegistry : ajoute (present=true) ou retire l'entrée "<namespace>.<nom>" du registre.
// Le registre vit dans un AUTRE namespace : l'ownerReference ne peut pas le nettoyer,
// c'est pour ça qu'il faut un finalizer.
func (r *WebAppReconciler) updateRegistry(ctx context.Context, wa *platformv1alpha1.WebApp, present bool) error {
	key := wa.Namespace + "." + wa.Name
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: RegistryName, Namespace: r.RegistryNamespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
		if cm.Data == nil {
			cm.Data = map[string]string{}
		}
		if present {
			cm.Data[key] = wa.Spec.Image
		} else {
			delete(cm.Data, key)
		}
		return nil
	})
	return err
}

// SetupWithManager : quand réveiller le robot ?
func (r *WebAppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// Une WebApp change… seulement son spec ou ses annotations (pas son statut).
		For(&platformv1alpha1.WebApp{}, builder.WithPredicates(
			predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{}))).
		// Un de SES Deployments ou Services change (supprimé, modifié à la main, pods prêts…).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
