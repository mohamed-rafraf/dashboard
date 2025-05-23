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

package kubernetes_test

import (
	"context"
	"testing"

	"k8c.io/dashboard/v2/pkg/provider"
	"k8c.io/dashboard/v2/pkg/provider/kubernetes"
	kubermaticv1 "k8c.io/kubermatic/sdk/v2/apis/kubermatic/v1"
	"k8c.io/kubermatic/v2/pkg/test/fake"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestGetPreset(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		name          string
		presetName    string
		projectID     string
		userInfo      provider.UserInfo
		presets       []ctrlruntimeclient.Object
		expected      *kubermaticv1.Preset
		expectedError string
	}{
		{
			name:       "test 1: get Preset for the specific email group and name",
			userInfo:   provider.UserInfo{Email: "test@example.com"},
			presetName: "test-3",
			presets: []ctrlruntimeclient.Object{
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-1",
					},
					Spec: kubermaticv1.PresetSpec{
						Fake: &kubermaticv1.Fake{
							Token: "aaaaa",
						},
					},
				},
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-2",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"test.com"},
						Fake: &kubermaticv1.Fake{
							Token: "bbbbb",
						},
					},
				},
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-3",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"example.com"},
						Fake: &kubermaticv1.Fake{
							Token: "abc",
						},
					},
				},
			},
			expected: &kubermaticv1.Preset{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-3",
				},
				Spec: kubermaticv1.PresetSpec{
					RequiredEmails: []string{"example.com"},
					Fake: &kubermaticv1.Fake{
						Token: "abc",
					},
				},
			},
		},
		{
			name:       "test 1: get Preset for the rest of the users",
			userInfo:   provider.UserInfo{Email: "test@example.com"},
			presetName: "test-1",
			presets: []ctrlruntimeclient.Object{
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-1",
					},
					Spec: kubermaticv1.PresetSpec{
						Fake: &kubermaticv1.Fake{
							Token: "aaaaa",
						},
					},
				},
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-2",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"test.com"},
						Fake: &kubermaticv1.Fake{
							Token: "bbbbb",
						},
					},
				},
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-3",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"example.com"},
						Fake: &kubermaticv1.Fake{
							Token: "abc",
						},
					},
				}},
			expected: &kubermaticv1.Preset{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-1",
				},
				Spec: kubermaticv1.PresetSpec{
					Fake: &kubermaticv1.Fake{
						Token: "aaaaa",
					},
				},
			},
		},
		{
			name:       "test 3: get Preset which doesn't belong to specific group",
			presetName: "test-2",
			userInfo:   provider.UserInfo{Email: "test@example.com"},
			presets: []ctrlruntimeclient.Object{
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-1",
					},
					Spec: kubermaticv1.PresetSpec{
						Fake: &kubermaticv1.Fake{
							Token: "aaaaa",
						},
					},
				},
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-2",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"acme.com"},
						Fake: &kubermaticv1.Fake{
							Token: "bbbbb",
						},
					},
				},
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-3",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"test.com"},
						Fake: &kubermaticv1.Fake{
							Token: "abc",
						},
					},
				},
			},
			expectedError: "preset.kubermatic.k8c.io \"test-2\" not found",
		},
		{
			name:       "test 4: get Preset which is scoped to a specific project",
			userInfo:   provider.UserInfo{Email: "test@example.com"},
			projectID:  "fake-project",
			presetName: "test-1",
			presets: []ctrlruntimeclient.Object{
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-1",
					},
					Spec: kubermaticv1.PresetSpec{
						Fake: &kubermaticv1.Fake{
							Token: "aaaaa",
						},
						Projects: []string{
							"fake-project",
						},
					},
				},
			},
			expected: &kubermaticv1.Preset{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-1",
				},
				Spec: kubermaticv1.PresetSpec{
					Fake: &kubermaticv1.Fake{
						Token: "aaaaa",
					},
					Projects: []string{
						"fake-project",
					},
				},
			},
		},
		{
			name:       "test 5: get Preset which is scoped to a different project",
			userInfo:   provider.UserInfo{Email: "test@example.com"},
			projectID:  "fake-project",
			presetName: "test-1",
			presets: []ctrlruntimeclient.Object{
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-1",
					},
					Spec: kubermaticv1.PresetSpec{
						Fake: &kubermaticv1.Fake{
							Token: "aaaaa",
						},
						Projects: []string{
							"fake-project-2",
						},
					},
				},
			},
			expectedError: "preset.kubermatic.k8c.io \"test-1\" not found",
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := fake.
				NewClientBuilder().
				WithObjects(tc.presets...).
				Build()

			provider, err := kubernetes.NewPresetProvider(fakeClient)
			if err != nil {
				t.Fatal(err)
			}
			preset, err := provider.GetPreset(context.Background(), &tc.userInfo, &tc.projectID, tc.presetName)
			if len(tc.expectedError) > 0 {
				if err == nil {
					t.Fatalf("expected error")
				}
				if err.Error() != tc.expectedError {
					t.Fatalf("expected: %s, got %v", tc.expectedError, err)
				}
			} else {
				tc.expected.ResourceVersion = preset.ResourceVersion
				if !equality.Semantic.DeepEqual(preset, tc.expected) {
					t.Fatalf("expected: %v, got %v", tc.expected, preset)
				}
			}
		})
	}
}

