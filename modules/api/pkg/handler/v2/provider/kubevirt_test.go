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

package provider_test

import (
	"fmt"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	kvapiv1 "kubevirt.io/api/core/v1"
	kvinstancetypev1alpha1 "kubevirt.io/api/instancetype/v1alpha1"

	"github.com/kubermatic/machine-controller/pkg/providerconfig/types"
	apiv1 "k8c.io/dashboard/v2/pkg/api/v1"
	providercommon "k8c.io/dashboard/v2/pkg/handler/common/provider"
	"k8c.io/dashboard/v2/pkg/handler/test"
	"k8c.io/dashboard/v2/pkg/handler/test/hack"
	"k8c.io/dashboard/v2/pkg/provider/cloud/kubevirt"
	kubermaticv1 "k8c.io/kubermatic/v2/pkg/apis/kubermatic/v1"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	fakectrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	kubevirtDatacenterName = "KubevirtDC"
)

func init() {
	utilruntime.Must(kvapiv1.AddToScheme(scheme.Scheme))
	utilruntime.Must(kvinstancetypev1alpha1.AddToScheme(scheme.Scheme))
	utilruntime.Must(cdiv1beta1.AddToScheme(scheme.Scheme))
}

var (
	// Cluster settings.
	clusterId    = "keen-snyder"
	clusterName  = "clusterAbc"
	fakeKvConfig = "eyJhcGlWZXJzaW9uIjoidjEiLCJjbHVzdGVycyI6W3siY2x1c3RlciI6eyJjZXJ0aWZpY2F0ZS1hdXRob3JpdHktZGF0YSI6IiIsInNlcnZlciI6Imh0dHBzOi8vOTUuMjE2LjIwLjE0Njo2NDQzIn0sIm5hbWUiOiJrdWJlcm5ldGVzIn1dLCJjb250ZXh0cyI6W3siY29udGV4dCI6eyJjbHVzdGVyIjoia3ViZXJuZXRlcyIsIm5hbWVzcGFjZSI6Imt1YmUtc3lzdGVtIiwidXNlciI6Imt1YmVybmV0ZXMtYWRtaW4ifSwibmFtZSI6Imt1YmVybmV0ZXMtYWRtaW5Aa3ViZXJuZXRlcyJ9XSwiY3VycmVudC1jb250ZXh0Ijoia3ViZXJuZXRlcy1hZG1pbkBrdWJlcm5ldGVzIiwia2luZCI6IkNvbmZpZyIsInByZWZlcmVuY2VzIjp7fSwidXNlcnMiOlt7Im5hbWUiOiJrdWJlcm5ldGVzLWFkbWluIiwidXNlciI6eyJjbGllbnQtY2VydGlmaWNhdGUtZGF0YSI6IiIsImNsaWVudC1rZXktZGF0YSI6IiJ9fV19"
	// Credential ref name.
	credentialref = "credentialref"
	credentialns  = "ns"
)

type KeyValue struct {
	Key   string
	Value string
}

func NewCredentialSecret(name, namespace string) *corev1.Secret {
	data := map[string][]byte{
		"kubeConfig": []byte(fakeKvConfig),
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
}

func GenKubeVirtKubermaticPreset() *kubermaticv1.Preset {
	return &kubermaticv1.Preset{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubermatic-preset",
		},
		Spec: kubermaticv1.PresetSpec{
			Kubevirt: &kubermaticv1.Kubevirt{
				Kubeconfig: fakeKvConfig,
			},
			Fake: &kubermaticv1.Fake{Token: "dummy_pluton_token"},
		},
	}
}

func setFakeNewKubeVirtClient(objects []ctrlruntimeclient.Object) {
	providercommon.NewKubeVirtClient = func(kubeconfig string, options kubevirt.ClientOptions) (*kubevirt.Client, error) {
		return &kubevirt.Client{
			Client: fakectrlruntimeclient.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(objects...).Build(),
		}, nil
	}
}

var (
	instancetypeOk1             = newClusterInstancetype(2, "4Gi")
	instancetypeOk2             = newClusterInstancetype(4, "8Gi")
	instancetypeNotInQuotaLimit = newClusterInstancetype(35, "256Gi")

	instancetypeListResponse = "{\"instancetypes\":" +
		"{\"custom\":[" +
		"{\"name\":\"cpu-2-memory-4Gi\",\"spec\":\"{\\\"cpu\\\":{\\\"guest\\\":2},\\\"memory\\\":{\\\"guest\\\":\\\"4Gi\\\"}}\"}," +
		"{\"name\":\"cpu-4-memory-8Gi\",\"spec\":\"{\\\"cpu\\\":{\\\"guest\\\":4},\\\"memory\\\":{\\\"guest\\\":\\\"8Gi\\\"}}\"}" +
		"]," +
		"\"kubermatic\":[" +
		"{\"name\":\"standard-2\",\"spec\":\"{\\\"cpu\\\":{\\\"guest\\\":2},\\\"memory\\\":{\\\"guest\\\":\\\"8Gi\\\"}}\"}," +
		"{\"name\":\"standard-4\",\"spec\":\"{\\\"cpu\\\":{\\\"guest\\\":4},\\\"memory\\\":{\\\"guest\\\":\\\"16Gi\\\"}}\"}," +
		"{\"name\":\"standard-8\",\"spec\":\"{\\\"cpu\\\":{\\\"guest\\\":8},\\\"memory\\\":{\\\"guest\\\":\\\"32Gi\\\"}}\"}]}}"
)

func newClusterInstancetype(cpu uint32, memory string) *kvinstancetypev1alpha1.VirtualMachineClusterInstancetype {
	return &kvinstancetypev1alpha1.VirtualMachineClusterInstancetype{
		ObjectMeta: metav1.ObjectMeta{
			Name: instancetypeName(cpu, memory),
		},
		Spec: getInstancetypeSpec(cpu, memory),
	}
}

func instancetypeName(cpu uint32, memory string) string {
	return fmt.Sprintf("cpu-%d-memory-%s", cpu, memory)
}

func getQuantity(q string) *resource.Quantity {
	res := resource.MustParse(q)
	return &res
}

