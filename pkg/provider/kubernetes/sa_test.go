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
	kubermaticv1 "k8c.io/kubermatic/v2/pkg/apis/kubermatic/v1"
	"k8c.io/kubermatic/v2/pkg/test/diff"

	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	fakectrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCreateProjectServiceAccount(t *testing.T) {
	// test data
	testcases := []struct {
		name                      string
		existingKubermaticObjects []ctrlruntimeclient.Object
		project                   *kubermaticv1.Project
		userInfo                  *provider.UserInfo
		saName                    string
		saGroup                   string
		expectedSA                *kubermaticv1.User
		expectedSAName            string
	}{
		{
			name:     "scenario 1, create service account `test` for editors group",
			userInfo: &provider.UserInfo{Email: "john@acme.com", Groups: []string{"owners-abcd"}},
			project:  genDefaultProject(),
			saName:   "test",
			saGroup:  "editors-my-first-project-ID",
			existingKubermaticObjects: []ctrlruntimeclient.Object{
				createAuthenitactedUser(),
				genDefaultProject(),
			},
			expectedSA: func() *kubermaticv1.User {
				sa := createSANoPrefix("test", "my-first-project-ID", "editors", "1")
				sa.ResourceVersion = ""
				return sa
			}(),
			expectedSAName: "1",
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := fakectrlruntimeclient.
				NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithObjects(tc.existingKubermaticObjects...).
				Build()

			fakeImpersonationClient := func(impCfg restclient.ImpersonationConfig) (ctrlruntimeclient.Client, error) {
				return fakeClient, nil
			}
			// act
			target := kubernetes.NewServiceAccountProvider(fakeImpersonationClient, fakeClient, "localhost")

			sa, err := target.CreateProjectServiceAccount(context.Background(), tc.userInfo, tc.project, tc.saName, tc.saGroup)

			// validate
			if err != nil {
				t.Fatal(err)
			}

			// remove autogenerated fields
			sa.Name = tc.expectedSAName
			sa.Spec.Email = ""
			sa.ResourceVersion = ""

			if !diff.SemanticallyEqual(tc.expectedSA, sa) {
				t.Fatalf("Objects differ:\n%v", diff.ObjectDiff(tc.expectedSA, sa))
			}
		})
	}
}

func TestListProjectServiceAccount(t *testing.T) {
	// test data
	testcases := []struct {
		name                      string
		existingKubermaticObjects []ctrlruntimeclient.Object
		project                   *kubermaticv1.Project
		saName                    string
		userInfo                  *provider.UserInfo
		expectedSA                []*kubermaticv1.User
	}{
		{
			name:     "scenario 1, get existing service accounts",
			userInfo: &provider.UserInfo{Email: "john@acme.com", Groups: []string{"owners-abcd"}},
			project:  genDefaultProject(),
			saName:   "test-1",
			existingKubermaticObjects: []ctrlruntimeclient.Object{
				createAuthenitactedUser(),
				createProjectSA("test-1", "my-first-project-ID", "editors", "1"),
				createProjectSA("test-2", "abcd", "viewers", "2"),
				createProjectSA("test-1", "dcba", "viewers", "3"),
			},
			expectedSA: []*kubermaticv1.User{
				createSANoPrefix("test-1", "my-first-project-ID", "editors", "1"),
			},
		},
		{
			name:     "scenario 2, service accounts not found for the project",
			userInfo: &provider.UserInfo{Email: "john@acme.com", Groups: []string{"owners-abcd"}},
			project:  genDefaultProject(),
			saName:   "test",
			existingKubermaticObjects: []ctrlruntimeclient.Object{
				createAuthenitactedUser(),
				createProjectSA("test", "bbbb", "editors", "1"),
				createProjectSA("fake", "abcd", "editors", "2"),
			},
			expectedSA: []*kubermaticv1.User{},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := fakectrlruntimeclient.
				NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithObjects(tc.existingKubermaticObjects...).
				Build()

			fakeImpersonationClient := func(impCfg restclient.ImpersonationConfig) (ctrlruntimeclient.Client, error) {
				return fakeClient, nil
			}
			// act
			target := kubernetes.NewServiceAccountProvider(fakeImpersonationClient, fakeClient, "localhost")

			saList, err := target.ListProjectServiceAccount(context.Background(), tc.userInfo, tc.project, &provider.ServiceAccountListOptions{ServiceAccountName: tc.saName})
			// validate
			if err != nil {
				t.Fatal(err)
			}

			for i := range saList {
				saList[i].ResourceVersion = ""
			}

			for i := range tc.expectedSA {
				tc.expectedSA[i].ResourceVersion = ""
			}

			if !diff.SemanticallyEqual(tc.expectedSA, saList) {
				t.Fatalf("Objects differ:\n%v", diff.ObjectDiff(tc.expectedSA, saList))
			}
		})
	}
}

