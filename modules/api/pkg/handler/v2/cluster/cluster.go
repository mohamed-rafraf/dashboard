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

package cluster

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-kit/kit/endpoint"
	"go.uber.org/zap"

	apiv1 "k8c.io/dashboard/v2/pkg/api/v1"
	apiv2 "k8c.io/dashboard/v2/pkg/api/v2"
	handlercommon "k8c.io/dashboard/v2/pkg/handler/common"
	"k8c.io/dashboard/v2/pkg/handler/middleware"
	"k8c.io/dashboard/v2/pkg/handler/v1/common"
	"k8c.io/dashboard/v2/pkg/provider"
	kubernetesprovider "k8c.io/dashboard/v2/pkg/provider/kubernetes"
	kubermaticv1 "k8c.io/kubermatic/sdk/v2/apis/kubermatic/v1"
	"k8c.io/kubermatic/v2/pkg/features"
	kubermaticlog "k8c.io/kubermatic/v2/pkg/log"
	utilerrors "k8c.io/kubermatic/v2/pkg/util/errors"
	"k8c.io/kubermatic/v2/pkg/version"
)

func CreateEndpoint(
	projectProvider provider.ProjectProvider,
	privilegedProjectProvider provider.PrivilegedProjectProvider,
	seedsGetter provider.SeedsGetter,
	credentialManager provider.PresetProvider,
	exposeStrategy kubermaticv1.ExposeStrategy,
	userInfoGetter provider.UserInfoGetter,
	settingsProvider provider.SettingsProvider,
	caBundle *x509.CertPool,
	configGetter provider.KubermaticConfigurationGetter,
	features features.FeatureGate,
) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(CreateClusterReq)

		config, err := configGetter(ctx)
		if err != nil {
			return nil, err
		}

		err = req.Validate(version.NewFromConfiguration(config))
		if err != nil {
			return nil, utilerrors.NewBadRequest("%v", err)
		}

		return handlercommon.CreateEndpoint(ctx, req.ProjectID, req.Body, projectProvider, privilegedProjectProvider,
			seedsGetter, credentialManager, exposeStrategy, userInfoGetter, caBundle, configGetter, features, settingsProvider)
	}
}

// ListEndpoint list clusters for the given project.
func ListEndpoint(
	projectProvider provider.ProjectProvider,
	privilegedProjectProvider provider.PrivilegedProjectProvider,
	seedsGetter provider.SeedsGetter,
	clusterProviderGetter provider.ClusterProviderGetter,
	userInfoGetter provider.UserInfoGetter,
	configGetter provider.KubermaticConfigurationGetter,
) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(ListClustersReq)
		allClusters := make([]*apiv1.Cluster, 0)

		seeds, err := seedsGetter()
		if err != nil {
			return nil, common.KubernetesErrorToHTTPError(err)
		}

		brokenSeeds := []string{}
		for _, seed := range seeds {
			if seed.Status.Phase == kubermaticv1.SeedInvalidPhase {
				kubermaticlog.Logger.Warnf("skipping seed %s as it is in an invalid phase", seed.Name)
				brokenSeeds = append(brokenSeeds, seed.Name)
				continue
			}

			// if a Seed is bad, log error and put seed's name on the list of broken seeds.
			seedClusterProvider, err := clusterProviderGetter(seed)
			if err != nil {
				kubermaticlog.Logger.Errorw("failed to create cluster provider", "seed", seed.Name, zap.Error(err))
				continue
			}
			seedClusters, err := handlercommon.GetClusters(
				ctx,
				userInfoGetter,
				seedClusterProvider,
				projectProvider,
				privilegedProjectProvider,
				seedsGetter,
				req.ProjectID,
				configGetter,
				req.ShowDeploymentMachineCount,
			)
			if err != nil {
				kubermaticlog.Logger.Errorw("failed to get clusters from seed ", "seed", seed.Name, zap.Error(err))
				brokenSeeds = append(brokenSeeds, seed.Name)
			} else {
				allClusters = append(allClusters, seedClusters...)
			}
		}

		clusterList := make(apiv1.ClusterList, len(allClusters))
		for idx, cluster := range allClusters {
			clusterList[idx] = *cluster
		}

		if len(brokenSeeds) > 0 {
			errMsg := "Failed to fetch data for one or more seeds. Please contact an administrator."

			user, err := userInfoGetter(ctx, "")
			if err != nil {
				return nil, err
			}
			if user.IsAdmin {
				brokenSeedsAsStr := strings.Join(brokenSeeds, `, `)
				errMsg = fmt.Sprintf("Failed to fetch data for following seeds: %s.", brokenSeedsAsStr)
			}

			return apiv2.ProjectClusterList{
				Clusters:     clusterList,
				ErrorMessage: &errMsg,
			}, nil
		}

		return apiv2.ProjectClusterList{
			Clusters: clusterList,
		}, nil
	}
}

