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

package serviceaccount

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"unicode/utf8"

	"github.com/go-kit/kit/endpoint"
	"github.com/gorilla/mux"

	apiv1 "k8c.io/dashboard/v2/pkg/api/v1"
	"k8c.io/dashboard/v2/pkg/handler/v1/common"
	"k8c.io/dashboard/v2/pkg/provider"
	"k8c.io/dashboard/v2/pkg/serviceaccount"
	kubermaticv1 "k8c.io/kubermatic/sdk/v2/apis/kubermatic/v1"
	utilerrors "k8c.io/kubermatic/v2/pkg/util/errors"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/rand"
)

// CreateTokenEndpoint creates a token for the given service account.
func CreateTokenEndpoint(projectProvider provider.ProjectProvider, privilegedProjectProvider provider.PrivilegedProjectProvider, serviceAccountProvider provider.ServiceAccountProvider, privilegedServiceAccount provider.PrivilegedServiceAccountProvider, serviceAccountTokenProvider provider.ServiceAccountTokenProvider, privilegedServiceAccountTokenProvider provider.PrivilegedServiceAccountTokenProvider, tokenAuthenticator serviceaccount.TokenAuthenticator, tokenGenerator serviceaccount.TokenGenerator, userInfoGetter provider.UserInfoGetter) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(addTokenReq)
		err := req.Validate()
		if err != nil {
			return nil, utilerrors.NewBadRequest("%v", err)
		}
		project, err := common.GetProject(ctx, userInfoGetter, projectProvider, privilegedProjectProvider, req.ProjectID, nil)
		if err != nil {
			return nil, common.KubernetesErrorToHTTPError(err)
		}

		sa, err := getSA(ctx, serviceAccountProvider, privilegedServiceAccount, userInfoGetter, project, req.ServiceAccountID, &provider.ServiceAccountGetOptions{RemovePrefix: false})
		if err != nil {
			return nil, common.KubernetesErrorToHTTPError(err)
		}

		// check if token name is already reserved for service account
		existingTokenList, err := listSAToken(ctx, userInfoGetter, serviceAccountTokenProvider, privilegedServiceAccountTokenProvider, project, sa, req.Body.Name)
		if err != nil {
			return nil, common.KubernetesErrorToHTTPError(err)
		}
		if len(existingTokenList) > 0 {
			return nil, utilerrors.NewAlreadyExists("token", req.Body.Name)
		}

		tokenID := rand.String(10)

		token, err := tokenGenerator.Generate(serviceaccount.Claims(sa.Spec.Email, project.Name, tokenID))
		if err != nil {
			return nil, utilerrors.New(http.StatusInternalServerError, "can not generate token data")
		}

		secret, err := createSAToken(ctx, userInfoGetter, serviceAccountTokenProvider, privilegedServiceAccountTokenProvider, sa, project.Name, req.Body.Name, tokenID, token)
		if err != nil {
			return nil, common.KubernetesErrorToHTTPError(err)
		}

		externalToken, err := convertInternalTokenToPrivateExternal(secret, tokenAuthenticator)
		if err != nil {
			return nil, utilerrors.New(http.StatusInternalServerError, err.Error())
		}

		return externalToken, nil
	}
}

func listSAToken(ctx context.Context, userInfoGetter provider.UserInfoGetter, serviceAccountTokenProvider provider.ServiceAccountTokenProvider, privilegedServiceAccountTokenProvider provider.PrivilegedServiceAccountTokenProvider, project *kubermaticv1.Project, sa *kubermaticv1.User, tokenID string) ([]*corev1.Secret, error) {
	options := &provider.ServiceAccountTokenListOptions{}
	if tokenID != "" {
		options.TokenID = tokenID
	}

	adminUserInfo, err := userInfoGetter(ctx, "")
	if err != nil {
		return nil, common.KubernetesErrorToHTTPError(err)
	}
	if adminUserInfo.IsAdmin {
		labelSelector, err := labels.Parse(fmt.Sprintf("%s=%s", kubermaticv1.ProjectIDLabelKey, project.Name))
		if err != nil {
			return nil, err
		}
		options.LabelSelector = labelSelector
		options.ServiceAccountID = sa.Name
		tokens, err := privilegedServiceAccountTokenProvider.ListUnsecured(ctx, options)
		if apierrors.IsNotFound(err) {
			return make([]*corev1.Secret, 0), nil
		}
		return tokens, err
	}

	userInfo, err := userInfoGetter(ctx, project.Name)
	if err != nil {
		return nil, err
	}
	return serviceAccountTokenProvider.List(ctx, userInfo, project, sa, options)
}

