// Copyright (c) 2023 Red Hat, Inc.

package manager

import (
	"context"
	"embed"
	"fmt"
	"k8s.io/client-go/rest"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/agent"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
)

//go:embed agenttemplates
var FS embed.FS // Resource templates used for deploying the Addon Agent to Spokes.

// Values is used to encapsulate template values for the Agent templates.
type Values struct {
	KubeConfigSecret string
	SpokeName        string
	AgentNamespace   string
	AgentReplicas    int
}

// createAgent is used for creating the Addon Agent configuration for the Addon Manager.
func createAgent(ctx context.Context, kubeConfig *rest.Config, options *Options) (agent.AgentAddon, error) {
	return addonfactory.
		NewAgentAddonFactory(AddonName, FS, "agenttemplates").
		WithGetValuesFuncs(getTemplateValuesFunc(options)).
		WithAgentRegistrationOption(getRegistrationOptionFunc(ctx, kubeConfig)).
		BuildTemplateAgentAddon()
}

// getTemplateValuesFunc is used to build a function for generating values to be used in the Addon Agent templates.
func getTemplateValuesFunc(options *Options) func(cluster *clusterv1.ManagedCluster, addon *addonv1alpha1.ManagedClusterAddOn) (addonfactory.Values, error) {
	return func(cluster *clusterv1.ManagedCluster, addon *addonv1alpha1.ManagedClusterAddOn) (addonfactory.Values, error) {
		values := Values{
			KubeConfigSecret: fmt.Sprintf("%s-hub-kubeconfig", addon.Name),
			SpokeName:        cluster.Name,
			AgentNamespace:   addon.Spec.InstallNamespace,
			AgentReplicas:    options.AgentReplicas,
		}
		return addonfactory.StructToValues(values), nil
	}
}
