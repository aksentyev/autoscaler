package serverscom

import (
	"fmt"
	"github.com/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/klog"
	schedulernodeinfo "k8s.io/kubernetes/pkg/scheduler/nodeinfo"
	"sync"
)

// todo store nodegroup Min Max in config
type nodeGroup struct {
	muClusterUpdate sync.Mutex
	id              string
	manager         manager
	size            *nodeGroupSize
	nodes           []cloudprovider.Instance
	notRunningNodes []cloudprovider.Instance
}

type (
	nodeGroupSize struct {
		min    int
		max    int
		target int
	}
)

// MaxSize returns maximum size of the node group.
func (ng *nodeGroup) MaxSize() int {
	return ng.size.max
}

// MinSize returns minimum size of the node group.
func (ng *nodeGroup) MinSize() int {
	return ng.size.min
}

// TargetSize returns the current target size of the node group. It is possible that the
// number of nodes in Kubernetes is different at the moment but should be equal
// to Size() once everything stabilizes (new nodes finish startup and registration or
// removed nodes are deleted completely). Implementation required.
func (ng *nodeGroup) TargetSize() (int, error) {
	ng.muClusterUpdate.Lock()
	defer ng.muClusterUpdate.Unlock()
	return ng.size.target, nil
}

// IncreaseSize increases the size of the node group. To delete a node you need
// to explicitly name it and use DeleteNode. This function should wait until
// node group size is updated. Implementation required.
func (ng *nodeGroup) IncreaseSize(delta int) error {
	if delta <= 0 {
		klog.V(1).Info("delta is less or equal to 0")
		return nil
	}

	if ng.size.target+delta > ng.size.max {
		return errors.New("delta value is too big, may cause nodegroup overflow")
	}

	ng.muClusterUpdate.Lock()
	defer ng.muClusterUpdate.Unlock()

	created, err := ng.manager.CreateNodes(delta, ng.id)
	if err != nil {
		return errors.Wrap(err, "create nodes error")
	}

	// save target size
	ng.size.target += created

	return nil
}

// DeleteNodes deletes nodes from this node group. Error is returned either on
// failure or if the given node doesn't belong to this node group. This function
// should wait until node group size is updated. Implementation required.
func (ng *nodeGroup) DeleteNodes(nodes []*apiv1.Node) error {
	// decrement the count by one  after a successful delete
	ng.muClusterUpdate.Lock()
	defer ng.muClusterUpdate.Unlock()
	for _, node := range nodes {
		err := ng.manager.DeleteNode(node)
		if err != nil {
			return fmt.Errorf("deleting node failed name=%s", node.Name)
		}
		ng.size.target--
	}

	return nil
}

// the following method is used to fix target size if it is inconsistent with registered nodes count. Before running this method
// DecreaseTargetSize decreases the target size of the node group. This function
// doesn't permit to delete any existing node and can be used only to reduce the
// request for new nodes that have not been yet fulfilled. Delta should be negative.
// It is assumed that cloud provider will not delete the existing nodes when there
// is an option to just decrease the target. Implementation required.
func (ng *nodeGroup) DecreaseTargetSize(delta int) error {
	ng.muClusterUpdate.Lock()
	defer ng.muClusterUpdate.Unlock()

	ng.size.target += delta

	return nil
}

// Id returns an unique identifier of the node group.
func (ng *nodeGroup) Id() string {
	return ng.id
}

// Debug returns a string containing all information regarding this node group.
func (ng *nodeGroup) Debug() string {
	return fmt.Sprintf("%s Min=%d Max=%d target=%d", ng.id, ng.size.min, ng.size.max, ng.size.target)
}

// Nodes returns a list of all nodes that belong to this node group.
// It is required that Instance objects returned by this method have Id field set.
// Other fields are optional.
func (ng *nodeGroup) Nodes() ([]cloudprovider.Instance, error) {
	return ng.nodes, nil
}

// TemplateNodeInfo returns a schedulernodeinfo.NodeInfo structure of an empty
// (as if just started) node. This will be used in scale-up simulations to
// predict what would a new node look like if a node group was expanded. The returned
// NodeInfo is expected to have a fully populated Node object, with all of the labels,
// capacity and allocatable information as well as all pods that are started on
// the node by default, using manifest (most likely only kube-proxy). Implementation optional.
func (ng *nodeGroup) TemplateNodeInfo() (*schedulernodeinfo.NodeInfo, error) {
	return nil, cloudprovider.ErrNotImplemented
}

// Exist checks if the node group really exists on the cloud provider side. Allows to tell the
// theoretical node group from the real one. Implementation required.
func (ng *nodeGroup) Exist() bool {
	return true
}

// Create creates the node group on the cloud provider side. Implementation optional.
func (ng *nodeGroup) Create() (cloudprovider.NodeGroup, error) {
	return nil, cloudprovider.ErrNotImplemented
}

// Delete deletes the node group on the cloud provider side.
// This will be executed only for autoprovisioned node groups, once their size drops to 0.
// Implementation optional.
func (ng *nodeGroup) Delete() error {
	return cloudprovider.ErrNotImplemented
}

// Autoprovisioned returns true if the node group is autoprovisioned. An autoprovisioned group
// was created by CA and can be deleted when scaled to 0.
func (ng *nodeGroup) Autoprovisioned() bool {
	return false
}
