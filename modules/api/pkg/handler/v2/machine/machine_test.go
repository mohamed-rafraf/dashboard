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

package machine_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apiv1 "k8c.io/dashboard/v2/pkg/api/v1"
	"k8c.io/dashboard/v2/pkg/handler/test"
	"k8c.io/dashboard/v2/pkg/handler/test/hack"
	"k8c.io/dashboard/v2/pkg/resources/machine"
	kubermaticv1 "k8c.io/kubermatic/sdk/v2/apis/kubermatic/v1"
	clusterv1alpha1 "k8c.io/machine-controller/sdk/apis/cluster/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"k8s.io/utils/ptr"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestCreateMachineDeployment(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		Name                   string
		Body                   string
		ExpectedResponse       string
		ProjectID              string
		ClusterID              string
		HTTPStatus             int
		ExistingProject        *kubermaticv1.Project
		ExistingKubermaticUser *kubermaticv1.User
		ExistingAPIUser        *apiv1.User
		ExistingCluster        *kubermaticv1.Cluster
		ExistingKubermaticObjs []ctrlruntimeclient.Object
	}{
		// scenario 1
		{
			Name:             "scenario 1: create a machine deployment that match the given spec",
			Body:             `{"spec":{"replicas":1,"template":{"cloud":{"digitalocean":{"size":"s-1vcpu-1gb","backups":false,"ipv6":false,"monitoring":false,"tags":[]}},"operatingSystem":{"ubuntu":{"distUpgradeOnBoot":false}},"network":{"cidr":"","gateway":"","dns":{"servers":null},"ipFamily":"IPv4"}}}}`,
			ExpectedResponse: `{"id":"%s","name":"%s","creationTimestamp":"0001-01-01T00:00:00Z","spec":{"replicas":1,"template":{"cloud":{"digitalocean":{"size":"s-1vcpu-1gb","backups":false,"ipv6":false,"monitoring":false,"tags":["kubernetes","kubernetes-cluster-defClusterID","system-cluster-defClusterID","system-project-my-first-project-ID"]}},"operatingSystem":{"ubuntu":{"distUpgradeOnBoot":false}},"network":{"cidr":"","gateway":"","dns":{"servers":null},"ipFamily":"IPv4"},"versions":{"kubelet":"9.9.9"},"labels":{"system/cluster":"defClusterID","system/project":"my-first-project-ID"}},"paused":false,"dynamicConfig":false},"status":{}}`,
			HTTPStatus:       http.StatusCreated,
			ProjectID:        test.GenDefaultProject().Name,
			ClusterID:        test.GenDefaultCluster().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				genTestCluster(true),
			),
			ExistingAPIUser: test.GenDefaultAPIUser(),
		},

		// scenario 2
		{
			Name:             "scenario 2: cluster components are not ready",
			Body:             `{"spec":{"cloud":{"digitalocean":{"size":"s-1vcpu-1gb","backups":false,"ipv6":false,"monitoring":false,"tags":[]}},"operatingSystem":{"ubuntu":{"distUpgradeOnBoot":false}},"network":{"cidr":"","gateway":"","dns":{"servers":null},"ipFamily":"IPv4"}}}`,
			ExpectedResponse: `{"error":{"code":503,"message":"Cluster components are not ready yet"}}`,
			HTTPStatus:       http.StatusServiceUnavailable,
			ProjectID:        test.GenDefaultProject().Name,
			ClusterID:        test.GenDefaultCluster().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				genTestCluster(false),
			),
			ExistingAPIUser: test.GenDefaultAPIUser(),
		},

		// scenario 3
		{
			Name:             "scenario 3: kubelet version is too old",
			Body:             `{"spec":{"replicas":1,"template":{"cloud":{"digitalocean":{"size":"s-1vcpu-1gb","backups":false,"ipv6":false,"monitoring":false,"tags":[]}},"operatingSystem":{"ubuntu":{"distUpgradeOnBoot":false}},"network":{"cidr":"","gateway":"","dns":{"servers":null},"ipFamily":"IPv4"},"versions":{"kubelet":"9.6.0"}}}}`,
			ExpectedResponse: `{"error":{"code":400,"message":"node deployment validation failed: kubelet version 9.6.0 is not compatible with control plane version 9.9.9"}}`,
			HTTPStatus:       http.StatusBadRequest,
			ProjectID:        test.GenDefaultProject().Name,
			ClusterID:        test.GenDefaultCluster().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				genTestCluster(true),
			),
			ExistingAPIUser: test.GenDefaultAPIUser(),
		},

		// scenario 4
		{
			Name:             "scenario 4: kubelet version is too new",
			Body:             `{"spec":{"replicas":1,"template":{"cloud":{"digitalocean":{"size":"s-1vcpu-1gb","backups":false,"ipv6":false,"monitoring":false,"tags":[]}},"operatingSystem":{"ubuntu":{"distUpgradeOnBoot":false}},"network":{"cidr":"","gateway":"","dns":{"servers":null},"ipFamily":"IPv4"},"versions":{"kubelet":"9.10.0"}}}}`,
			ExpectedResponse: `{"error":{"code":400,"message":"node deployment validation failed: kubelet version 9.10.0 is not compatible with control plane version 9.9.9"}}`,
			HTTPStatus:       http.StatusBadRequest,
			ProjectID:        test.GenDefaultProject().Name,
			ClusterID:        test.GenDefaultCluster().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				genTestCluster(true),
			),
			ExistingAPIUser: test.GenDefaultAPIUser(),
		},

		// scenario 5
		{
			Name:             "scenario 5: set taints",
			Body:             `{"spec":{"replicas":1,"template":{"taints": [{"key":"foo","value":"bar","effect":"NoExecute"}],"cloud":{"digitalocean":{"size":"s-1vcpu-1gb","backups":false,"ipv6":false,"monitoring":false,"tags":[]}},"operatingSystem":{"ubuntu":{"distUpgradeOnBoot":false}},"network":{"cidr":"","gateway":"","dns":{"servers":null},"ipFamily":"IPv4"},"versions":{"kubelet":"9.9.9"}}}}`,
			ExpectedResponse: `{"id":"%s","name":"%s","creationTimestamp":"0001-01-01T00:00:00Z","spec":{"replicas":1,"template":{"cloud":{"digitalocean":{"size":"s-1vcpu-1gb","backups":false,"ipv6":false,"monitoring":false,"tags":["kubernetes","kubernetes-cluster-defClusterID","system-cluster-defClusterID","system-project-my-first-project-ID"]}},"operatingSystem":{"ubuntu":{"distUpgradeOnBoot":false}},"network":{"cidr":"","gateway":"","dns":{"servers":null},"ipFamily":"IPv4"},"versions":{"kubelet":"9.9.9"},"labels":{"system/cluster":"defClusterID","system/project":"my-first-project-ID"},"taints":[{"key":"foo","value":"bar","effect":"NoExecute"}]},"paused":false,"dynamicConfig":false},"status":{}}`,
			HTTPStatus:       http.StatusCreated,
			ProjectID:        test.GenDefaultProject().Name,
			ClusterID:        test.GenDefaultCluster().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				genTestCluster(true),
			),
			ExistingAPIUser: test.GenDefaultAPIUser(),
		},

		// scenario 6
		{
			Name:             "scenario 6: invalid taint",
			Body:             `{"spec":{"replicas":1,"template":{"taints": [{"key":"foo","value":"bar","effect":"BAD_EFFECT"}],"cloud":{"digitalocean":{"size":"s-1vcpu-1gb","backups":false,"ipv6":false,"monitoring":false,"tags":[]}},"operatingSystem":{"ubuntu":{"distUpgradeOnBoot":false}},"network":{"cidr":"","gateway":"","dns":{"servers":null},"ipFamily":"IPv4"},"versions":{"kubelet":"9.9.9"}}}}`,
			ExpectedResponse: `{"error":{"code":400,"message":"node deployment validation failed: taint effect 'BAD_EFFECT' not allowed. Allowed: NoExecute, NoSchedule, PreferNoSchedule"}}`,
			HTTPStatus:       http.StatusBadRequest,
			ProjectID:        test.GenDefaultProject().Name,
			ClusterID:        test.GenDefaultCluster().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				genTestCluster(true),
			),
			ExistingAPIUser: test.GenDefaultAPIUser(),
		},

		// scenario 7
		{
			Name:             "scenario 7: create a machine deployment with dynamic config",
			Body:             `{"spec":{"replicas":1,"dynamicConfig":true,"template":{"cloud":{"digitalocean":{"size":"s-1vcpu-1gb","backups":false,"ipv6":false,"monitoring":false,"tags":[]}},"operatingSystem":{"ubuntu":{"distUpgradeOnBoot":false}},"network":{"cidr":"","gateway":"","dns":{"servers":null},"ipFamily":"IPv4"}}}}`,
			ExpectedResponse: `{"error":{"code":400,"message":"node deployment validation failed: dynamic config cannot be configured for Kubernetes 1.24 or higher"}}`,
			HTTPStatus:       http.StatusBadRequest,
			ProjectID:        test.GenDefaultProject().Name,
			ClusterID:        test.GenDefaultCluster().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				genTestCluster(true),
			),
			ExistingAPIUser: test.GenDefaultAPIUser(),
		},

		// scenario 8
		{
			Name:             "scenario 8: create a machine deployment with annotations",
			Body:             `{"annotations":{"test/annotations":"true"},"spec":{"replicas":1,"template":{"cloud":{"digitalocean":{"size":"s-1vcpu-1gb","backups":false,"ipv6":false,"monitoring":false,"tags":[]}},"operatingSystem":{"ubuntu":{"distUpgradeOnBoot":false}},"network":{"cidr":"","gateway":"","dns":{"servers":null},"ipFamily":"IPv4"}}}}`,
			ExpectedResponse: `{"id":"%s","name":"%s","annotations":{"test/annotations":"true"},"creationTimestamp":"0001-01-01T00:00:00Z","spec":{"replicas":1,"template":{"cloud":{"digitalocean":{"size":"s-1vcpu-1gb","backups":false,"ipv6":false,"monitoring":false,"tags":["kubernetes","kubernetes-cluster-defClusterID","system-cluster-defClusterID","system-project-my-first-project-ID"]}},"operatingSystem":{"ubuntu":{"distUpgradeOnBoot":false}},"network":{"cidr":"","gateway":"","dns":{"servers":null},"ipFamily":"IPv4"},"versions":{"kubelet":"9.9.9"},"labels":{"system/cluster":"defClusterID","system/project":"my-first-project-ID"}},"paused":false,"dynamicConfig":false},"status":{}}`,
			HTTPStatus:       http.StatusCreated,
			ProjectID:        test.GenDefaultProject().Name,
			ClusterID:        test.GenDefaultCluster().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				genTestCluster(true),
			),
			ExistingAPIUser: test.GenDefaultAPIUser(),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v2/projects/%s/clusters/%s/machinedeployments", tc.ProjectID, tc.ClusterID), strings.NewReader(tc.Body))
			res := httptest.NewRecorder()
			kubermaticObj := []ctrlruntimeclient.Object{}
			kubermaticObj = append(kubermaticObj, tc.ExistingKubermaticObjs...)
			ep, err := test.CreateTestEndpoint(*tc.ExistingAPIUser, []ctrlruntimeclient.Object{}, kubermaticObj, nil, hack.NewTestRouting)
			if err != nil {
				t.Fatalf("failed to create test endpoint: %v", err)
			}

			ep.ServeHTTP(res, req)

			if res.Code != tc.HTTPStatus {
				t.Fatalf("Expected HTTP status code %d, got %d: %s", tc.HTTPStatus, res.Code, res.Body.String())
			}

			// Since Node Deployment's ID, name and match labels are automatically generated by the system just rewrite them.
			nd := &apiv1.NodeDeployment{}
			var expectedResponse string
			err = json.Unmarshal(res.Body.Bytes(), nd)
			if err != nil {
				t.Fatal(err)
			}
			if tc.HTTPStatus > 399 {
				expectedResponse = tc.ExpectedResponse
			} else {
				expectedResponse = fmt.Sprintf(tc.ExpectedResponse, nd.Name, nd.Name)
			}

			test.CompareWithResult(t, res, expectedResponse)
		})
	}
}

