/*
Copyright 2020 The Kubermatic Kubernetes Platform contributors.

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

package machine

import (
	"fmt"
	"reflect"

	apiv1 "k8c.io/dashboard/v2/pkg/api/v1"
)

var userNameMap = map[string]string{
	"Digitalocean:Ubuntu":                "root",
	"Digitalocean:ContainerLinux":        "core",
	"Digitalocean:Flatcar":               "core",
	"Hetzner:Ubuntu":                     "root",
	"Azure:Ubuntu":                       "ubuntu",
	"Azure:ContainerLinux":               "core",
	"Azure:Flatcar":                      "core",
	"Azure:RHEL":                         "rhel",
	"VSphere:Ubuntu":                     "ubuntu",
	"VSphere:ContainerLinux":             "core",
	"VSphere:Flatcar":                    "core",
	"VSphere:RHEL":                       "cloud-user",
	"AWS:Ubuntu":                         "ubuntu",
	"AWS:ContainerLinux":                 "core",
	"AWS:Flatcar":                        "core",
	"AWS:RHEL":                           "ec2-user",
	"Openstack:RHEL":                     "cloud-user",
	"Openstack:Ubuntu":                   "ubuntu",
	"Openstack:ContainerLinux":           "core",
	"Openstack:Flatcar":                  "core",
	"Packet:Ubuntu":                      "root",
	"Packet:ContainerLinux":              "core",
	"Packet:Flatcar":                     "core",
	"GCP:Ubuntu":                         "ubuntu",
	"GCP:RHEL":                           "cloud-user",
	"GCP:ContainerLinux":                 "core",
	"GCP:Flatcar":                        "core",
	"VMwareCloudDirector:Ubuntu":         "ubuntu",
	"VMwareCloudDirector:ContainerLinux": "core",
	"VMwareCloudDirector:Flatcar":        "core",
	"VMwareCloudDirector:RHEL":           "cloud-user",
}

// GetSSHUserName returns SSH login name for the provider and distribution.
func GetSSHUserName(distribution *apiv1.OperatingSystemSpec, cloudProvider *apiv1.NodeCloudSpec) (string, error) {
	distributionName, err := getDistributionName(distribution)
	if err != nil {
		return "", err
	}

	providerName, err := getProviderName(cloudProvider)
	if err != nil {
		return "", err
	}

	loginName, ok := userNameMap[fmt.Sprintf("%s:%s", providerName, distributionName)]

	if ok {
		return loginName, nil
	}

	return "unknown", nil
}

func getDistributionName(distribution *apiv1.OperatingSystemSpec) (string, error) {
	val := reflect.ValueOf(distribution).Elem()

	for i := 0; i < val.NumField(); i++ {
		if !val.Field(i).IsNil() {
			return val.Type().Field(i).Name, nil
		}
	}

	return "", fmt.Errorf("no operating system set")
}

func getProviderName(cloudProvider *apiv1.NodeCloudSpec) (string, error) {
	val := reflect.ValueOf(cloudProvider).Elem()

	for i := 0; i < val.NumField(); i++ {
		if !val.Field(i).IsNil() {
			return val.Type().Field(i).Name, nil
		}
	}

	return "", fmt.Errorf("no cloud provider set")
}
