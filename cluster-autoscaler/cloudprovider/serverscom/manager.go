package serverscom

import (
	"encoding/json"
	"fmt"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/networks"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/images"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/autoscaler/cluster-autoscaler/version"
	"k8s.io/klog"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const EnvRegionName = "OS_REGION_NAME"

const defaultNodeGroupMaxSize = 10

const (
	nodeProvisionTimeout   = 300 * time.Second
	nodeClusterJoinTimeout = 180 * time.Second
)

var ErrNotFound = errors.New("not found")

type manager interface {
	GetNodeGroups() ([]*nodeGroup, error)
	NodeGroupForNode(name string) (*nodeGroup, error)
	DeleteNode(node *apiv1.Node) error
	GetNodeStatus(node *apiv1.Node) (cloudprovider.InstanceStatus, error)
	GetKnownNetworks() ([]Network, error)
	CreateNodes(count int, ngName string) (int, error)
	Lock()
	Unlock()
}

type (
	managerServersCom struct {
		sync.Mutex
		osClient        *gophercloud.ServiceClient
		nodeGroups      map[string]*nodeGroup // map[ngName]nodeGroup
		scaleProperties scaleProperties
	}
	scaleProperties struct {
		defaultFlavor     string
		flavors           map[string]string         // map[ngName]flavor
		networksPrefixes  []string                  // [netPrefix,...]
		nodeGroups        map[string]nodeGroupProps // map[ngName]prop
		newNodeNamePrefix string
		imageName         string
	}
	managerConfig struct {
		DefaultFlavor     string                    `yaml:"default_flavor"       json:"default_flavor"`
		Flavors           map[string]string         `yaml:"flavors"              json:"flavors"` // map[ngName]flavor
		NetworkPrefixes   []string                  `yaml:"network_prefixes"     json:"network_prefixes"`
		AuthCredsFromEnv  bool                      `yaml:"auth_creds_from_env"  json:"auth_creds_from_env"`
		NodeGroups        map[string]nodeGroupProps `yaml:"node_groups"          json:"node_groups"`
		NewNodeNamePrefix string                    `yaml:"new_node_name_prefix" json:"new_node_name_prefix"`
		Image             string                    `yaml:"image"                json:"image"`
		Credentials       credentials               `yaml:"credentials"          json:"credentials"`
	}

	credentials struct {
		AuthUrl    string `yaml:"auth_url"    json:"auth_url"`
		TenantName string `yaml:"tenant_name" json:"tenant_name"`
		Password   string `yaml:"password"    json:"password"`
		Username   string `yaml:"username"    json:"username"`
		DomainName string `yaml:"domain_name" json:"domain_name"`
	}

	nodeGroupProps struct {
		Min int `yaml:"min" json:"min"`
		Max int `yaml:"max" json:"max"`
	}

	Network struct {
		servers.Network
		Label string
	}
)

func NewConfig(configReader io.Reader) (managerConfig, error) {
	var cfg managerConfig
	cfgBytes, err := ioutil.ReadAll(configReader)
	if err != nil {
		return cfg, errors.Wrap(err, "read config error")
	}

	// if config starts from { or [ consider it is json
	if first := string(cfgBytes)[0]; first == '[' || first == '{' {
		err = json.Unmarshal(cfgBytes, &cfg)
	} else {
		err = yaml.Unmarshal(cfgBytes, &cfg)
	}

	if err != nil {
		return cfg, errors.Wrap(err, "unmarshal config error")
	}

	if cfg.DefaultFlavor == "" {
		return cfg, errors.Wrap(err, "default flavor should be set")
	}

	return cfg, nil
}

func newManager(configReader io.Reader) (*managerServersCom, error) {
	if configReader == nil {
		return nil, errors.New("config is nil")
	}

	cfg, err := NewConfig(configReader)
	if err != nil {
		return nil, errors.Wrap(err, "create config error")
	}

	var opts gophercloud.AuthOptions
	if cfg.AuthCredsFromEnv {
		var err error
		opts, err = openstack.AuthOptionsFromEnv()
		if err != nil {
			return nil, fmt.Errorf("get auth options from env error: %v", err)
		}
	} else {
		opts = gophercloud.AuthOptions{
			IdentityEndpoint: cfg.Credentials.AuthUrl,
			TenantName:       cfg.Credentials.TenantName,
			Password:         cfg.Credentials.Password,
			Username:         cfg.Credentials.Username,
			DomainName:       cfg.Credentials.DomainName,
		}
	}

	provider, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		return nil, fmt.Errorf("get authentificated client error: %v", err)
	}

	userAgent := gophercloud.UserAgent{}
	userAgent.Prepend(fmt.Sprintf("cluster-autoscaler/%s", version.ClusterAutoscalerVersion))
	provider.UserAgent = userAgent

	client, err := openstack.NewComputeV2(provider, gophercloud.EndpointOpts{
		Region: os.Getenv(EnvRegionName),
	})
	if err != nil {
		return nil, fmt.Errorf("get service client v2 error: %v", err)
	}

	// fix zero values
	for ngName := range cfg.NodeGroups {
		if cfg.NodeGroups[ngName].Max == 0 {
			p := cfg.NodeGroups[ngName]
			p.Max = defaultNodeGroupMaxSize
			cfg.NodeGroups[ngName] = p
		}
	}

	return &managerServersCom{
		osClient: client,
		scaleProperties: scaleProperties{
			defaultFlavor:     cfg.DefaultFlavor,
			flavors:           cfg.Flavors,
			networksPrefixes:  cfg.NetworkPrefixes,
			nodeGroups:        cfg.NodeGroups,
			imageName:         cfg.Image,
			newNodeNamePrefix: cfg.NewNodeNamePrefix,
		},
		nodeGroups: make(map[string]*nodeGroup),
	}, nil
}