func TestGetProjectServiceAccount(t *testing.T) {
	// test data
	testcases := []struct {
		name                      string
		existingKubermaticObjects []ctrlruntimeclient.Object
		project                   *kubermaticv1.Project
		saName                    string
		userInfo                  *provider.UserInfo
		expectedSA                *kubermaticv1.User
	}{
		{
			name:     "scenario 1, get existing service account",
			userInfo: &provider.UserInfo{Email: "john@acme.com", Groups: []string{"owners-abcd"}},
			project:  genDefaultProject(),
			saName:   "1",
			existingKubermaticObjects: []ctrlruntimeclient.Object{
				createAuthenitactedUser(),
				createProjectSA("test-1", "my-first-project-ID", "editors", "1"),
				createProjectSA("test-2", "abcd", "viewers", "2"),
				createProjectSA("test-1", "dcba", "viewers", "3"),
			},
			expectedSA: func() *kubermaticv1.User {
				sa := createSANoPrefix("test-1", "my-first-project-ID", "editors", "1")
				sa.Kind = "User"
				sa.APIVersion = "kubermatic.k8c.io/v1"
				return sa
			}(),
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := fakectrlruntimeclient.
				NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithObjects(tc.existingKubermaticObjects...).
				Build()

			fakeImpersonationClient := func(impCfg restclient.ImpersonationConfig) (ctrlruntimeclient.Client, error) {
				return fakeClient, nil
			}
			// act
			target := kubernetes.NewServiceAccountProvider(fakeImpersonationClient, fakeClient, "localhost")

			sa, err := target.GetProjectServiceAccount(context.Background(), tc.userInfo, tc.saName, nil)
			// validate
			if err != nil {
				t.Fatal(err)
			}

			tc.expectedSA.ResourceVersion = sa.ResourceVersion

			if !diff.SemanticallyEqual(tc.expectedSA, sa) {
				t.Fatalf("Objects differ:\n%v", diff.ObjectDiff(tc.expectedSA, sa))
			}
		})
	}
}

func TestUpdateProjectServiceAccount(t *testing.T) {
	// test data
	testcases := []struct {
		name                      string
		existingKubermaticObjects []ctrlruntimeclient.Object
		project                   *kubermaticv1.Project
		saName                    string
		newName                   string
		userInfo                  *provider.UserInfo
		expectedSA                *kubermaticv1.User
	}{
		{
			name:     "scenario 1, change name for service account",
			userInfo: &provider.UserInfo{Email: "john@acme.com", Groups: []string{"owners-abcd"}},
			project:  genDefaultProject(),
			saName:   "1",
			existingKubermaticObjects: []ctrlruntimeclient.Object{
				createAuthenitactedUser(),
				createProjectSA("test-1", "my-first-project-ID", "viewers", "1"),
				createProjectSA("test-2", "abcd", "viewers", "2"),
				createProjectSA("test-1", "dcba", "viewers", "3"),
			},
			newName: "new-name",
			expectedSA: func() *kubermaticv1.User {
				sa := createSANoPrefix("new-name", "my-first-project-ID", "viewers", "1")
				sa.Kind = "User"
				sa.APIVersion = "kubermatic.k8c.io/v1"
				sa.ResourceVersion = "1"
				return sa
			}(),
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := fakectrlruntimeclient.
				NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithObjects(tc.existingKubermaticObjects...).
				Build()

			fakeImpersonationClient := func(impCfg restclient.ImpersonationConfig) (ctrlruntimeclient.Client, error) {
				return fakeClient, nil
			}
			// act
			target := kubernetes.NewServiceAccountProvider(fakeImpersonationClient, fakeClient, "localhost")

			sa, err := target.GetProjectServiceAccount(context.Background(), tc.userInfo, tc.saName, nil)
			if err != nil {
				t.Fatal(err)
			}

			sa.Spec.Name = tc.newName

			expectedSA, err := target.UpdateProjectServiceAccount(context.Background(), tc.userInfo, sa)
			if err != nil {
				t.Fatal(err)
			}

			tc.expectedSA.ResourceVersion = expectedSA.ResourceVersion

			if !diff.SemanticallyEqual(tc.expectedSA, expectedSA) {
				t.Fatalf("Objects differ:\n%v", diff.ObjectDiff(tc.expectedSA, expectedSA))
			}
		})
	}
}

func TestDeleteProjectServiceAccount(t *testing.T) {
	// test data
	testcases := []struct {
		name                      string
		existingKubermaticObjects []ctrlruntimeclient.Object
		saName                    string
		userInfo                  *provider.UserInfo
	}{
		{
			name:     "scenario 1, delete service account",
			userInfo: &provider.UserInfo{Email: "john@acme.com", Groups: []string{"owners-abcd"}},
			saName:   "1",
			existingKubermaticObjects: []ctrlruntimeclient.Object{
				createAuthenitactedUser(),
				createProjectSA("test-1", "my-first-project-ID", "viewers", "1"),
				createProjectSA("test-2", "abcd", "viewers", "2"),
				createProjectSA("test-1", "dcba", "viewers", "3"),
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			fakeClient := fakectrlruntimeclient.
				NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithObjects(tc.existingKubermaticObjects...).
				Build()

			fakeImpersonationClient := func(impCfg restclient.ImpersonationConfig) (ctrlruntimeclient.Client, error) {
				return fakeClient, nil
			}
			// act
			target := kubernetes.NewServiceAccountProvider(fakeImpersonationClient, fakeClient, "localhost")

			err := target.DeleteProjectServiceAccount(context.Background(), tc.userInfo, tc.saName)
			if err != nil {
				t.Fatal(err)
			}

			_, err = target.GetProjectServiceAccount(context.Background(), tc.userInfo, tc.saName, nil)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
