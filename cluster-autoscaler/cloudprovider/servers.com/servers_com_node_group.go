package servers_com

import (
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	schedulernodeinfo "k8s.io/kubernetes/pkg/scheduler/nodeinfo"
)

type serverscomNodeGroup struct {
}

// MaxSize returns maximum size of the node group.
func (scng *serverscomNodeGroup) MaxSize() int {

}

// MinSize returns minimum size of the node group.
func (scng *serverscomNodeGroup) MinSize() int {

}

// TargetSize returns the current target size of the node group. It is possible that the
// number of nodes in Kubernetes is different at the moment but should be equal
// to Size() once everything stabilizes (new nodes finish startup and registration or
// removed nodes are deleted completely). Implementation required.
func (scng *serverscomNodeGroup) TargetSize() (int, error) {

}

// IncreaseSize increases the size of the node group. To delete a node you need
// to explicitly name it and use DeleteNode. This function should wait until
// node group size is updated. Implementation required.
func (scng *serverscomNodeGroup) IncreaseSize(delta int) error {

}

// DeleteNodes deletes nodes from this node group. Error is returned either on
// failure or if the given node doesn't belong to this node group. This function
// should wait until node group size is updated. Implementation required.
func (scng *serverscomNodeGroup) DeleteNodes([]*apiv1.Node) error {

}

// DecreaseTargetSize decreases the target size of the node group. This function
// doesn't permit to delete any existing node and can be used only to reduce the
// request for new nodes that have not been yet fulfilled. Delta should be negative.
// It is assumed that cloud provider will not delete the existing nodes when there
// is an option to just decrease the target. Implementation required.
func (scng *serverscomNodeGroup) DecreaseTargetSize(delta int) error {

}

// Id returns an unique identifier of the node group.
func (scng *serverscomNodeGroup) Id() string {

}

// Debug returns a string containing all information regarding this node group.
func (scng *serverscomNodeGroup) Debug() string {

}

// Nodes returns a list of all nodes that belong to this node group.
// It is required that Instance objects returned by this method have Id field set.
// Other fields are optional.
func (scng *serverscomNodeGroup) Nodes() ([]cloudprovider.Instance, error) {

}

// TemplateNodeInfo returns a schedulernodeinfo.NodeInfo structure of an empty
// (as if just started) node. This will be used in scale-up simulations to
// predict what would a new node look like if a node group was expanded. The returned
// NodeInfo is expected to have a fully populated Node object, with all of the labels,
// capacity and allocatable information as well as all pods that are started on
// the node by default, using manifest (most likely only kube-proxy). Implementation optional.
func (scng *serverscomNodeGroup) TemplateNodeInfo() (*schedulernodeinfo.NodeInfo, error) {

}

// Exist checks if the node group really exists on the cloud provider side. Allows to tell the
// theoretical node group from the real one. Implementation required.
func (scng *serverscomNodeGroup) Exist() bool {

}

// Create creates the node group on the cloud provider side. Implementation optional.
func (scng *serverscomNodeGroup) Create() (cloudprovider.NodeGroup, error) {

}

// Delete deletes the node group on the cloud provider side.
// This will be executed only for autoprovisioned node groups, once their size drops to 0.
// Implementation optional.
func (scng *serverscomNodeGroup) Delete() error {

}

// Autoprovisioned returns true if the node group is autoprovisioned. An autoprovisioned group
// was created by CA and can be deleted when scaled to 0.
func (scng *serverscomNodeGroup) Autoprovisioned() bool {

}