func (m *managerServersCom) GetNodeGroups() ([]*nodeGroup, error) {
	nodeGroups := make(map[string]*nodeGroup)

	for ngName, ngProps := range m.scaleProperties.nodeGroups {
		nodeGroups[ngName] = &nodeGroup{
			id: ngName,
			size: &nodeGroupSize{
				min:    ngProps.Min,
				max:    ngProps.Max,
				target: 0,
			},
			nodes:   []cloudprovider.Instance{},
			manager: m,
		}
	}

	// get actual servers servers and build ng group
	servers, err := m.getAllNodes()
	if err != nil {
		return nil, fmt.Errorf("get all nodes error=%v", err)
	}

	for _, srv := range servers {
		ngName := m.nodeGroupName(srv.Name)
		if ngName == "" {
			continue
		}

		if _, ok := nodeGroups[ngName]; ok {
			status, ok := statusMapping[srv.Status]
			if !ok {
				status = nodeStatusRunning
				status.ErrorInfo = &cloudprovider.InstanceErrorInfo{ErrorMessage: "unknown instance state=" + srv.Status}
			}

			nodeGroups[ngName].size.target++
			nodeGroups[ngName].nodes = append(nodeGroups[ngName].nodes, cloudprovider.Instance{Id: srv.Name, Status: &status})
		}
	}

	res := make([]*nodeGroup, 0, len(nodeGroups))
	for _, ng := range nodeGroups {
		klog.V(4).Infof("found nodegroup %s, min=%d max=%d target=%d", ng.id, ng.size.min, ng.size.max, ng.size.target)
		res = append(res, ng)
	}

	m.Lock()
	defer m.Unlock()
	m.nodeGroups = nodeGroups

	return res, nil
}

func (m *managerServersCom) getAllNodes() ([]servers.Server, error) {
	// todo think about how to not get all nodes each time
	allPages, err := servers.List(m.osClient, servers.ListOpts{}).AllPages()
	if err != nil {

		return nil, err
	}

	return servers.ExtractServers(allPages)
}

var nodeGroupHostNameRegex = regexp.MustCompile(`kube\d*-(\w+)-`) // todo set from the config

func (m *managerServersCom) nodeGroupName(hostname string) string {
	if strings.Contains(hostname, m.scaleProperties.newNodeNamePrefix) {
		return strings.Split(
			strings.TrimPrefix(hostname, m.scaleProperties.newNodeNamePrefix+"-"),
			"-",
		)[0]
	}

	res := nodeGroupHostNameRegex.FindStringSubmatch(hostname)
	if len(res) == 2 {
		return res[1]
	}

	return ""
}

func (m *managerServersCom) NodeGroupForNode(hostname string) (*nodeGroup, error) {
	m.Lock()
	defer m.Unlock()

	ngName := m.nodeGroupName(hostname)
	n, ok := m.nodeGroups[ngName]
	if !ok {
		return n, fmt.Errorf("unknown node group=%s", ngName)
	}

	return n, nil
}