func TestGetPresets(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		name      string
		userInfo  provider.UserInfo
		projectID string
		presets   []ctrlruntimeclient.Object
		expected  []kubermaticv1.Preset
	}{
		{
			name:     "test 1: get Presets for the specific email group and all users",
			userInfo: provider.UserInfo{Email: "test@example.com"},
			presets: []ctrlruntimeclient.Object{
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-1",
					},
					Spec: kubermaticv1.PresetSpec{
						Fake: &kubermaticv1.Fake{
							Token: "aaaaa",
						},
					},
				},
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-2",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"com"},
						Fake: &kubermaticv1.Fake{
							Token: "bbbbb",
						},
					},
				},
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-3",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"example.com"},
						Fake: &kubermaticv1.Fake{
							Token: "abc",
						},
					},
				},
			},
			expected: []kubermaticv1.Preset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-1",
					},
					Spec: kubermaticv1.PresetSpec{
						Fake: &kubermaticv1.Fake{
							Token: "aaaaa",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-3",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"example.com"},
						Fake: &kubermaticv1.Fake{
							Token: "abc",
						},
					},
				},
			},
		},
		{
			name:     "test 2: get Presets for the all users, not for the specific email group",
			userInfo: provider.UserInfo{Email: "test@example.com"},
			presets: []ctrlruntimeclient.Object{
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-1",
					},
					Spec: kubermaticv1.PresetSpec{
						Fake: &kubermaticv1.Fake{
							Token: "aaaaa",
						},
					},
				},
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-2",
					},
					Spec: kubermaticv1.PresetSpec{
						Fake: &kubermaticv1.Fake{
							Token: "bbbbb",
						},
					},
				},
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-3",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"test.com"},
						Fake: &kubermaticv1.Fake{
							Token: "abc",
						},
					},
				},
			},
			expected: []kubermaticv1.Preset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-1",
					},
					Spec: kubermaticv1.PresetSpec{
						Fake: &kubermaticv1.Fake{
							Token: "aaaaa",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-2",
					},
					Spec: kubermaticv1.PresetSpec{
						Fake: &kubermaticv1.Fake{
							Token: "bbbbb",
						},
					},
				},
			},
		},
		{
			name:     "test 3: get Presets for a specific user",
			userInfo: provider.UserInfo{Email: "test@example.com"},
			presets: []ctrlruntimeclient.Object{
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-1",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"test@example.com", "pleaseno.org"},
						Fake: &kubermaticv1.Fake{
							Token: "aaaaa",
						},
					},
				},
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-2",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"foo@bar.com", "pleaseno.org"},
						Fake: &kubermaticv1.Fake{
							Token: "bbbbb",
						},
					},
				},
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-3",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"foobar@example.com"},
						Fake: &kubermaticv1.Fake{
							Token: "abc",
						},
					},
				},
			},
			expected: []kubermaticv1.Preset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-1",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"test@example.com", "pleaseno.org"},
						Fake: &kubermaticv1.Fake{
							Token: "aaaaa",
						},
					},
				},
			},
		},
		{
			name:     "test 4: get Presets for a specific user including group mail preset",
			userInfo: provider.UserInfo{Email: "test@example.com"},
			presets: []ctrlruntimeclient.Object{
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-1",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"test@example.com", "pleaseno.org"},
						Fake: &kubermaticv1.Fake{
							Token: "aaaaa",
						},
					},
				},
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-2",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"foo@bar.com"},
						Fake: &kubermaticv1.Fake{
							Token: "bbbbb",
						},
					},
				},
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-3",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"example.com", "foobar.com"},
						Fake: &kubermaticv1.Fake{
							Token: "abc",
						},
					},
				},
			},
			expected: []kubermaticv1.Preset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-1",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"test@example.com", "pleaseno.org"},
						Fake: &kubermaticv1.Fake{
							Token: "aaaaa",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-3",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"example.com", "foobar.com"},
						Fake: &kubermaticv1.Fake{
							Token: "abc",
						},
					},
				},
			},
		},
		{
			name:     "test 5: get Presets for a specific user including generic preset",
			userInfo: provider.UserInfo{Email: "test@example.com"},
			presets: []ctrlruntimeclient.Object{
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-1",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"test@example.com"},
						Fake: &kubermaticv1.Fake{
							Token: "aaaaa",
						},
					},
				},
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-2",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"foo@bar.com"},
						Fake: &kubermaticv1.Fake{
							Token: "bbbbb",
						},
					},
				},
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-3",
					},
					Spec: kubermaticv1.PresetSpec{
						Fake: &kubermaticv1.Fake{
							Token: "abc",
						},
					},
				},
			},
			expected: []kubermaticv1.Preset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-1",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"test@example.com"},
						Fake: &kubermaticv1.Fake{
							Token: "aaaaa",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-3",
					},
					Spec: kubermaticv1.PresetSpec{
						Fake: &kubermaticv1.Fake{
							Token: "abc",
						},
					},
				},
			},
		},
		{
			name:      "test 6: get Presets for a specific project",
			userInfo:  provider.UserInfo{Email: "test@example.com"},
			projectID: "fake-project",
			presets: []ctrlruntimeclient.Object{
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-1",
					},
					Spec: kubermaticv1.PresetSpec{
						Fake: &kubermaticv1.Fake{
							Token: "aaaaa",
						},
					},
				},
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-2",
					},
					Spec: kubermaticv1.PresetSpec{
						Fake: &kubermaticv1.Fake{
							Token: "bbbbb",
						},
						Projects: []string{
							"fake-project-2",
						},
					},
				},
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-3",
					},
					Spec: kubermaticv1.PresetSpec{
						Fake: &kubermaticv1.Fake{
							Token: "abc",
						},
						Projects: []string{
							"fake-project",
						},
					},
				},
			},
			expected: []kubermaticv1.Preset{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-1",
					},
					Spec: kubermaticv1.PresetSpec{
						Fake: &kubermaticv1.Fake{
							Token: "aaaaa",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-3",
					},
					Spec: kubermaticv1.PresetSpec{
						Fake: &kubermaticv1.Fake{
							Token: "abc",
						},
						Projects: []string{
							"fake-project",
						},
					},
				},
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := fake.
				NewClientBuilder().
				WithObjects(tc.presets...).
				Build()

			provider, err := kubernetes.NewPresetProvider(fakeClient)
			if err != nil {
				t.Fatal(err)
			}
			presets, err := provider.GetPresets(context.Background(), &tc.userInfo, &tc.projectID)
			if err != nil {
				t.Fatal(err)
			}

			for n := range presets {
				presets[n].ResourceVersion = ""
			}

			if !equality.Semantic.DeepEqual(presets, tc.expected) {
				t.Fatalf("expected: %v, got %v", tc.expected, presets)
			}
		})
	}
}