func TestDeleteMachineDeploymentNode(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		Name                    string
		HTTPStatus              int
		NodeIDToDelete          string
		ClusterIDToSync         string
		ProjectIDToSync         string
		ExistingAPIUser         *apiv1.User
		ExistingNodes           []*corev1.Node
		ExistingMachines        []*clusterv1alpha1.Machine
		ExistingKubermaticObjs  []ctrlruntimeclient.Object
		ExpectedHTTPStatusOnGet int
		ExpectedResponseOnGet   string
		ExpectedNodeCount       int
	}{
		// scenario 1
		{
			Name:            "scenario 1: delete the machine node that belong to the given cluster",
			HTTPStatus:      http.StatusOK,
			NodeIDToDelete:  "venus",
			ClusterIDToSync: test.GenDefaultCluster().Name,
			ProjectIDToSync: test.GenDefaultProject().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				test.GenDefaultCluster(),
			),
			ExistingAPIUser: test.GenDefaultAPIUser(),
			ExistingNodes: []*corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "venus"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "mars"}},
			},
			ExistingMachines: []*clusterv1alpha1.Machine{
				genTestMachine("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123", "some-other": "xyz"}, nil),
				genTestMachine("mars", `{"cloudProvider":"aws","cloudProviderSpec":{"token":"dummy-token","region":"eu-central-1","availabilityZone":"eu-central-1a","vpcId":"vpc-819f62e9","subnetId":"subnet-2bff4f43","instanceType":"t2.micro","diskSize":50}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":false}}`, map[string]string{"md-id": "123", "some-other": "xyz"}, nil),
			},
			//
			// even though the machine object was deleted the associated node object was not. When the client GETs the previously deleted "node" it will get a valid response.
			// That is only true for testing, but in a real cluster, the node object will get deleted by the garbage-collector as it has a ownerRef set.
			ExpectedHTTPStatusOnGet: http.StatusOK,
			ExpectedResponseOnGet:   `{"id":"venus","name":"venus","creationTimestamp":"0001-01-01T00:00:00Z","spec":{"cloud":{},"operatingSystem":{},"versions":{"kubelet":""}},"status":{"machineName":"","capacity":{"cpu":"0","memory":"0"},"allocatable":{"cpu":"0","memory":"0"},"nodeInfo":{"kernelVersion":"","containerRuntime":"","containerRuntimeVersion":"","kubeletVersion":"","operatingSystem":"","architecture":""}}}`,
			ExpectedNodeCount:       1,
		},
		// scenario 2
		{
			Name:            "scenario 2: the admin John can delete any cluster machine node",
			HTTPStatus:      http.StatusOK,
			NodeIDToDelete:  "venus",
			ClusterIDToSync: test.GenDefaultCluster().Name,
			ProjectIDToSync: test.GenDefaultProject().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				test.GenDefaultCluster(),
				test.GenAdminUser("John", "john@acme.com", true),
			),
			ExistingAPIUser: test.GenAPIUser("John", "john@acme.com"),
			ExistingNodes: []*corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "venus"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "mars"}},
			},
			ExistingMachines: []*clusterv1alpha1.Machine{
				genTestMachine("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123", "some-other": "xyz"}, nil),
				genTestMachine("mars", `{"cloudProvider":"aws","cloudProviderSpec":{"token":"dummy-token","region":"eu-central-1","availabilityZone":"eu-central-1a","vpcId":"vpc-819f62e9","subnetId":"subnet-2bff4f43","instanceType":"t2.micro","diskSize":50}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":false}}`, map[string]string{"md-id": "123", "some-other": "xyz"}, nil),
			},
			//
			// even though the machine object was deleted the associated node object was not. When the client GETs the previously deleted "node" it will get a valid response.
			// That is only true for testing, but in a real cluster, the node object will get deleted by the garbage-collector as it has a ownerRef set.
			ExpectedHTTPStatusOnGet: http.StatusOK,
			ExpectedResponseOnGet:   `{"id":"venus","name":"venus","creationTimestamp":"0001-01-01T00:00:00Z","spec":{"cloud":{},"operatingSystem":{},"versions":{"kubelet":""}},"status":{"machineName":"","capacity":{"cpu":"0","memory":"0"},"allocatable":{"cpu":"0","memory":"0"},"nodeInfo":{"kernelVersion":"","containerRuntime":"","containerRuntimeVersion":"","kubeletVersion":"","operatingSystem":"","architecture":""}}}`,
			ExpectedNodeCount:       1,
		},
		// scenario 3
		{
			Name:            "scenario 3: the user John can not delete Bob's cluster machine node",
			HTTPStatus:      http.StatusForbidden,
			NodeIDToDelete:  "venus",
			ClusterIDToSync: test.GenDefaultCluster().Name,
			ProjectIDToSync: test.GenDefaultProject().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				test.GenDefaultCluster(),
				test.GenAdminUser("John", "john@acme.com", false),
			),
			ExistingAPIUser: test.GenAPIUser("John", "john@acme.com"),
			ExistingNodes: []*corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "venus"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "mars"}},
			},
			ExistingMachines: []*clusterv1alpha1.Machine{
				genTestMachine("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123", "some-other": "xyz"}, nil),
				genTestMachine("mars", `{"cloudProvider":"aws","cloudProviderSpec":{"token":"dummy-token","region":"eu-central-1","availabilityZone":"eu-central-1a","vpcId":"vpc-819f62e9","subnetId":"subnet-2bff4f43","instanceType":"t2.micro","diskSize":50}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":false}}`, map[string]string{"md-id": "123", "some-other": "xyz"}, nil),
			},
			ExpectedHTTPStatusOnGet: http.StatusForbidden,
			ExpectedResponseOnGet:   `{"error":{"code":403,"message":"forbidden: \"john@acme.com\" doesn't belong to project my-first-project-ID"}}`,
			ExpectedNodeCount:       2,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v2/projects/%s/clusters/%s/machinedeployments/nodes/%s", tc.ProjectIDToSync, tc.ClusterIDToSync, tc.NodeIDToDelete), strings.NewReader(""))
			res := httptest.NewRecorder()
			kubermaticObj := []ctrlruntimeclient.Object{}
			machineObj := []ctrlruntimeclient.Object{}
			kubernetesObj := []ctrlruntimeclient.Object{}
			kubermaticObj = append(kubermaticObj, tc.ExistingKubermaticObjs...)
			for _, existingNode := range tc.ExistingNodes {
				kubernetesObj = append(kubernetesObj, existingNode)
			}
			for _, existingMachine := range tc.ExistingMachines {
				machineObj = append(machineObj, existingMachine)
			}
			ep, clientsSets, err := test.CreateTestEndpointAndGetClients(*tc.ExistingAPIUser, nil, kubernetesObj, machineObj, kubermaticObj, nil, hack.NewTestRouting)
			if err != nil {
				t.Fatalf("failed to create test endpoint: %v", err)
			}

			ep.ServeHTTP(res, req)

			if res.Code != tc.HTTPStatus {
				t.Fatalf("Expected HTTP status code %d, got %d: %s", tc.HTTPStatus, res.Code, res.Body.String())
			}

			machines := &clusterv1alpha1.MachineList{}
			if err := clientsSets.FakeClient.List(context.Background(), machines); err != nil {
				t.Fatalf("failed to list machines from fake client: %v", err)
			}

			if machineCount := len(machines.Items); machineCount != tc.ExpectedNodeCount {
				t.Errorf("Expected %d machines to be gone but got %d", tc.ExpectedNodeCount, machineCount)
			}
		})
	}
}

func TestListMachineDeployments(t *testing.T) {
	t.Parallel()
	var replicas int32 = 1
	var paused = false
	testcases := []struct {
		Name                       string
		ExpectedResponse           []apiv1.NodeDeployment
		HTTPStatus                 int
		ProjectIDToSync            string
		ClusterIDToSync            string
		ExistingProject            *kubermaticv1.Project
		ExistingKubermaticUser     *kubermaticv1.User
		ExistingAPIUser            *apiv1.User
		ExistingCluster            *kubermaticv1.Cluster
		ExistingMachineDeployments []*clusterv1alpha1.MachineDeployment
		ExistingKubermaticObjs     []ctrlruntimeclient.Object
	}{
		// scenario 1
		{
			Name:            "scenario 1: list machine deployments that belong to the given cluster",
			HTTPStatus:      http.StatusOK,
			ClusterIDToSync: test.GenDefaultCluster().Name,
			ProjectIDToSync: test.GenDefaultProject().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				test.GenDefaultCluster(),
			),
			ExistingAPIUser: test.GenDefaultAPIUser(),
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false),
				genTestMachineDeployment("mars", `{"cloudProvider":"aws","cloudProviderSpec":{"token":"dummy-token","region":"eu-central-1","availabilityZone":"eu-central-1a","vpcId":"vpc-819f62e9","subnetId":"subnet-2bff4f43","instanceType":"t2.micro","diskSize":50,"diskType":"standard"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":false}}`, nil, false),
			},
			ExpectedResponse: []apiv1.NodeDeployment{
				{
					ObjectMeta: apiv1.ObjectMeta{
						ID:   "venus",
						Name: "venus",
					},
					Spec: apiv1.NodeDeploymentSpec{
						Template: apiv1.NodeSpec{
							Cloud: apiv1.NodeCloudSpec{
								Digitalocean: &apiv1.DigitaloceanNodeSpec{
									Size: "2GB",
								},
							},
							OperatingSystem: apiv1.OperatingSystemSpec{
								Ubuntu: &apiv1.UbuntuSpec{
									DistUpgradeOnBoot: true,
								},
							},
							Versions: apiv1.NodeVersionInfo{
								Kubelet: "v9.9.9",
							},
						},
						Replicas:      replicas,
						Paused:        &paused,
						DynamicConfig: ptr.To(false),
					},
					Status: clusterv1alpha1.MachineDeploymentStatus{},
				},
				{
					ObjectMeta: apiv1.ObjectMeta{
						ID:   "mars",
						Name: "mars",
					},
					Spec: apiv1.NodeDeploymentSpec{
						Template: apiv1.NodeSpec{
							Cloud: apiv1.NodeCloudSpec{
								AWS: &apiv1.AWSNodeSpec{
									InstanceType:     "t2.micro",
									VolumeSize:       50,
									VolumeType:       "standard",
									AvailabilityZone: "eu-central-1a",
									SubnetID:         "subnet-2bff4f43",
								},
							},
							OperatingSystem: apiv1.OperatingSystemSpec{
								Ubuntu: &apiv1.UbuntuSpec{
									DistUpgradeOnBoot: false,
								},
							},
							Versions: apiv1.NodeVersionInfo{
								Kubelet: "v9.9.9",
							},
						},
						Replicas:      replicas,
						Paused:        &paused,
						DynamicConfig: ptr.To(false),
					},
					Status: clusterv1alpha1.MachineDeploymentStatus{},
				},
			},
		},
		// scenario 2
		{
			Name:            "scenario 2: the admin John can list machine deployments that belong to the given Bob's cluster",
			HTTPStatus:      http.StatusOK,
			ClusterIDToSync: test.GenDefaultCluster().Name,
			ProjectIDToSync: test.GenDefaultProject().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				test.GenDefaultCluster(),
				test.GenAdminUser("John", "john@acme.com", true),
			),
			ExistingAPIUser: test.GenAPIUser("John", "john@acme.com"),
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false),
				genTestMachineDeployment("mars", `{"cloudProvider":"aws","cloudProviderSpec":{"token":"dummy-token","region":"eu-central-1","availabilityZone":"eu-central-1a","vpcId":"vpc-819f62e9","subnetId":"subnet-2bff4f43","instanceType":"t2.micro","diskSize":50,"diskType":"standard"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":false}}`, nil, false),
			},
			ExpectedResponse: []apiv1.NodeDeployment{
				{
					ObjectMeta: apiv1.ObjectMeta{
						ID:   "venus",
						Name: "venus",
					},
					Spec: apiv1.NodeDeploymentSpec{
						Template: apiv1.NodeSpec{
							Cloud: apiv1.NodeCloudSpec{
								Digitalocean: &apiv1.DigitaloceanNodeSpec{
									Size: "2GB",
								},
							},
							OperatingSystem: apiv1.OperatingSystemSpec{
								Ubuntu: &apiv1.UbuntuSpec{
									DistUpgradeOnBoot: true,
								},
							},
							Versions: apiv1.NodeVersionInfo{
								Kubelet: "v9.9.9",
							},
						},
						Replicas:      replicas,
						Paused:        &paused,
						DynamicConfig: ptr.To(false),
					},
					Status: clusterv1alpha1.MachineDeploymentStatus{},
				},
				{
					ObjectMeta: apiv1.ObjectMeta{
						ID:   "mars",
						Name: "mars",
					},
					Spec: apiv1.NodeDeploymentSpec{
						Template: apiv1.NodeSpec{
							Cloud: apiv1.NodeCloudSpec{
								AWS: &apiv1.AWSNodeSpec{
									InstanceType:     "t2.micro",
									VolumeSize:       50,
									VolumeType:       "standard",
									AvailabilityZone: "eu-central-1a",
									SubnetID:         "subnet-2bff4f43",
								},
							},
							OperatingSystem: apiv1.OperatingSystemSpec{
								Ubuntu: &apiv1.UbuntuSpec{
									DistUpgradeOnBoot: false,
								},
							},
							Versions: apiv1.NodeVersionInfo{
								Kubelet: "v9.9.9",
							},
						},
						Replicas:      replicas,
						Paused:        &paused,
						DynamicConfig: ptr.To(false),
					},
					Status: clusterv1alpha1.MachineDeploymentStatus{},
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v2/projects/%s/clusters/%s/machinedeployments",
				tc.ProjectIDToSync, tc.ClusterIDToSync), strings.NewReader(""))
			res := httptest.NewRecorder()
			kubermaticObj := []ctrlruntimeclient.Object{}
			machineObj := []ctrlruntimeclient.Object{}
			kubernetesObj := []ctrlruntimeclient.Object{}
			kubermaticObj = append(kubermaticObj, tc.ExistingKubermaticObjs...)
			for _, existingMachineDeployment := range tc.ExistingMachineDeployments {
				machineObj = append(machineObj, existingMachineDeployment)
			}
			ep, _, err := test.CreateTestEndpointAndGetClients(*tc.ExistingAPIUser, nil, kubernetesObj, machineObj, kubermaticObj, nil, hack.NewTestRouting)
			if err != nil {
				t.Fatalf("failed to create test endpoint: %v", err)
			}

			ep.ServeHTTP(res, req)

			if res.Code != tc.HTTPStatus {
				t.Fatalf("Expected HTTP status code %d, got %d: %s", tc.HTTPStatus, res.Code, res.Body.String())
			}

			actualNodeDeployments := test.NodeDeploymentSliceWrapper{}
			actualNodeDeployments.DecodeOrDie(res.Body, t).Sort()

			wrappedExpectedNodeDeployments := test.NodeDeploymentSliceWrapper(tc.ExpectedResponse)
			wrappedExpectedNodeDeployments.Sort()

			actualNodeDeployments.EqualOrDie(wrappedExpectedNodeDeployments, t)
		})
	}
}

