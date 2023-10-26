// Copyright (c) 2023 Red Hat, Inc.

package actions

// This file contains the action for moving AddonDeploymentConfigs between spokes.

import (
	"context"
	"fmt"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// moveAddonDeploymentConfigs is used for moving all AddonDeploymentConfig resources found from the OLD spoke namespace
// to the NEW one, and delete the old ones.
func moveAddonDeploymentConfigs(ctx context.Context, options Options) {
	logger := log.FromContext(ctx)

	// fetch AddOnDeploymentConfigs from previous OLD cluster and copy them to the NEW one
	oldConfigs := &addonv1alpha1.AddOnDeploymentConfigList{}
	if err := options.Client.List(ctx, oldConfigs, &client.ListOptions{Namespace: options.OldSpoke}); err != nil {
		logger.Info(fmt.Sprintf("no AddOnDeploymentConfigs found on %s", options.OldSpoke))
	} else {
		// iterate over all found configs, create new ones for the NEW spoke, and delete the OLD ones
		for _, oldConfig := range oldConfigs.Items {
			newConfig := oldConfig.DeepCopy()
			newConfig.SetName(oldConfig.Name)
			newConfig.SetNamespace(options.NewSpoke)
			if err = options.Client.Create(ctx, newConfig); err != nil {
				logger.Error(err, fmt.Sprintf("failed creating AddOnDeploymentConfig %s in %s", newConfig.Name, options.NewSpoke))
			}
			if err = options.Client.Delete(ctx, &oldConfig); err != nil {
				logger.Error(err, fmt.Sprintf("failed deleting AddOnDeploymentConfig %s from %s", oldConfig.Name, options.OldSpoke))
			}
		}
	}
}

// init is registering moveAddonDeploymentConfigs for running.
func init() {
	actionFuncs = append(actionFuncs, moveAddonDeploymentConfigs)
}
