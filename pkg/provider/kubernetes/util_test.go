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
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"io"
	"sort"
	"time"

	constrainttemplatev1 "github.com/open-policy-agent/frameworks/constraint/pkg/apis/templates/v1"
	"gopkg.in/square/go-jose.v2/jwt"

	kubermaticv1 "k8c.io/kubermatic/v2/pkg/apis/kubermatic/v1"
	kubermaticv1helper "k8c.io/kubermatic/v2/pkg/apis/kubermatic/v1/helper"
	"k8c.io/dashboard/v2/pkg/provider/kubernetes"
	"k8c.io/kubermatic/v2/pkg/serviceaccount"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// TestFakeToken signed JWT token with fake data.
	TestFakeToken = "eyJhbGciOiJIUzI1NiJ9.eyJlbWFpbCI6InRlc3RAZXhhbXBsZS5jb20iLCJleHAiOjE3NDQ0NjQ1OTYsImlhdCI6MTY0OTc3MDE5NiwibmJmIjoxNjQ5NzcwMTk2LCJwcm9qZWN0X2lkIjoidGVzdFByb2plY3QiLCJ0b2tlbl9pZCI6InRlc3RUb2tlbiJ9.IGcnVhrTGeemEZ_dOGCRE1JXwpSMWJEbrG8hylpTEUY"

	// TestFakeFinalizer is a dummy finalizer with no special meaning.
	TestFakeFinalizer = "test.kubermatic.k8c.io/dummy"
)

type fakeJWTTokenGenerator struct {
}

// Generate generates new fake token.
func (j *fakeJWTTokenGenerator) Generate(claims *jwt.Claims, privateClaims *serviceaccount.CustomTokenClaim) (string, error) {
	return TestFakeToken, nil
}

func createAuthenitactedUser() *kubermaticv1.User {
	testUserName := "user1"
	testUserEmail := "john@acme.com"
	return &kubermaticv1.User{
		ObjectMeta: metav1.ObjectMeta{},
		Spec: kubermaticv1.UserSpec{
			Name:  testUserName,
			Email: testUserEmail,
		},
	}
}

func createBinding(name, projectID, email, group string) *kubermaticv1.UserProjectBinding {
	binding := genBinding(projectID, email, group)
	binding.Kind = kubermaticv1.UserProjectBindingKind
	binding.Name = name
	return binding
}

func createProjectSA(name, projectName, group, id string) *kubermaticv1.User {
	sa := genProjectServiceAccount(id, name, group, projectName)
	// remove autogenerated values
	sa.Spec.Email = ""

	return sa
}

func createSANoPrefix(name, projectName, group, id string) *kubermaticv1.User {
	sa := createProjectSA(name, projectName, group, id)
	sa.Name = kubermaticv1helper.RemoveProjectServiceAccountPrefix(sa.Name)
	return sa
}

func sortTokenByName(tokens []*corev1.Secret) {
	sort.SliceStable(tokens, func(i, j int) bool {
		mi, mj := tokens[i], tokens[j]
		return mi.Name < mj.Name
	})
}

// genUser generates a User resource
// note if the id is empty then it will be auto generated.
func genUser(id, name, email string) *kubermaticv1.User {
	if len(id) == 0 {
		// the name of the object is derived from the email address and encoded as sha256
		id = fmt.Sprintf("%x", sha256.Sum256([]byte(email)))
	}

	h := sha512.New512_224()
	if _, err := io.WriteString(h, email); err != nil {
		// not nice, better to use t.Error
		panic("unable to generate a test user: " + err.Error())
	}

	return &kubermaticv1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: id,
			UID:  types.UID(fmt.Sprintf("fake-uid-%s", id)),
		},
		Spec: kubermaticv1.UserSpec{
			Name:  name,
			Email: email,
		},
	}
}

// genProject generates new empty project.
func genProject(name string, phase kubermaticv1.ProjectPhase, creationTime time.Time) *kubermaticv1.Project {
	return &kubermaticv1.Project{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Project",
			APIVersion: "kubermatic.k8c.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              fmt.Sprintf("%s-%s", name, "ID"),
			CreationTimestamp: metav1.NewTime(creationTime),
		},
		Spec: kubermaticv1.ProjectSpec{
			Name: name,
		},
		Status: kubermaticv1.ProjectStatus{
			Phase: phase,
		},
	}
}

// genDefaultProject generates a default project.
func genDefaultProject() *kubermaticv1.Project {
	return genProject("my-first-project", kubermaticv1.ProjectActive, defaultCreationTimestamp())
}

// defaultCreationTimestamp returns default test timestamp.
func defaultCreationTimestamp() time.Time {
	return time.Date(2013, 02, 03, 19, 54, 0, 0, time.UTC)
}