func getInstancetypeSpec(cpu uint32, memory string) kvinstancetypev1alpha1.VirtualMachineInstancetypeSpec {
	return kvinstancetypev1alpha1.VirtualMachineInstancetypeSpec{
		CPU: kvinstancetypev1alpha1.CPUInstancetype{
			Guest: cpu,
		},
		Memory: kvinstancetypev1alpha1.MemoryInstancetype{
			Guest: *getQuantity(memory),
		},
	}
}

func TestListInstanceTypeEndpoint(t *testing.T) {
	testcases := []struct {
		Name                       string
		HTTPRequestMethod          string
		HTTPRequestURL             string
		HTTPRequestHeaders         []KeyValue
		Body                       string
		ExpectedResponse           string
		HTTPStatus                 int
		ExistingKubermaticObjects  []ctrlruntimeclient.Object
		ExistingKubevirtObjects    []ctrlruntimeclient.Object
		ExistingKubevirtK8sObjects []ctrlruntimeclient.Object
		ExistingK8sObjects         []ctrlruntimeclient.Object
		ExistingAPIUser            apiv1.User
	}{
		// KUBEVIRT INSTANCETYPE LIST
		{
			Name:               "scenario 1: kubevirt kubeconfig provided",
			HTTPRequestMethod:  http.MethodGet,
			HTTPRequestURL:     "/api/v2/providers/kubevirt/instancetypes",
			HTTPRequestHeaders: []KeyValue{{Key: "Kubeconfig", Value: fakeKvConfig}},
			Body:               ``,
			HTTPStatus:         http.StatusOK,
			ExistingKubermaticObjects: []ctrlruntimeclient.Object{
				test.GenDefaultProject(),
			},
			ExistingKubevirtObjects: []ctrlruntimeclient.Object{instancetypeOk1, instancetypeOk2, instancetypeNotInQuotaLimit},

			ExistingAPIUser:  *test.GenDefaultAPIUser(),
			ExpectedResponse: instancetypeListResponse,
		},
		{
			Name:               "scenario 2: kubevirt kubeconfig from kubermatic preset",
			HTTPRequestMethod:  http.MethodGet,
			HTTPRequestURL:     "/api/v2/providers/kubevirt/instancetypes",
			HTTPRequestHeaders: []KeyValue{{Key: "Credential", Value: "kubermatic-preset"}},
			Body:               ``,
			HTTPStatus:         http.StatusOK,
			ExistingKubermaticObjects: []ctrlruntimeclient.Object{
				test.GenDefaultProject(),
				GenKubeVirtKubermaticPreset(),
			},
			ExistingKubevirtObjects: []ctrlruntimeclient.Object{instancetypeOk1, instancetypeOk2, instancetypeNotInQuotaLimit},
			ExistingAPIUser:         *test.GenDefaultAPIUser(),
			ExpectedResponse:        instancetypeListResponse,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			setFakeNewKubeVirtClient(append(tc.ExistingKubevirtObjects, tc.ExistingKubevirtK8sObjects...))

			req := httptest.NewRequest(tc.HTTPRequestMethod, tc.HTTPRequestURL, strings.NewReader(tc.Body))
			for _, h := range tc.HTTPRequestHeaders {
				req.Header.Add(h.Key, h.Value)
			}
			res := httptest.NewRecorder()
			ep, err := test.CreateTestEndpoint(tc.ExistingAPIUser, tc.ExistingK8sObjects, tc.ExistingKubermaticObjects, nil, hack.NewTestRouting)
			if err != nil {
				t.Fatalf("failed to create test endpoint: %v", err)
			}

			// act
			ep.ServeHTTP(res, req)

			// validate
			if res.Code != tc.HTTPStatus {
				t.Fatalf("Expected HTTP status code %d, got %d: %s", tc.HTTPStatus, res.Code, res.Body.String())
			}
			test.CompareWithResult(t, res, tc.ExpectedResponse)
		})
	}
}

