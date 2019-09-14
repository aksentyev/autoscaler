package serverscom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/testhelper"
	"github.com/gophercloud/gophercloud/testhelper/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

func TestNewConfig(t *testing.T) {
	t.Run("yaml", func(t *testing.T) {
		r := bytes.NewBuffer([]byte(`
default_flavor: SSD.80
network_prefixes:
 - global_private
 - local_private
node_groups:
 worker:
`))
		cfg, err := NewConfig(r)
		assert.NoError(t, err)
		assert.NotEmpty(t, cfg.DefaultFlavor)
		assert.Len(t, cfg.NetworkPrefixes, 2)
		assert.Len(t, cfg.NodeGroups, 1)
	})

	t.Run("json", func(t *testing.T) {
		r := bytes.NewBuffer([]byte(`{"default_flavor": "SSD.80", "network_prefixes": [ "global_private", "local_private" ], "auth_creds_from_env": false, "node_groups": {"worker": {}}, "new_node_name_prefix": "kube2-scaler-mow-", "image": "scalermow", "credentials": { "auth_url": "https://auth.servers.us01.cloud.servers.com:5000/v3/", "tenant_name": "1000", "password": "4321", "username": "1234", "domain_name": "default" }}`))

		cfg, err := NewConfig(r)
		assert.NoError(t, err)
		assert.NotEmpty(t, cfg.DefaultFlavor)
		assert.Len(t, cfg.NetworkPrefixes, 2)
		assert.Len(t, cfg.NodeGroups, 1)
	})
}

func TestManager_GetKnownNetworks(t *testing.T) {
	testhelper.SetupHTTP()
	defer testhelper.TeardownHTTP()

	testhelper.Mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println(r.URL)
	})

	testhelper.Mux.HandleFunc(urlOsNetworks, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		fmt.Fprint(w, respOsNetworks)
	})

	m := newTestManager(client.ServiceClient())

	nets, err := m.GetKnownNetworks()
	assert.NoError(t, err)

	require.Len(t, nets, 3)
	assert.Equal(t, "0a2844d4-114e-48cd-9997-0cd2d95191e7", nets[0].UUID)
	assert.Equal(t, "global_private", nets[0].Label)
	assert.Equal(t, "2d933a6e-d5d0-4511-a2f9-9b9b95086491", nets[1].UUID)
	assert.Equal(t, "local_private", nets[1].Label)
	assert.Equal(t, "0b1b8097-fa5a-4041-bf83-76e03aa4bab3", nets[2].UUID)
	assert.Equal(t, "internet_23.105.240.64/27", nets[2].Label)
}

func TestManager_GetNodeGroups(t *testing.T) {
	testhelper.SetupHTTP()
	defer testhelper.TeardownHTTP()

	testhelper.Mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println(r.URL)
	})

	testhelper.Mux.HandleFunc(urlServersDetail, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		fmt.Fprint(w, respServersDetail)
	})

	m := newTestManager(client.ServiceClient())
	m.scaleProperties.nodeGroups = map[string]nodeGroupProps{
		"kafka": {Min: 2, Max: 5},
	}
	m.scaleProperties.newNodeNamePrefix = "kube2-mow"

	groups, err := m.GetNodeGroups()
	assert.NoError(t, err)

	require.Len(t, groups, 1)
	ng := groups[0]
	assert.Equal(t, "kafka", ng.id)
	assert.Equal(t, 3, ng.size.target)
	assert.Equal(t, 2, ng.size.min)
	assert.Equal(t, 5, ng.size.max)
	assert.Len(t, ng.nodes, 3)
}

func TestManager_NodeGroupForNode(t *testing.T) {
	m := managerServersCom{}
	m.scaleProperties.newNodeNamePrefix = "kube2-scaler-mow"
	m.nodeGroups = map[string]*nodeGroup{
		"worker": {
			id: "worker",
		},
	}

	ng, err := m.NodeGroupForNode("kube2-scaler-mow-worker-91e12557-732f-41f0-a23c-93fd7acda037")

	require.NoError(t, err)
	assert.Equal(t, "worker", ng.id)
}

func TestManagerConfig(t *testing.T) {
	printJson(t, managerConfig{
		NetworkPrefixes: []string{
			"global_private",
			"local_private",
		},
	})
}

