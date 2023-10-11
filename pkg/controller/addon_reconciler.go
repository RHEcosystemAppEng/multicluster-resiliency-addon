// Copyright (c) 2023 Red Hat, Inc.

package controller

// This file hosts our AddonReconciler implementation registering for the framework's ManagedClusterAddOn CRs.

import (
	"context"
	"fmt"
	apiv1 "github.com/rhecosystemappeng/multicluster-resiliency-addon/api/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// AddonReconciler is a receiver representing the MultiCluster-Resiliency-Addon operator reconciler for
// ManagedClusterAddOn CRs.
type AddonReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile is watching ManagedClusterAddOn CRs and creating/updating/deleting the corresponding ResilientCluster CRs.
// Note, the permissions listed here for the controller-gen are required for the Addon framework. Permissions for our
// own Addon are listed in ClusterReconciler.Reconcile.
//
// +kubebuilder:rbac:groups="",resources=configmaps;events,verbs=get;list;watch;create;update;delete;deletecollection;patch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=get;create
// +kubebuilder:rbac:groups=certificates.k8s.io,resources=certificatesigningrequests;certificatesigningrequests/approval,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups=certificates.k8s.io,resources=signers,verbs=approve
// +kubebuilder:rbac:groups=cluster.open-cluster-management.io,resources=managedclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=work.open-cluster-management.io,resources=manifestworks,verbs=create;update;get;list;watch;delete;deletecollection;patch
// +kubebuilder:rbac:groups=addon.open-cluster-management.io,resources=clustermanagementaddons/finalizers,verbs=update
// +kubebuilder:rbac:groups=addon.open-cluster-management.io,resources=clustermanagementaddons,verbs=get;list;watch
// +kubebuilder:rbac:groups=addon.open-cluster-management.io,resources=managedclusteraddons,verbs=get;list;watch
// +kubebuilder:rbac:groups=addon.open-cluster-management.io,resources=managedclusteraddons,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=addon.open-cluster-management.io,resources=managedclusteraddons/finalizers,verbs=update
// +kubebuilder:rbac:groups=addon.open-cluster-management.io,resources=managedclusteraddons/status,verbs=update;patch
func (r *AddonReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// name and namespace are identical for both the ManagedClusterAddon and ResilientCluster crs
	subject := types.NamespacedName{
		Namespace: req.Namespace,
		Name:      req.Name,
	}

	// fetch the ManagedClusterAddon cr, end loop if not found
	mca := &addonv1alpha1.ManagedClusterAddOn{}
	if err := r.Client.Get(ctx, subject, mca); err != nil {
		if errors.IsNotFound(err) {
			logger.Info(fmt.Sprintf("%s ManagedClusterAddOn not found", subject.String()))
			return ctrl.Result{}, nil
		}
		logger.Error(err, fmt.Sprintf("%s ManagedClusterAddOn failed fetching", subject.String()))
		return ctrl.Result{}, err
	}

	// fetch the ResilientCluster cr, note if found or not
	rc := &apiv1.ResilientCluster{}
	rcFound := true
	if err := r.Client.Get(ctx, subject, rc); err != nil {
		// only not-found errors are acceptable here
		if !errors.IsNotFound(err) {
			logger.Error(err, fmt.Sprintf("%s ResilientCluster failed fetching", subject.String()))
			return ctrl.Result{}, err
		}
		rcFound = false
	}

	// is the ManagedClusterAddon being deleted? if so, we need to verify deletion of the corresponding ResilientCluster
	if !mca.DeletionTimestamp.IsZero() {
		// do we have a ResilientCluster? if so, we need to delete it
		if rcFound {
			// if the ResilientCluster IS NOT already being deleted, we need to delete it ourselves
			if rc.DeletionTimestamp.IsZero() {
				// we define proper ownership while creating the instance, this is just a fail-safe
				if err := r.Client.Delete(ctx, rc); err != nil {
					logger.Error(err, fmt.Sprintf("%s ResilientCluster deletion failed", subject.String()))
					return ctrl.Result{}, err
				}
			}
		}

		// no further progress is required while deleting objects
		return ctrl.Result{}, nil
	}

	// generate a new status for the ResilientCluster based on the ManagedClusterAddon
	currentStatus := generateCurrentClusterStatus(mca)

	// do we have a corresponding ResilientCluster? we need to either create or update it
	if rcFound {
		// ResilientCluster exists, we need to update it's previous and current statuses
		rc.Status.PreviousStatus = rc.Status.CurrentStatus
		rc.Status.CurrentStatus = currentStatus

		if err := r.Client.Update(ctx, rc); err != nil {
			logger.Error(err, fmt.Sprintf("%s ResilientCluster update failed", subject.String()))
			return ctrl.Result{}, err
		}
	} else {
		// ResilientCluster doesn't exist, we need to create it
		rc.SetName(subject.Name)
		rc.SetNamespace(subject.Namespace)
		rc.SetFinalizers([]string{mcraFinalizerName})
		rc.SetOwnerReferences([]metav1.OwnerReference{
			*metav1.NewControllerRef(mca, mca.GetObjectKind().GroupVersionKind()),
		})

		// for new instances, the current status is also the initial status
		// new instances do not require a PreviousStatus
		rc.Status.InitialStatus = currentStatus
		rc.Status.CurrentStatus = currentStatus

		if err := r.Client.Create(ctx, rc); err != nil {
			logger.Error(err, fmt.Sprintf("%s ResilientCluster creation failed", subject.String()))
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager is used for setting up the controller named 'mcra-managed-cluster-agent-controller' with the manager.
// It uses predicates as event filters for verifying only handling ManagedClusterAddon CRs for our own Addon.
func (r *AddonReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("mcra-managed-cluster-agent-controller").
		For(&addonv1alpha1.ManagedClusterAddOn{}).
		WithEventFilter(verifyOwnerPredicate).
		Complete(r)
}

// verifyOwnerPredicate is a predicate for verifying an object is owned by our Addon. Used to verify events.
var verifyOwnerPredicate = predicate.Funcs{
	CreateFunc: func(createEvent event.CreateEvent) bool {
		return verifyObjectOwner(createEvent.Object)
	},
	DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
		return verifyObjectOwner(deleteEvent.Object)
	},
	UpdateFunc: func(updateEvent event.UpdateEvent) bool {
		return verifyObjectOwner(updateEvent.ObjectOld) && verifyObjectOwner(updateEvent.ObjectNew)
	},
	GenericFunc: func(genericEvent event.GenericEvent) bool {
		return verifyObjectOwner(genericEvent.Object)
	},
}

// verifyObjectOwner is a utility function returning true if one of the owners for a client.Object is this Addon.
func verifyObjectOwner(obj client.Object) bool {
	for _, owner := range obj.GetOwnerReferences() {
		if owner.Kind == "ClusterManagementAddOn" && owner.Name == "multicluster-resiliency-addon" {
			return true
		}
	}
	return false
}

// generateCurrentClusterStatus is used for generating a ClusterStatus from based on a ManagedClusterAddon. For future
// features, this is potentially where we can add further logic for determining whether of not the Spoke is available.
func generateCurrentClusterStatus(mca *addonv1alpha1.ManagedClusterAddOn) apiv1.ClusterStatus {
	status := apiv1.ClusterStatus{
		Availability: apiv1.ClusterNotAvailable,
		Time:         metav1.Now(),
	}

	// look for an Available condition in the MCA and set RC availability accordingly
	if meta.IsStatusConditionTrue(mca.Status.Conditions, "Available") {
		status.Availability = apiv1.ClusterAvailable
	}

	return status
}
