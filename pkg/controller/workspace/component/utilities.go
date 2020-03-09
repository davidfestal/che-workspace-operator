//
// Copyright (c) 2019-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package component

import (
	"strings"

	config "github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/config"
	"github.com/eclipse/che-plugin-broker/model"
	corev1 "k8s.io/api/core/v1"

	workspaceApi "github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	. "github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/model"
)

func emptyIfNil(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

func createVolumeMounts(wkspCtx WorkspaceContext, mountSources *bool, devfileVolumes []workspaceApi.Volume, pluginVolumes []model.Volume) []corev1.VolumeMount {
	volumeName := config.ControllerCfg.GetWorkspacePVCName()

	var volumeMounts []corev1.VolumeMount
	for _, volDef := range devfileVolumes {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			MountPath: volDef.ContainerPath,
			Name:      volumeName,
			SubPath:   wkspCtx.WorkspaceId + "/" + volDef.Name + "/",
		})
	}
	for _, volDef := range pluginVolumes {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			MountPath: volDef.MountPath,
			Name:      volumeName,
			SubPath:   wkspCtx.WorkspaceId + "/" + volDef.Name + "/",
		})
	}

	if mountSources != nil && *mountSources {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			MountPath: DefaultProjectsSourcesRoot,
			Name:      volumeName,
			SubPath:   wkspCtx.WorkspaceId + DefaultProjectsSourcesRoot,
		})
	}

	return volumeMounts
}

func interpolate(someString string, wkspCtx WorkspaceContext) string {
	for _, envVar := range commonEnvironmentVariables(wkspCtx) {
		someString = strings.ReplaceAll(someString, "${"+envVar.Name+"}", envVar.Value)
	}
	return someString
}