func TestListInstancetypeNoCredentialsEndpoint(t *testing.T) {
	testcases := []struct {
		Name                       string
		HTTPRequestMethod          string
		HTTPRequestURL             string
		HTTPRequestHeaders         []KeyValue
		Body                       string
		ExpectedResponse           string
		HTTPStatus                 int
		ExistingKubermaticObjects  []ctrlruntimeclient.Object
		ExistingKubevirtObjects    []ctrlruntimeclient.Object
		ExistingKubevirtK8sObjects []ctrlruntimeclient.Object
		ExistingK8sObjects         []ctrlruntimeclient.Object
		ExistingAPIUser            apiv1.User
	}{
		// KUBEVIRT INSTANCE TYPE LIST No Credentials
		{
			Name:              "scenario 1: kubevirt kubeconfig from cluster",
			HTTPRequestMethod: http.MethodGet,
			HTTPRequestURL:    fmt.Sprintf("/api/v2/projects/%s/clusters/%s/providers/kubevirt/instancetypes", test.GenDefaultProject().Name, clusterId),
			Body:              ``,
			HTTPStatus:        http.StatusOK,
			ExistingKubermaticObjects: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				func() *kubermaticv1.Cluster {
					cluster := test.GenCluster(clusterId, clusterName, test.GenDefaultProject().Name, time.Date(2013, 02, 03, 19, 54, 0, 0, time.UTC))
					cluster.Spec.Cloud = kubermaticv1.CloudSpec{
						DatacenterName: kubevirtDatacenterName,
						Kubevirt: &kubermaticv1.KubevirtCloudSpec{
							Kubeconfig: fakeKvConfig,
						},
					}
					return cluster
				}(),
			),
			ExistingKubevirtObjects: []ctrlruntimeclient.Object{instancetypeOk1, instancetypeOk2, instancetypeNotInQuotaLimit},
			ExistingAPIUser:         *test.GenDefaultAPIUser(),
			ExpectedResponse:        instancetypeListResponse,
		},
		{
			Name:              "scenario 2: - kubevirt kubeconfig from credential reference (secret)",
			HTTPRequestMethod: http.MethodGet,
			HTTPRequestURL:    fmt.Sprintf("/api/v2/projects/%s/clusters/%s/providers/kubevirt/instancetypes", test.GenDefaultProject().Name, clusterId),
			Body:              ``,
			HTTPStatus:        http.StatusOK,
			ExistingKubermaticObjects: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				func() *kubermaticv1.Cluster {
					cluster := test.GenCluster(clusterId, clusterName, test.GenDefaultProject().Name, time.Date(2013, 02, 03, 19, 54, 0, 0, time.UTC))
					cluster.Spec.Cloud = kubermaticv1.CloudSpec{
						DatacenterName: kubevirtDatacenterName,
						Kubevirt: &kubermaticv1.KubevirtCloudSpec{
							CredentialsReference: &types.GlobalSecretKeySelector{
								ObjectReference: corev1.ObjectReference{Name: credentialref, Namespace: credentialns},
							},
						},
					}
					return cluster
				}(),
			),
			ExistingK8sObjects:      []ctrlruntimeclient.Object{NewCredentialSecret(credentialref, credentialns)},
			ExistingKubevirtObjects: []ctrlruntimeclient.Object{instancetypeOk1, instancetypeOk2, instancetypeNotInQuotaLimit},
			ExistingAPIUser:         *test.GenDefaultAPIUser(),
			ExpectedResponse:        instancetypeListResponse,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			setFakeNewKubeVirtClient(append(tc.ExistingKubevirtObjects, tc.ExistingKubevirtK8sObjects...))

			req := httptest.NewRequest(tc.HTTPRequestMethod, tc.HTTPRequestURL, strings.NewReader(tc.Body))
			for _, h := range tc.HTTPRequestHeaders {
				req.Header.Add(h.Key, h.Value)
			}
			res := httptest.NewRecorder()
			ep, err := test.CreateTestEndpoint(tc.ExistingAPIUser, tc.ExistingK8sObjects, tc.ExistingKubermaticObjects, nil, hack.NewTestRouting)
			if err != nil {
				t.Fatalf("failed to create test endpoint: %v", err)
			}

			// act
			ep.ServeHTTP(res, req)

			// validate
			if res.Code != tc.HTTPStatus {
				t.Fatalf("Expected HTTP status code %d, got %d: %s", tc.HTTPStatus, res.Code, res.Body.String())
			}
			test.CompareWithResult(t, res, tc.ExpectedResponse)
		})
	}
}

var (
	preferenceCores        = newClusterPreference(kvinstancetypev1alpha1.PreferCores)
	preferenceSockets      = newClusterPreference(kvinstancetypev1alpha1.PreferSockets)
	preferenceListResponse = "{\"preferences\":" +
		"{\"custom\":[" +
		"{\"name\":\"preferCores\",\"spec\":\"{\\\"cpu\\\":{\\\"preferredCPUTopology\\\":\\\"preferCores\\\"}}\"}," +
		"{\"name\":\"preferSockets\",\"spec\":\"{\\\"cpu\\\":{\\\"preferredCPUTopology\\\":\\\"preferSockets\\\"}}\"}]," +
		"\"kubermatic\":[" +
		"{\"name\":\"sockets-advantage\",\"spec\":\"{\\\"cpu\\\":{\\\"preferredCPUTopology\\\":\\\"preferSockets\\\"}}\"}]}}"
)

func newClusterPreference(topology kvinstancetypev1alpha1.PreferredCPUTopology) *kvinstancetypev1alpha1.VirtualMachineClusterPreference {
	return &kvinstancetypev1alpha1.VirtualMachineClusterPreference{
		ObjectMeta: metav1.ObjectMeta{
			Name: string(topology),
		},
		Spec: kvinstancetypev1alpha1.VirtualMachinePreferenceSpec{
			CPU: &kvinstancetypev1alpha1.CPUPreferences{
				PreferredCPUTopology: topology,
			},
		},
	}
}

func TestPreferenceEndpoint(t *testing.T) {
	testcases := []struct {
		Name                       string
		HTTPRequestMethod          string
		HTTPRequestURL             string
		HTTPRequestHeaders         []KeyValue
		Body                       string
		ExpectedResponse           string
		HTTPStatus                 int
		ExistingKubermaticObjects  []ctrlruntimeclient.Object
		ExistingKubevirtObjects    []ctrlruntimeclient.Object
		ExistingKubevirtK8sObjects []ctrlruntimeclient.Object
		ExistingK8sObjects         []ctrlruntimeclient.Object
		ExistingAPIUser            apiv1.User
	}{
		// KUBEVIRT PREFERENCE LIST
		{
			Name:               "scenario 1: kubevirt kubeconfig provided",
			HTTPRequestMethod:  http.MethodGet,
			HTTPRequestURL:     "/api/v2/providers/kubevirt/preferences",
			HTTPRequestHeaders: []KeyValue{{Key: "Kubeconfig", Value: fakeKvConfig}},
			Body:               ``,
			HTTPStatus:         http.StatusOK,
			ExistingKubermaticObjects: []ctrlruntimeclient.Object{
				test.GenDefaultProject(),
			},
			ExistingKubevirtObjects: []ctrlruntimeclient.Object{preferenceCores, preferenceSockets},

			ExistingAPIUser:  *test.GenDefaultAPIUser(),
			ExpectedResponse: preferenceListResponse,
		},
		{
			Name:               "scenario 2: kubevirt kubeconfig from kubermatic preset",
			HTTPRequestMethod:  http.MethodGet,
			HTTPRequestURL:     "/api/v2/providers/kubevirt/preferences",
			HTTPRequestHeaders: []KeyValue{{Key: "Credential", Value: "kubermatic-preset"}},
			Body:               ``,
			HTTPStatus:         http.StatusOK,
			ExistingKubermaticObjects: []ctrlruntimeclient.Object{
				test.GenDefaultProject(),
				GenKubeVirtKubermaticPreset(),
			},
			ExistingKubevirtObjects: []ctrlruntimeclient.Object{preferenceCores, preferenceSockets},
			ExistingAPIUser:         *test.GenDefaultAPIUser(),
			ExpectedResponse:        preferenceListResponse,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			setFakeNewKubeVirtClient(append(tc.ExistingKubevirtObjects, tc.ExistingKubevirtK8sObjects...))

			req := httptest.NewRequest(tc.HTTPRequestMethod, tc.HTTPRequestURL, strings.NewReader(tc.Body))
			for _, h := range tc.HTTPRequestHeaders {
				req.Header.Add(h.Key, h.Value)
			}
			res := httptest.NewRecorder()
			ep, err := test.CreateTestEndpoint(tc.ExistingAPIUser, tc.ExistingK8sObjects, tc.ExistingKubermaticObjects, nil, hack.NewTestRouting)
			if err != nil {
				t.Fatalf("failed to create test endpoint: %v", err)
			}

			// act
			ep.ServeHTTP(res, req)

			// validate
			if res.Code != tc.HTTPStatus {
				t.Fatalf("Expected HTTP status code %d, got %d: %s", tc.HTTPStatus, res.Code, res.Body.String())
			}
			test.CompareWithResult(t, res, tc.ExpectedResponse)
		})
	}
}

