// +build serverscom

package builder

import (
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/serverscom"
	"k8s.io/autoscaler/cluster-autoscaler/config"
)

// AvailableCloudProviders supported by the cloud provider builder.
var AvailableCloudProviders = []string{
	cloudprovider.ServersComProviderName,
}

// DefaultCloudProvider for Servers.com-only build is Servers.com.
const DefaultCloudProvider = cloudprovider.ServersComProviderName

func buildCloudProvider(opts config.AutoscalingOptions, do cloudprovider.NodeGroupDiscoveryOptions, rl *cloudprovider.ResourceLimiter) cloudprovider.CloudProvider {
	switch opts.CloudProviderName {
	case cloudprovider.ServersComProviderName:
		return serverscom.BuildServersCom(opts, do, rl)
	}

	return nil
}
