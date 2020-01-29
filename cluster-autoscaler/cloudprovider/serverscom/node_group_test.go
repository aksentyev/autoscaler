package serverscom

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"testing"
)

func TestNodeGroup_LastScaledNodeID(t *testing.T) {
	ng := nodeGroup{
		id: "results",
		manager: &managerServersCom{
			scaleProperties: scaleProperties{
				newNodeNamePrefix: "kube2-scaler-ams"},
		},
	}

	ng.nodes = []cloudprovider.Instance{
		{Id: "kube2-scaler-ams-results-5"},
		{Id: "kube2-scaler-ams-results-6"},
	}

	res, err := ng.LastScaledNodeID()
	assert.NoError(t, err)
	assert.Equal(t, 6, res)
}

func TestNodeGroup_NewPostfix(t *testing.T) {
	c := 0
	generator := func() string {
		defer func() { c++ }()
		return fmt.Sprintf("test-%d", c)
	}

	ng := new(nodeGroup)
	ng.nodes = append(ng.nodes,
		cloudprovider.Instance{Id: "test-0"},
		cloudprovider.Instance{Id: "test-1"},
	)
	name := ng.newPostfix(generator, "kube2-scaler-ams-worker-test-2")
	assert.Equal(t, "test-3", name)
}