func TestGetMachineDeployment(t *testing.T) {
	t.Parallel()
	var replicas int32 = 1
	var paused = false
	testcases := []struct {
		Name                       string
		ExpectedResponse           apiv1.NodeDeployment
		HTTPStatus                 int
		ProjectIDToSync            string
		ClusterIDToSync            string
		ExistingProject            *kubermaticv1.Project
		ExistingKubermaticUser     *kubermaticv1.User
		ExistingAPIUser            *apiv1.User
		ExistingCluster            *kubermaticv1.Cluster
		ExistingMachineDeployments []*clusterv1alpha1.MachineDeployment
		ExistingKubermaticObjs     []ctrlruntimeclient.Object
	}{
		// scenario 1
		{
			Name:            "scenario 1: get machine deployment that belong to the given cluster",
			HTTPStatus:      http.StatusOK,
			ClusterIDToSync: test.GenDefaultCluster().Name,
			ProjectIDToSync: test.GenDefaultProject().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				test.GenDefaultCluster(),
			),
			ExistingAPIUser: test.GenDefaultAPIUser(),
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false),
			},
			ExpectedResponse: apiv1.NodeDeployment{
				ObjectMeta: apiv1.ObjectMeta{
					ID:   "venus",
					Name: "venus",
				},
				Spec: apiv1.NodeDeploymentSpec{
					Template: apiv1.NodeSpec{
						Cloud: apiv1.NodeCloudSpec{
							Digitalocean: &apiv1.DigitaloceanNodeSpec{
								Size: "2GB",
							},
						},
						OperatingSystem: apiv1.OperatingSystemSpec{
							Ubuntu: &apiv1.UbuntuSpec{
								DistUpgradeOnBoot: true,
							},
						},
						Versions: apiv1.NodeVersionInfo{
							Kubelet: "v9.9.9",
						},
					},
					Replicas:      replicas,
					Paused:        &paused,
					DynamicConfig: ptr.To(false),
				},
				Status: clusterv1alpha1.MachineDeploymentStatus{},
			},
		},

		// scenario 2
		{
			Name:            "scenario 2: get machine deployment that belong to the given cluster and has dynamic config set up",
			HTTPStatus:      http.StatusOK,
			ClusterIDToSync: test.GenDefaultCluster().Name,
			ProjectIDToSync: test.GenDefaultProject().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				test.GenDefaultCluster(),
			),
			ExistingAPIUser: test.GenDefaultAPIUser(),
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, true),
			},
			ExpectedResponse: apiv1.NodeDeployment{
				ObjectMeta: apiv1.ObjectMeta{
					ID:   "venus",
					Name: "venus",
				},
				Spec: apiv1.NodeDeploymentSpec{
					Template: apiv1.NodeSpec{
						Cloud: apiv1.NodeCloudSpec{
							Digitalocean: &apiv1.DigitaloceanNodeSpec{
								Size: "2GB",
							},
						},
						OperatingSystem: apiv1.OperatingSystemSpec{
							Ubuntu: &apiv1.UbuntuSpec{
								DistUpgradeOnBoot: true,
							},
						},
						Versions: apiv1.NodeVersionInfo{
							Kubelet: "v9.9.9",
						},
					},
					Replicas:      replicas,
					Paused:        &paused,
					DynamicConfig: ptr.To(true),
				},
				Status: clusterv1alpha1.MachineDeploymentStatus{},
			},
		},
		// scenario 3
		{
			Name:            "scenario 1: the admin John can get any cluster machine deployment",
			HTTPStatus:      http.StatusOK,
			ClusterIDToSync: test.GenDefaultCluster().Name,
			ProjectIDToSync: test.GenDefaultProject().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				test.GenDefaultCluster(),
				test.GenAdminUser("John", "john@acme.com", true),
			),
			ExistingAPIUser: test.GenAPIUser("John", "john@acme.com"),
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false),
			},
			ExpectedResponse: apiv1.NodeDeployment{
				ObjectMeta: apiv1.ObjectMeta{
					ID:   "venus",
					Name: "venus",
				},
				Spec: apiv1.NodeDeploymentSpec{
					Template: apiv1.NodeSpec{
						Cloud: apiv1.NodeCloudSpec{
							Digitalocean: &apiv1.DigitaloceanNodeSpec{
								Size: "2GB",
							},
						},
						OperatingSystem: apiv1.OperatingSystemSpec{
							Ubuntu: &apiv1.UbuntuSpec{
								DistUpgradeOnBoot: true,
							},
						},
						Versions: apiv1.NodeVersionInfo{
							Kubelet: "v9.9.9",
						},
					},
					Replicas:      replicas,
					Paused:        &paused,
					DynamicConfig: ptr.To(false),
				},
				Status: clusterv1alpha1.MachineDeploymentStatus{},
			},
		},
		{
			Name:            "scenario 4: get machine deployment that has cluster-autoscaler min/max replicas set",
			HTTPStatus:      http.StatusOK,
			ClusterIDToSync: test.GenDefaultCluster().Name,
			ProjectIDToSync: test.GenDefaultProject().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				test.GenDefaultCluster(),
			),
			ExistingAPIUser: test.GenDefaultAPIUser(),
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				func() *clusterv1alpha1.MachineDeployment {
					md := genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false)
					md.Annotations = map[string]string{
						machine.AutoscalerMinSizeAnnotation: "5",
						machine.AutoscalerMaxSizeAnnotation: "7",
					}
					return md
				}(),
			},
			ExpectedResponse: apiv1.NodeDeployment{
				ObjectMeta: apiv1.ObjectMeta{
					ID:   "venus",
					Name: "venus",
					Annotations: map[string]string{
						machine.AutoscalerMinSizeAnnotation: "5",
						machine.AutoscalerMaxSizeAnnotation: "7",
					},
				},
				Spec: apiv1.NodeDeploymentSpec{
					Template: apiv1.NodeSpec{
						Cloud: apiv1.NodeCloudSpec{
							Digitalocean: &apiv1.DigitaloceanNodeSpec{
								Size: "2GB",
							},
						},
						OperatingSystem: apiv1.OperatingSystemSpec{
							Ubuntu: &apiv1.UbuntuSpec{
								DistUpgradeOnBoot: true,
							},
						},
						Versions: apiv1.NodeVersionInfo{
							Kubelet: "v9.9.9",
						},
					},
					Replicas:      replicas,
					Paused:        &paused,
					DynamicConfig: ptr.To(false),
					MinReplicas:   ptr.To[uint32](5),
					MaxReplicas:   ptr.To[uint32](7),
				},
				Status: clusterv1alpha1.MachineDeploymentStatus{},
			},
		},
		{
			Name:            "scenario 5: get machine deployment that has cluster-autoscaler min replicas set",
			HTTPStatus:      http.StatusOK,
			ClusterIDToSync: test.GenDefaultCluster().Name,
			ProjectIDToSync: test.GenDefaultProject().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				test.GenDefaultCluster(),
			),
			ExistingAPIUser: test.GenDefaultAPIUser(),
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				func() *clusterv1alpha1.MachineDeployment {
					md := genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false)
					md.Annotations = map[string]string{
						machine.AutoscalerMinSizeAnnotation: "5",
					}
					return md
				}(),
			},
			ExpectedResponse: apiv1.NodeDeployment{
				ObjectMeta: apiv1.ObjectMeta{
					ID:   "venus",
					Name: "venus",
					Annotations: map[string]string{
						machine.AutoscalerMinSizeAnnotation: "5",
					},
				},
				Spec: apiv1.NodeDeploymentSpec{
					Template: apiv1.NodeSpec{
						Cloud: apiv1.NodeCloudSpec{
							Digitalocean: &apiv1.DigitaloceanNodeSpec{
								Size: "2GB",
							},
						},
						OperatingSystem: apiv1.OperatingSystemSpec{
							Ubuntu: &apiv1.UbuntuSpec{
								DistUpgradeOnBoot: true,
							},
						},
						Versions: apiv1.NodeVersionInfo{
							Kubelet: "v9.9.9",
						},
					},
					Replicas:      replicas,
					Paused:        &paused,
					DynamicConfig: ptr.To(false),
					MinReplicas:   ptr.To[uint32](5),
				},
				Status: clusterv1alpha1.MachineDeploymentStatus{},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v2/projects/%s/clusters/%s/machinedeployments/venus",
				tc.ProjectIDToSync, tc.ClusterIDToSync), strings.NewReader(""))
			res := httptest.NewRecorder()
			kubermaticObj := []ctrlruntimeclient.Object{}
			machineObj := []ctrlruntimeclient.Object{}
			kubernetesObj := []ctrlruntimeclient.Object{}
			kubermaticObj = append(kubermaticObj, tc.ExistingKubermaticObjs...)
			for _, existingMachineDeployment := range tc.ExistingMachineDeployments {
				machineObj = append(machineObj, existingMachineDeployment)
			}
			ep, _, err := test.CreateTestEndpointAndGetClients(*tc.ExistingAPIUser, nil, kubernetesObj, machineObj, kubermaticObj, nil, hack.NewTestRouting)
			if err != nil {
				t.Fatalf("failed to create test endpoint: %v", err)
			}

			ep.ServeHTTP(res, req)

			if res.Code != tc.HTTPStatus {
				t.Fatalf("Expected HTTP status code %d, got %d: %s", tc.HTTPStatus, res.Code, res.Body.String())
			}

			bytes, err := json.Marshal(tc.ExpectedResponse)
			if err != nil {
				t.Fatalf("failed to marshall expected response %v", err)
			}

			test.CompareWithResult(t, res, string(bytes))
		})
	}
}

