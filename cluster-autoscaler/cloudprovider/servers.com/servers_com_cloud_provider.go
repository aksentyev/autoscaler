package servers_com

import (
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/autoscaler/cluster-autoscaler/utils/errors"
)

// serversComCloudProvider implements CloudProvider interface from cluster-autoscaler/cloudprovider module.
type serversComCloudProvider struct {
}

// Name returns name of the cloud provider.
func (scp *serversComCloudProvider) Name() string {

}

// NodeGroups returns all node groups configured for this cloud provider.
func (scp *serversComCloudProvider) NodeGroups() []cloudprovider.NodeGroup {

}

// NodeGroupForNode returns the node group for the given node, nil if the node
// should not be processed by cluster autoscaler, or non-nil error if such
// occurred. Must be implemented.
func (scp *serversComCloudProvider) NodeGroupForNode(*apiv1.Node) (cloudprovider.NodeGroup, error) {

}

// Pricing returns pricing model for this cloud provider or error if not available.
// Implementation optional.
func (scp *serversComCloudProvider) Pricing() (cloudprovider.PricingModel, errors.AutoscalerError) {

}

// GetAvailableMachineTypes get all machine types that can be requested from the cloud provider.
// Implementation optional.
func (scp *serversComCloudProvider) GetAvailableMachineTypes() ([]string, error) {

}

// NewNodeGroup builds a theoretical node group based on the node definition provided. The node group is not automatically
// created on the cloud provider side. The node group is not returned by NodeGroups() until it is created.
// Implementation optional.
func (scp *serversComCloudProvider) NewNodeGroup(machineType string, labels map[string]string, systemLabels map[string]string, taints []apiv1.Taint, extraResources map[string]resource.Quantity) (cloudprovider.NodeGroup, error) {

}

// GetResourceLimiter returns struct containing limits (max, min) for resources (cores, memory etc.).
func (scp *serversComCloudProvider) GetResourceLimiter() (*cloudprovider.ResourceLimiter, error) {

}

// GPULabel returns the label added to nodes with GPU resource.
func (scp *serversComCloudProvider) GPULabel() string {

}

// GetAvailableGPUTypes return all available GPU types cloud provider supports.
func (scp *serversComCloudProvider) GetAvailableGPUTypes() map[string]struct{} {

}

// Cleanup cleans up open resources before the cloud provider is destroyed, i.e. go routines etc.
func (scp *serversComCloudProvider) Cleanup() error {

}

// Refresh is called before every main loop and can be used to dynamically update cloud provider state.
// In particular the list of node groups returned by NodeGroups can change as a result of CloudProvider.Refresh().
func (scp *serversComCloudProvider) Refresh() error {

}
