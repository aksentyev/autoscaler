package serverscom

import (
	"io"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/autoscaler/cluster-autoscaler/config"
	"k8s.io/autoscaler/cluster-autoscaler/utils/errors"
	"k8s.io/klog"
	"os"
)

// serversComCloudProvider implements CloudProvider interface from cluster-autoscaler/cloudprovider module.
type serversComCloudProvider struct {
	manager         manager
	resourceLimiter *cloudprovider.ResourceLimiter
}

// todo
// BuildServersCom builds the Servers.com cloud provider.
func BuildServersCom(
	opts config.AutoscalingOptions,
	do cloudprovider.NodeGroupDiscoveryOptions,
	rl *cloudprovider.ResourceLimiter,
) cloudprovider.CloudProvider {
	var config io.ReadCloser

	// Should be loaded with --cloud-config /etc/kubernetes/kube_openstack_config from master node
	if opts.CloudConfig != "" {
		var err error
		config, err = os.Open(opts.CloudConfig)
		if err != nil {
			klog.Fatalf("Couldn't open cloud provider configuration %s: %#v", opts.CloudConfig, err)
		}
		defer config.Close()
	}

	manager, err := newManager(config)
	if err != nil {
		klog.Fatalf("Couldn't open cloud provider configuration %s: %#v", opts.CloudConfig, err)
	}

	return &serversComCloudProvider{
		manager:         manager,
		resourceLimiter: rl,
	}
}

// Name returns name of the cloud provider.
func (scp *serversComCloudProvider) Name() string {
	return cloudprovider.ServersComProviderName
}

// NodeGroups returns all node groups configured for this cloud provider.
func (scp *serversComCloudProvider) NodeGroups() []cloudprovider.NodeGroup {
	nodeGroups, err := scp.manager.GetNodeGroups()
	if err != nil {
		klog.Errorf("get nodegroups error: %s", err)
		return nil
	}

	res := make([]cloudprovider.NodeGroup, len(nodeGroups))
	for i, ng := range nodeGroups {
		res[i] = ng
	}
	return res
}

// NodeGroupForNode returns the node group for the given node, nil if the node
// should not be processed by cluster autoscaler, or non-nil error if such
// occurred. Must be implemented.
func (scp *serversComCloudProvider) NodeGroupForNode(node *apiv1.Node) (cloudprovider.NodeGroup, error) {
	ng, err := scp.manager.NodeGroupForNode(node.Name)
	if err != nil {
		return nil, nil
	}

	return ng, nil
}

// Cleanup cleans up open resources before the cloud provider is destroyed, i.e. go routines etc.
func (scp *serversComCloudProvider) Cleanup() error {
	return nil
}

// Refresh is called before every main loop and can be used to dynamically update cloud provider state.
// In particular the list of node groups returned by NodeGroups can change as a result of CloudProvider.Refresh().
func (scp *serversComCloudProvider) Refresh() error {
	return nil // todo
}

// NewNodeGroup builds a theoretical node group based on the node definition provided. The node group is not automatically
// created on the cloud provider side. The node group is not returned by NodeGroups() until it is created.
// Implementation optional.
func (scp *serversComCloudProvider) NewNodeGroup(machineType string, labels map[string]string, systemLabels map[string]string, taints []apiv1.Taint, extraResources map[string]resource.Quantity) (cloudprovider.NodeGroup, error) {
	return nil, cloudprovider.ErrNotImplemented
}

// Pricing returns pricing model for this cloud provider or error if not available.
// Implementation optional.
func (scp *serversComCloudProvider) Pricing() (cloudprovider.PricingModel, errors.AutoscalerError) {
	return nil, cloudprovider.ErrNotImplemented
}

// GetAvailableMachineTypes get all machine types that can be requested from the cloud provider.
// Implementation optional.
func (scp *serversComCloudProvider) GetAvailableMachineTypes() ([]string, error) {
	return nil, cloudprovider.ErrNotImplemented // todo?
}

// GPULabel returns the label added to nodes with GPU resource.
func (scp *serversComCloudProvider) GPULabel() string {
	return ""
}

// GetAvailableGPUTypes return all available GPU types cloud provider supports.
func (scp *serversComCloudProvider) GetAvailableGPUTypes() map[string]struct{} {
	return nil
}

// GetResourceLimiter returns struct containing limits (Max, Min) for resources (cores, memory etc.).
func (scp *serversComCloudProvider) GetResourceLimiter() (*cloudprovider.ResourceLimiter, error) {
	return scp.resourceLimiter, nil
}