func TestListMachineDeploymentNodes(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		Name                       string
		ExpectedResponse           []apiv1.Node
		HTTPStatus                 int
		ProjectIDToSync            string
		ClusterIDToSync            string
		ExistingProject            *kubermaticv1.Project
		ExistingKubermaticUser     *kubermaticv1.User
		ExistingAPIUser            *apiv1.User
		ExistingCluster            *kubermaticv1.Cluster
		ExistingNodes              []*corev1.Node
		ExistingMachineDeployments []*clusterv1alpha1.MachineDeployment
		ExistingMachines           []*clusterv1alpha1.Machine
		ExistingKubermaticObjs     []ctrlruntimeclient.Object
		MachineDeploymentID        string
	}{
		// scenario 1
		{
			Name:            "scenario 1: list nodes that belong to the given machine deployment",
			HTTPStatus:      http.StatusOK,
			ClusterIDToSync: test.GenDefaultCluster().Name,
			ProjectIDToSync: test.GenDefaultProject().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				test.GenDefaultCluster(),
			),
			ExistingAPIUser: test.GenDefaultAPIUser(),
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123"}, false),
				genTestMachineDeployment("mars", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false),
			},
			MachineDeploymentID: "venus",
			ExistingMachines: []*clusterv1alpha1.Machine{
				genTestMachine("venus-1", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"},"operatingSystem":"ubuntu","operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123", "some-other": "xyz"}, nil),
				genTestMachine("venus-2", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"},"operatingSystem":"ubuntu","operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123", "xyz": "abc"}, nil),
				genTestMachine("mars-1", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"},"operatingSystem":"ubuntu","operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "345", "xyz": "abc"}, nil),
				genTestMachine("mars-2", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"},"operatingSystem":"ubuntu","operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, nil),
			},
			ExpectedResponse: []apiv1.Node{
				{
					ObjectMeta: apiv1.ObjectMeta{
						ID:   "venus-1",
						Name: "",
					},
					Spec: apiv1.NodeSpec{
						Cloud: apiv1.NodeCloudSpec{
							Digitalocean: &apiv1.DigitaloceanNodeSpec{
								Size: "2GB",
							},
						},
						OperatingSystem: apiv1.OperatingSystemSpec{
							Ubuntu: &apiv1.UbuntuSpec{
								DistUpgradeOnBoot: true,
							},
						},
						SSHUserName: "root",
						Versions: apiv1.NodeVersionInfo{
							Kubelet: "v9.9.9",
						},
					},
					Status: apiv1.NodeStatus{
						MachineName: "venus-1",
						Capacity:    apiv1.NodeResources{},
						Allocatable: apiv1.NodeResources{},
					},
				},
				{
					ObjectMeta: apiv1.ObjectMeta{
						ID:   "venus-2",
						Name: "",
					},
					Spec: apiv1.NodeSpec{
						Cloud: apiv1.NodeCloudSpec{
							Digitalocean: &apiv1.DigitaloceanNodeSpec{
								Size: "2GB",
							},
						},
						OperatingSystem: apiv1.OperatingSystemSpec{
							Ubuntu: &apiv1.UbuntuSpec{
								DistUpgradeOnBoot: true,
							},
						},
						SSHUserName: "root",
						Versions: apiv1.NodeVersionInfo{
							Kubelet: "v9.9.9",
						},
					},
					Status: apiv1.NodeStatus{
						MachineName: "venus-2",
						Capacity:    apiv1.NodeResources{},
						Allocatable: apiv1.NodeResources{},
					},
				},
			},
		},
		// scenario 2
		{
			Name:            "scenario 2: the admin John can list any nodes that belong to the given machine deployment",
			HTTPStatus:      http.StatusOK,
			ClusterIDToSync: test.GenDefaultCluster().Name,
			ProjectIDToSync: test.GenDefaultProject().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				test.GenDefaultCluster(),
				test.GenAdminUser("John", "john@acme.com", true),
			),
			ExistingAPIUser: test.GenAPIUser("John", "john@acme.com"),
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123"}, false),
				genTestMachineDeployment("mars", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false),
			},
			MachineDeploymentID: "venus",
			ExistingMachines: []*clusterv1alpha1.Machine{
				genTestMachine("venus-1", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"},"operatingSystem":"ubuntu","operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123", "some-other": "xyz"}, nil),
				genTestMachine("venus-2", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"},"operatingSystem":"ubuntu","operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123", "xyz": "abc"}, nil),
				genTestMachine("mars-1", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"},"operatingSystem":"ubuntu","operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "345", "xyz": "abc"}, nil),
				genTestMachine("mars-2", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"},"operatingSystem":"ubuntu","operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, nil),
			},
			ExpectedResponse: []apiv1.Node{
				{
					ObjectMeta: apiv1.ObjectMeta{
						ID:   "venus-1",
						Name: "",
					},
					Spec: apiv1.NodeSpec{
						Cloud: apiv1.NodeCloudSpec{
							Digitalocean: &apiv1.DigitaloceanNodeSpec{
								Size: "2GB",
							},
						},
						OperatingSystem: apiv1.OperatingSystemSpec{
							Ubuntu: &apiv1.UbuntuSpec{
								DistUpgradeOnBoot: true,
							},
						},
						SSHUserName: "root",
						Versions: apiv1.NodeVersionInfo{
							Kubelet: "v9.9.9",
						},
					},
					Status: apiv1.NodeStatus{
						MachineName: "venus-1",
						Capacity:    apiv1.NodeResources{},
						Allocatable: apiv1.NodeResources{},
					},
				},
				{
					ObjectMeta: apiv1.ObjectMeta{
						ID:   "venus-2",
						Name: "",
					},
					Spec: apiv1.NodeSpec{
						Cloud: apiv1.NodeCloudSpec{
							Digitalocean: &apiv1.DigitaloceanNodeSpec{
								Size: "2GB",
							},
						},
						OperatingSystem: apiv1.OperatingSystemSpec{
							Ubuntu: &apiv1.UbuntuSpec{
								DistUpgradeOnBoot: true,
							},
						},
						SSHUserName: "root",
						Versions: apiv1.NodeVersionInfo{
							Kubelet: "v9.9.9",
						},
					},
					Status: apiv1.NodeStatus{
						MachineName: "venus-2",
						Capacity:    apiv1.NodeResources{},
						Allocatable: apiv1.NodeResources{},
					},
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v2/projects/%s/clusters/%s/machinedeployments/%s/nodes", tc.ProjectIDToSync, tc.ClusterIDToSync, tc.MachineDeploymentID), strings.NewReader(""))
			res := httptest.NewRecorder()
			kubermaticObj := []ctrlruntimeclient.Object{}
			machineObj := []ctrlruntimeclient.Object{}
			kubernetesObj := []ctrlruntimeclient.Object{}
			for _, existingNode := range tc.ExistingNodes {
				kubernetesObj = append(kubernetesObj, existingNode)
			}
			for _, existingMachineDeployment := range tc.ExistingMachineDeployments {
				machineObj = append(machineObj, existingMachineDeployment)
			}
			for _, existingMachine := range tc.ExistingMachines {
				machineObj = append(machineObj, existingMachine)
			}
			kubermaticObj = append(kubermaticObj, tc.ExistingKubermaticObjs...)
			ep, _, err := test.CreateTestEndpointAndGetClients(*tc.ExistingAPIUser, nil, kubernetesObj, machineObj, kubermaticObj, nil, hack.NewTestRouting)
			if err != nil {
				t.Fatalf("failed to create test endpoint: %v", err)
			}

			ep.ServeHTTP(res, req)

			if res.Code != tc.HTTPStatus {
				t.Fatalf("Expected HTTP status code %d, got %d: %s", tc.HTTPStatus, res.Code, res.Body.String())
			}

			actualNodes := test.NodeV1SliceWrapper{}
			actualNodes.DecodeOrDie(res.Body, t).Sort()

			wrappedExpectedNodes := test.NodeV1SliceWrapper(tc.ExpectedResponse)
			wrappedExpectedNodes.Sort()

			actualNodes.EqualOrDie(wrappedExpectedNodes, t)
		})
	}
}

