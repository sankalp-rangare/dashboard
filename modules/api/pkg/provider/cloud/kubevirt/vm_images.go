/*
Copyright 2022 The Kubermatic Kubernetes Platform contributors.

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

package kubevirt

import (
	"context"
	"fmt"
	"github.com/kubermatic/machine-controller/pkg/providerconfig/types"
	v2 "k8c.io/dashboard/v2/pkg/api/v2"
	v1 "k8c.io/kubermatic/v2/pkg/apis/kubermatic/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	httpSource                 = "http"
	pvcSource                  = "pvc"
	osAnnotationForCustomDisk  = "cdi.kubevirt.io/os-type"
	customImageByAdminDVPrefix = "global"
)

func ListVMImages(ctx context.Context, source v1.ImageSources, client ctrlruntimeclient.Client, cluster *v1.Cluster) (*v2.VMImageList, error) {
	vmImageList := v2.VMImageList{VMImages: make(map[v2.VMImageCategory][]*v2.VMDiskImage)}

	isCloningEnabled := source.HTTP != nil && source.HTTP.ImageCloning.Enable

	existingDVList := cdiv1beta1.DataVolumeList{}
	if isCloningEnabled || source.EnableCustomImages {
		listOption := ctrlruntimeclient.ListOptions{
			Namespace: KubeVirtImagesNamespace,
		}
		if err := client.List(ctx, &existingDVList, &listOption); ctrlruntimeclient.IgnoreNotFound(err) != nil {
			return nil, err
		}
	}

	vmImageList.VMImages[v2.Standard] = listStandardVMImages(source, existingDVList)
	if source.EnableCustomImages {
		vmImageList.VMImages[v2.CustomImage] = listCustomVMImages(existingDVList, cluster)
	}

	return &vmImageList, nil
}

func listStandardVMImages(source v1.ImageSources, existingDVList cdiv1beta1.DataVolumeList) []*v2.VMDiskImage {
	var standardImages []*v2.VMDiskImage

	if source.HTTP != nil && source.HTTP.OperatingSystems != nil {
		standardHttpImages := v2.VMDiskImage{Source: httpSource, Images: source.HTTP.OperatingSystems}
		standardImages = append(standardImages, &standardHttpImages)

		// if cloning is enabled, return only standard DVs which exist on infra-cluster.
		if source.HTTP.ImageCloning.Enable {
			standardClonedImages := v2.VMDiskImage{Source: pvcSource, Images: make(map[types.OperatingSystem]v1.OSVersions)}
			existingDVSet := sets.String{}
			for _, edv := range existingDVList.Items {
				existingDVSet.Insert(edv.Name)
			}

			for os, versionMap := range standardHttpImages.Images {
				for version, _ := range versionMap {
					standardDVName := fmt.Sprintf("%s-%s", os, version)
					if existingDVSet.Has(standardDVName) {
						if _, exist := standardClonedImages.Images[os]; !exist {
							standardClonedImages.Images[os] = make(map[string]string)
						}
						standardClonedImages.Images[os][version] = fmt.Sprintf("%s/%s", KubeVirtImagesNamespace, standardDVName)
					}
				}
			}
			standardImages = append(standardImages, &standardClonedImages)
		}
	}
	return standardImages
}

func listCustomVMImages(existingDVList cdiv1beta1.DataVolumeList, cluster *v1.Cluster) []*v2.VMDiskImage {
	customImages := v2.VMDiskImage{Source: pvcSource, Images: make(map[types.OperatingSystem]v1.OSVersions)}

	// get custom-images by admin.
	for _, dv := range existingDVList.Items {
		if osType := getOSTypeFromAnnotation(dv.Annotations); osType != "" {
			if _, exist := customImages.Images[osType]; !exist {
				customImages.Images[osType] = make(map[string]string)
			}
			customImages.Images[osType][fmt.Sprintf("%s-%s", customImageByAdminDVPrefix, dv.Name)] = fmt.Sprintf("%s/%s", dv.Namespace, dv.Name)
		}
	}

	// get custom-images by user from cluster-object.
	if cluster != nil && cluster.Spec.Cloud.Kubevirt != nil {
		for _, dv := range cluster.Spec.Cloud.Kubevirt.PreAllocatedDataVolumes {
			if osType := getOSTypeFromAnnotation(dv.Annotations); osType != "" {
				if _, exist := customImages.Images[osType]; !exist {
					customImages.Images[osType] = make(map[string]string)
				}
				customImages.Images[osType][dv.Name] = fmt.Sprintf("%s/%s", cluster.Status.NamespaceName, dv.Name)
			}
		}
	}

	return []*v2.VMDiskImage{&customImages}
}

func getOSTypeFromAnnotation(annotation map[string]string) types.OperatingSystem {
	if os, exist := annotation[osAnnotationForCustomDisk]; exist {
		osType := types.OperatingSystem(os)
		if _, isValid := v1.SupportedKubeVirtOS[osType]; isValid {
			return osType
		}
	}
	return ""
}