func (m *managerServersCom) getNodeID(name string) (string, error) {
	pag := servers.List(m.osClient, servers.ListOpts{Name: name})
	allPages, err := pag.AllPages()
	if err != nil {
		return "", errors.Wrap(err, "get server error")
	}
	allServers, err := servers.ExtractServers(allPages)
	if err != nil {
		return "", errors.Wrap(err, "extract servers error")
	}

	if len(allServers) > 1 {
		return "", errors.New("more than one server found for the provided name")
	}

	if len(allServers) == 0 {
		return "", ErrNotFound
	}

	return allServers[0].ID, nil
}

func (m *managerServersCom) GetKnownNetworks() ([]Network, error) {
	if len(m.scaleProperties.networksPrefixes) == 0 {
		return nil, errors.New("networks list is undefined")
	}

	var nets []Network
	allPages, err := networks.List(m.osClient).AllPages()
	if err != nil {
		return nil, errors.Wrap(err, "list networks error")
	}
	allNetworks, err := networks.ExtractNetworks(allPages)
	if err != nil {
		return nil, errors.Wrap(err, "extract networks error")
	}

nextPrefix:
	for _, prefix := range m.scaleProperties.networksPrefixes {
		for _, net := range allNetworks {
			if strings.Contains(net.Label, prefix) {
				nets = append(nets, Network{Network:
				servers.Network{
					UUID: net.ID,
				},
					Label: net.Label,
				})

				continue nextPrefix
			}
		}
	}

	return nets, nil
}

func (m *managerServersCom) DeleteNode(node *apiv1.Node) error {
	id, err := m.getNodeID(node.Name)
	if err != nil && err != ErrNotFound {
		return errors.Wrap(err, "get node id error")
	}

	res := servers.Delete(m.osClient, id)
	if res.Err != nil {
		return errors.Wrap(res.Err, "delete node error")
	}

	return nil
}

func (m *managerServersCom) getImageRef(name string) (string, error) {
	pages, err := images.ListDetail(m.osClient, &images.ListOpts{
		Limit:  1,
		Status: "active",
		Name:   m.scaleProperties.imageName,
	}).AllPages()
	if err != nil {
		return "", errors.Wrap(err, "get images error")
	}

	images, err := images.ExtractImages(pages)
	if err != nil {
		return "", errors.Wrap(err, "extract images error")
	}

	imagesMap := make(map[string]string)
	lastUpdated := ""
	for _, image := range images {
		if image.Name != name {
			continue
		}
		if image.Created > lastUpdated {
			lastUpdated = image.Created
		}
		imagesMap[image.Created] = image.ID
	}

	if lastUpdated == "" {
		return "", fmt.Errorf("cannot find image with name=%s", name)
	}

	return imagesMap[lastUpdated], nil
}

func (m *managerServersCom) CreateNodes(count int, ngName string) (int, error) {
	var (
		wg      sync.WaitGroup
		errList []string
		mtx     sync.Mutex
		created int64
	)

	m.Lock()
	ngLen := len(m.nodeGroups[ngName].nodes)
	m.Unlock()

	for i := 0; i < count; i++ {
		newNodeName := fmt.Sprintf("%s-%s-%d", m.scaleProperties.newNodeNamePrefix, ngName, ngLen+1+i)

		wg.Add(1)
		go func() {
			defer wg.Done()
			err := m.createNode(ngName, newNodeName)
			if err != nil {
				mtx.Lock()
				errList = append(errList, err.Error())
				mtx.Unlock()
				return
			}

			atomic.AddInt64(&created, 1)
		}()
	}

	wg.Wait()

	if len(errList) != 0 {
		return int(created), errors.New(strings.Join(errList, "; "))
	}

	return int(created), nil
}