func createSAToken(ctx context.Context, userInfoGetter provider.UserInfoGetter, serviceAccountTokenProvider provider.ServiceAccountTokenProvider, privilegedServiceAccountTokenProvider provider.PrivilegedServiceAccountTokenProvider, sa *kubermaticv1.User, projectID, tokenName, tokenID, tokenData string) (*corev1.Secret, error) {
	adminUserInfo, err := userInfoGetter(ctx, "")
	if err != nil {
		return nil, err
	}
	if adminUserInfo.IsAdmin {
		return privilegedServiceAccountTokenProvider.CreateUnsecured(ctx, sa, projectID, tokenName, tokenID, tokenData)
	}

	userInfo, err := userInfoGetter(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return serviceAccountTokenProvider.Create(ctx, userInfo, sa, projectID, tokenName, tokenID, tokenData)
}

// ListTokenEndpoint gets token for the service account.
func ListTokenEndpoint(projectProvider provider.ProjectProvider, privilegedProjectProvider provider.PrivilegedProjectProvider, serviceAccountProvider provider.ServiceAccountProvider, privilegedServiceAccount provider.PrivilegedServiceAccountProvider, serviceAccountTokenProvider provider.ServiceAccountTokenProvider, privilegedServiceAccountTokenProvider provider.PrivilegedServiceAccountTokenProvider, tokenAuthenticator serviceaccount.TokenAuthenticator, userInfoGetter provider.UserInfoGetter) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		resultList := make([]*apiv1.PublicServiceAccountToken, 0)
		req := request.(commonTokenReq)
		err := req.Validate()
		if err != nil {
			return nil, utilerrors.NewBadRequest("%v", err)
		}

		project, err := common.GetProject(ctx, userInfoGetter, projectProvider, privilegedProjectProvider, req.ProjectID, nil)
		if err != nil {
			return nil, common.KubernetesErrorToHTTPError(err)
		}

		sa, err := getSA(ctx, serviceAccountProvider, privilegedServiceAccount, userInfoGetter, project, req.ServiceAccountID, &provider.ServiceAccountGetOptions{RemovePrefix: false})
		if err != nil {
			return nil, common.KubernetesErrorToHTTPError(err)
		}

		existingSecretList, err := listSAToken(ctx, userInfoGetter, serviceAccountTokenProvider, privilegedServiceAccountTokenProvider, project, sa, "")
		if err != nil {
			return nil, common.KubernetesErrorToHTTPError(err)
		}

		var errorList []string
		for _, secret := range existingSecretList {
			externalToken, err := convertInternalTokenToPublicExternal(secret, tokenAuthenticator)
			if err != nil {
				errorList = append(errorList, err.Error())
				continue
			}
			resultList = append(resultList, externalToken)
		}

		if len(errorList) > 0 {
			return nil, utilerrors.NewWithDetails(http.StatusInternalServerError, "failed to get some service account tokens, please examine details field for more info", errorList)
		}

		return resultList, nil
	}
}

// UpdateTokenEndpoint updates and regenerates the token for the given service account.
func UpdateTokenEndpoint(projectProvider provider.ProjectProvider, privilegedProjectProvider provider.PrivilegedProjectProvider, serviceAccountProvider provider.ServiceAccountProvider, privilegedServiceAccount provider.PrivilegedServiceAccountProvider, serviceAccountTokenProvider provider.ServiceAccountTokenProvider, privilegedServiceAccountTokenProvider provider.PrivilegedServiceAccountTokenProvider, tokenAuthenticator serviceaccount.TokenAuthenticator, tokenGenerator serviceaccount.TokenGenerator, userInfoGetter provider.UserInfoGetter) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(updateTokenReq)
		err := req.Validate()
		if err != nil {
			return nil, utilerrors.NewBadRequest("%v", err)
		}

		secret, err := updateEndpoint(ctx, projectProvider, privilegedProjectProvider, serviceAccountProvider, privilegedServiceAccount, serviceAccountTokenProvider, privilegedServiceAccountTokenProvider, userInfoGetter, tokenGenerator, req.ProjectID, req.ServiceAccountID, req.TokenID, req.Body.Name, true)
		if err != nil {
			return nil, common.KubernetesErrorToHTTPError(err)
		}

		externalToken, err := convertInternalTokenToPrivateExternal(secret, tokenAuthenticator)
		if err != nil {
			return nil, utilerrors.New(http.StatusInternalServerError, err.Error())
		}

		return externalToken, nil
	}
}