func TestListPreferenceNoCredentialsEndpoint(t *testing.T) {
	testcases := []struct {
		Name                       string
		HTTPRequestMethod          string
		HTTPRequestURL             string
		HTTPRequestHeaders         []KeyValue
		Body                       string
		ExpectedResponse           string
		HTTPStatus                 int
		ExistingKubermaticObjects  []ctrlruntimeclient.Object
		ExistingKubevirtObjects    []ctrlruntimeclient.Object
		ExistingKubevirtK8sObjects []ctrlruntimeclient.Object
		ExistingK8sObjects         []ctrlruntimeclient.Object
		ExistingAPIUser            apiv1.User
	}{
		// KUBEVIRT PREFERENCE LIST No Credentials
		{
			Name:              "scenario 1: kubevirt kubeconfig from cluster",
			HTTPRequestMethod: http.MethodGet,
			HTTPRequestURL:    fmt.Sprintf("/api/v2/projects/%s/clusters/%s/providers/kubevirt/preferences", test.GenDefaultProject().Name, clusterId),
			Body:              ``,
			HTTPStatus:        http.StatusOK,
			ExistingKubermaticObjects: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				func() *kubermaticv1.Cluster {
					cluster := test.GenCluster(clusterId, clusterName, test.GenDefaultProject().Name, time.Date(2013, 02, 03, 19, 54, 0, 0, time.UTC))
					cluster.Spec.Cloud = kubermaticv1.CloudSpec{
						DatacenterName: kubevirtDatacenterName,
						Kubevirt: &kubermaticv1.KubevirtCloudSpec{
							Kubeconfig: fakeKvConfig,
						},
					}
					return cluster
				}(),
			),
			ExistingKubevirtObjects: []ctrlruntimeclient.Object{preferenceCores, preferenceSockets},
			ExistingAPIUser:         *test.GenDefaultAPIUser(),
			ExpectedResponse:        preferenceListResponse,
		},
		{
			Name:              "scenario 2: - kubevirt kubeconfig from credential reference (secret)",
			HTTPRequestMethod: http.MethodGet,
			HTTPRequestURL:    fmt.Sprintf("/api/v2/projects/%s/clusters/%s/providers/kubevirt/preferences", test.GenDefaultProject().Name, clusterId),
			Body:              ``,
			HTTPStatus:        http.StatusOK,
			ExistingKubermaticObjects: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				func() *kubermaticv1.Cluster {
					cluster := test.GenCluster(clusterId, clusterName, test.GenDefaultProject().Name, time.Date(2013, 02, 03, 19, 54, 0, 0, time.UTC))
					cluster.Spec.Cloud = kubermaticv1.CloudSpec{
						DatacenterName: kubevirtDatacenterName,
						Kubevirt: &kubermaticv1.KubevirtCloudSpec{
							CredentialsReference: &types.GlobalSecretKeySelector{
								ObjectReference: corev1.ObjectReference{Name: credentialref, Namespace: credentialns},
							},
						},
					}
					return cluster
				}(),
			),
			ExistingK8sObjects:      []ctrlruntimeclient.Object{NewCredentialSecret(credentialref, credentialns)},
			ExistingKubevirtObjects: []ctrlruntimeclient.Object{preferenceCores, preferenceSockets},
			ExistingAPIUser:         *test.GenDefaultAPIUser(),
			ExpectedResponse:        preferenceListResponse,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			setFakeNewKubeVirtClient(append(tc.ExistingKubevirtObjects, tc.ExistingKubevirtK8sObjects...))

			req := httptest.NewRequest(tc.HTTPRequestMethod, tc.HTTPRequestURL, strings.NewReader(tc.Body))
			for _, h := range tc.HTTPRequestHeaders {
				req.Header.Add(h.Key, h.Value)
			}
			res := httptest.NewRecorder()
			ep, err := test.CreateTestEndpoint(tc.ExistingAPIUser, tc.ExistingK8sObjects, tc.ExistingKubermaticObjects, nil, hack.NewTestRouting)
			if err != nil {
				t.Fatalf("failed to create test endpoint: %v", err)
			}

			// act
			ep.ServeHTTP(res, req)

			// validate
			if res.Code != tc.HTTPStatus {
				t.Fatalf("Expected HTTP status code %d, got %d: %s", tc.HTTPStatus, res.Code, res.Body.String())
			}
			test.CompareWithResult(t, res, tc.ExpectedResponse)
		})
	}
}

var (
	reclaimPolicy = corev1.PersistentVolumeReclaimDelete
	storageClass1 = storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "storageclass-1",
		},
		ReclaimPolicy: &reclaimPolicy,
	}
	storageClass2 = storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "storageclass-2",
		},
	}

	storageClassListResponse = `[{"name":"storageclass-1"},{"name":"storageclass-2"}]`
)