func TestListNodesForCluster(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		Name                   string
		ExpectedResponse       []apiv1.Node
		HTTPStatus             int
		ProjectIDToSync        string
		ClusterIDToSync        string
		ExistingProject        *kubermaticv1.Project
		ExistingKubermaticUser *kubermaticv1.User
		ExistingAPIUser        *apiv1.User
		ExistingCluster        *kubermaticv1.Cluster
		ExistingNodes          []*corev1.Node
		ExistingMachines       []*clusterv1alpha1.Machine
		ExistingKubermaticObjs []ctrlruntimeclient.Object
	}{
		// scenario 1
		{
			Name:            "scenario 1: list nodes that belong to the given cluster",
			HTTPStatus:      http.StatusOK,
			ClusterIDToSync: test.GenDefaultCluster().Name,
			ProjectIDToSync: test.GenDefaultProject().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				test.GenDefaultCluster(),
			),
			ExistingAPIUser: test.GenDefaultAPIUser(),
			ExistingNodes: []*corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "venus"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "mars"}},
			},
			ExistingMachines: []*clusterv1alpha1.Machine{
				genTestMachine("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"},"operatingSystem":"ubuntu","operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123", "some-other": "xyz"}, nil),
				genTestMachine("mars", `{"cloudProvider":"aws","cloudProviderSpec":{"token":"dummy-token","region":"eu-central-1","availabilityZone":"eu-central-1a","vpcId":"vpc-819f62e9","subnetId":"subnet-2bff4f43","instanceType":"t2.micro","diskSize":50,"diskType":"standard"},"operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":false}}`, map[string]string{"md-id": "123", "some-other": "xyz"}, nil),
			},
			ExpectedResponse: []apiv1.Node{
				{
					ObjectMeta: apiv1.ObjectMeta{
						ID:   "venus",
						Name: "venus",
					},
					Spec: apiv1.NodeSpec{
						Cloud: apiv1.NodeCloudSpec{
							Digitalocean: &apiv1.DigitaloceanNodeSpec{
								Size: "2GB",
							},
						},
						OperatingSystem: apiv1.OperatingSystemSpec{
							Ubuntu: &apiv1.UbuntuSpec{
								DistUpgradeOnBoot: true,
							},
						},
						SSHUserName: "root",
						Versions: apiv1.NodeVersionInfo{
							Kubelet: "v9.9.9",
						},
					},
					Status: apiv1.NodeStatus{
						MachineName: "venus",
						Capacity: apiv1.NodeResources{
							CPU:    "0",
							Memory: "0",
						},
						Allocatable: apiv1.NodeResources{
							CPU:    "0",
							Memory: "0",
						},
					},
				},
				{
					ObjectMeta: apiv1.ObjectMeta{
						ID:   "mars",
						Name: "mars",
					},
					Spec: apiv1.NodeSpec{
						Cloud: apiv1.NodeCloudSpec{
							AWS: &apiv1.AWSNodeSpec{
								InstanceType:     "t2.micro",
								VolumeSize:       50,
								VolumeType:       "standard",
								AvailabilityZone: "eu-central-1a",
								SubnetID:         "subnet-2bff4f43",
							},
						},
						OperatingSystem: apiv1.OperatingSystemSpec{
							Ubuntu: &apiv1.UbuntuSpec{
								DistUpgradeOnBoot: false,
							},
						},
						SSHUserName: "ubuntu",
						Versions: apiv1.NodeVersionInfo{
							Kubelet: "v9.9.9",
						},
					},
					Status: apiv1.NodeStatus{
						MachineName: "mars",
						Capacity: apiv1.NodeResources{
							CPU:    "0",
							Memory: "0",
						},
						Allocatable: apiv1.NodeResources{
							CPU:    "0",
							Memory: "0",
						},
					},
				},
			},
		},
		// scenario 2
		{
			Name:            "scenario 2: the admin John can list nodes that belong to the given Bob's cluster",
			HTTPStatus:      http.StatusOK,
			ClusterIDToSync: test.GenDefaultCluster().Name,
			ProjectIDToSync: test.GenDefaultProject().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				test.GenDefaultCluster(),
				test.GenAdminUser("John", "john@acme.com", true),
			),
			ExistingAPIUser: test.GenAPIUser("John", "john@acme.com"),
			ExistingNodes: []*corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "venus"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "mars"}},
			},
			ExistingMachines: []*clusterv1alpha1.Machine{
				genTestMachine("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"},"operatingSystem":"ubuntu","operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123", "some-other": "xyz"}, nil),
				genTestMachine("mars", `{"cloudProvider":"aws","cloudProviderSpec":{"token":"dummy-token","region":"eu-central-1","availabilityZone":"eu-central-1a","vpcId":"vpc-819f62e9","subnetId":"subnet-2bff4f43","instanceType":"t2.micro","diskSize":50,"diskType":"standard"},"operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":false}}`, map[string]string{"md-id": "123", "some-other": "xyz"}, nil),
			},
			ExpectedResponse: []apiv1.Node{
				{
					ObjectMeta: apiv1.ObjectMeta{
						ID:   "venus",
						Name: "venus",
					},
					Spec: apiv1.NodeSpec{
						Cloud: apiv1.NodeCloudSpec{
							Digitalocean: &apiv1.DigitaloceanNodeSpec{
								Size: "2GB",
							},
						},
						OperatingSystem: apiv1.OperatingSystemSpec{
							Ubuntu: &apiv1.UbuntuSpec{
								DistUpgradeOnBoot: true,
							},
						},
						SSHUserName: "root",
						Versions: apiv1.NodeVersionInfo{
							Kubelet: "v9.9.9",
						},
					},
					Status: apiv1.NodeStatus{
						MachineName: "venus",
						Capacity: apiv1.NodeResources{
							CPU:    "0",
							Memory: "0",
						},
						Allocatable: apiv1.NodeResources{
							CPU:    "0",
							Memory: "0",
						},
					},
				},
				{
					ObjectMeta: apiv1.ObjectMeta{
						ID:   "mars",
						Name: "mars",
					},
					Spec: apiv1.NodeSpec{
						Cloud: apiv1.NodeCloudSpec{
							AWS: &apiv1.AWSNodeSpec{
								InstanceType:     "t2.micro",
								VolumeSize:       50,
								VolumeType:       "standard",
								AvailabilityZone: "eu-central-1a",
								SubnetID:         "subnet-2bff4f43",
							},
						},
						OperatingSystem: apiv1.OperatingSystemSpec{
							Ubuntu: &apiv1.UbuntuSpec{
								DistUpgradeOnBoot: false,
							},
						},
						SSHUserName: "ubuntu",
						Versions: apiv1.NodeVersionInfo{
							Kubelet: "v9.9.9",
						},
					},
					Status: apiv1.NodeStatus{
						MachineName: "mars",
						Capacity: apiv1.NodeResources{
							CPU:    "0",
							Memory: "0",
						},
						Allocatable: apiv1.NodeResources{
							CPU:    "0",
							Memory: "0",
						},
					},
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v2/projects/%s/clusters/%s/nodes", tc.ProjectIDToSync, tc.ClusterIDToSync), strings.NewReader(""))
			res := httptest.NewRecorder()
			kubermaticObj := []ctrlruntimeclient.Object{}
			machineObj := []ctrlruntimeclient.Object{}
			kubernetesObj := []ctrlruntimeclient.Object{}
			kubermaticObj = append(kubermaticObj, tc.ExistingKubermaticObjs...)
			for _, existingNode := range tc.ExistingNodes {
				kubernetesObj = append(kubernetesObj, existingNode)
			}
			for _, existingMachine := range tc.ExistingMachines {
				machineObj = append(machineObj, existingMachine)
			}
			ep, _, err := test.CreateTestEndpointAndGetClients(*tc.ExistingAPIUser, nil, kubernetesObj, machineObj, kubermaticObj, nil, hack.NewTestRouting)
			if err != nil {
				t.Fatalf("failed to create test endpoint: %v", err)
			}

			ep.ServeHTTP(res, req)

			if res.Code != tc.HTTPStatus {
				t.Fatalf("Expected HTTP status code %d, got %d: %s", tc.HTTPStatus, res.Code, res.Body.String())
			}

			actualNodes := test.NodeV1SliceWrapper{}
			actualNodes.DecodeOrDie(res.Body, t).Sort()

			wrappedExpectedNodes := test.NodeV1SliceWrapper(tc.ExpectedResponse)
			wrappedExpectedNodes.Sort()

			actualNodes.EqualOrDie(wrappedExpectedNodes, t)
		})
	}
}

func TestMachineDeploymentMetrics(t *testing.T) {
	t.Parallel()

	cpuQuantity, err := resource.ParseQuantity("290104582")
	if err != nil {
		t.Fatal(err)
	}
	memoryQuantity, err := resource.ParseQuantity("687202304")
	if err != nil {
		t.Fatal(err)
	}

	testcases := []struct {
		Name                       string
		ExpectedResponse           string
		HTTPStatus                 int
		ProjectIDToSync            string
		ClusterIDToSync            string
		ExistingProject            *kubermaticv1.Project
		ExistingKubermaticUser     *kubermaticv1.User
		ExistingAPIUser            *apiv1.User
		ExistingCluster            *kubermaticv1.Cluster
		ExistingNodes              []*corev1.Node
		ExistingMachineDeployments []*clusterv1alpha1.MachineDeployment
		ExistingMachines           []*clusterv1alpha1.Machine
		ExistingKubermaticObjs     []ctrlruntimeclient.Object
		ExistingMetrics            []*v1beta1.NodeMetrics
		MachineDeploymentID        string
	}{
		// scenario 1
		{
			Name:            "scenario 1: get metrics for the machine deployment nodes",
			HTTPStatus:      http.StatusOK,
			ClusterIDToSync: test.GenDefaultCluster().Name,
			ProjectIDToSync: test.GenDefaultProject().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				test.GenDefaultCluster(),
			),
			ExistingAPIUser: test.GenDefaultAPIUser(),
			ExistingNodes: []*corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "venus-1"}, Status: corev1.NodeStatus{Allocatable: map[corev1.ResourceName]resource.Quantity{"cpu": cpuQuantity, "memory": memoryQuantity}}},
			},
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123"}, false),
				genTestMachineDeployment("mars", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false),
			},
			MachineDeploymentID: "venus",
			ExistingMachines: []*clusterv1alpha1.Machine{
				genTestMachine("venus-1", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"},"operatingSystem":"ubuntu","containerRuntimeInfo":{"name":"docker","version":"1.13"},"operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123", "some-other": "xyz"}, nil),
				genTestMachine("venus-2", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"},"operatingSystem":"ubuntu","containerRuntimeInfo":{"name":"docker","version":"1.13"},"operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123", "xyz": "abc"}, nil),
			},
			ExistingMetrics: []*v1beta1.NodeMetrics{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "venus-1"},
					Usage:      map[corev1.ResourceName]resource.Quantity{"cpu": cpuQuantity, "memory": memoryQuantity},
				},
			},
			ExpectedResponse: `[{"name":"venus-1","memoryTotalBytes":655,"memoryAvailableBytes":655,"memoryUsedPercentage":100,"cpuTotalMillicores":290104582000,"cpuAvailableMillicores":290104582000,"cpuUsedPercentage":100}]`,
		},
		// scenario 2
		{
			Name:            "scenario 2: the admin John can get any metrics for the machine deployment nodes",
			HTTPStatus:      http.StatusOK,
			ClusterIDToSync: test.GenDefaultCluster().Name,
			ProjectIDToSync: test.GenDefaultProject().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				test.GenDefaultCluster(),
				test.GenAdminUser("John", "john@acme.com", true),
			),
			ExistingAPIUser: test.GenAPIUser("John", "john@acme.com"),
			ExistingNodes: []*corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "venus-1"}, Status: corev1.NodeStatus{Allocatable: map[corev1.ResourceName]resource.Quantity{"cpu": cpuQuantity, "memory": memoryQuantity}}},
			},
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123"}, false),
				genTestMachineDeployment("mars", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false),
			},
			MachineDeploymentID: "venus",
			ExistingMachines: []*clusterv1alpha1.Machine{
				genTestMachine("venus-1", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"},"operatingSystem":"ubuntu","containerRuntimeInfo":{"name":"docker","version":"1.13"},"operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123", "some-other": "xyz"}, nil),
				genTestMachine("venus-2", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"},"operatingSystem":"ubuntu","containerRuntimeInfo":{"name":"docker","version":"1.13"},"operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123", "xyz": "abc"}, nil),
			},
			ExistingMetrics: []*v1beta1.NodeMetrics{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "venus-1"},
					Usage:      map[corev1.ResourceName]resource.Quantity{"cpu": cpuQuantity, "memory": memoryQuantity},
				},
			},
			ExpectedResponse: `[{"name":"venus-1","memoryTotalBytes":655,"memoryAvailableBytes":655,"memoryUsedPercentage":100,"cpuTotalMillicores":290104582000,"cpuAvailableMillicores":290104582000,"cpuUsedPercentage":100}]`,
		},
		// scenario 3
		{
			Name:            "scenario 3: the user John can not get Bob's metrics",
			HTTPStatus:      http.StatusForbidden,
			ClusterIDToSync: test.GenDefaultCluster().Name,
			ProjectIDToSync: test.GenDefaultProject().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				test.GenDefaultCluster(),
				test.GenAdminUser("John", "john@acme.com", false),
			),
			ExistingAPIUser: test.GenAPIUser("John", "john@acme.com"),
			ExistingNodes: []*corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "venus-1"}, Status: corev1.NodeStatus{Allocatable: map[corev1.ResourceName]resource.Quantity{"cpu": cpuQuantity, "memory": memoryQuantity}}},
			},
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123"}, false),
				genTestMachineDeployment("mars", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false),
			},
			MachineDeploymentID: "venus",
			ExistingMachines: []*clusterv1alpha1.Machine{
				genTestMachine("venus-1", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"},"operatingSystem":"ubuntu","containerRuntimeInfo":{"name":"docker","version":"1.13"},"operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123", "some-other": "xyz"}, nil),
				genTestMachine("venus-2", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"},"operatingSystem":"ubuntu","containerRuntimeInfo":{"name":"docker","version":"1.13"},"operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123", "xyz": "abc"}, nil),
			},
			ExistingMetrics: []*v1beta1.NodeMetrics{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "venus-1"},
					Usage:      map[corev1.ResourceName]resource.Quantity{"cpu": cpuQuantity, "memory": memoryQuantity},
				},
			},
			ExpectedResponse: `{"error":{"code":403,"message":"forbidden: \"john@acme.com\" doesn't belong to project my-first-project-ID"}}`,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v2/projects/%s/clusters/%s/machinedeployments/%s/nodes/metrics", tc.ProjectIDToSync, tc.ClusterIDToSync, tc.MachineDeploymentID), strings.NewReader(""))
			res := httptest.NewRecorder()
			kubermaticObj := []ctrlruntimeclient.Object{}
			machineObj := []ctrlruntimeclient.Object{}
			kubernetesObj := []ctrlruntimeclient.Object{}
			for _, existingNode := range tc.ExistingNodes {
				kubernetesObj = append(kubernetesObj, existingNode)
			}
			for _, existingMetric := range tc.ExistingMetrics {
				machineObj = append(machineObj, existingMetric)
			}
			for _, existingMachineDeployment := range tc.ExistingMachineDeployments {
				machineObj = append(machineObj, existingMachineDeployment)
			}
			for _, existingMachine := range tc.ExistingMachines {
				machineObj = append(machineObj, existingMachine)
			}
			kubermaticObj = append(kubermaticObj, tc.ExistingKubermaticObjs...)
			ep, _, err := test.CreateTestEndpointAndGetClients(*tc.ExistingAPIUser, nil, kubernetesObj, machineObj, kubermaticObj, nil, hack.NewTestRouting)
			if err != nil {
				t.Fatalf("failed to create test endpoint: %v", err)
			}

			ep.ServeHTTP(res, req)

			if res.Code != tc.HTTPStatus {
				t.Fatalf("Expected HTTP status code %d, got %d: %s", tc.HTTPStatus, res.Code, res.Body.String())
			}
			test.CompareWithResult(t, res, tc.ExpectedResponse)
		})
	}
}