// PatchTokenEndpoint patches the token name.
func PatchTokenEndpoint(projectProvider provider.ProjectProvider, privilegedProjectProvider provider.PrivilegedProjectProvider, serviceAccountProvider provider.ServiceAccountProvider, privilegedServiceAccount provider.PrivilegedServiceAccountProvider, serviceAccountTokenProvider provider.ServiceAccountTokenProvider, privilegedServiceAccountTokenProvider provider.PrivilegedServiceAccountTokenProvider, tokenAuthenticator serviceaccount.TokenAuthenticator, tokenGenerator serviceaccount.TokenGenerator, userInfoGetter provider.UserInfoGetter) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(patchTokenReq)
		err := req.Validate()
		if err != nil {
			return nil, utilerrors.NewBadRequest("%v", err)
		}

		tokenReq := &apiv1.PublicServiceAccountToken{}
		if err := json.Unmarshal(req.Body, tokenReq); err != nil {
			return nil, utilerrors.NewBadRequest("%v", err)
		}
		if len(tokenReq.Name) == 0 {
			return nil, utilerrors.NewBadRequest("new name can not be empty")
		}

		secret, err := updateEndpoint(ctx, projectProvider, privilegedProjectProvider, serviceAccountProvider, privilegedServiceAccount, serviceAccountTokenProvider, privilegedServiceAccountTokenProvider, userInfoGetter, tokenGenerator, req.ProjectID, req.ServiceAccountID, req.TokenID, tokenReq.Name, false)
		if err != nil {
			return nil, common.KubernetesErrorToHTTPError(err)
		}

		externalToken, err := convertInternalTokenToPublicExternal(secret, tokenAuthenticator)
		if err != nil {
			return nil, utilerrors.New(http.StatusInternalServerError, err.Error())
		}

		return externalToken, nil
	}
}

// DeleteTokenEndpoint deletes the token from service account.
func DeleteTokenEndpoint(projectProvider provider.ProjectProvider, privilegedProjectProvider provider.PrivilegedProjectProvider, serviceAccountProvider provider.ServiceAccountProvider, privilegedServiceAccount provider.PrivilegedServiceAccountProvider, serviceAccountTokenProvider provider.ServiceAccountTokenProvider, privilegedServiceAccountTokenProvider provider.PrivilegedServiceAccountTokenProvider, userInfoGetter provider.UserInfoGetter) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(deleteTokenReq)
		err := req.Validate()
		if err != nil {
			return nil, utilerrors.NewBadRequest("%v", err)
		}

		project, err := common.GetProject(ctx, userInfoGetter, projectProvider, privilegedProjectProvider, req.ProjectID, nil)
		if err != nil {
			return nil, common.KubernetesErrorToHTTPError(err)
		}

		_, err = getSA(ctx, serviceAccountProvider, privilegedServiceAccount, userInfoGetter, project, req.ServiceAccountID, &provider.ServiceAccountGetOptions{RemovePrefix: false})
		if err != nil {
			return nil, common.KubernetesErrorToHTTPError(err)
		}

		if err := deleteSAToken(ctx, userInfoGetter, serviceAccountTokenProvider, privilegedServiceAccountTokenProvider, req.ProjectID, req.TokenID); err != nil {
			return nil, common.KubernetesErrorToHTTPError(err)
		}
		return nil, nil
	}
}

func deleteSAToken(ctx context.Context, userInfoGetter provider.UserInfoGetter, serviceAccountTokenProvider provider.ServiceAccountTokenProvider, privilegedServiceAccountTokenProvider provider.PrivilegedServiceAccountTokenProvider, projectID, tokenID string) error {
	adminUserInfo, err := userInfoGetter(ctx, "")
	if err != nil {
		return err
	}
	if adminUserInfo.IsAdmin {
		return privilegedServiceAccountTokenProvider.DeleteUnsecured(ctx, tokenID)
	}

	userInfo, err := userInfoGetter(ctx, projectID)
	if err != nil {
		return err
	}
	return serviceAccountTokenProvider.Delete(ctx, userInfo, tokenID)
}