func TestListStorageClassEndpoint(t *testing.T) {
	testcases := []struct {
		Name                       string
		HTTPRequestMethod          string
		HTTPRequestURL             string
		HTTPRequestHeaders         []KeyValue
		Body                       string
		ExpectedResponse           string
		HTTPStatus                 int
		ExistingKubermaticObjects  []ctrlruntimeclient.Object
		ExistingKubevirtObjects    []ctrlruntimeclient.Object
		ExistingKubevirtK8sObjects []ctrlruntimeclient.Object
		ExistingK8sObjects         []ctrlruntimeclient.Object
		ExistingAPIUser            apiv1.User
	}{
		// LIST Storage classes
		{
			Name:               "scenario 1: list storage classes- kubevirt kubeconfig provided",
			HTTPRequestMethod:  http.MethodGet,
			HTTPRequestURL:     "/api/v2/providers/kubevirt/storageclasses",
			HTTPRequestHeaders: []KeyValue{{Key: "Kubeconfig", Value: fakeKvConfig}},
			Body:               ``,
			HTTPStatus:         http.StatusOK,
			ExistingKubermaticObjects: []ctrlruntimeclient.Object{
				test.GenDefaultProject(),
			},
			ExistingKubevirtK8sObjects: []ctrlruntimeclient.Object{&storageClass1, &storageClass2},
			ExistingAPIUser:            *test.GenDefaultAPIUser(),
			ExpectedResponse:           storageClassListResponse,
		},
		{
			Name:               "scenario 2: list storage classes- kubevirt from kubermatic preset",
			HTTPRequestMethod:  http.MethodGet,
			HTTPRequestURL:     "/api/v2/providers/kubevirt/storageclasses",
			HTTPRequestHeaders: []KeyValue{{Key: "Credential", Value: "kubermatic-preset"}},
			Body:               ``,
			HTTPStatus:         http.StatusOK,
			ExistingKubermaticObjects: []ctrlruntimeclient.Object{
				test.GenDefaultProject(),
				GenKubeVirtKubermaticPreset(),
			},
			ExistingKubevirtK8sObjects: []ctrlruntimeclient.Object{&storageClass1, &storageClass2},
			ExistingAPIUser:            *test.GenDefaultAPIUser(),
			ExpectedResponse:           storageClassListResponse,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			setFakeNewKubeVirtClient(append(tc.ExistingKubevirtObjects, tc.ExistingKubevirtK8sObjects...))

			req := httptest.NewRequest(tc.HTTPRequestMethod, tc.HTTPRequestURL, strings.NewReader(tc.Body))
			for _, h := range tc.HTTPRequestHeaders {
				req.Header.Add(h.Key, h.Value)
			}
			res := httptest.NewRecorder()
			ep, err := test.CreateTestEndpoint(tc.ExistingAPIUser, tc.ExistingK8sObjects, tc.ExistingKubermaticObjects, nil, hack.NewTestRouting)
			if err != nil {
				t.Fatalf("failed to create test endpoint: %v", err)
			}

			// act
			ep.ServeHTTP(res, req)

			// validate
			if res.Code != tc.HTTPStatus {
				t.Fatalf("Expected HTTP status code %d, got %d: %s", tc.HTTPStatus, res.Code, res.Body.String())
			}
			test.CompareWithResult(t, res, tc.ExpectedResponse)
		})
	}
}

func TestListStorageClassNoCredentialsEndpoint(t *testing.T) {
	testcases := []struct {
		Name                       string
		HTTPRequestMethod          string
		HTTPRequestURL             string
		HTTPRequestHeaders         []KeyValue
		Body                       string
		ExpectedResponse           string
		HTTPStatus                 int
		ExistingKubermaticObjects  []ctrlruntimeclient.Object
		ExistingKubevirtObjects    []ctrlruntimeclient.Object
		ExistingKubevirtK8sObjects []ctrlruntimeclient.Object
		ExistingK8sObjects         []ctrlruntimeclient.Object
		ExistingAPIUser            apiv1.User
	}{
		// LIST Storage classes No Credentials
		{
			Name:               "scenario 1: list storage classes- kubevirt kubeconfig from cluster",
			HTTPRequestMethod:  http.MethodGet,
			HTTPRequestURL:     fmt.Sprintf("/api/v2/projects/%s/clusters/%s/providers/kubevirt/storageclasses", test.GenDefaultProject().Name, clusterId),
			HTTPRequestHeaders: []KeyValue{{Key: "Credential", Value: "kubermatic-preset"}},
			Body:               ``,
			HTTPStatus:         http.StatusOK,
			ExistingKubermaticObjects: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				func() *kubermaticv1.Cluster {
					cluster := test.GenCluster(clusterId, clusterName, test.GenDefaultProject().Name, time.Date(2013, 02, 03, 19, 54, 0, 0, time.UTC))
					cluster.Spec.Cloud = kubermaticv1.CloudSpec{
						DatacenterName: kubevirtDatacenterName,
						Kubevirt: &kubermaticv1.KubevirtCloudSpec{
							Kubeconfig: fakeKvConfig,
						},
					}
					return cluster
				}(),
			),
			ExistingKubevirtK8sObjects: []ctrlruntimeclient.Object{&storageClass1, &storageClass2},
			ExistingAPIUser:            *test.GenDefaultAPIUser(),
			ExpectedResponse:           storageClassListResponse,
		},
		{
			Name:               "scenario 2: list storage classes- kubevirt kubeconfig from credential reference (secret)",
			HTTPRequestMethod:  http.MethodGet,
			HTTPRequestURL:     fmt.Sprintf("/api/v2/projects/%s/clusters/%s/providers/kubevirt/storageclasses", test.GenDefaultProject().Name, clusterId),
			HTTPRequestHeaders: []KeyValue{{Key: "Credential", Value: "kubermatic-preset"}},
			Body:               ``,
			HTTPStatus:         http.StatusOK,
			ExistingKubermaticObjects: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(),
				func() *kubermaticv1.Cluster {
					cluster := test.GenCluster(clusterId, clusterName, test.GenDefaultProject().Name, time.Date(2013, 02, 03, 19, 54, 0, 0, time.UTC))
					cluster.Spec.Cloud = kubermaticv1.CloudSpec{
						DatacenterName: kubevirtDatacenterName,
						Kubevirt: &kubermaticv1.KubevirtCloudSpec{
							CredentialsReference: &types.GlobalSecretKeySelector{
								ObjectReference: corev1.ObjectReference{Name: credentialref, Namespace: credentialns},
							},
						},
					}
					return cluster
				}(),
			),
			ExistingKubevirtK8sObjects: []ctrlruntimeclient.Object{&storageClass1, &storageClass2},
			ExistingK8sObjects:         []ctrlruntimeclient.Object{NewCredentialSecret(credentialref, credentialns)},
			ExistingAPIUser:            *test.GenDefaultAPIUser(),
			ExpectedResponse:           storageClassListResponse,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			setFakeNewKubeVirtClient(append(tc.ExistingKubevirtObjects, tc.ExistingKubevirtK8sObjects...))

			req := httptest.NewRequest(tc.HTTPRequestMethod, tc.HTTPRequestURL, strings.NewReader(tc.Body))
			for _, h := range tc.HTTPRequestHeaders {
				req.Header.Add(h.Key, h.Value)
			}
			res := httptest.NewRecorder()
			ep, err := test.CreateTestEndpoint(tc.ExistingAPIUser, tc.ExistingK8sObjects, tc.ExistingKubermaticObjects, nil, hack.NewTestRouting)
			if err != nil {
				t.Fatalf("failed to create test endpoint: %v", err)
			}

			// act
			ep.ServeHTTP(res, req)

			// validate
			if res.Code != tc.HTTPStatus {
				t.Fatalf("Expected HTTP status code %d, got %d: %s", tc.HTTPStatus, res.Code, res.Body.String())
			}
			test.CompareWithResult(t, res, tc.ExpectedResponse)
		})
	}
}