// genProjectServiceAccount generates a Service Account resource.
func genProjectServiceAccount(id, name, group, projectName string) *kubermaticv1.User {
	userName := kubermaticv1helper.EnsureProjectServiceAccountPrefix(id)

	user := genUser(id, name, fmt.Sprintf("%s@sa.kubermatic.io", userName))
	user.Name = userName
	user.UID = ""
	user.Labels = map[string]string{kubernetes.ServiceAccountLabelGroup: fmt.Sprintf("%s-%s", group, projectName)}
	user.Spec.Project = projectName

	return user
}

// genBinding generates a binding.
func genBinding(projectID, email, group string) *kubermaticv1.UserProjectBinding {
	return &kubermaticv1.UserProjectBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s-%s", projectID, email, group),
		},
		Spec: kubermaticv1.UserProjectBindingSpec{
			UserEmail: email,
			ProjectID: projectID,
			Group:     fmt.Sprintf("%s-%s", group, projectID),
		},
	}
}

func genSecret(projectID, saID, name, id string) *corev1.Secret {
	secret := &corev1.Secret{}
	secret.Name = fmt.Sprintf("sa-token-%s", id)
	secret.Type = "Opaque"
	secret.Namespace = "kubermatic"
	secret.Data = map[string][]byte{}
	secret.Data["token"] = []byte(TestFakeToken)
	secret.Labels = map[string]string{
		kubermaticv1.ProjectIDLabelKey: projectID,
		"name":                         name,
	}
	secret.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: kubermaticv1.SchemeGroupVersion.String(),
			Kind:       kubermaticv1.UserKindName,
			UID:        "",
			Name:       saID,
		},
	}

	return secret
}

func genClusterSpec(name string) *kubermaticv1.ClusterSpec {
	return &kubermaticv1.ClusterSpec{
		Cloud: kubermaticv1.CloudSpec{
			DatacenterName: "FakeDatacenter",
			Fake:           &kubermaticv1.FakeCloudSpec{Token: "SecretToken"},
		},
		HumanReadableName: name,
	}
}

func genCluster(name, clusterType, projectID, workerName, userEmail string) *kubermaticv1.Cluster {
	cluster := &kubermaticv1.Cluster{}

	labels := map[string]string{
		kubermaticv1.ProjectIDLabelKey: projectID,
	}
	if len(workerName) > 0 {
		labels[kubermaticv1.WorkerNameLabelKey] = workerName
	}

	cluster.Labels = labels
	cluster.Name = name
	cluster.Finalizers = []string{TestFakeFinalizer}
	cluster.Spec = *genClusterSpec(name)
	cluster.Status = kubermaticv1.ClusterStatus{
		UserEmail:     userEmail,
		NamespaceName: kubernetes.NamespaceName(name),
	}

	return cluster
}

func genConstraintTemplate(name string) *kubermaticv1.ConstraintTemplate {
	ct := &kubermaticv1.ConstraintTemplate{}
	ct.Kind = "ConstraintTemplate"
	ct.APIVersion = kubermaticv1.SchemeGroupVersion.String()
	ct.Name = name
	ct.Spec = kubermaticv1.ConstraintTemplateSpec{
		CRD: constrainttemplatev1.CRD{
			Spec: constrainttemplatev1.CRDSpec{
				Names: constrainttemplatev1.Names{
					Kind:       "labelconstraint",
					ShortNames: []string{"lc"},
				},
			},
		},
		Targets: []constrainttemplatev1.Target{
			{
				Target: "admission.k8s.gatekeeper.sh",
				Rego: `
		package k8srequiredlabels

        deny[{"msg": msg, "details": {"missing_labels": missing}}] {
          provided := {label | input.review.object.metadata.labels[label]}
          required := {label | label := input.parameters.labels[_]}
          missing := required - provided
          count(missing) > 0
          msg := sprintf("you must provide labels: %v", [missing])
        }`,
			},
		},
		Selector: kubermaticv1.ConstraintTemplateSelector{
			Providers: []string{"aws", "gcp"},
			LabelSelector: metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "cluster",
						Operator: metav1.LabelSelectorOpExists,
					},
				},
				MatchLabels: map[string]string{
					"deployment": "prod",
					"domain":     "sales",
				},
			},
		},
	}

	return ct
}

func genGroupProjectBinding(name, projectID, group, role string) *kubermaticv1.GroupProjectBinding {
	gbp := &kubermaticv1.GroupProjectBinding{}
	gbp.Name = name
	gbp.Spec.Role = role
	gbp.Spec.Group = group
	gbp.Spec.ProjectID = projectID
	gbp.Labels = map[string]string{
		kubermaticv1.ProjectIDLabelKey: projectID,
	}
	return gbp
}
