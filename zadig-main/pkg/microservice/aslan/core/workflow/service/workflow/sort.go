/*
Copyright 2021 The KodeRover Authors.

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

package workflow

import (
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/base"
)

type ByTaskKind []map[string]interface{}

func (a ByTaskKind) Len() int      { return len(a) }
func (a ByTaskKind) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByTaskKind) Less(i, j int) bool {
	iPreview, _ := base.ToPreview(a[i])
	jPreview, _ := base.ToPreview(a[j])
	return SubtaskOrder[iPreview.TaskType] < SubtaskOrder[jPreview.TaskType]
}

var SubtaskOrder = map[config.TaskType]int{
	config.TaskType("jira"):            1,
	config.TaskType("pipeline"):        2,
	config.TaskType("buildv2"):         3,
	config.TaskType("buildv3"):         4,
	config.TaskType("jenkins_build"):   5,
	config.TaskType("docker_build"):    6,
	config.TaskType("archive"):         7,
	config.TaskType("artifact"):        8,
	config.TaskType("artifact_deploy"): 9,
	config.TaskType("deploy"):          10,
	config.TaskType("testingv2"):       11,
	config.TaskType("security"):        12,
	config.TaskType("distribute2kodo"): 13,
	config.TaskType("release_image"):   14,
	config.TaskType("reset_image"):     15,
	config.TaskType("trigger"):         16,
	config.TaskType("extension"):       17,
}

type ByStageKind []*commonmodels.Stage

func (a ByStageKind) Len() int      { return len(a) }
func (a ByStageKind) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByStageKind) Less(i, j int) bool {
	iPreview := a[i].TaskType
	jPreview := a[j].TaskType
	return SubtaskOrder[iPreview] < SubtaskOrder[jPreview]
}