var (
	customImage      = newDataVolume("test", kubevirt.KubeVirtImagesNamespace)
	standardClonedDV = newDataVolume("ubuntu-18.04", kubevirt.KubeVirtImagesNamespace)
)

func newDataVolume(name, namespace string) cdiv1beta1.DataVolume {
	return cdiv1beta1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: map[string]string{"cdi.kubevirt.io/os-type": "ubuntu"},
		},
	}
}

func TestListVMImagesEndpoint(t *testing.T) {
	testCases := []struct {
		Name                       string
		HTTPRequestMethod          string
		HTTPRequestURL             string
		HTTPRequestHeaders         []KeyValue
		ExpectedResponse           string
		HTTPStatus                 int
		ExistingKubermaticObjects  []ctrlruntimeclient.Object
		ExistingKubevirtObjects    []ctrlruntimeclient.Object
		ExistingKubevirtK8sObjects []ctrlruntimeclient.Object
		ExistingAPIUser            apiv1.User
	}{
		{
			Name:               "scenario 1: list images with standard-images from seed and custom-images-by-admin - provided kubevirt kubeconfig",
			HTTPRequestMethod:  http.MethodGet,
			HTTPRequestURL:     "/api/v2/providers/kubevirt/vmimages",
			HTTPRequestHeaders: []KeyValue{{Key: "Kubeconfig", Value: fakeKvConfig}, {Key: "DatacenterName", Value: "kubevirt-dc"}},
			ExpectedResponse:   `{"vmImages":{"custom":[{"source":"pvc","images":{"ubuntu":{"global-test":"kubevirt-images/test"}}}],"standard":[{"source":"http","images":{"ubuntu":{"18.04":"http://upstream.com/ubuntu.img"}}}]}}`,
			HTTPStatus:         200,
			ExistingKubermaticObjects: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(func(seed *kubermaticv1.Seed) {
					seed.Spec.Datacenters["kubevirt-dc"] = kubermaticv1.Datacenter{
						Spec: kubermaticv1.DatacenterSpec{
							Kubevirt: &kubermaticv1.DatacenterSpecKubevirt{
								Images: kubermaticv1.ImageSources{
									EnableCustomImages: true,
									HTTP: &kubermaticv1.HTTPSource{
										OperatingSystems: map[types.OperatingSystem]kubermaticv1.OSVersions{"ubuntu": map[string]string{"18.04": "http://upstream.com/ubuntu.img"}},
									}},
							},
						},
					}
				}),
			),
			ExistingKubevirtK8sObjects: []ctrlruntimeclient.Object{&customImage},
			ExistingAPIUser:            *test.GenDefaultAPIUser(),
		},
		{
			Name:               "scenario 2: list images image-source from seed and custom-images-by-admin - provided kubermatic-preset",
			HTTPRequestMethod:  http.MethodGet,
			HTTPRequestURL:     "/api/v2/providers/kubevirt/vmimages",
			HTTPRequestHeaders: []KeyValue{{Key: "Credential", Value: "kubermatic-preset"}, {Key: "DatacenterName", Value: "kubevirt-dc"}},
			ExpectedResponse:   `{"vmImages":{"custom":[{"source":"pvc","images":{"ubuntu":{"global-test":"kubevirt-images/test"}}}],"standard":[{"source":"http","images":{"ubuntu":{"18.04":"http://upstream.com/ubuntu.img"}}}]}}`,
			HTTPStatus:         200,
			ExistingKubermaticObjects: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(func(seed *kubermaticv1.Seed) {
					seed.Spec.Datacenters["kubevirt-dc"] = kubermaticv1.Datacenter{
						Spec: kubermaticv1.DatacenterSpec{
							Kubevirt: &kubermaticv1.DatacenterSpecKubevirt{
								Images: kubermaticv1.ImageSources{
									EnableCustomImages: true,
									HTTP: &kubermaticv1.HTTPSource{
										OperatingSystems: map[types.OperatingSystem]kubermaticv1.OSVersions{"ubuntu": map[string]string{"18.04": "http://upstream.com/ubuntu.img"}},
									}},
							},
						},
					}
				}),
				GenKubeVirtKubermaticPreset(),
			),
			ExistingKubevirtK8sObjects: []ctrlruntimeclient.Object{&customImage},
			ExistingAPIUser:            *test.GenDefaultAPIUser(),
		},
		{
			Name:               "scenario 3: list standard-images from seed with cloning-enabled - provided kubevirt kubeconfig",
			HTTPRequestMethod:  http.MethodGet,
			HTTPRequestURL:     "/api/v2/providers/kubevirt/vmimages",
			HTTPRequestHeaders: []KeyValue{{Key: "Kubeconfig", Value: fakeKvConfig}, {Key: "DatacenterName", Value: "kubevirt-dc"}},
			ExpectedResponse:   `{"vmImages":{"standard":[{"source":"http","images":{"ubuntu":{"18.04":"http://upstream.com/ubuntu.img"}}},{"source":"pvc","images":{"ubuntu":{"18.04":"kubevirt-images/ubuntu-18.04"}}}]}}`,
			HTTPStatus:         200,
			ExistingKubermaticObjects: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(func(seed *kubermaticv1.Seed) {
					seed.Spec.Datacenters["kubevirt-dc"] = kubermaticv1.Datacenter{
						Spec: kubermaticv1.DatacenterSpec{
							Kubevirt: &kubermaticv1.DatacenterSpecKubevirt{
								DNSPolicy: "",
								Images: kubermaticv1.ImageSources{HTTP: &kubermaticv1.HTTPSource{
									ImageCloning:     kubermaticv1.ImageCloning{Enable: true},
									OperatingSystems: map[types.OperatingSystem]kubermaticv1.OSVersions{"ubuntu": map[string]string{"18.04": "http://upstream.com/ubuntu.img"}},
								}},
							},
						},
					}
				}),
			),
			ExistingKubevirtK8sObjects: []ctrlruntimeclient.Object{&standardClonedDV},
			ExistingAPIUser:            *test.GenDefaultAPIUser(),
		},
		{
			Name:               "scenario 4: invalid response without datacenter name",
			HTTPRequestMethod:  http.MethodGet,
			HTTPRequestURL:     "/api/v2/providers/kubevirt/vmimages",
			HTTPRequestHeaders: []KeyValue{{Key: "Kubeconfig", Value: fakeKvConfig}},
			ExpectedResponse:   `{"error":{"code":400,"message":"invalid request"}}`,
			HTTPStatus:         400,
			ExistingKubermaticObjects: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(func(seed *kubermaticv1.Seed) {
					seed.Spec.Datacenters["kubevirt-dc"] = kubermaticv1.Datacenter{
						Spec: kubermaticv1.DatacenterSpec{
							Kubevirt: &kubermaticv1.DatacenterSpecKubevirt{
								DNSPolicy: "",
								Images: kubermaticv1.ImageSources{HTTP: &kubermaticv1.HTTPSource{
									OperatingSystems: map[types.OperatingSystem]kubermaticv1.OSVersions{"ubuntu": map[string]string{"18.04": "http://upstream.com/ubuntu.img"}},
								}},
							},
						},
					}
				}),
			),
			ExistingKubevirtK8sObjects: []ctrlruntimeclient.Object{&customImage},
			ExistingAPIUser:            *test.GenDefaultAPIUser(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			setFakeNewKubeVirtClient(append(tc.ExistingKubevirtObjects, tc.ExistingKubevirtK8sObjects...))
			req := httptest.NewRequest(tc.HTTPRequestMethod, tc.HTTPRequestURL, nil)
			for _, h := range tc.HTTPRequestHeaders {
				req.Header.Add(h.Key, h.Value)
			}
			res := httptest.NewRecorder()
			ep, err := test.CreateTestEndpoint(tc.ExistingAPIUser, nil, tc.ExistingKubermaticObjects, nil, hack.NewTestRouting)
			if err != nil {
				t.Fatalf("failed to create test endpoint: %v", err)
			}

			// act
			ep.ServeHTTP(res, req)

			// validate
			if res.Code != tc.HTTPStatus {
				t.Fatalf("Expected HTTP status code %d, got %d: %s", tc.HTTPStatus, res.Code, res.Body.String())
			}
			test.CompareWithResult(t, res, tc.ExpectedResponse)
		})
	}
}

