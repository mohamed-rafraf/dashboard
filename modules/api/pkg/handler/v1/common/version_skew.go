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

package common

import (
	"context"
	"errors"
	"fmt"
	"strings"

	semverlib "github.com/Masterminds/semver/v3"

	"k8c.io/dashboard/v2/pkg/provider"
	kubermaticv1 "k8c.io/kubermatic/sdk/v2/apis/kubermatic/v1"
	"k8c.io/kubermatic/v2/pkg/validation/nodeupdate"
	clusterv1alpha1 "k8c.io/machine-controller/sdk/apis/cluster/v1alpha1"

	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// CheckClusterVersionSkew returns a list of machines and/or machine deployments
// that are running kubelet at a version incompatible with the cluster's control plane.
func CheckClusterVersionSkew(ctx context.Context, userInfoGetter provider.UserInfoGetter, clusterProvider provider.ClusterProvider, cluster *kubermaticv1.Cluster, projectID string) ([]string, error) {
	client, err := GetClusterClient(ctx, userInfoGetter, clusterProvider, cluster, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create a machine client: %w", err)
	}

	// get deduplicated list of all used kubelet versions
	kubeletVersions, err := getKubeletVersions(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to get the list of kubelet versions used in the cluster: %w", err)
	}

	// this is where the incompatible versions shall be saved
	incompatibleVersionsSet := map[string]bool{}

	clusterVersion := cluster.Spec.Version.Semver()
	for _, ver := range kubeletVersions {
		kubeletVersion, parseErr := semverlib.NewVersion(ver)
		if parseErr != nil {
			return nil, fmt.Errorf("failed to parse kubelet version: %w", parseErr)
		}

		if err = nodeupdate.EnsureVersionCompatible(clusterVersion, kubeletVersion); err != nil {
			// VersionSkewError says it's incompatible
			if errors.Is(err, nodeupdate.VersionSkewError{}) {
				incompatibleVersionsSet[kubeletVersion.String()] = true
				continue
			}

			// other error types
			return nil, fmt.Errorf("failed to check compatibility between kubelet %q and control plane %q: %w", kubeletVersion, clusterVersion, err)
		}
	}

	// collect the deduplicated map entries into a slice
	var incompatibleVersionsList []string
	for ver := range incompatibleVersionsSet {
		incompatibleVersionsList = append(incompatibleVersionsList, ver)
	}

	return incompatibleVersionsList, nil
}

// getKubeletVersions returns the list of all kubelet versions used by a given cluster's Machines and MachineDeployments.
func getKubeletVersions(ctx context.Context, client ctrlruntimeclient.Client) ([]string, error) {
	machineList := &clusterv1alpha1.MachineList{}
	if err := client.List(ctx, machineList); err != nil {
		return nil, fmt.Errorf("failed to load machines from cluster: %w", err)
	}

	machineDeployments := &clusterv1alpha1.MachineDeploymentList{}
	if err := client.List(ctx, machineDeployments); err != nil {
		return nil, KubernetesErrorToHTTPError(err)
	}

	kubeletVersionsSet := map[string]bool{}

	// first let's go through the legacy non-MD nodes
	for _, m := range machineList.Items {
		// Only list Machines that are not controlled, i.e. by Machine Set.
		if len(m.OwnerReferences) == 0 {
			ver := strings.TrimSpace(m.Spec.Versions.Kubelet)
			kubeletVersionsSet[ver] = true
		}
	}

	// now the deployments
	for _, md := range machineDeployments.Items {
		ver := strings.TrimSpace(md.Spec.Template.Spec.Versions.Kubelet)
		kubeletVersionsSet[ver] = true
	}

	// deduplicated list
	kubeletVersionList := []string{}
	for ver := range kubeletVersionsSet {
		kubeletVersionList = append(kubeletVersionList, ver)
	}

	return kubeletVersionList, nil
}
