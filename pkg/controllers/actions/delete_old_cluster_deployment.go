// Copyright (c) 2023 Red Hat, Inc.

package actions

// This file contains the action for deleting a ClusterDeployment from the OLD spoke.

import (
	"context"
	hivev1 "github.com/openshift/hive/apis/hive/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// deleteOldClusterDeployment is used for deleting Hive's ClusterDeployment from the OLD spoke.
func deleteOldClusterDeployment(ctx context.Context, options Options) {
	logger := log.FromContext(ctx)
	logger.Info("deleting old cluster deployment", "old-spoke", options.OldSpoke)

	// the ClusterDeployment resides in the cluster-namespace with a matching name
	oldDeploymentSubject := types.NamespacedName{
		Namespace: options.OldSpoke,
		Name:      options.OldSpoke,
	}

	// fetch ClusterDeployment from previous OLD cluster and delete it if exists
	oldDeployment := &hivev1.ClusterDeployment{}
	if err := options.Client.Get(ctx, oldDeploymentSubject, oldDeployment); err != nil {
		logger.Info("no ClusterDeployments found", "old-spoke", options.OldSpoke)
	} else {
		if err = options.Client.Delete(ctx, oldDeployment); err != nil {
			logger.Error(err, "failed deleting ClusterDeployment", "old-spoke", options.OldSpoke)
		}
	}
}

// init is registering deleteOldClusterDeployment for running.
func init() {
	actionFuncs = append(actionFuncs, deleteOldClusterDeployment)
}