func TestListVMImagesEndpointNoCredentials(t *testing.T) {
	testCases := []struct {
		Name                       string
		HTTPRequestMethod          string
		HTTPRequestURL             string
		HTTPRequestHeaders         []KeyValue
		ExpectedResponse           string
		HTTPStatus                 int
		ExistingKubermaticObjects  []ctrlruntimeclient.Object
		ExistingKubevirtObjects    []ctrlruntimeclient.Object
		ExistingKubevirtK8sObjects []ctrlruntimeclient.Object
		ExistingK8sObjects         []ctrlruntimeclient.Object
		ExistingAPIUser            apiv1.User
	}{
		{
			Name:              "scenario 1: list images with standard-images from seed and custom-images-by-admin - provided kubevirt kubeconfig from cluster",
			HTTPRequestMethod: http.MethodGet,
			HTTPRequestURL:    fmt.Sprintf("/api/v2/projects/%s/clusters/%s/providers/kubevirt/vmimages", test.GenDefaultProject().Name, clusterId),
			ExpectedResponse:  `{"vmImages":{"custom":[{"source":"pvc","images":{"ubuntu":{"global-test":"kubevirt-images/test"}}}],"standard":[{"source":"http","images":{"ubuntu":{"18.04":"http://upstream.com/ubuntu.img"}}}]}}`,
			HTTPStatus:        200,
			ExistingKubermaticObjects: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(func(seed *kubermaticv1.Seed) {
					seed.Spec.Datacenters["kubevirt-dc"] = kubermaticv1.Datacenter{
						Spec: kubermaticv1.DatacenterSpec{
							Kubevirt: &kubermaticv1.DatacenterSpecKubevirt{
								Images: kubermaticv1.ImageSources{
									EnableCustomImages: true,
									HTTP: &kubermaticv1.HTTPSource{
										OperatingSystems: map[types.OperatingSystem]kubermaticv1.OSVersions{"ubuntu": map[string]string{"18.04": "http://upstream.com/ubuntu.img"}},
									}},
							},
						},
					}
				}),
				func() *kubermaticv1.Cluster {
					cluster := test.GenCluster(clusterId, clusterName, test.GenDefaultProject().Name, time.Date(2013, 02, 03, 19, 54, 0, 0, time.UTC))
					cluster.Spec.Cloud = kubermaticv1.CloudSpec{
						DatacenterName: "kubevirt-dc",
						Kubevirt: &kubermaticv1.KubevirtCloudSpec{
							Kubeconfig: fakeKvConfig,
						},
					}
					return cluster
				}(),
			),
			ExistingKubevirtK8sObjects: []ctrlruntimeclient.Object{&customImage},
			ExistingAPIUser:            *test.GenDefaultAPIUser(),
		},
		{
			Name:              "scenario 2: list images image-source from seed and custom-images-by-admin - kubevirt kubeconfig from credential reference (secret)",
			HTTPRequestMethod: http.MethodGet,
			HTTPRequestURL:    fmt.Sprintf("/api/v2/projects/%s/clusters/%s/providers/kubevirt/vmimages", test.GenDefaultProject().Name, clusterId),
			ExpectedResponse:  `{"vmImages":{"custom":[{"source":"pvc","images":{"ubuntu":{"global-test":"kubevirt-images/test"}}}],"standard":[{"source":"http","images":{"ubuntu":{"18.04":"http://upstream.com/ubuntu.img"}}}]}}`,
			HTTPStatus:        200,
			ExistingKubermaticObjects: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(func(seed *kubermaticv1.Seed) {
					seed.Spec.Datacenters["kubevirt-dc"] = kubermaticv1.Datacenter{
						Spec: kubermaticv1.DatacenterSpec{
							Kubevirt: &kubermaticv1.DatacenterSpecKubevirt{
								Images: kubermaticv1.ImageSources{
									EnableCustomImages: true,
									HTTP: &kubermaticv1.HTTPSource{
										OperatingSystems: map[types.OperatingSystem]kubermaticv1.OSVersions{"ubuntu": map[string]string{"18.04": "http://upstream.com/ubuntu.img"}},
									}},
							},
						},
					}
				}),
				func() *kubermaticv1.Cluster {
					cluster := test.GenCluster(clusterId, clusterName, test.GenDefaultProject().Name, time.Date(2013, 02, 03, 19, 54, 0, 0, time.UTC))
					cluster.Spec.Cloud = kubermaticv1.CloudSpec{
						DatacenterName: "kubevirt-dc",
						Kubevirt: &kubermaticv1.KubevirtCloudSpec{
							CredentialsReference: &types.GlobalSecretKeySelector{
								ObjectReference: corev1.ObjectReference{Name: credentialref, Namespace: credentialns},
							},
						},
					}
					return cluster
				}(),
			),
			ExistingKubevirtK8sObjects: []ctrlruntimeclient.Object{&customImage},
			ExistingK8sObjects:         []ctrlruntimeclient.Object{NewCredentialSecret(credentialref, credentialns)},
			ExistingAPIUser:            *test.GenDefaultAPIUser(),
		},
		{
			Name:              "scenario 3: list images with standard-images from seed, custom-images-by-admin and custom-images-by-user - provided kubevirt kubeconfig from cluster",
			HTTPRequestMethod: http.MethodGet,
			HTTPRequestURL:    fmt.Sprintf("/api/v2/projects/%s/clusters/%s/providers/kubevirt/vmimages", test.GenDefaultProject().Name, clusterId),
			ExpectedResponse:  `{"vmImages":{"custom":[{"source":"pvc","images":{"ubuntu":{"custom-disk":"cluster-keen-snyder/custom-disk","global-test":"kubevirt-images/test"}}}],"standard":[{"source":"http","images":{"ubuntu":{"18.04":"http://upstream.com/ubuntu.img"}}}]}}`,
			HTTPStatus:        200,
			ExistingKubermaticObjects: test.GenDefaultKubermaticObjects(
				test.GenTestSeed(func(seed *kubermaticv1.Seed) {
					seed.Spec.Datacenters["kubevirt-dc"] = kubermaticv1.Datacenter{
						Spec: kubermaticv1.DatacenterSpec{
							Kubevirt: &kubermaticv1.DatacenterSpecKubevirt{
								Images: kubermaticv1.ImageSources{
									EnableCustomImages: true,
									HTTP: &kubermaticv1.HTTPSource{
										OperatingSystems: map[types.OperatingSystem]kubermaticv1.OSVersions{"ubuntu": map[string]string{"18.04": "http://upstream.com/ubuntu.img"}},
									}},
							},
						},
					}
				}),
				func() *kubermaticv1.Cluster {
					cluster := test.GenCluster(clusterId, clusterName, test.GenDefaultProject().Name, time.Date(2013, 02, 03, 19, 54, 0, 0, time.UTC))
					cluster.Spec.Cloud = kubermaticv1.CloudSpec{
						DatacenterName: "kubevirt-dc",
						Kubevirt: &kubermaticv1.KubevirtCloudSpec{
							Kubeconfig: fakeKvConfig,
							PreAllocatedDataVolumes: []kubermaticv1.PreAllocatedDataVolume{
								{
									Name:         "custom-disk",
									Annotations:  map[string]string{"cdi.kubevirt.io/os-type": "ubuntu"},
									URL:          "http:test.com/ubuntu.img",
									Size:         "10Gi",
									StorageClass: "test-sc",
								},
							},
						},
					}
					return cluster
				}(),
			),
			ExistingKubevirtK8sObjects: []ctrlruntimeclient.Object{&customImage},
			ExistingAPIUser:            *test.GenDefaultAPIUser(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			setFakeNewKubeVirtClient(append(tc.ExistingKubevirtObjects, tc.ExistingKubevirtK8sObjects...))
			req := httptest.NewRequest(tc.HTTPRequestMethod, tc.HTTPRequestURL, nil)
			for _, h := range tc.HTTPRequestHeaders {
				req.Header.Add(h.Key, h.Value)
			}
			res := httptest.NewRecorder()
			ep, err := test.CreateTestEndpoint(tc.ExistingAPIUser, tc.ExistingK8sObjects, tc.ExistingKubermaticObjects, nil, hack.NewTestRouting)
			if err != nil {
				t.Fatalf("failed to create test endpoint: %v", err)
			}

			// act
			ep.ServeHTTP(res, req)

			// validate
			if res.Code != tc.HTTPStatus {
				t.Fatalf("Expected HTTP status code %d, got %d: %s", tc.HTTPStatus, res.Code, res.Body.String())
			}
			test.CompareWithResult(t, res, tc.ExpectedResponse)
		})
	}
}