func TestPatchMachineDeployment(t *testing.T) {
	t.Parallel()

	var replicas int32 = 1
	var replicasUpdated int32 = 3
	var minReplicas = 1
	var minReplicasUpdated = 2
	var maxReplicas = 10
	var kubeletVerUpdated = "v9.8.0"

	testcases := []struct {
		Name                       string
		Body                       string
		ExpectedResponse           string
		HTTPStatus                 int
		cluster                    string
		project                    string
		ExistingAPIUser            *apiv1.User
		NodeDeploymentID           string
		ExistingMachineDeployments []*clusterv1alpha1.MachineDeployment
		ExistingKubermaticObjs     []ctrlruntimeclient.Object
	}{
		// Scenario 1: Update replicas count.
		{
			Name:             "Scenario 1: Update replicas count",
			Body:             fmt.Sprintf(`{"spec":{"replicas":%v}}`, replicasUpdated),
			ExpectedResponse: fmt.Sprintf(`{"id":"venus","name":"venus","creationTimestamp":"0001-01-01T00:00:00Z","spec":{"replicas":%v,"template":{"cloud":{"digitalocean":{"size":"2GB","backups":false,"ipv6":false,"monitoring":false,"tags":["kubernetes","kubernetes-cluster-defClusterID","system-cluster-defClusterID","system-project-my-first-project-ID"]}},"operatingSystem":{"ubuntu":{"distUpgradeOnBoot":true}},"network":{"cidr":"","gateway":"","dns":{"servers":null},"ipFamily":"IPv4"},"versions":{"kubelet":"v9.9.9"},"labels":{"system/cluster":"defClusterID","system/project":"my-first-project-ID"}},"paused":false,"dynamicConfig":false},"status":{}}`, replicasUpdated),
			cluster:          "keen-snyder",
			HTTPStatus:       http.StatusOK,
			project:          test.GenDefaultProject().Name,
			ExistingAPIUser:  test.GenDefaultAPIUser(),
			NodeDeploymentID: "venus",
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false),
			},
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				genTestCluster(true),
			),
		},
		// Scenario 2: Update kubelet version.
		{
			Name:                       "Scenario 2: Update kubelet version",
			Body:                       fmt.Sprintf(`{"spec":{"template":{"versions":{"kubelet":"%v"}}}}`, kubeletVerUpdated),
			ExpectedResponse:           fmt.Sprintf(`{"id":"venus","name":"venus","creationTimestamp":"0001-01-01T00:00:00Z","spec":{"replicas":%v,"template":{"cloud":{"digitalocean":{"size":"2GB","backups":false,"ipv6":false,"monitoring":false,"tags":["kubernetes","kubernetes-cluster-defClusterID","system-cluster-defClusterID","system-project-my-first-project-ID"]}},"operatingSystem":{"ubuntu":{"distUpgradeOnBoot":true}},"network":{"cidr":"","gateway":"","dns":{"servers":null},"ipFamily":"IPv4"},"versions":{"kubelet":"%v"},"labels":{"system/cluster":"defClusterID","system/project":"my-first-project-ID"}},"paused":false,"dynamicConfig":false},"status":{}}`, replicas, kubeletVerUpdated),
			cluster:                    "keen-snyder",
			HTTPStatus:                 http.StatusOK,
			project:                    test.GenDefaultProject().Name,
			ExistingAPIUser:            test.GenDefaultAPIUser(),
			NodeDeploymentID:           "venus",
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false)},
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				genTestCluster(true),
			),
		},
		// Scenario 3: Change to paused.
		{
			Name:                       "Scenario 3: Change to paused",
			Body:                       `{"spec":{"paused":true}}`,
			ExpectedResponse:           `{"id":"venus","name":"venus","creationTimestamp":"0001-01-01T00:00:00Z","spec":{"replicas":1,"template":{"cloud":{"digitalocean":{"size":"2GB","backups":false,"ipv6":false,"monitoring":false,"tags":["kubernetes","kubernetes-cluster-defClusterID","system-cluster-defClusterID","system-project-my-first-project-ID"]}},"operatingSystem":{"ubuntu":{"distUpgradeOnBoot":true}},"network":{"cidr":"","gateway":"","dns":{"servers":null},"ipFamily":"IPv4"},"versions":{"kubelet":"v9.9.9"},"labels":{"system/cluster":"defClusterID","system/project":"my-first-project-ID"}},"paused":true,"dynamicConfig":false},"status":{}}`,
			cluster:                    "keen-snyder",
			HTTPStatus:                 http.StatusOK,
			project:                    test.GenDefaultProject().Name,
			ExistingAPIUser:            test.GenDefaultAPIUser(),
			NodeDeploymentID:           "venus",
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false)},
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				genTestCluster(true),
			),
		},
		// Scenario 4: Downgrade to too old kubelet version
		{
			Name:                       "Scenario 4: Downgrade kubelet to too old",
			Body:                       `{"spec":{"template":{"versions":{"kubelet":"9.6.0"}}}}`,
			ExpectedResponse:           `{"error":{"code":400,"message":"kubelet version 9.6.0 is not compatible with control plane version 9.9.9"}}`,
			cluster:                    "keen-snyder",
			HTTPStatus:                 http.StatusBadRequest,
			project:                    test.GenDefaultProject().Name,
			ExistingAPIUser:            test.GenDefaultAPIUser(),
			NodeDeploymentID:           "venus",
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false)},
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				genTestCluster(true),
			),
		},
		// Scenario 5: Upgrade kubelet to a too new version
		{
			Name:                       "Scenario 5: Upgrade kubelet to too new",
			Body:                       `{"spec":{"template":{"versions":{"kubelet":"9.10.0"}}}}`,
			ExpectedResponse:           `{"error":{"code":400,"message":"kubelet version 9.10.0 is not compatible with control plane version 9.9.9"}}`,
			cluster:                    "keen-snyder",
			HTTPStatus:                 http.StatusBadRequest,
			project:                    test.GenDefaultProject().Name,
			ExistingAPIUser:            test.GenDefaultAPIUser(),
			NodeDeploymentID:           "venus",
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false)},
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				genTestCluster(true),
			),
		},
		// Scenario 6: The admin John can update any node deployment.
		{
			Name:                       "Scenario 6: The admin John can update any machine deployment",
			Body:                       fmt.Sprintf(`{"spec":{"replicas":%v}}`, replicasUpdated),
			ExpectedResponse:           fmt.Sprintf(`{"id":"venus","name":"venus","creationTimestamp":"0001-01-01T00:00:00Z","spec":{"replicas":%v,"template":{"cloud":{"digitalocean":{"size":"2GB","backups":false,"ipv6":false,"monitoring":false,"tags":["kubernetes","kubernetes-cluster-defClusterID","system-cluster-defClusterID","system-project-my-first-project-ID"]}},"operatingSystem":{"ubuntu":{"distUpgradeOnBoot":true}},"network":{"cidr":"","gateway":"","dns":{"servers":null},"ipFamily":"IPv4"},"versions":{"kubelet":"v9.9.9"},"labels":{"system/cluster":"defClusterID","system/project":"my-first-project-ID"}},"paused":false,"dynamicConfig":false},"status":{}}`, replicasUpdated),
			cluster:                    "keen-snyder",
			HTTPStatus:                 http.StatusOK,
			project:                    test.GenDefaultProject().Name,
			ExistingAPIUser:            test.GenAPIUser("John", "john@acme.com"),
			NodeDeploymentID:           "venus",
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false)},
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				genTestCluster(true),
				test.GenAdminUser("John", "john@acme.com", true),
			),
		},
		// Scenario 7: The user John can not update Bob's node deployment.
		{
			Name:                       "Scenario 7: The user John can not update Bob's machine deployment",
			Body:                       fmt.Sprintf(`{"spec":{"replicas":%v}}`, replicasUpdated),
			ExpectedResponse:           `{"error":{"code":403,"message":"forbidden: \"john@acme.com\" doesn't belong to project my-first-project-ID"}}`,
			cluster:                    "keen-snyder",
			HTTPStatus:                 http.StatusForbidden,
			project:                    test.GenDefaultProject().Name,
			ExistingAPIUser:            test.GenAPIUser("John", "john@acme.com"),
			NodeDeploymentID:           "venus",
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false)},
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				genTestCluster(true),
				test.GenAdminUser("John", "john@acme.com", false),
			),
		},
		// Scenario 8: Update replicas count in range
		{
			Name: "Scenario 8: Update replicas count in range",
			Body: fmt.Sprintf(`{"spec":{"replicas":%v}}`, replicasUpdated),
			ExpectedResponse: fmt.Sprintf(
				`{"id":"venus","name":"venus","annotations":{"cluster.k8s.io/cluster-api-autoscaler-node-group-min-size":"%[2]v"},"creationTimestamp":"0001-01-01T00:00:00Z","spec":{"replicas":%[1]v,"template":{"cloud":{"digitalocean":{"size":"2GB","backups":false,"ipv6":false,"monitoring":false,"tags":["kubernetes","kubernetes-cluster-defClusterID","system-cluster-defClusterID","system-project-my-first-project-ID"]}},"operatingSystem":{"ubuntu":{"distUpgradeOnBoot":true}},"network":{"cidr":"","gateway":"","dns":{"servers":null},"ipFamily":"IPv4"},"versions":{"kubelet":"v9.9.9"},"labels":{"system/cluster":"defClusterID","system/project":"my-first-project-ID"}},"paused":false,"dynamicConfig":false,"minReplicas":%[2]v},"status":{}}`,
				replicasUpdated,
				minReplicas,
			),
			cluster:          "keen-snyder",
			HTTPStatus:       http.StatusOK,
			project:          test.GenDefaultProject().Name,
			ExistingAPIUser:  test.GenDefaultAPIUser(),
			NodeDeploymentID: "venus",
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				func() *clusterv1alpha1.MachineDeployment {
					md := genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false)
					md.Annotations = map[string]string{
						machine.AutoscalerMinSizeAnnotation: "1",
					}
					return md
				}(),
			},
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				genTestCluster(true),
			),
		},
		// Scenario 9: Try to update to replicas count over max
		{
			Name:             "Scenario 9: Try to update replicas count over max",
			Body:             fmt.Sprintf(`{"spec":{"replicas":%v}}`, replicasUpdated),
			ExpectedResponse: `{"error":{"code":400,"message":"replica count (3) cannot be higher then autoscaler maxreplicas (2)"}}`,
			cluster:          "keen-snyder",
			HTTPStatus:       http.StatusBadRequest,
			project:          test.GenDefaultProject().Name,
			ExistingAPIUser:  test.GenDefaultAPIUser(),
			NodeDeploymentID: "venus",
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				func() *clusterv1alpha1.MachineDeployment {
					md := genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false)
					md.Annotations = map[string]string{
						machine.AutoscalerMaxSizeAnnotation: "2",
					}
					return md
				}(),
			},
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				genTestCluster(true),
			),
		},
		// Scenario 10: Try to update replicas count below min
		{
			Name:             "Scenario 10: Try to update replicas count below min",
			Body:             fmt.Sprintf(`{"spec":{"replicas":%v}}`, replicasUpdated),
			ExpectedResponse: `{"error":{"code":400,"message":"replica count (3) cannot be lower then autoscaler minreplicas (5)"}}`,
			cluster:          "keen-snyder",
			HTTPStatus:       http.StatusBadRequest,
			project:          test.GenDefaultProject().Name,
			ExistingAPIUser:  test.GenDefaultAPIUser(),
			NodeDeploymentID: "venus",
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				func() *clusterv1alpha1.MachineDeployment {
					md := genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false)
					md.Annotations = map[string]string{
						machine.AutoscalerMinSizeAnnotation: "5",
					}
					return md
				}(),
			},
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				genTestCluster(true),
			),
		},
		// Scenario 11: Update autoscaling configuration - increase min
		{
			Name: "Scenario 11: Update autoscaling configuration - increase min",
			Body: fmt.Sprintf(`{"spec":{"minReplicas": %d}}`, minReplicasUpdated),
			ExpectedResponse: fmt.Sprintf(
				`{"id":"venus","name":"venus","annotations":{"cluster.k8s.io/cluster-api-autoscaler-node-group-min-size":"%[1]v"},"creationTimestamp":"0001-01-01T00:00:00Z","spec":{"replicas":5,"template":{"cloud":{"digitalocean":{"size":"2GB","backups":false,"ipv6":false,"monitoring":false,"tags":["kubernetes","kubernetes-cluster-defClusterID","system-cluster-defClusterID","system-project-my-first-project-ID"]}},"operatingSystem":{"ubuntu":{"distUpgradeOnBoot":true}},"network":{"cidr":"","gateway":"","dns":{"servers":null},"ipFamily":"IPv4"},"versions":{"kubelet":"v9.9.9"},"labels":{"system/cluster":"defClusterID","system/project":"my-first-project-ID"}},"paused":false,"dynamicConfig":false,"minReplicas":%[1]v},"status":{}}`,
				minReplicasUpdated,
			),
			cluster:          "keen-snyder",
			HTTPStatus:       http.StatusOK,
			project:          test.GenDefaultProject().Name,
			ExistingAPIUser:  test.GenDefaultAPIUser(),
			NodeDeploymentID: "venus",
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				func() *clusterv1alpha1.MachineDeployment {
					md := genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false)
					md.Annotations = map[string]string{
						machine.AutoscalerMinSizeAnnotation: fmt.Sprintf("%d", minReplicas),
					}
					md.Spec.Replicas = ptr.To[int32](5)
					return md
				}(),
			},
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				genTestCluster(true),
			),
		},
		// Scenario 12: Unset autoscaling configuration
		{
			Name:             "Scenario 12: Unset autoscaling configuration",
			Body:             `{"spec":{"minReplicas":null,"maxReplicas":null}}`,
			ExpectedResponse: `{"id":"venus","name":"venus","creationTimestamp":"0001-01-01T00:00:00Z","spec":{"replicas":5,"template":{"cloud":{"digitalocean":{"size":"2GB","backups":false,"ipv6":false,"monitoring":false,"tags":["kubernetes","kubernetes-cluster-defClusterID","system-cluster-defClusterID","system-project-my-first-project-ID"]}},"operatingSystem":{"ubuntu":{"distUpgradeOnBoot":true}},"network":{"cidr":"","gateway":"","dns":{"servers":null},"ipFamily":"IPv4"},"versions":{"kubelet":"v9.9.9"},"labels":{"system/cluster":"defClusterID","system/project":"my-first-project-ID"}},"paused":false,"dynamicConfig":false},"status":{}}`,
			cluster:          "keen-snyder",
			HTTPStatus:       http.StatusOK,
			project:          test.GenDefaultProject().Name,
			ExistingAPIUser:  test.GenDefaultAPIUser(),
			NodeDeploymentID: "venus",
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				func() *clusterv1alpha1.MachineDeployment {
					md := genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false)
					md.Annotations = map[string]string{
						machine.AutoscalerMinSizeAnnotation: fmt.Sprintf("%d", minReplicas),
						machine.AutoscalerMaxSizeAnnotation: fmt.Sprintf("%d", maxReplicas),
					}
					md.Spec.Replicas = ptr.To[int32](5)
					return md
				}(),
			},
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				genTestCluster(true),
			),
		},
		// Scenario 13: Create autoscaling configuration
		{
			Name: "Scenario 13: Create autoscaling configuration",
			Body: fmt.Sprintf(`{"spec":{"minReplicas": %[1]d,"maxReplicas": %[2]d}}`, minReplicas, maxReplicas),
			ExpectedResponse: fmt.Sprintf(
				`{"id":"venus","name":"venus","annotations":{"cluster.k8s.io/cluster-api-autoscaler-node-group-max-size":"%[1]v","cluster.k8s.io/cluster-api-autoscaler-node-group-min-size":"%[2]v"},"creationTimestamp":"0001-01-01T00:00:00Z","spec":{"replicas":1,"template":{"cloud":{"digitalocean":{"size":"2GB","backups":false,"ipv6":false,"monitoring":false,"tags":["kubernetes","kubernetes-cluster-defClusterID","system-cluster-defClusterID","system-project-my-first-project-ID"]}},"operatingSystem":{"ubuntu":{"distUpgradeOnBoot":true}},"network":{"cidr":"","gateway":"","dns":{"servers":null},"ipFamily":"IPv4"},"versions":{"kubelet":"v9.9.9"},"labels":{"system/cluster":"defClusterID","system/project":"my-first-project-ID"}},"paused":false,"dynamicConfig":false,"minReplicas":%[2]d,"maxReplicas":%[1]d},"status":{}}`,
				maxReplicas,
				minReplicas,
			),
			cluster:          "keen-snyder",
			HTTPStatus:       http.StatusOK,
			project:          test.GenDefaultProject().Name,
			ExistingAPIUser:  test.GenDefaultAPIUser(),
			NodeDeploymentID: "venus",
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false),
			},
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				genTestCluster(true),
			),
		},
		// Scenario 14: Use illegal autoscaling values
		{
			Name:             "Scenario 14: Use illegal autoscaling values",
			Body:             fmt.Sprintf(`{"spec":{"minReplicas": %[1]d,"maxReplicas": %[2]d}}`, 0, -2),
			ExpectedResponse: fmt.Sprintf(`{"error":{"code":400,"message":"cannot decode patched nodedeployment: {\"spec\":{\"minReplicas\": %d,\"maxReplicas\": %d}}"}}`, 0, -2),
			cluster:          "keen-snyder",
			HTTPStatus:       http.StatusBadRequest,
			project:          test.GenDefaultProject().Name,
			ExistingAPIUser:  test.GenDefaultAPIUser(),
			NodeDeploymentID: "venus",
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false),
			},
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				genTestCluster(true),
			),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v2/projects/%s/clusters/%s/machinedeployments/%s",
				test.GenDefaultProject().Name, test.GenDefaultCluster().Name, tc.NodeDeploymentID), strings.NewReader(tc.Body))
			res := httptest.NewRecorder()
			kubermaticObj := []ctrlruntimeclient.Object{}
			machineDeploymentObjects := []ctrlruntimeclient.Object{}
			kubernetesObj := []ctrlruntimeclient.Object{}
			kubermaticObj = append(kubermaticObj, tc.ExistingKubermaticObjs...)
			for _, existingMachineDeployment := range tc.ExistingMachineDeployments {
				machineDeploymentObjects = append(machineDeploymentObjects, existingMachineDeployment)
			}
			ep, _, err := test.CreateTestEndpointAndGetClients(*tc.ExistingAPIUser, nil, kubernetesObj, machineDeploymentObjects, kubermaticObj, nil, hack.NewTestRouting)
			if err != nil {
				t.Fatalf("failed to create test endpoint: %v", err)
			}

			ep.ServeHTTP(res, req)

			if res.Code != tc.HTTPStatus {
				t.Fatalf("Expected HTTP status code %d, got %d: %s", tc.HTTPStatus, res.Code, res.Body.String())
			}

			test.CompareWithResult(t, res, tc.ExpectedResponse)
		})
	}
}