func newTestManager(osClient *gophercloud.ServiceClient) managerServersCom {
	return managerServersCom{
		osClient:   osClient,
		nodeGroups: map[string]*nodeGroup{},
		scaleProperties: scaleProperties{
			defaultFlavor: "",
			flavors:       nil,
			networksPrefixes: []string{
				"global_private",
				"local_private",
				"internet",
			},
		},
	}
}

func printJson(t *testing.T, v interface{}) {
	b, err := json.MarshalIndent(v, "", "  ")
	assert.NoError(t, err)
	fmt.Println(string(b))
}

var (
	urlServersDetail  = "/servers/detail"
	respServersDetail = `
{"servers": [{"OS-EXT-STS:task_state": null, "addresses": {"local_private": [{"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:23:d7:32", "version": 4, "addr": "192.168.0.18", "OS-EXT-IPS:type": "fixed"}], "global_private": [{"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:84:fa:68", "version": 4, "addr": "10.214.3.13", "OS-EXT-IPS:type": "fixed"}], "internet_188.42.181.128/27": [{"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:8e:85:6e", "version": 4, "addr": "188.42.181.148", "OS-EXT-IPS:type": "fixed"}]}, "links": [{"href": "https://compute.servers.mo01.cloud.servers.com:8774/v2.1/servers/7f8c3960-f801-41f2-9153-7cfc4b54c78e", "rel": "self"}, {"href": "https://compute.servers.mo01.cloud.servers.com:8774/servers/7f8c3960-f801-41f2-9153-7cfc4b54c78e", "rel": "bookmark"}], "image": {"id": "608c93b1-8f31-4738-b1b7-e2de62c366d1", "links": [{"href": "https://compute.servers.mo01.cloud.servers.com:8774/images/608c93b1-8f31-4738-b1b7-e2de62c366d1", "rel": "bookmark"}]}, "OS-EXT-STS:vm_state": "active", "OS-SRV-USG:launched_at": "2018-12-12T13:12:48.000000", "flavor": {"id": "80", "links": [{"href": "https://compute.servers.mo01.cloud.servers.com:8774/flavors/80", "rel": "bookmark"}]}, "id": "7f8c3960-f801-41f2-9153-7cfc4b54c78e", "user_id": "00e0263a4eef47139393f85b6ba7f949", "OS-DCF:diskConfig": "MANUAL", "accessIPv4": "", "accessIPv6": "", "progress": 0, "OS-EXT-STS:power_state": 1, "OS-EXT-AZ:availability_zone": "nova", "config_drive": "", "status": "ACTIVE", "updated": "2018-12-13T13:11:19Z", "hostId": "6e63fb111bbf6e19ac839a56d2fdcb040f3063b5e238a7b9cf7d4e34", "OS-SRV-USG:terminated_at": null, "key_name": "ekoz-aviasales", "name": "kube2-master-mow-3.int.avs.io", "created": "2018-12-12T13:12:15Z", "tenant_id": "ef95ebe262484a7a95605d43d0c0d187", "os-extended-volumes:volumes_attached": [], "metadata": {}}, {"OS-EXT-STS:task_state": null, "addresses": {"local_private": [{"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:bd:ab:97", "version": 4, "addr": "192.168.0.17", "OS-EXT-IPS:type": "fixed"}], "global_private": [{"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:bb:c4:c7", "version": 4, "addr": "10.214.3.12", "OS-EXT-IPS:type": "fixed"}], "internet_23.105.240.32/27": [{"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:28:5e:6e", "version": 4, "addr": "23.105.240.35", "OS-EXT-IPS:type": "fixed"}]}, "links": [{"href": "https://compute.servers.mo01.cloud.servers.com:8774/v2.1/servers/1eacd1cf-ec20-4cb0-b12f-437b10429fe6", "rel": "self"}, {"href": "https://compute.servers.mo01.cloud.servers.com:8774/servers/1eacd1cf-ec20-4cb0-b12f-437b10429fe6", "rel": "bookmark"}], "image": {"id": "608c93b1-8f31-4738-b1b7-e2de62c366d1", "links": [{"href": "https://compute.servers.mo01.cloud.servers.com:8774/images/608c93b1-8f31-4738-b1b7-e2de62c366d1", "rel": "bookmark"}]}, "OS-EXT-STS:vm_state": "active", "OS-SRV-USG:launched_at": "2018-12-12T13:12:12.000000", "flavor": {"id": "80", "links": [{"href": "https://compute.servers.mo01.cloud.servers.com:8774/flavors/80", "rel": "bookmark"}]}, "id": "1eacd1cf-ec20-4cb0-b12f-437b10429fe6", "user_id": "00e0263a4eef47139393f85b6ba7f949", "OS-DCF:diskConfig": "MANUAL", "accessIPv4": "", "accessIPv6": "", "progress": 0, "OS-EXT-STS:power_state": 1, "OS-EXT-AZ:availability_zone": "nova", "config_drive": "", "status": "ACTIVE", "updated": "2018-12-13T13:10:42Z", "hostId": "a224a49f9d5471af9bc2fe23e7d522d2b9db99a8aa93aefddd7e8158", "OS-SRV-USG:terminated_at": null, "key_name": "ekoz-aviasales", "name": "kube2-master-mow-2.int.avs.io", "created": "2018-12-12T13:11:43Z", "tenant_id": "ef95ebe262484a7a95605d43d0c0d187", "os-extended-volumes:volumes_attached": [], "metadata": {}}, {"OS-EXT-STS:task_state": null, "addresses": {"local_private": [{"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:c4:a8:3d", "version": 4, "addr": "192.168.0.15", "OS-EXT-IPS:type": "fixed"}], "global_private": [{"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:a5:7f:c2", "version": 4, "addr": "10.214.3.10", "OS-EXT-IPS:type": "fixed"}], "internet_188.42.181.128/27": [{"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:10:91:7e", "version": 4, "addr": "188.42.181.137", "OS-EXT-IPS:type": "fixed"}]}, "links": [{"href": "https://compute.servers.mo01.cloud.servers.com:8774/v2.1/servers/f2ed8cba-b776-4829-84d1-34d6de927675", "rel": "self"}, {"href": "https://compute.servers.mo01.cloud.servers.com:8774/servers/f2ed8cba-b776-4829-84d1-34d6de927675", "rel": "bookmark"}], "image": {"id": "608c93b1-8f31-4738-b1b7-e2de62c366d1", "links": [{"href": "https://compute.servers.mo01.cloud.servers.com:8774/images/608c93b1-8f31-4738-b1b7-e2de62c366d1", "rel": "bookmark"}]}, "OS-EXT-STS:vm_state": "active", "OS-SRV-USG:launched_at": "2018-12-12T13:03:47.000000", "flavor": {"id": "80", "links": [{"href": "https://compute.servers.mo01.cloud.servers.com:8774/flavors/80", "rel": "bookmark"}]}, "id": "f2ed8cba-b776-4829-84d1-34d6de927675", "user_id": "00e0263a4eef47139393f85b6ba7f949", "OS-DCF:diskConfig": "MANUAL", "accessIPv4": "", "accessIPv6": "", "progress": 0, "OS-EXT-STS:power_state": 1, "OS-EXT-AZ:availability_zone": "nova", "config_drive": "", "status": "ACTIVE", "updated": "2019-11-20T15:35:47Z", "hostId": "a224a49f9d5471af9bc2fe23e7d522d2b9db99a8aa93aefddd7e8158", "OS-SRV-USG:terminated_at": null, "key_name": "ekoz-aviasales", "name": "kube2-master-mow-1.int.avs.io", "created": "2018-12-12T13:03:05Z", "tenant_id": "ef95ebe262484a7a95605d43d0c0d187", "os-extended-volumes:volumes_attached": [], "metadata": {}}, {"OS-EXT-STS:task_state": null, "addresses": {"local_private": [{"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:a5:17:7c", "version": 4, "addr": "192.168.0.14", "OS-EXT-IPS:type": "fixed"}], "global_private": [{"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:32:9a:b4", "version": 4, "addr": "10.214.3.9", "OS-EXT-IPS:type": "fixed"}], "internet_188.42.181.128/27": [{"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:a1:21:51", "version": 4, "addr": "188.42.181.147", "OS-EXT-IPS:type": "fixed"}]}, "links": [{"href": "https://compute.servers.mo01.cloud.servers.com:8774/v2.1/servers/9a90a932-67a0-44d3-b16d-7b9d7bae3879", "rel": "self"}, {"href": "https://compute.servers.mo01.cloud.servers.com:8774/servers/9a90a932-67a0-44d3-b16d-7b9d7bae3879", "rel": "bookmark"}], "image": {"id": "af7a4250-7ef5-457d-9c0c-ea5987b81a30", "links": [{"href": "https://compute.servers.mo01.cloud.servers.com:8774/images/af7a4250-7ef5-457d-9c0c-ea5987b81a30", "rel": "bookmark"}]}, "OS-EXT-STS:vm_state": "active", "OS-SRV-USG:launched_at": "2018-10-23T11:39:54.000000", "flavor": {"id": "80", "links": [{"href": "https://compute.servers.mo01.cloud.servers.com:8774/flavors/80", "rel": "bookmark"}]}, "id": "9a90a932-67a0-44d3-b16d-7b9d7bae3879", "user_id": "cfd4e6abddfb4cf1b6510ecc4fa81898", "OS-DCF:diskConfig": "MANUAL", "accessIPv4": "", "accessIPv6": "", "progress": 0, "OS-EXT-STS:power_state": 1, "OS-EXT-AZ:availability_zone": "nova", "config_drive": "", "status": "ACTIVE", "updated": "2018-10-26T11:40:26Z", "hostId": "a224a49f9d5471af9bc2fe23e7d522d2b9db99a8aa93aefddd7e8158", "OS-SRV-USG:terminated_at": null, "key_name": "makhortov", "name": "kube2-kafka-mow-3.int.avs.io", "created": "2018-10-16T12:17:03Z", "tenant_id": "ef95ebe262484a7a95605d43d0c0d187", "os-extended-volumes:volumes_attached": [], "metadata": {}}, {"OS-EXT-STS:task_state": null, "addresses": {"local_private": [{"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:7b:74:2f", "version": 4, "addr": "192.168.0.13", "OS-EXT-IPS:type": "fixed"}], "global_private": [{"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:a6:64:98", "version": 4, "addr": "10.214.3.8", "OS-EXT-IPS:type": "fixed"}], "internet_188.42.181.128/27": [{"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:6f:fd:e4", "version": 4, "addr": "188.42.181.144", "OS-EXT-IPS:type": "fixed"}]}, "links": [{"href": "https://compute.servers.mo01.cloud.servers.com:8774/v2.1/servers/19e2afc4-f100-4af4-a091-55fd2c500b77", "rel": "self"}, {"href": "https://compute.servers.mo01.cloud.servers.com:8774/servers/19e2afc4-f100-4af4-a091-55fd2c500b77", "rel": "bookmark"}], "image": {"id": "af7a4250-7ef5-457d-9c0c-ea5987b81a30", "links": [{"href": "https://compute.servers.mo01.cloud.servers.com:8774/images/af7a4250-7ef5-457d-9c0c-ea5987b81a30", "rel": "bookmark"}]}, "OS-EXT-STS:vm_state": "active", "OS-SRV-USG:launched_at": "2018-10-23T11:41:25.000000", "flavor": {"id": "80", "links": [{"href": "https://compute.servers.mo01.cloud.servers.com:8774/flavors/80", "rel": "bookmark"}]}, "id": "19e2afc4-f100-4af4-a091-55fd2c500b77", "user_id": "cfd4e6abddfb4cf1b6510ecc4fa81898", "OS-DCF:diskConfig": "MANUAL", "accessIPv4": "", "accessIPv6": "", "progress": 0, "OS-EXT-STS:power_state": 1, "OS-EXT-AZ:availability_zone": "nova", "config_drive": "", "status": "ACTIVE", "updated": "2018-12-04T03:17:22Z", "hostId": "08414e0a4e81380e00fd099ac17eed7b353394d830f7628fa825a481", "OS-SRV-USG:terminated_at": null, "key_name": "makhortov", "name": "kube2-kafka-mow-2.int.avs.io", "created": "2018-10-16T12:16:48Z", "tenant_id": "ef95ebe262484a7a95605d43d0c0d187", "os-extended-volumes:volumes_attached": [], "metadata": {}}, {"OS-EXT-STS:task_state": null, "addresses": {"local_private": [{"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:60:c7:7e", "version": 4, "addr": "192.168.0.12", "OS-EXT-IPS:type": "fixed"}], "global_private": [{"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:01:b0:a8", "version": 4, "addr": "10.214.3.7", "OS-EXT-IPS:type": "fixed"}], "internet_88.212.241.224/27": [{"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:21:70:2f", "version": 4, "addr": "88.212.241.245", "OS-EXT-IPS:type": "fixed"}]}, "links": [{"href": "https://compute.servers.mo01.cloud.servers.com:8774/v2.1/servers/2571b52a-9adc-4359-8028-ece2d6656321", "rel": "self"}, {"href": "https://compute.servers.mo01.cloud.servers.com:8774/servers/2571b52a-9adc-4359-8028-ece2d6656321", "rel": "bookmark"}], "image": {"id": "af7a4250-7ef5-457d-9c0c-ea5987b81a30", "links": [{"href": "https://compute.servers.mo01.cloud.servers.com:8774/images/af7a4250-7ef5-457d-9c0c-ea5987b81a30", "rel": "bookmark"}]}, "OS-EXT-STS:vm_state": "active", "OS-SRV-USG:launched_at": "2018-10-23T11:38:27.000000", "flavor": {"id": "80", "links": [{"href": "https://compute.servers.mo01.cloud.servers.com:8774/flavors/80", "rel": "bookmark"}]}, "id": "2571b52a-9adc-4359-8028-ece2d6656321", "user_id": "cfd4e6abddfb4cf1b6510ecc4fa81898", "OS-DCF:diskConfig": "MANUAL", "accessIPv4": "", "accessIPv6": "", "progress": 0, "OS-EXT-STS:power_state": 1, "OS-EXT-AZ:availability_zone": "nova", "config_drive": "", "status": "ACTIVE", "updated": "2018-12-04T03:32:51Z", "hostId": "08414e0a4e81380e00fd099ac17eed7b353394d830f7628fa825a481", "OS-SRV-USG:terminated_at": null, "key_name": "makhortov", "name": "kube2-kafka-mow-1.int.avs.io", "created": "2018-10-16T12:16:19Z", "tenant_id": "ef95ebe262484a7a95605d43d0c0d187", "os-extended-volumes:volumes_attached": [], "metadata": {}}, {"OS-EXT-STS:task_state": null, "addresses": {"local_private": [{"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:21:05:b5", "version": 4, "addr": "192.168.0.6", "OS-EXT-IPS:type": "fixed"}], "global_private": [{"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:44:37:5c", "version": 4, "addr": "10.214.3.6", "OS-EXT-IPS:type": "fixed"}], "internet_88.212.241.224/27": [{"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:df:14:b1", "version": 4, "addr": "88.212.241.243", "OS-EXT-IPS:type": "fixed"}]}, "links": [{"href": "https://compute.servers.mo01.cloud.servers.com:8774/v2.1/servers/66db195b-5db5-42a4-8148-409814f4841c", "rel": "self"}, {"href": "https://compute.servers.mo01.cloud.servers.com:8774/servers/66db195b-5db5-42a4-8148-409814f4841c", "rel": "bookmark"}], "image": {"id": "15f38d85-a9fb-469c-8984-a2a05acd936e", "links": [{"href": "https://compute.servers.mo01.cloud.servers.com:8774/images/15f38d85-a9fb-469c-8984-a2a05acd936e", "rel": "bookmark"}]}, "OS-EXT-STS:vm_state": "active", "OS-SRV-USG:launched_at": "2016-10-18T18:44:08.000000", "flavor": {"id": "120", "links": [{"href": "https://compute.servers.mo01.cloud.servers.com:8774/flavors/120", "rel": "bookmark"}]}, "id": "66db195b-5db5-42a4-8148-409814f4841c", "user_id": "00e0263a4eef47139393f85b6ba7f949", "OS-DCF:diskConfig": "MANUAL", "accessIPv4": "", "accessIPv6": "", "progress": 0, "OS-EXT-STS:power_state": 1, "OS-EXT-AZ:availability_zone": "nova", "config_drive": "", "status": "ACTIVE", "updated": "2018-07-04T15:12:41Z", "hostId": "9c64543598335f559468c068d7947a22a08227a071a4eae13abbe76d", "OS-SRV-USG:terminated_at": null, "key_name": "ekoz-aviasales", "name": "launchkit.int.avs.io", "created": "2016-10-18T18:43:51Z", "tenant_id": "ef95ebe262484a7a95605d43d0c0d187", "os-extended-volumes:volumes_attached": [], "metadata": {}}]}
`
)

