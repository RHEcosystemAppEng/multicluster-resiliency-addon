// Copyright (c) 2023 Red Hat, Inc.

package controller

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// verifyObject takes a function and returns a predicate that verifies the event object for all events against
// the function.
func verifyObject(fn func(obj client.Object) bool) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			return fn(createEvent.Object)
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return fn(deleteEvent.Object)
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			return fn(updateEvent.ObjectOld) && fn(updateEvent.ObjectNew)
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return fn(genericEvent.Object)
		},
	}
}

// ownerIsOurAddon is a utility function returning true if one of the owners for a client.Object is this
// Addon. Use it with verifyObject.
func ownerIsOurAddon(obj client.Object) bool {
	for _, owner := range obj.GetOwnerReferences() {
		if owner.Kind == "ClusterManagementAddOn" && owner.Name == "multicluster-resiliency-addon" {
			return true
		}
	}
	return false
}

// hasAnnotation is a utility function that takes an annotation and returns a function that takes a client.Object and
// returns true if it contains the aforementioned annotation. Use it with verifyObject.
func hasAnnotation(annotation string) func(obj client.Object) bool {
	return func(obj client.Object) bool {
		for _, anno := range obj.GetAnnotations() {
			if annotation == anno {
				return true
			}
		}
		return false
	}
}