func getSAToken(ctx context.Context, userInfoGetter provider.UserInfoGetter, serviceAccountTokenProvider provider.ServiceAccountTokenProvider, privilegedServiceAccountTokenProvider provider.PrivilegedServiceAccountTokenProvider, projectID, tokenID string) (*corev1.Secret, error) {
	adminUserInfo, err := userInfoGetter(ctx, "")
	if err != nil {
		return nil, err
	}
	if adminUserInfo.IsAdmin {
		return privilegedServiceAccountTokenProvider.GetUnsecured(ctx, tokenID)
	}

	userInfo, err := userInfoGetter(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return serviceAccountTokenProvider.Get(ctx, userInfo, tokenID)
}

func updateEndpoint(ctx context.Context, projectProvider provider.ProjectProvider, privilegedProjectProvider provider.PrivilegedProjectProvider, serviceAccountProvider provider.ServiceAccountProvider,
	privilegedServiceAccount provider.PrivilegedServiceAccountProvider, serviceAccountTokenProvider provider.ServiceAccountTokenProvider, privilegedServiceAccountTokenProvider provider.PrivilegedServiceAccountTokenProvider, userInfoGetter provider.UserInfoGetter, tokenGenerator serviceaccount.TokenGenerator,
	projectID, saID, tokenID, newName string, regenerateToken bool,
) (*corev1.Secret, error) {
	project, err := common.GetProject(ctx, userInfoGetter, projectProvider, privilegedProjectProvider, projectID, nil)
	if err != nil {
		return nil, err
	}

	sa, err := getSA(ctx, serviceAccountProvider, privilegedServiceAccount, userInfoGetter, project, saID, &provider.ServiceAccountGetOptions{RemovePrefix: false})
	if err != nil {
		return nil, err
	}

	existingSecret, err := getSAToken(ctx, userInfoGetter, serviceAccountTokenProvider, privilegedServiceAccountTokenProvider, projectID, tokenID)
	if err != nil {
		return nil, err
	}
	existingName, ok := existingSecret.Labels["name"]
	if !ok {
		return nil, fmt.Errorf("can not find token name in secret %s", existingSecret.Name)
	}

	if newName == existingName && !regenerateToken {
		return existingSecret, nil
	}

	if newName != existingName {
		// check if token name is already reserved for service account
		existingTokenList, err := listSAToken(ctx, userInfoGetter, serviceAccountTokenProvider, privilegedServiceAccountTokenProvider, project, sa, newName)
		if err != nil {
			return nil, err
		}
		if len(existingTokenList) > 0 {
			return nil, utilerrors.NewAlreadyExists("token", newName)
		}
		existingSecret.Labels["name"] = newName
	}

	if regenerateToken {
		token, err := tokenGenerator.Generate(serviceaccount.Claims(sa.Spec.Email, project.Name, existingSecret.Name))
		if err != nil {
			return nil, fmt.Errorf("can not generate token data")
		}

		existingSecret.Data["token"] = []byte(token)
	}

	secret, err := updateSAToken(ctx, userInfoGetter, serviceAccountTokenProvider, privilegedServiceAccountTokenProvider, existingSecret, projectID)
	if err != nil {
		return nil, err
	}

	return secret, nil
}

func updateSAToken(ctx context.Context, userInfoGetter provider.UserInfoGetter, serviceAccountTokenProvider provider.ServiceAccountTokenProvider, privilegedServiceAccountTokenProvider provider.PrivilegedServiceAccountTokenProvider, token *corev1.Secret, projectID string) (*corev1.Secret, error) {
	adminUserInfo, err := userInfoGetter(ctx, "")
	if err != nil {
		return nil, err
	}
	if adminUserInfo.IsAdmin {
		return privilegedServiceAccountTokenProvider.UpdateUnsecured(ctx, token)
	}

	userInfo, err := userInfoGetter(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return serviceAccountTokenProvider.Update(ctx, userInfo, token)
}

// addTokenReq defines HTTP request for addTokenToServiceAccount
// swagger:parameters addTokenToServiceAccount
type addTokenReq struct {
	commonTokenReq
	// in: body
	Body apiv1.ServiceAccountToken
}

// commonTokenReq defines HTTP request for listServiceAccountTokens
// swagger:parameters listServiceAccountTokens
type commonTokenReq struct {
	common.ProjectReq
	serviceAccountIDReq
}

// tokenIDReq represents a request that contains the token ID in the path.
type tokenIDReq struct {
	// in: path
	TokenID string `json:"token_id"`
}

// updateTokenReq defines HTTP request for updateServiceAccountToken
// swagger:parameters updateServiceAccountToken
type updateTokenReq struct {
	commonTokenReq
	tokenIDReq
	// in: body
	Body apiv1.PublicServiceAccountToken
}

// patchTokenReq defines HTTP request for patchServiceAccountToken
// swagger:parameters patchServiceAccountToken
type patchTokenReq struct {
	commonTokenReq
	tokenIDReq
	// in: body
	Body []byte
}

// deleteTokenReq defines HTTP request for deleteServiceAccountToken
// swagger:parameters deleteServiceAccountToken
type deleteTokenReq struct {
	commonTokenReq
	tokenIDReq
}

// Validate validates addTokenReq request.
func (r addTokenReq) Validate() error {
	if len(r.Body.Name) == 0 || len(r.ProjectID) == 0 || len(r.ServiceAccountID) == 0 {
		return fmt.Errorf("the name, service account ID and project ID cannot be empty")
	}
	if utf8.RuneCountInString(r.Body.Name) > 50 {
		return fmt.Errorf("the name is too long, max 50 chars")
	}

	return nil
}

// Validate validates commonTokenReq request.
func (r commonTokenReq) Validate() error {
	if len(r.ProjectID) == 0 || len(r.ServiceAccountID) == 0 {
		return fmt.Errorf("service account ID and project ID cannot be empty")
	}

	return nil
}

// Validate validates updateTokenReq request.
func (r updateTokenReq) Validate() error {
	if err := r.commonTokenReq.Validate(); err != nil {
		return err
	}
	if len(r.TokenID) == 0 {
		return fmt.Errorf("token ID cannot be empty")
	}
	if len(r.Body.Name) == 0 {
		return fmt.Errorf("new name can not be empty")
	}
	if r.TokenID != r.Body.ID {
		return fmt.Errorf("token ID mismatch, you requested to update token %q but body contains token %q", r.TokenID, r.Body.ID)
	}

	return nil
}

// Validate validates updateTokenReq request.
func (r patchTokenReq) Validate() error {
	if err := r.commonTokenReq.Validate(); err != nil {
		return err
	}
	if len(r.TokenID) == 0 {
		return fmt.Errorf("token ID cannot be empty")
	}
	if len(r.Body) == 0 {
		return fmt.Errorf("body can not be empty")
	}

	return nil
}

// Validate validates updateTokenReq request.
func (r deleteTokenReq) Validate() error {
	if err := r.commonTokenReq.Validate(); err != nil {
		return err
	}
	if len(r.TokenID) == 0 {
		return fmt.Errorf("token ID cannot be empty")
	}

	return nil
}

// DecodeAddReq  decodes an HTTP request into addReq.
func DecodeAddTokenReq(c context.Context, r *http.Request) (interface{}, error) {
	var req addTokenReq

	rawReq, err := DecodeTokenReq(c, r)
	if err != nil {
		return nil, err
	}
	tokenReq := rawReq.(commonTokenReq)
	req.ServiceAccountID = tokenReq.ServiceAccountID
	req.ProjectID = tokenReq.ProjectID

	if err := json.NewDecoder(r.Body).Decode(&req.Body); err != nil {
		return nil, err
	}

	return req, nil
}

// DecodeTokenReq  decodes an HTTP request into addReq.
func DecodeTokenReq(c context.Context, r *http.Request) (interface{}, error) {
	var req commonTokenReq

	prjReq, err := common.DecodeProjectRequest(c, r)
	if err != nil {
		return nil, err
	}
	req.ProjectReq = prjReq.(common.ProjectReq)

	saIDReq, err := decodeServiceAccountIDReq(c, r)
	if err != nil {
		return nil, err
	}
	req.ServiceAccountID = saIDReq.ServiceAccountID

	return req, nil
}

// DecodeUpdateTokenReq  decodes an HTTP request into updateTokenReq.
func DecodeUpdateTokenReq(c context.Context, r *http.Request) (interface{}, error) {
	var req updateTokenReq

	rawReq, err := DecodeTokenReq(c, r)
	if err != nil {
		return nil, err
	}
	tokenReq := rawReq.(commonTokenReq)
	req.ServiceAccountID = tokenReq.ServiceAccountID
	req.ProjectID = tokenReq.ProjectID

	if err := json.NewDecoder(r.Body).Decode(&req.Body); err != nil {
		return nil, err
	}

	tokenID, err := decodeTokenIDReq(c, r)
	if err != nil {
		return nil, err
	}

	req.TokenID = tokenID.TokenID

	return req, nil
}

// DecodePatchTokenReq  decodes an HTTP request into patchTokenReq.
func DecodePatchTokenReq(c context.Context, r *http.Request) (interface{}, error) {
	var req patchTokenReq

	rawReq, err := DecodeTokenReq(c, r)
	if err != nil {
		return nil, err
	}
	tokenReq := rawReq.(commonTokenReq)
	req.ServiceAccountID = tokenReq.ServiceAccountID
	req.ProjectID = tokenReq.ProjectID

	req.Body, err = io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	tokenID, err := decodeTokenIDReq(c, r)
	if err != nil {
		return nil, err
	}

	req.TokenID = tokenID.TokenID

	return req, nil
}

// DecodeDeleteTokenReq  decodes an HTTP request into deleteTokenReq.
func DecodeDeleteTokenReq(c context.Context, r *http.Request) (interface{}, error) {
	var req deleteTokenReq

	rawReq, err := DecodeTokenReq(c, r)
	if err != nil {
		return nil, err
	}
	tokenReq := rawReq.(commonTokenReq)
	req.ServiceAccountID = tokenReq.ServiceAccountID
	req.ProjectID = tokenReq.ProjectID

	tokenID, err := decodeTokenIDReq(c, r)
	if err != nil {
		return nil, err
	}

	req.TokenID = tokenID.TokenID

	return req, nil
}

func decodeTokenIDReq(c context.Context, r *http.Request) (tokenIDReq, error) {
	var req tokenIDReq

	tokenID, ok := mux.Vars(r)["token_id"]
	if !ok {
		return req, fmt.Errorf("'token_id' parameter is required")
	}
	req.TokenID = tokenID

	return req, nil
}

func convertInternalTokenToPrivateExternal(internal *corev1.Secret, authenticator serviceaccount.TokenAuthenticator) (*apiv1.ServiceAccountToken, error) {
	externalToken := &apiv1.ServiceAccountToken{}
	public, err := convertInternalTokenToPublicExternal(internal, authenticator)
	if err != nil {
		return nil, err
	}
	externalToken.PublicServiceAccountToken = *public
	token, ok := internal.Data["token"]
	if !ok {
		return nil, fmt.Errorf("can not find token data in secret %s", internal.Name)
	}
	externalToken.Token = string(token)
	return externalToken, nil
}

func convertInternalTokenToPublicExternal(internal *corev1.Secret, authenticator serviceaccount.TokenAuthenticator) (*apiv1.PublicServiceAccountToken, error) {
	externalToken := &apiv1.PublicServiceAccountToken{}
	token, ok := internal.Data["token"]
	if !ok {
		return nil, fmt.Errorf("can not find token data")
	}

	externalToken.ID = internal.Name
	name, ok := internal.Labels["name"]
	if !ok {
		return nil, fmt.Errorf("can not find token name in secret %s", internal.Name)
	}
	externalToken.Name = name

	externalToken.CreationTimestamp = apiv1.NewTime(internal.CreationTimestamp.Time)

	publicClaim, _, err := authenticator.Authenticate(string(token))
	// set invalidated flag to true if you can't authenticate token
	// It will force the user to regenerate token
	if err != nil {
		externalToken.Invalidated = true
		return externalToken, nil
	}

	externalToken.Expiry = apiv1.NewTime(publicClaim.Expiry.Time())

	return externalToken, nil
}