var (
	urlOsNetworks  = "/os-networks"
	respOsNetworks = `
{"networks": [{"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "global_private", "id": "0a2844d4-114e-48cd-9997-0cd2d95191e7", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_23.105.240.64/27", "id": "0b1b8097-fa5a-4041-bf83-76e03aa4bab3", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_88.212.241.96/27", "id": "134ca3d6-b6f8-4bae-aadc-4c13219b7b2d", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_88.212.241.32/27", "id": "22840db2-b9cc-4785-a2ad-462dc996271a", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "local_private", "id": "2d933a6e-d5d0-4511-a2f9-9b9b95086491", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_188.42.181.192/27", "id": "38d66238-f8d9-434c-bb72-8d948f8c3fab", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_88.212.238.192/27", "id": "42bae52a-ae02-4ea8-9404-0b4dacd83154", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_23.105.225.224/27", "id": "46846924-1394-45d9-a675-b3df53f8adda", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_23.105.240.0/27", "id": "48415712-a435-4d59-a0b6-9f454a34e97e", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_23.105.225.160/27", "id": "57789992-d8f9-4558-97d0-d16246079dc9", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_23.105.240.96/27", "id": "65ba4421-bd65-455c-aa75-5fb36a4fdc6d", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_23.105.231.96/27", "id": "6b5b99b9-b0df-40ab-b6bb-3540caf790b3", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_23.105.225.128/27", "id": "7379c937-824a-48bc-abe8-52ebcc9a3890", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_88.212.238.128/27", "id": "813cd43a-276e-461a-99b6-d0e37356d3d8", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_88.212.238.96/27", "id": "8cbbaa1b-a4c9-4818-bfbf-f24cd3c2f423", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_88.212.241.224/27", "id": "8e94804b-63a6-4bab-befd-9bcd27fb31e4", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_188.42.181.224/27", "id": "92e5ad9b-7e87-473c-a00a-641d5c96a71a", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_23.105.240.32/27", "id": "9d892306-115a-4809-a6c9-7de25907281e", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_88.212.238.224/27", "id": "a05cb870-7423-46ae-9109-dde4ba877256", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_88.212.238.64/27", "id": "aa2b74d2-4690-4387-9a3f-6d8220dabe0c", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_188.42.181.160/27", "id": "ad8192af-1822-437c-9155-20a976986a12", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_88.212.241.64/27", "id": "bcc6d37f-5641-471d-bfee-3756da3ef429", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_88.212.241.128/27", "id": "c73add07-022c-4a7c-8ac9-c81c7bac1807", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_188.42.181.128/27", "id": "cdc77f48-c699-47e4-aa7f-69f9e2bb3ca8", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_23.105.231.64/27", "id": "d98b8c36-b785-4e4d-b657-dc8f3573a689", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_88.212.238.160/27", "id": "de3f0102-51ce-4605-b817-d8d41a0d29af", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_88.212.241.0/27", "id": "eb982e9b-bd4c-4916-953c-f5e8aba2f943", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_88.212.241.192/27", "id": "ec663f27-e5eb-4037-9fb8-c7fef9025291", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_88.212.241.160/27", "id": "efdfb4e9-7f11-461a-b330-6bd80aead7f8", "netmask_v6": null}, {"cidr_v6": null, "dns2": null, "dns1": null, "gateway": null, "broadcast": null, "netmask": null, "gateway_v6": null, "cidr": null, "label": "internet_23.105.225.192/27", "id": "f1fc6bc4-a35f-4f89-8f8a-1db862868207", "netmask_v6": null}]}

`
)
