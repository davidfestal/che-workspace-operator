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
	"regexp"
	"strconv"
	"strings"

	"github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/server"
	"github.com/eclipse/che-plugin-broker/model"

	workspaceApi "github.com/che-incubator/che-workspace-operator/pkg/apis/workspace/v1alpha1"
	modelutils "github.com/che-incubator/che-workspace-operator/pkg/controller/modelutils/k8s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/config"
	. "github.com/che-incubator/che-workspace-operator/pkg/controller/workspace/model"
)

func setupDockerimageComponent(wkspCtx WorkspaceContext, commands []workspaceApi.CommandSpec, component *workspaceApi.ComponentSpec) (*ComponentInstanceStatus, error) {
	componentInstanceStatus := &ComponentInstanceStatus{
		Containers:                 map[string]ContainerDescription{},
		Endpoints:                  []workspaceApi.Endpoint{},
		ContributedRuntimeCommands: []CheWorkspaceCommand{},
	}

	workspacePodContributions := &workspaceApi.WorkspacePodContributions{}
	componentInstanceStatus.WorkspacePodContributions = workspacePodContributions
	componentInstanceStatus.ExternalObjects = []runtime.Object{}

	var containerName string
	if component.Alias == "" {
		re := regexp.MustCompile(`[^-a-zA-Z0-9_]`)
		containerName = re.ReplaceAllString(*component.Image, "-")
	} else {
		containerName = component.Alias
	}

	var exposedPorts []int = modelutils.EndpointPortsToInts(component.Endpoints)

	var limitOrDefault string

	if *component.MemoryLimit == "" {
		limitOrDefault = "128M"
	} else {
		limitOrDefault = *component.MemoryLimit
	}

	limit, err := resource.ParseQuantity(limitOrDefault)
	if err != nil {
		return nil, err
	}

	volumeMounts := createVolumeMounts(wkspCtx, component.MountSources, component.Volumes, []model.Volume{})

	var envVars []corev1.EnvVar
	for _, envVarDef := range component.Env {
		envVars = append(envVars, corev1.EnvVar{
			Name:  envVarDef.Name,
			Value: strings.ReplaceAll(envVarDef.Value, "$(CHE_PROJECTS_ROOT)", DefaultProjectsSourcesRoot),
		})
	}
	envVars = append(envVars, corev1.EnvVar{
		Name:  "CHE_MACHINE_NAME",
		Value: containerName,
	})
	container := corev1.Container{
		Name:            containerName,
		Image:           *component.Image,
		ImagePullPolicy: corev1.PullPolicy(ControllerCfg.GetSidecarPullPolicy()),
		Ports:           modelutils.BuildContainerPorts(exposedPorts, corev1.ProtocolTCP),
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				"memory": limit,
			},
			Requests: corev1.ResourceList{
				"memory": limit,
			},
		},
		VolumeMounts: volumeMounts,
		Env:          append(envVars, commonEnvironmentVariables(wkspCtx)...),
	}
	if component.Command != nil {
		container.Command = *component.Command
	}
	if component.Args != nil {
		container.Args = *component.Args
	}

	workspacePodContributions.Containers = append(workspacePodContributions.Containers, container)

	for _, service := range createK8sServicesForContainers(wkspCtx, containerName, exposedPorts) {
		componentInstanceStatus.ExternalObjects = append(componentInstanceStatus.ExternalObjects, &service)
	}

	componentInstanceStatus.Endpoints = component.Endpoints

	containerAttributes := map[string]string{}
	if limitAsInt64, canBeConverted := limit.AsInt64(); canBeConverted {
		containerAttributes[server.MEMORY_LIMIT_ATTRIBUTE] = strconv.FormatInt(limitAsInt64, 10)
		containerAttributes[server.MEMORY_REQUEST_ATTRIBUTE] = strconv.FormatInt(limitAsInt64, 10)
	}
	containerAttributes[server.CONTAINER_SOURCE_ATTRIBUTE] = server.RECIPE_CONTAINER_SOURCE
	componentInstanceStatus.Containers[containerName] = ContainerDescription{
		Attributes: containerAttributes,
		Ports:      exposedPorts,
	}

	for _, command := range commands {
		if len(command.Actions) == 0 {
			continue
		}
		action := command.Actions[0]
		if component.Alias == "" ||
			action.Component == nil ||
			*action.Component != component.Alias {
			continue
		}
		attributes := map[string]string{
			server.COMMAND_WORKING_DIRECTORY_ATTRIBUTE:        interpolate(emptyIfNil(action.Workdir), wkspCtx),
			server.COMMAND_ACTION_REFERENCE_ATTRIBUTE:         emptyIfNil(action.Reference),
			server.COMMAND_ACTION_REFERENCE_CONTENT_ATTRIBUTE: emptyIfNil(action.ReferenceContent),
			server.COMMAND_MACHINE_NAME_ATTRIBUTE:             containerName,
			server.COMPONENT_ALIAS_COMMAND_ATTRIBUTE:          *action.Component,
		}
		for attrName, attrValue := range command.Attributes {
			attributes[attrName] = attrValue
		}
		componentInstanceStatus.ContributedRuntimeCommands = append(componentInstanceStatus.ContributedRuntimeCommands,
			CheWorkspaceCommand{
				Name:        command.Name,
				CommandLine: emptyIfNil(action.Command),
				Type:        action.Type,
				Attributes:  attributes,
			})
	}

	return componentInstanceStatus, nil
}