func GetEndpoint(projectProvider provider.ProjectProvider, privilegedProjectProvider provider.PrivilegedProjectProvider, seedsGetter provider.SeedsGetter, userInfoGetter provider.UserInfoGetter, configGetter provider.KubermaticConfigurationGetter) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(GetClusterReq)
		return handlercommon.GetEndpoint(ctx, projectProvider, privilegedProjectProvider, seedsGetter, userInfoGetter, req.ProjectID, req.ClusterID, configGetter)
	}
}

func DeleteEndpoint(sshKeyProvider provider.SSHKeyProvider, privilegedSSHKeyProvider provider.PrivilegedSSHKeyProvider, projectProvider provider.ProjectProvider, privilegedProjectProvider provider.PrivilegedProjectProvider, userInfoGetter provider.UserInfoGetter) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(DeleteReq)
		return handlercommon.DeleteEndpoint(ctx, userInfoGetter, req.ProjectID, req.ClusterID, req.DeleteVolumes, req.DeleteLoadBalancers, sshKeyProvider, privilegedSSHKeyProvider, projectProvider, privilegedProjectProvider)
	}
}

func PatchEndpoint(projectProvider provider.ProjectProvider, privilegedProjectProvider provider.PrivilegedProjectProvider,
	seedsGetter provider.SeedsGetter, userInfoGetter provider.UserInfoGetter, caBundle *x509.CertPool, configGetter provider.KubermaticConfigurationGetter, features features.FeatureGate) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(PatchReq)
		return handlercommon.PatchEndpoint(ctx, userInfoGetter, req.ProjectID, req.ClusterID, req.Patch, seedsGetter,
			projectProvider, privilegedProjectProvider, caBundle, configGetter, features, req.SkipKubeletVersionValidation)
	}
}

func GetClusterEventsEndpoint(projectProvider provider.ProjectProvider, privilegedProjectProvider provider.PrivilegedProjectProvider, userInfoGetter provider.UserInfoGetter) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(EventsReq)
		return handlercommon.GetClusterEventsEndpoint(ctx, userInfoGetter, req.ProjectID, req.ClusterID, req.Type, projectProvider, privilegedProjectProvider)
	}
}

func HealthEndpoint(projectProvider provider.ProjectProvider, privilegedProjectProvider provider.PrivilegedProjectProvider, userInfoGetter provider.UserInfoGetter) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(GetClusterReq)
		return handlercommon.HealthEndpoint(ctx, userInfoGetter, req.ProjectID, req.ClusterID, projectProvider, privilegedProjectProvider)
	}
}

func MigrateEndpointToExternalCCM(projectProvider provider.ProjectProvider, privilegedProjectProvider provider.PrivilegedProjectProvider, seedsGetter provider.SeedsGetter, userInfoGetter provider.UserInfoGetter, configGetter provider.KubermaticConfigurationGetter) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(GetClusterReq)
		return handlercommon.MigrateEndpointToExternalCCM(ctx, userInfoGetter, req.ProjectID, req.ClusterID, projectProvider, seedsGetter, privilegedProjectProvider, configGetter)
	}
}

func GetMetricsEndpoint(projectProvider provider.ProjectProvider, privilegedProjectProvider provider.PrivilegedProjectProvider, userInfoGetter provider.UserInfoGetter) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(GetClusterReq)
		return handlercommon.GetMetricsEndpoint(ctx, userInfoGetter, req.ProjectID, req.ClusterID, projectProvider, privilegedProjectProvider)
	}
}

func ListNamespaceEndpoint(projectProvider provider.ProjectProvider, privilegedProjectProvider provider.PrivilegedProjectProvider, userInfoGetter provider.UserInfoGetter) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(GetClusterReq)
		return handlercommon.ListNamespaceEndpoint(ctx, userInfoGetter, req.ProjectID, req.ClusterID, projectProvider, privilegedProjectProvider)
	}
}