func TestCredentialEndpoint(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		name              string
		presetName        string
		userInfo          provider.UserInfo
		projectID         string
		expectedError     string
		cloudSpec         kubermaticv1.CloudSpec
		expectedCloudSpec *kubermaticv1.CloudSpec
		dc                *kubermaticv1.Datacenter
		presets           []ctrlruntimeclient.Object
	}{
		{
			name:       "test 1: set credentials for Fake provider",
			presetName: "test",
			userInfo:   provider.UserInfo{Email: "test@example.com"},
			projectID:  "fake-project",
			presets: []ctrlruntimeclient.Object{
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "fake",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"com"},
						Fake: &kubermaticv1.Fake{
							Token: "abcd",
						},
					},
				},
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"example.com"},
						Fake: &kubermaticv1.Fake{
							Token: "abc",
						},
					},
				},
			},
			cloudSpec:         kubermaticv1.CloudSpec{Fake: &kubermaticv1.FakeCloudSpec{}},
			expectedCloudSpec: &kubermaticv1.CloudSpec{Fake: &kubermaticv1.FakeCloudSpec{Token: "abc"}},
		},
		{
			name:       "test 2: set credentials for GCP provider",
			presetName: "test",
			userInfo:   provider.UserInfo{Email: "test@example.com"},
			projectID:  "fake-project",
			presets: []ctrlruntimeclient.Object{
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"example.com"},
						GCP: &kubermaticv1.GCP{
							ServiceAccount: "test_service_accouont",
						},
					},
				},
			},

			cloudSpec:         kubermaticv1.CloudSpec{GCP: &kubermaticv1.GCPCloudSpec{}},
			expectedCloudSpec: &kubermaticv1.CloudSpec{GCP: &kubermaticv1.GCPCloudSpec{ServiceAccount: "test_service_accouont"}},
		},
		{
			name:       "test 3: set credentials for AWS provider",
			presetName: "test",
			userInfo:   provider.UserInfo{Email: "test@example.com"},
			projectID:  "fake-project",
			presets: []ctrlruntimeclient.Object{
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"example.com"},
						AWS: &kubermaticv1.AWS{
							SecretAccessKey: "secret", AccessKeyID: "key",
						},
					},
				},
			},

			cloudSpec:         kubermaticv1.CloudSpec{AWS: &kubermaticv1.AWSCloudSpec{}},
			expectedCloudSpec: &kubermaticv1.CloudSpec{AWS: &kubermaticv1.AWSCloudSpec{AccessKeyID: "key", SecretAccessKey: "secret"}},
		},
		{
			name:       "test 4: set credentials for Hetzner provider",
			presetName: "test",
			userInfo:   provider.UserInfo{Email: "test@example.com"},
			projectID:  "fake-project",
			presets: []ctrlruntimeclient.Object{
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"example.com"},
						Hetzner: &kubermaticv1.Hetzner{
							Token:   "secret",
							Network: "test",
						},
					},
				},
			},
			cloudSpec:         kubermaticv1.CloudSpec{Hetzner: &kubermaticv1.HetznerCloudSpec{}},
			expectedCloudSpec: &kubermaticv1.CloudSpec{Hetzner: &kubermaticv1.HetznerCloudSpec{Token: "secret", Network: "test"}},
		},
		{
			name:       "test 5: set credentials for Packet provider",
			presetName: "test",
			userInfo:   provider.UserInfo{Email: "test@example.com"},
			projectID:  "fake-project",
			presets: []ctrlruntimeclient.Object{
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"example.com"},
						Packet: &kubermaticv1.Packet{
							APIKey: "secret", ProjectID: "project",
						},
					},
				},
			},
			cloudSpec:         kubermaticv1.CloudSpec{Packet: &kubermaticv1.PacketCloudSpec{}},
			expectedCloudSpec: &kubermaticv1.CloudSpec{Packet: &kubermaticv1.PacketCloudSpec{APIKey: "secret", ProjectID: "project", BillingCycle: "hourly"}},
		},
		{
			name:       "test 6: set credentials for DigitalOcean provider",
			presetName: "test",
			userInfo:   provider.UserInfo{Email: "test@example.com"},
			projectID:  "fake-project",
			presets: []ctrlruntimeclient.Object{
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "fake",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"example"},
						Digitalocean: &kubermaticv1.Digitalocean{
							Token: "abcdefg",
						},
					},
				},
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"example.com"},
						Digitalocean: &kubermaticv1.Digitalocean{
							Token: "abcd",
						},
					},
				},
			},
			cloudSpec:         kubermaticv1.CloudSpec{Digitalocean: &kubermaticv1.DigitaloceanCloudSpec{}},
			expectedCloudSpec: &kubermaticv1.CloudSpec{Digitalocean: &kubermaticv1.DigitaloceanCloudSpec{Token: "abcd"}},
		},
		{
			name:       "test 7: set credentials for OpenStack provider",
			presetName: "test",
			userInfo:   provider.UserInfo{Email: "test@example.com"},
			projectID:  "fake-project",
			presets: []ctrlruntimeclient.Object{
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"example.com"},
						Openstack: &kubermaticv1.Openstack{
							Project: "a", Domain: "b", Password: "c", Username: "d",
						},
					},
				},
			},
			dc:                &kubermaticv1.Datacenter{Spec: kubermaticv1.DatacenterSpec{Openstack: &kubermaticv1.DatacenterSpecOpenstack{EnforceFloatingIP: false}}},
			cloudSpec:         kubermaticv1.CloudSpec{Openstack: &kubermaticv1.OpenstackCloudSpec{}},
			expectedCloudSpec: &kubermaticv1.CloudSpec{Openstack: &kubermaticv1.OpenstackCloudSpec{Project: "a", Domain: "b", Password: "c", Username: "d"}},
		},
		{
			name:       "test 8: set credentials for Vsphere provider",
			presetName: "test",
			userInfo:   provider.UserInfo{Email: "test@example.com"},
			projectID:  "fake-project",
			presets: []ctrlruntimeclient.Object{
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"example.com"},
						VSphere: &kubermaticv1.VSphere{
							Username: "bob", Password: "secret",
						},
					},
				},
			},
			dc:                &kubermaticv1.Datacenter{Spec: kubermaticv1.DatacenterSpec{VSphere: &kubermaticv1.DatacenterSpecVSphere{DefaultStoragePolicy: "fake_storage_policy"}}},
			cloudSpec:         kubermaticv1.CloudSpec{VSphere: &kubermaticv1.VSphereCloudSpec{}},
			expectedCloudSpec: &kubermaticv1.CloudSpec{VSphere: &kubermaticv1.VSphereCloudSpec{Password: "secret", Username: "bob", StoragePolicy: "fake_storage_policy"}},
		},
		{
			name:       "test 9: set credentials for Azure provider",
			presetName: "test",
			userInfo:   provider.UserInfo{Email: "test@example.com"},
			projectID:  "fake-project",
			presets: []ctrlruntimeclient.Object{
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"example.com"},
						Azure: &kubermaticv1.Azure{
							SubscriptionID: "a", ClientID: "b", ClientSecret: "c", TenantID: "d",
						},
					},
				},
			},
			cloudSpec:         kubermaticv1.CloudSpec{Azure: &kubermaticv1.AzureCloudSpec{}},
			expectedCloudSpec: &kubermaticv1.CloudSpec{Azure: &kubermaticv1.AzureCloudSpec{SubscriptionID: "a", ClientID: "b", ClientSecret: "c", TenantID: "d"}},
		},
		{
			name:       "test 10: no credentials for Azure provider",
			presetName: "test",
			userInfo:   provider.UserInfo{Email: "test@example.com"},
			projectID:  "fake-project",
			presets: []ctrlruntimeclient.Object{
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"example.com"},
					},
				},
			},
			cloudSpec:     kubermaticv1.CloudSpec{Azure: &kubermaticv1.AzureCloudSpec{}},
			expectedError: "the preset test doesn't contain credential for Azure provider",
		},
		{
			name:       "test 11: cloud provider spec is empty",
			presetName: "test",
			userInfo:   provider.UserInfo{Email: "test@example.com"},
			projectID:  "fake-project",
			presets: []ctrlruntimeclient.Object{
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"example.com"},
						Azure: &kubermaticv1.Azure{
							SubscriptionID: "a", ClientID: "b", ClientSecret: "c", TenantID: "d",
						},
					},
				},
			},
			cloudSpec:     kubermaticv1.CloudSpec{},
			expectedError: "can not find provider to set credentials",
		},
		{
			name:       "test 12: set credentials for Kubevirt provider",
			presetName: "test",
			userInfo:   provider.UserInfo{Email: "test@example.com"},
			projectID:  "fake-project",
			presets: []ctrlruntimeclient.Object{
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"example.com"},
						Kubevirt: &kubermaticv1.Kubevirt{
							Kubeconfig: "test",
						},
					},
				},
			},
			cloudSpec:         kubermaticv1.CloudSpec{Kubevirt: &kubermaticv1.KubevirtCloudSpec{}},
			expectedCloudSpec: &kubermaticv1.CloudSpec{Kubevirt: &kubermaticv1.KubevirtCloudSpec{Kubeconfig: "test"}},
		},
		{
			name:       "test 13: credential with wrong email domain returns error",
			presetName: "test",
			userInfo:   provider.UserInfo{Email: "test@example.com"},
			projectID:  "fake-project",
			presets: []ctrlruntimeclient.Object{
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"test.com"},
						Azure: &kubermaticv1.Azure{
							SubscriptionID: "a", ClientID: "b", ClientSecret: "c", TenantID: "d",
						},
					},
				},
			},

			cloudSpec:     kubermaticv1.CloudSpec{Azure: &kubermaticv1.AzureCloudSpec{}},
			expectedError: "preset.kubermatic.k8c.io \"test\" not found",
		},
		{
			name:       "test 14: set credentials for Alibaba provider",
			presetName: "test",
			userInfo:   provider.UserInfo{Email: "test@example.com"},
			projectID:  "fake-project",
			presets: []ctrlruntimeclient.Object{
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: kubermaticv1.PresetSpec{
						RequiredEmails: []string{"example.com"},
						Alibaba: &kubermaticv1.Alibaba{
							AccessKeySecret: "secret", AccessKeyID: "key",
						},
					},
				},
			},

			cloudSpec:         kubermaticv1.CloudSpec{Alibaba: &kubermaticv1.AlibabaCloudSpec{}},
			expectedCloudSpec: &kubermaticv1.CloudSpec{Alibaba: &kubermaticv1.AlibabaCloudSpec{AccessKeyID: "key", AccessKeySecret: "secret"}},
		},
		{
			name:       "test 15: set credentials for Fake provider with project-scoped preset",
			presetName: "test",
			userInfo:   provider.UserInfo{Email: "test@example.com"},
			projectID:  "fake-project",
			presets: []ctrlruntimeclient.Object{
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: kubermaticv1.PresetSpec{
						Fake: &kubermaticv1.Fake{
							Token: "abc",
						},
						Projects: []string{
							"fake-project",
						},
					},
				},
			},
			cloudSpec:         kubermaticv1.CloudSpec{Fake: &kubermaticv1.FakeCloudSpec{}},
			expectedCloudSpec: &kubermaticv1.CloudSpec{Fake: &kubermaticv1.FakeCloudSpec{Token: "abc"}},
		},
		{
			name:       "test 16: set credentials for Fake provider with out of scope preset",
			presetName: "test",
			userInfo:   provider.UserInfo{Email: "test@example.com"},
			projectID:  "fake-project",
			presets: []ctrlruntimeclient.Object{
				&kubermaticv1.Preset{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: kubermaticv1.PresetSpec{
						Fake: &kubermaticv1.Fake{
							Token: "abc",
						},
						Projects: []string{
							"fake-project-2",
						},
					},
				},
			},
			cloudSpec:     kubermaticv1.CloudSpec{Fake: &kubermaticv1.FakeCloudSpec{}},
			expectedError: "preset.kubermatic.k8c.io \"test\" not found",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := fake.
				NewClientBuilder().
				WithObjects(tc.presets...).
				Build()

			provider, err := kubernetes.NewPresetProvider(fakeClient)
			if err != nil {
				t.Fatal(err)
			}
			cloudResult, err := provider.SetCloudCredentials(context.Background(), &tc.userInfo, tc.projectID, tc.presetName, tc.cloudSpec, tc.dc)

			if len(tc.expectedError) > 0 {
				if err == nil {
					t.Fatalf("expected error")
				}
				if err.Error() != tc.expectedError {
					t.Fatalf("expected: %s, got %v", tc.expectedError, err)
				}
			} else if !equality.Semantic.DeepEqual(cloudResult, tc.expectedCloudSpec) {
				t.Fatalf("expected: %v, got %v", tc.expectedCloudSpec, cloudResult)
			}
		})
	}
}