func (m *managerServersCom) createNode(ngName, nodeName string) error {

	flavor := m.scaleProperties.flavors[ngName]
	if flavor == "" {
		flavor = m.scaleProperties.defaultFlavor
	}

	networks, err := m.GetKnownNetworks()
	if err != nil {
		return errors.Wrap(err, "get networks error")
	}

	imageRef, err := m.getImageRef(m.scaleProperties.imageName)
	if err != nil {
		return errors.Wrap(err, "get image ref error")
	}

	opts := servers.CreateOpts{
		Name:          nodeName,
		ImageRef:      imageRef,
		FlavorName:    flavor,
		ServiceClient: m.osClient,
	}

	for _, n := range networks {
		opts.Networks = append(opts.Networks, n.Network)
	}

	klog.V(4).Infof("creating node with params=(%s)", strCreateOpts(opts))

	res := servers.Create(m.osClient, opts)

	if res.Err != nil {
		return errors.Wrap(res.Err, "create server error")
	}

	srv, err := res.Extract()
	if err != nil {
		return errors.Wrap(err, "extract server info error")
	}

	timer := time.NewTimer(nodeProvisionTimeout)

loop:
	for {
		select {
		case <-timer.C:
			break loop
		default:
			status, err := m.GetNodeStatusByID(srv.ID)
			if err != nil {
				klog.V(0).Infof("node '%s': failed to check status", nodeName)
			}

			if status.ErrorInfo != nil {
				klog.V(2).Infof("node '%s': status is abnormal: %v", nodeName, status.ErrorInfo.ErrorMessage)
			} else {
				klog.V(2).Infof("node '%s': status=%s", nodeName, strState(status.State))
			}
			if status == nodeStatusRunning {
				klog.V(2).Infof("node '%s': waiting for %s to allow node to join the cluster...",
					nodeName, nodeClusterJoinTimeout)
				time.Sleep(nodeClusterJoinTimeout)

				klog.V(2).Infof("consider that node is successfully bootstrappped")
				return nil
			}

			time.Sleep(time.Second)
		}
	}

	return errors.New("node " + nodeName + " did not reach active state")
}

var (
	NodeNotFound = errors.New("node was not found")
)

func (m *managerServersCom) GetNodeStatus(node *apiv1.Node) (cloudprovider.InstanceStatus, error) {
	id, err := m.getNodeID(node.Name)
	if err != nil {
		return cloudprovider.InstanceStatus{}, errors.Wrap(err, "get node id error")
	}

	return m.GetNodeStatusByID(id)
}

func (m *managerServersCom) GetNodeStatusByID(id string) (cloudprovider.InstanceStatus, error) {
	var ret cloudprovider.InstanceStatus
	res := servers.Get(m.osClient, id)
	if _, ok := res.Result.Err.(gophercloud.ErrDefault404); ok {
		return ret, NodeNotFound
	}

	srv, err := res.Extract()
	if err != nil {
		return ret, errors.Wrap(err, "extract server error")
	}

	if status, ok := statusMapping[srv.Status]; ok {
		return status, nil
	} else {
		ret.ErrorInfo = &cloudprovider.InstanceErrorInfo{
			ErrorClass:   cloudprovider.OtherErrorClass,
			ErrorMessage: "abnormal status " + srv.Status,
		}
		return ret, nil
	}

	return ret, nil
}

var (
	nodeStatusCreating = cloudprovider.InstanceStatus{
		State:     cloudprovider.InstanceCreating,
		ErrorInfo: nil,
	}
	nodeStatusRunning = cloudprovider.InstanceStatus{
		State:     cloudprovider.InstanceRunning,
		ErrorInfo: nil,
	}
	nodeStatusDeleting = cloudprovider.InstanceStatus{
		State:     cloudprovider.InstanceDeleting,
		ErrorInfo: nil,
	}
	nodeStatusError = cloudprovider.InstanceStatus{
		ErrorInfo: nil,
	}
)

// https://docs.openstack.org/api-guide/compute/server_concepts.html
var statusMapping = map[string]cloudprovider.InstanceStatus{
	"BUILD":        nodeStatusCreating,
	"ACTIVE":       nodeStatusRunning,
	"SOFT_DELETED": nodeStatusDeleting,
}

func strState(st cloudprovider.InstanceState) string {
	ret := "Unknown"
	switch st {
	case cloudprovider.InstanceCreating:
		ret = "Creating"
	case cloudprovider.InstanceRunning:
		ret = "Running"
	case cloudprovider.InstanceDeleting:
		ret = "Deleting"
	}
	return ret
}

func strCreateOpts(opts servers.CreateOpts) string {
	return fmt.Sprintf("name=%s imageref=%s flavor=%s networks_count=%d",
		opts.Name,
		opts.ImageRef,
		opts.FlavorName,
		len(opts.Networks))
}