func AssignSSHKeyEndpoint(sshKeyProvider provider.SSHKeyProvider, privilegedSSHKeyProvider provider.PrivilegedSSHKeyProvider, projectProvider provider.ProjectProvider, privilegedProjectProvider provider.PrivilegedProjectProvider, userInfoGetter provider.UserInfoGetter) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(AssignSSHKeysReq)
		return handlercommon.AssignSSHKeyEndpoint(ctx, userInfoGetter, req.ProjectID, req.ClusterID, req.KeyID, projectProvider, privilegedProjectProvider, sshKeyProvider, privilegedSSHKeyProvider)
	}
}

func DetachSSHKeyEndpoint(sshKeyProvider provider.SSHKeyProvider, privilegedSSHKeyProvider provider.PrivilegedSSHKeyProvider, projectProvider provider.ProjectProvider, privilegedProjectProvider provider.PrivilegedProjectProvider, userInfoGetter provider.UserInfoGetter) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(AssignSSHKeysReq)
		return handlercommon.DetachSSHKeyEndpoint(ctx, userInfoGetter, req.ProjectID, req.ClusterID, req.KeyID, projectProvider, privilegedProjectProvider, sshKeyProvider, privilegedSSHKeyProvider)
	}
}

func ListSSHKeysEndpoint(sshKeyProvider provider.SSHKeyProvider, projectProvider provider.ProjectProvider, privilegedProjectProvider provider.PrivilegedProjectProvider, userInfoGetter provider.UserInfoGetter) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(ListSSHKeysReq)
		return handlercommon.ListSSHKeysEndpoint(ctx, userInfoGetter, req.ProjectID, req.ClusterID, projectProvider, privilegedProjectProvider, sshKeyProvider)
	}
}

func RevokeAdminTokenEndpoint(projectProvider provider.ProjectProvider, privilegedProjectProvider provider.PrivilegedProjectProvider, userInfoGetter provider.UserInfoGetter) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(adminTokenReq)
		clusterProvider := ctx.Value(middleware.ClusterProviderContextKey).(provider.ClusterProvider)

		cluster, err := handlercommon.GetCluster(ctx, projectProvider, privilegedProjectProvider, userInfoGetter, req.ProjectID, req.ClusterID, nil)
		if err != nil {
			return nil, err
		}

		return nil, common.KubernetesErrorToHTTPError(clusterProvider.RevokeAdminKubeconfig(ctx, cluster))
	}
}

func RevokeViewerTokenEndpoint(projectProvider provider.ProjectProvider, privilegedProjectProvider provider.PrivilegedProjectProvider, userInfoGetter provider.UserInfoGetter) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(adminTokenReq)
		clusterProvider := ctx.Value(middleware.ClusterProviderContextKey).(provider.ClusterProvider)

		cluster, err := handlercommon.GetCluster(ctx, projectProvider, privilegedProjectProvider, userInfoGetter, req.ProjectID, req.ClusterID, nil)
		if err != nil {
			return nil, err
		}

		return nil, common.KubernetesErrorToHTTPError(clusterProvider.RevokeViewerKubeconfig(ctx, cluster))
	}
}

// AdminTokenReq defines HTTP request data for revokeClusterAdminTokenV2 and revokeClusterViewerTokenV2 endpoints.
// swagger:parameters revokeClusterAdminTokenV2 revokeClusterViewerTokenV2
type adminTokenReq struct {
	common.ProjectReq
	// in: path
	ClusterID string `json:"cluster_id"`
}

// GetSeedCluster returns the AssignSSHKeysReq object.
func (req adminTokenReq) GetSeedCluster() apiv1.SeedCluster {
	return apiv1.SeedCluster{
		ClusterID: req.ClusterID,
	}
}

func DecodeAdminTokenReq(c context.Context, r *http.Request) (interface{}, error) {
	var req adminTokenReq
	clusterID, err := common.DecodeClusterID(c, r)
	if err != nil {
		return nil, err
	}
	req.ClusterID = clusterID

	projectReq, err := common.DecodeProjectRequest(c, r)
	if err != nil {
		return nil, err
	}
	req.ProjectReq = projectReq.(common.ProjectReq)
	return req, nil
}