func TestListNodeDeploymentNodesEvents(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		Name                       string
		HTTPStatus                 int
		ExpectedResult             string
		ProjectIDToSync            string
		ClusterIDToSync            string
		ExistingProject            *kubermaticv1.Project
		ExistingKubermaticUser     *kubermaticv1.User
		ExistingAPIUser            *apiv1.User
		ExistingCluster            *kubermaticv1.Cluster
		ExistingNodes              []*corev1.Node
		ExistingMachineDeployments []*clusterv1alpha1.MachineDeployment
		ExistingMachines           []*clusterv1alpha1.Machine
		ExistingKubermaticObjs     []ctrlruntimeclient.Object
		ExistingEvents             []*corev1.Event
		MachineDeploymentID        string
		QueryParams                string
	}{
		// scenario 1
		{
			Name:            "scenario 1: list all events",
			HTTPStatus:      http.StatusOK,
			ClusterIDToSync: test.GenDefaultCluster().Name,
			ProjectIDToSync: test.GenDefaultProject().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				test.GenDefaultCluster(),
			),
			ExistingAPIUser: test.GenDefaultAPIUser(),
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123"}, false),
			},
			MachineDeploymentID: "venus",
			ExistingMachines: []*clusterv1alpha1.Machine{
				genTestMachine("venus-1", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"},"operatingSystem":"ubuntu","containerRuntimeInfo":{"name":"docker","version":"1.13"},"operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123", "some-other": "xyz"}, nil),
			},
			ExistingEvents: []*corev1.Event{
				test.GenTestEvent("event-1", corev1.EventTypeNormal, "Started", "message started", "Machine", "venus-1-machine", "venus-1"),
				test.GenTestEvent("event-2", corev1.EventTypeWarning, "Killed", "message killed", "Machine", "venus-1-machine", "venus-1"),
			},
			ExpectedResult: `[{"name":"event-1","creationTimestamp":"0001-01-01T00:00:00Z","message":"message started","type":"Normal","involvedObject":{"type":"Node","namespace":"kube-system","name":"venus-1"},"lastTimestamp":"0001-01-01T00:00:00Z","count":1},{"name":"event-2","creationTimestamp":"0001-01-01T00:00:00Z","message":"message killed","type":"Warning","involvedObject":{"type":"Node","namespace":"kube-system","name":"venus-1"},"lastTimestamp":"0001-01-01T00:00:00Z","count":1}]`,
		},
		// scenario 2
		{
			Name:            "scenario 2: list all warning events",
			QueryParams:     "?type=warning",
			HTTPStatus:      http.StatusOK,
			ClusterIDToSync: test.GenDefaultCluster().Name,
			ProjectIDToSync: test.GenDefaultProject().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				test.GenDefaultCluster(),
			),
			ExistingAPIUser: test.GenDefaultAPIUser(),
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123"}, false),
			},
			MachineDeploymentID: "venus",
			ExistingMachines: []*clusterv1alpha1.Machine{
				genTestMachine("venus-1", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"},"operatingSystem":"ubuntu","containerRuntimeInfo":{"name":"docker","version":"1.13"},"operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123", "some-other": "xyz"}, nil),
			},
			ExistingEvents: []*corev1.Event{
				test.GenTestEvent("event-1", corev1.EventTypeNormal, "Started", "message started", "Machine", "venus-1-machine", "venus-1"),
				test.GenTestEvent("event-2", corev1.EventTypeWarning, "Killed", "message killed", "Machine", "venus-1-machine", "venus-1"),
			},
			ExpectedResult: `[{"name":"event-2","creationTimestamp":"0001-01-01T00:00:00Z","message":"message killed","type":"Warning","involvedObject":{"type":"Node","namespace":"kube-system","name":"venus-1"},"lastTimestamp":"0001-01-01T00:00:00Z","count":1}]`,
		},
		// scenario 3
		{
			Name:            "scenario 3: list all normal events",
			QueryParams:     "?type=normal",
			HTTPStatus:      http.StatusOK,
			ClusterIDToSync: test.GenDefaultCluster().Name,
			ProjectIDToSync: test.GenDefaultProject().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				test.GenDefaultCluster(),
			),
			ExistingAPIUser: test.GenDefaultAPIUser(),
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123"}, false),
			},
			MachineDeploymentID: "venus",
			ExistingMachines: []*clusterv1alpha1.Machine{
				genTestMachine("venus-1", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"},"operatingSystem":"ubuntu","containerRuntimeInfo":{"name":"docker","version":"1.13"},"operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123", "some-other": "xyz"}, nil),
			},
			ExistingEvents: []*corev1.Event{
				test.GenTestEvent("event-1", corev1.EventTypeNormal, "Started", "message started", "Machine", "venus-1-machine", "venus-1"),
				test.GenTestEvent("event-2", corev1.EventTypeWarning, "Killed", "message killed", "Machine", "venus-1-machine", "venus-1"),
			},
			ExpectedResult: `[{"name":"event-1","creationTimestamp":"0001-01-01T00:00:00Z","message":"message started","type":"Normal","involvedObject":{"type":"Node","namespace":"kube-system","name":"venus-1"},"lastTimestamp":"0001-01-01T00:00:00Z","count":1}]`,
		},
		// scenario 4
		{
			Name:            "scenario 4: the admin John can list any events",
			HTTPStatus:      http.StatusOK,
			ClusterIDToSync: test.GenDefaultCluster().Name,
			ProjectIDToSync: test.GenDefaultProject().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				test.GenDefaultCluster(),
				test.GenAdminUser("John", "john@acme.com", true),
			),
			ExistingAPIUser: test.GenAPIUser("John", "john@acme.com"),
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123"}, false),
			},
			MachineDeploymentID: "venus",
			ExistingMachines: []*clusterv1alpha1.Machine{
				genTestMachine("venus-1", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"},"operatingSystem":"ubuntu","containerRuntimeInfo":{"name":"docker","version":"1.13"},"operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123", "some-other": "xyz"}, nil),
			},
			ExistingEvents: []*corev1.Event{
				test.GenTestEvent("event-1", corev1.EventTypeNormal, "Started", "message started", "Machine", "venus-1-machine", "venus-1"),
				test.GenTestEvent("event-2", corev1.EventTypeWarning, "Killed", "message killed", "Machine", "venus-1-machine", "venus-1"),
			},
			ExpectedResult: `[{"name":"event-1","creationTimestamp":"0001-01-01T00:00:00Z","message":"message started","type":"Normal","involvedObject":{"type":"Node","namespace":"kube-system","name":"venus-1"},"lastTimestamp":"0001-01-01T00:00:00Z","count":1},{"name":"event-2","creationTimestamp":"0001-01-01T00:00:00Z","message":"message killed","type":"Warning","involvedObject":{"type":"Node","namespace":"kube-system","name":"venus-1"},"lastTimestamp":"0001-01-01T00:00:00Z","count":1}]`,
		},
		// scenario 5
		{
			Name:            "scenario 5: the user John can not list Bob's events",
			HTTPStatus:      http.StatusForbidden,
			ClusterIDToSync: test.GenDefaultCluster().Name,
			ProjectIDToSync: test.GenDefaultProject().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				test.GenDefaultCluster(),
				test.GenAdminUser("John", "john@acme.com", false),
			),
			ExistingAPIUser: test.GenAPIUser("John", "john@acme.com"),
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123"}, false),
			},
			MachineDeploymentID: "venus",
			ExistingMachines: []*clusterv1alpha1.Machine{
				genTestMachine("venus-1", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"},"operatingSystem":"ubuntu","containerRuntimeInfo":{"name":"docker","version":"1.13"},"operatingSystemSpec":{"distUpgradeOnBoot":true}}`, map[string]string{"md-id": "123", "some-other": "xyz"}, nil),
			},
			ExistingEvents: []*corev1.Event{
				test.GenTestEvent("event-1", corev1.EventTypeNormal, "Started", "message started", "Machine", "venus-1-machine", "venus-1"),
				test.GenTestEvent("event-2", corev1.EventTypeWarning, "Killed", "message killed", "Machine", "venus-1-machine", "venus-1"),
			},
			ExpectedResult: `{"error":{"code":403,"message":"forbidden: \"john@acme.com\" doesn't belong to project my-first-project-ID"}}`,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v2/projects/%s/clusters/%s/machinedeployments/%s/nodes/events%s", tc.ProjectIDToSync, tc.ClusterIDToSync, tc.MachineDeploymentID, tc.QueryParams), strings.NewReader(""))
			res := httptest.NewRecorder()
			kubermaticObj := []ctrlruntimeclient.Object{}
			machineObj := []ctrlruntimeclient.Object{}
			kubernetesObj := []ctrlruntimeclient.Object{}
			for _, existingNode := range tc.ExistingNodes {
				kubernetesObj = append(kubernetesObj, existingNode)
			}
			for _, existingEvents := range tc.ExistingEvents {
				kubernetesObj = append(kubernetesObj, existingEvents)
			}
			for _, existingMachineDeployment := range tc.ExistingMachineDeployments {
				machineObj = append(machineObj, existingMachineDeployment)
			}
			for _, existingMachine := range tc.ExistingMachines {
				machineObj = append(machineObj, existingMachine)
			}
			kubermaticObj = append(kubermaticObj, tc.ExistingKubermaticObjs...)

			ep, _, err := test.CreateTestEndpointAndGetClients(*tc.ExistingAPIUser, nil, kubernetesObj, machineObj, kubermaticObj, nil, hack.NewTestRouting)
			if err != nil {
				t.Fatalf("failed to create test endpoint: %v", err)
			}

			ep.ServeHTTP(res, req)

			if res.Code != tc.HTTPStatus {
				t.Fatalf("Expected HTTP status code %d, got %d: %s", tc.HTTPStatus, res.Code, res.Body.String())
			}

			test.CompareWithResult(t, res, tc.ExpectedResult)
		})
	}
}

