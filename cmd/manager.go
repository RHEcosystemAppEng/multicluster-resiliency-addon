// Copyright (c) 2023 Red Hat, Inc.

package cmd

import (
	"github.com/rhecosystemappeng/multicluster-resiliency-addon/pkg/manager"
	"github.com/rhecosystemappeng/multicluster-resiliency-addon/pkg/version"
	"open-cluster-management.io/addon-framework/pkg/cmd/factory"
)

// init is used for creating the Manager Commend, incorporate its flags, and binding it to the root MCRA Command.
func init() {
	mgr := manager.NewManager()

	mgrCmd := factory.
		NewControllerCommandConfig("multicluster-resiliency-addon-manager", version.Get(), mgr.Run).
		NewCommand()
	mgrCmd.Use = "manager"
	mgrCmd.Short = "Multicluster Resiliency Addon Manager"

	mgrCmd.Flags().StringVar(&mgr.Options.ControllerMetricAddr, "controller-metric-address", ":8080", "TODO")
	mgrCmd.Flags().StringVar(&mgr.Options.ControllerProbeAddr, "controller-probe-address", ":8081", "TODO")
	mgrCmd.Flags().BoolVar(&mgr.Options.ControllerLeaderElection, "controller-leader-election", false, "TODO")
	mgrCmd.Flags().IntVar(&mgr.Options.AgentReplicas, "agent-replicas", 1, "TODO")

	mcraCmd.AddCommand(mgrCmd)
}