// ListSSHKeysReq defines HTTP request data for listSSHKeysAssignedToClusterV2 endpoint
// swagger:parameters listSSHKeysAssignedToClusterV2
type ListSSHKeysReq struct {
	common.ProjectReq
	// in: path
	ClusterID string `json:"cluster_id"`
}

func DecodeListSSHKeysReq(c context.Context, r *http.Request) (interface{}, error) {
	var req ListSSHKeysReq
	clusterID, err := common.DecodeClusterID(c, r)
	if err != nil {
		return nil, err
	}
	req.ClusterID = clusterID

	projectReq, err := common.DecodeProjectRequest(c, r)
	if err != nil {
		return nil, err
	}
	req.ProjectReq = projectReq.(common.ProjectReq)
	return req, nil
}

// GetSeedCluster returns the AssignSSHKeysReq object.
func (req ListSSHKeysReq) GetSeedCluster() apiv1.SeedCluster {
	return apiv1.SeedCluster{
		ClusterID: req.ClusterID,
	}
}

// AssignSSHKeysReq defines HTTP request data for assignSSHKeyToClusterV2  endpoint
// swagger:parameters assignSSHKeyToClusterV2 detachSSHKeyFromClusterV2
type AssignSSHKeysReq struct {
	common.ProjectReq
	// in: path
	ClusterID string `json:"cluster_id"`
	// in: path
	KeyID string `json:"key_id"`
}

// GetSeedCluster returns the AssignSSHKeysReq object.
func (req AssignSSHKeysReq) GetSeedCluster() apiv1.SeedCluster {
	return apiv1.SeedCluster{
		ClusterID: req.ClusterID,
	}
}

func DecodeAssignSSHKeyReq(c context.Context, r *http.Request) (interface{}, error) {
	var req AssignSSHKeysReq
	clusterID, err := common.DecodeClusterID(c, r)
	if err != nil {
		return nil, err
	}
	req.ClusterID = clusterID

	projectReq, err := common.DecodeProjectRequest(c, r)
	if err != nil {
		return nil, err
	}
	req.ProjectReq = projectReq.(common.ProjectReq)

	keyID, err := common.DecodeSSHKeyID(c, r)
	if err != nil {
		return nil, err
	}
	req.KeyID = keyID

	return req, nil
}

// EventsReq defines HTTP request for getClusterEventsV2 endpoint
// swagger:parameters getClusterEventsV2
type EventsReq struct {
	common.ProjectReq
	// in: path
	// required: true
	ClusterID string `json:"cluster_id"`

	// in: query
	Type string `json:"type,omitempty"`
}

// GetSeedCluster returns the SeedCluster object.
func (req EventsReq) GetSeedCluster() apiv1.SeedCluster {
	return apiv1.SeedCluster{
		ClusterID: req.ClusterID,
	}
}

func DecodeGetClusterEvents(c context.Context, r *http.Request) (interface{}, error) {
	var req EventsReq

	projectReq, err := common.DecodeProjectRequest(c, r)
	if err != nil {
		return nil, err
	}
	req.ProjectReq = projectReq.(common.ProjectReq)
	clusterID, err := common.DecodeClusterID(c, r)
	if err != nil {
		return nil, err
	}
	req.ClusterID = clusterID

	req.Type = r.URL.Query().Get("type")
	if len(req.Type) > 0 {
		if req.Type == "warning" || req.Type == "normal" {
			return req, nil
		}
		return nil, fmt.Errorf("wrong query parameter, unsupported type: %s", req.Type)
	}

	return req, nil
}

// PatchReq defines HTTP request for patchCluster endpoint
// swagger:parameters patchClusterV2
type PatchReq struct {
	common.ProjectReq
	// in: path
	// required: true
	ClusterID string `json:"cluster_id"`

	// in: body
	Patch json.RawMessage

	// in: query
	// required: false
	SkipKubeletVersionValidation bool `json:"skip_kubelet_version_validation,omitempty"`
}