func TestDeleteMachineDeployment(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		Name                        string
		HTTPStatus                  int
		MachineIDToDelete           string
		ClusterIDToSync             string
		ProjectIDToSync             string
		ExistingAPIUser             *apiv1.User
		ExistingNodes               []*corev1.Node
		ExistingMachineDeployments  []*clusterv1alpha1.MachineDeployment
		ExistingKubermaticObjs      []ctrlruntimeclient.Object
		ExpectedHTTPStatusOnGet     int
		ExpectedNodeDeploymentCount int
	}{
		// scenario 1
		{
			Name:              "scenario 1: delete the machine deployments that belong to the given cluster",
			HTTPStatus:        http.StatusOK,
			MachineIDToDelete: "venus",
			ClusterIDToSync:   test.GenDefaultCluster().Name,
			ProjectIDToSync:   test.GenDefaultProject().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				test.GenDefaultCluster(),
			),
			ExistingAPIUser: test.GenDefaultAPIUser(),
			ExistingNodes: []*corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "venus"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "mars"}},
			},
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false),
				genTestMachineDeployment("mars", `{"cloudProvider":"aws","cloudProviderSpec":{"token":"dummy-token","region":"eu-central-1","availabilityZone":"eu-central-1a","vpcId":"vpc-819f62e9","subnetId":"subnet-2bff4f43","instanceType":"t2.micro","diskSize":50}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":false}}`, nil, false),
			},
			// Even though the machine deployment object was deleted the associated node object was not.
			// When the client GETs the previously deleted "node" it will get a valid response.
			// That is only true for testing, but in a real cluster, the node object will get deleted by the garbage-collector as it has a ownerRef set.
			ExpectedHTTPStatusOnGet:     http.StatusOK,
			ExpectedNodeDeploymentCount: 1,
		},
		// scenario 2
		{
			Name:              "scenario 2: the admin John can delete any cluster machine deployment",
			HTTPStatus:        http.StatusOK,
			MachineIDToDelete: "venus",
			ClusterIDToSync:   test.GenDefaultCluster().Name,
			ProjectIDToSync:   test.GenDefaultProject().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				test.GenDefaultCluster(),
				test.GenAdminUser("John", "john@acme.com", true),
			),
			ExistingAPIUser: test.GenAPIUser("John", "john@acme.com"),
			ExistingNodes: []*corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "venus"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "mars"}},
			},
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false),
				genTestMachineDeployment("mars", `{"cloudProvider":"aws","cloudProviderSpec":{"token":"dummy-token","region":"eu-central-1","availabilityZone":"eu-central-1a","vpcId":"vpc-819f62e9","subnetId":"subnet-2bff4f43","instanceType":"t2.micro","diskSize":50}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":false}}`, nil, false),
			},
			// Even though the machine deployment object was deleted the associated node object was not.
			// When the client GETs the previously deleted "node" it will get a valid response.
			// That is only true for testing, but in a real cluster, the node object will get deleted by the garbage-collector as it has a ownerRef set.
			ExpectedHTTPStatusOnGet:     http.StatusOK,
			ExpectedNodeDeploymentCount: 1,
		},
		// scenario 3
		{
			Name:              "scenario 3: the user John can delete Bob's cluster machine deployment",
			HTTPStatus:        http.StatusForbidden,
			MachineIDToDelete: "venus",
			ClusterIDToSync:   test.GenDefaultCluster().Name,
			ProjectIDToSync:   test.GenDefaultProject().Name,
			ExistingKubermaticObjs: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				test.GenDefaultCluster(),
				test.GenAdminUser("John", "john@acme.com", false),
			),
			ExistingAPIUser: test.GenAPIUser("John", "john@acme.com"),
			ExistingNodes: []*corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "venus"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "mars"}},
			},
			ExistingMachineDeployments: []*clusterv1alpha1.MachineDeployment{
				genTestMachineDeployment("venus", `{"cloudProvider":"digitalocean","cloudProviderSpec":{"token":"dummy-token","region":"fra1","size":"2GB"}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":true}}`, nil, false),
				genTestMachineDeployment("mars", `{"cloudProvider":"aws","cloudProviderSpec":{"token":"dummy-token","region":"eu-central-1","availabilityZone":"eu-central-1a","vpcId":"vpc-819f62e9","subnetId":"subnet-2bff4f43","instanceType":"t2.micro","diskSize":50}, "operatingSystem":"ubuntu", "operatingSystemSpec":{"distUpgradeOnBoot":false}}`, nil, false),
			},
			ExpectedHTTPStatusOnGet:     http.StatusForbidden,
			ExpectedNodeDeploymentCount: 2,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v2/projects/%s/clusters/%s/machinedeployments/%s",
				tc.ProjectIDToSync, tc.ClusterIDToSync, tc.MachineIDToDelete), strings.NewReader(""))
			res := httptest.NewRecorder()
			kubermaticObj := []ctrlruntimeclient.Object{}
			machineDeploymentObjects := []ctrlruntimeclient.Object{}
			kubernetesObj := []ctrlruntimeclient.Object{}
			kubermaticObj = append(kubermaticObj, tc.ExistingKubermaticObjs...)
			for _, existingNode := range tc.ExistingNodes {
				kubernetesObj = append(kubernetesObj, existingNode)
			}
			for _, existingMachineDeployment := range tc.ExistingMachineDeployments {
				machineDeploymentObjects = append(machineDeploymentObjects, existingMachineDeployment)
			}
			ep, clientsSets, err := test.CreateTestEndpointAndGetClients(*tc.ExistingAPIUser, nil, kubernetesObj, machineDeploymentObjects, kubermaticObj, nil, hack.NewTestRouting)
			if err != nil {
				t.Fatalf("failed to create test endpoint: %v", err)
			}

			ep.ServeHTTP(res, req)

			if res.Code != tc.HTTPStatus {
				t.Fatalf("Expected HTTP status code %d, got %d: %s", tc.HTTPStatus, res.Code, res.Body.String())
			}

			machineDeployments := &clusterv1alpha1.MachineDeploymentList{}
			if err := clientsSets.FakeClient.List(context.Background(), machineDeployments); err != nil {
				t.Fatalf("failed to list MachineDeployments: %v", err)
			}

			if machineDeploymentCount := len(machineDeployments.Items); machineDeploymentCount != tc.ExpectedNodeDeploymentCount {
				t.Errorf("Expected to find %d  machineDeployments but got %d", tc.ExpectedNodeDeploymentCount, machineDeploymentCount)
			}
		})
	}
}

func genTestCluster(isControllerReady bool) *kubermaticv1.Cluster {
	controllerStatus := kubermaticv1.HealthStatusDown
	if isControllerReady {
		controllerStatus = kubermaticv1.HealthStatusUp
	}
	cluster := test.GenDefaultCluster()
	cluster.Status = kubermaticv1.ClusterStatus{
		ExtendedHealth: kubermaticv1.ExtendedClusterHealth{
			Apiserver:                    kubermaticv1.HealthStatusUp,
			ApplicationController:        kubermaticv1.HealthStatusUp,
			Scheduler:                    kubermaticv1.HealthStatusUp,
			Controller:                   controllerStatus,
			MachineController:            kubermaticv1.HealthStatusUp,
			Etcd:                         kubermaticv1.HealthStatusUp,
			CloudProviderInfrastructure:  kubermaticv1.HealthStatusUp,
			UserClusterControllerManager: kubermaticv1.HealthStatusUp,
		},
	}
	cluster.Spec.Cloud = kubermaticv1.CloudSpec{
		DatacenterName: "regular-do1",
	}
	return cluster
}

func genTestMachine(name, rawProviderSpec string, labels map[string]string, ownerRef []metav1.OwnerReference) *clusterv1alpha1.Machine {
	return test.GenTestMachine(name, rawProviderSpec, labels, ownerRef)
}

func genTestMachineDeployment(name, rawProviderSpec string, selector map[string]string, dynamicConfig bool) *clusterv1alpha1.MachineDeployment {
	return test.GenTestMachineDeployment(name, rawProviderSpec, selector, dynamicConfig)
}
