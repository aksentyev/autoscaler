// +build ignore

/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/json"
	"html/template"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/azure"
	klog "k8s.io/klog/v2"
)

var packageTemplate = template.Must(template.New("").Parse(`/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// This file was generated by go generate; DO NOT EDIT

package azure

// InstanceType is the sepc of Azure instance
type InstanceType struct {
	InstanceType string
	VCPU         int64
	MemoryMb     int64
	GPU          int64
}

// InstanceTypes is a map of azure resources
var InstanceTypes = map[string]*InstanceType{
{{- range .InstanceTypes }}
	"{{ .InstanceType }}": {
		InstanceType: "{{ .InstanceType }}",
		VCPU:         {{ .VCPU }},
		MemoryMb:     {{ .MemoryMb }},
		GPU:          {{ .GPU }},
	},
{{- end }}
}
`))

type InstanceCapabilities struct {
	Name  string
	Value string
}

type RawInstanceType struct {
	Name         string
	ResourceType string
	Capabilities []InstanceCapabilities
}

func getAllAzureVirtualMachineTypes() (result map[string]*azure.InstanceType, err error) {
	cmd := exec.Command("az", "vm", "list-skus", "-o", "json")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	bytes, err := ioutil.ReadAll(stdout)
	if err != nil {
		return nil, err
	}

	if err := cmd.Wait(); err != nil {
		return nil, err
	}

	allInstances := make([]RawInstanceType, 0)
	err = json.Unmarshal(bytes, &allInstances)
	if err != nil {
		return nil, err
	}

	virtualMachines := make(map[string]*azure.InstanceType)
	for _, instance := range allInstances {
		if strings.EqualFold(instance.ResourceType, "virtualMachines") {
			var virtualMachine azure.InstanceType
			virtualMachine.InstanceType = instance.Name
			for _, capability := range instance.Capabilities {
				switch capability.Name {
				case "vCPUs":
					virtualMachine.VCPU, err = strconv.ParseInt(capability.Value, 10, 64)
					if err != nil {
						return nil, err
					}
				case "MemoryGB":
					memoryMb, err := strconv.ParseFloat(capability.Value, 10)
					if err != nil {
						return nil, err
					}
					virtualMachine.MemoryMb = int64(memoryMb) * 1024
				case "GPUs":
					virtualMachine.GPU, err = strconv.ParseInt(capability.Value, 10, 64)
					if err != nil {
						return nil, err
					}
				}
			}
			virtualMachines[virtualMachine.InstanceType] = &virtualMachine
		}
	}

	return virtualMachines, err
}

func main() {
	instanceTypes, err := getAllAzureVirtualMachineTypes()
	if err != nil {
		klog.Fatal(err)
	}

	f, err := os.Create("azure_instance_types.go")
	if err != nil {
		klog.Fatal(err)
	}

	defer f.Close()

	err = packageTemplate.Execute(f, struct {
		InstanceTypes map[string]*azure.InstanceType
	}{
		InstanceTypes: instanceTypes,
	})

	if err != nil {
		klog.Fatal(err)
	}
}