func DecodePatchReq(c context.Context, r *http.Request) (interface{}, error) {
	var req PatchReq
	var skipKubeletVersionValidation bool

	projectReq, err := common.DecodeProjectRequest(c, r)
	if err != nil {
		return nil, err
	}
	req.ProjectReq = projectReq.(common.ProjectReq)
	clusterID, err := common.DecodeClusterID(c, r)
	if err != nil {
		return nil, err
	}
	req.ClusterID = clusterID

	if req.Patch, err = io.ReadAll(r.Body); err != nil {
		return nil, err
	}

	queryParam := r.URL.Query().Get("skip_kubelet_version_validation")
	if queryParam != "" {
		skipKubeletVersionValidation, err = strconv.ParseBool(queryParam)
		if err != nil {
			return nil, fmt.Errorf("wrong query parameter `skip_kubelet_version_validation`: %w", err)
		}
	}
	req.SkipKubeletVersionValidation = skipKubeletVersionValidation

	return req, nil
}

// GetSeedCluster returns the SeedCluster object.
func (req PatchReq) GetSeedCluster() apiv1.SeedCluster {
	return apiv1.SeedCluster{
		ClusterID: req.ClusterID,
	}
}

// DeleteReq defines HTTP request for deleteCluster endpoint
// swagger:parameters deleteClusterV2
type DeleteReq struct {
	common.ProjectReq
	// in: path
	// required: true
	ClusterID string `json:"cluster_id"`
	// in: header
	// DeleteVolumes if true all cluster PV's and PVC's will be deleted from cluster
	DeleteVolumes bool
	// in: header
	// DeleteLoadBalancers if true all load balancers will be deleted from cluster
	DeleteLoadBalancers bool
}

// GetSeedCluster returns the SeedCluster object.
func (req DeleteReq) GetSeedCluster() apiv1.SeedCluster {
	return apiv1.SeedCluster{
		ClusterID: req.ClusterID,
	}
}

func DecodeDeleteReq(c context.Context, r *http.Request) (interface{}, error) {
	var req DeleteReq

	clusterID, err := common.DecodeClusterID(c, r)
	if err != nil {
		return nil, err
	}
	req.ClusterID = clusterID

	projectReq, err := common.DecodeProjectRequest(c, r)
	if err != nil {
		return nil, err
	}
	req.ProjectReq = projectReq.(common.ProjectReq)

	headerValue := r.Header.Get("DeleteVolumes")
	if len(headerValue) > 0 {
		deleteVolumes, err := strconv.ParseBool(headerValue)
		if err != nil {
			return nil, err
		}
		req.DeleteVolumes = deleteVolumes
	}

	headerValue = r.Header.Get("DeleteLoadBalancers")
	if len(headerValue) > 0 {
		deleteLB, err := strconv.ParseBool(headerValue)
		if err != nil {
			return nil, err
		}
		req.DeleteLoadBalancers = deleteLB
	}

	return req, nil
}

// ListClustersReq defines HTTP request for listClusters endpoint.
// swagger:parameters listClustersV2
type ListClustersReq struct {
	common.ProjectReq

	// in: query
	ShowDeploymentMachineCount bool `json:"show_dm_count"`
}

func DecodeListClustersReq(c context.Context, r *http.Request) (interface{}, error) {
	var req ListClustersReq

	pr, err := common.DecodeProjectRequest(c, r)
	if err != nil {
		return nil, err
	}
	req.ProjectReq = pr.(common.ProjectReq)

	showDeploymentMachineCount := r.URL.Query().Get("show_dm_count")
	if strings.EqualFold(showDeploymentMachineCount, "true") {
		req.ShowDeploymentMachineCount = true
	}

	return req, nil
}

// GetClusterReq defines HTTP request for getCluster endpoint.
// swagger:parameters getClusterV2 getClusterHealthV2 getOidcClusterKubeconfigV2 getClusterKubeconfigV2 getClusterMetricsV2 listNamespaceV2 getClusterUpgradesV2 listAWSSizesNoCredentialsV2 listAWSSubnetsNoCredentialsV2 listGCPNetworksNoCredentialsV2 listGCPZonesNoCredentialsV2 listHetznerSizesNoCredentialsV2 listDigitaloceanSizesNoCredentialsV2 migrateClusterToExternalCCM getClusterOidc listKubeVirtInstancetypesNoCredentials listKubevirtStorageClassesNoCredentials getKubevirtStorageClassesNoCredentials listKubeVirtVPCsNoCredentials listKubeVirtSubnetsNoCredentials
type GetClusterReq struct {
	common.ProjectReq
	// in: path
	// required: true
	ClusterID string `json:"cluster_id"`
}

func DecodeGetClusterReq(c context.Context, r *http.Request) (interface{}, error) {
	var req GetClusterReq
	clusterID, err := common.DecodeClusterID(c, r)
	if err != nil {
		return nil, err
	}

	req.ClusterID = clusterID

	pr, err := common.DecodeProjectRequest(c, r)
	if err != nil {
		return nil, err
	}
	req.ProjectReq = pr.(common.ProjectReq)

	return req, nil
}

// GetSeedCluster returns the SeedCluster object.
func (req GetClusterReq) GetSeedCluster() apiv1.SeedCluster {
	return apiv1.SeedCluster{
		ClusterID: req.ClusterID,
	}
}

// CreateClusterReq defines HTTP request for createCluster
// swagger:parameters createClusterV2
type CreateClusterReq struct {
	common.ProjectReq
	// in: body
	Body apiv1.CreateClusterSpec

	// private field for the seed name. Needed for the cluster provider.
	seedName string
}

// GetSeedCluster returns the SeedCluster object.
func (req CreateClusterReq) GetSeedCluster() apiv1.SeedCluster {
	return apiv1.SeedCluster{
		SeedName: req.seedName,
	}
}

func DecodeCreateReq(c context.Context, r *http.Request) (interface{}, error) {
	var req CreateClusterReq

	pr, err := common.DecodeProjectRequest(c, r)
	if err != nil {
		return nil, err
	}
	req.ProjectReq = pr.(common.ProjectReq)

	if err := json.NewDecoder(r.Body).Decode(&req.Body); err != nil {
		return nil, err
	}

	if len(req.Body.Cluster.Type) == 0 {
		req.Body.Cluster.Type = apiv1.KubernetesClusterType
	}

	seedName, err := FindSeedNameForDatacenter(c, req.Body.Cluster.Spec.Cloud.DatacenterName)
	if err != nil {
		return nil, err
	}
	req.seedName = seedName
	return req, nil
}

// Validate validates CreateEndpoint request.
func (req CreateClusterReq) Validate(updateManager common.UpdateManager) error {
	if len(req.ProjectID) == 0 {
		return fmt.Errorf("the project ID cannot be empty")
	}
	return handlercommon.ValidateClusterSpec(updateManager, req.Body)
}

func FindSeedNameForDatacenter(ctx context.Context, datacenter string) (string, error) {
	seedsGetter, ok := ctx.Value(middleware.SeedsGetterContextKey).(provider.SeedsGetter)
	if !ok {
		return "", fmt.Errorf("seeds getter is not set")
	}
	seeds, err := seedsGetter()
	if err != nil {
		return "", fmt.Errorf("failed to list seeds: %w", err)
	}
	for name, seed := range seeds {
		if _, ok := seed.Spec.Datacenters[datacenter]; ok {
			return name, nil
		}
	}
	return "", fmt.Errorf("can not find seed for datacenter %s", datacenter)
}

// GetClusterProviderFromRequest returns cluster and cluster provider based on the provided request.
func GetClusterProviderFromRequest(
	ctx context.Context,
	request interface{},
	projectProvider provider.ProjectProvider,
	privilegedProjectProvider provider.PrivilegedProjectProvider,
	userInfoGetter provider.UserInfoGetter,
) (*kubermaticv1.Cluster, *kubernetesprovider.ClusterProvider, error) {
	req, ok := request.(GetClusterReq)
	if !ok {
		return nil, nil, utilerrors.New(http.StatusBadRequest, "invalid request")
	}

	cluster, err := handlercommon.GetCluster(ctx, projectProvider, privilegedProjectProvider, userInfoGetter, req.ProjectID, req.ClusterID, nil)
	if err != nil {
		return nil, nil, utilerrors.New(http.StatusInternalServerError, err.Error())
	}

	rawClusterProvider, ok := ctx.Value(middleware.PrivilegedClusterProviderContextKey).(provider.PrivilegedClusterProvider)
	if !ok {
		return nil, nil, utilerrors.New(http.StatusInternalServerError, "no clusterProvider in request")
	}
	clusterProvider, ok := rawClusterProvider.(*kubernetesprovider.ClusterProvider)
	if !ok {
		return nil, nil, utilerrors.New(http.StatusInternalServerError, "failed to assert clusterProvider")
	}
	return cluster, clusterProvider, nil
}
