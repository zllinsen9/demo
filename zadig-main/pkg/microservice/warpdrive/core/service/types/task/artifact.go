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

package task

import (
	"fmt"

	"github.com/koderover/zadig/pkg/microservice/warpdrive/config"
)

type Artifact struct {
	TaskType   config.TaskType `bson:"type"                          json:"type"`
	Enabled    bool            `bson:"enabled"                       json:"enabled"`
	TaskStatus config.Status   `bson:"status"                        json:"status"`
	Name       string          `bson:"name"                          json:"name"`
	Image      string          `bson:"image"                         json:"image"`
	RegistryID string          `bson:"registry_id"                   json:"registry_id"`
	Timeout    int             `bson:"timeout,omitempty"             json:"timeout,omitempty"`
	Error      string          `bson:"error,omitempty"               json:"error,omitempty"`
	StartTime  int64           `bson:"start_time,omitempty"          json:"start_time,omitempty"`
	EndTime    int64           `bson:"end_time,omitempty"            json:"end_time,omitempty"`
	LogFile    string          `bson:"log_file"                      json:"log_file"`
	IsRestart  bool            `bson:"is_restart"                    json:"is_restart"`
}

func (d *Artifact) ToSubTask() (map[string]interface{}, error) {
	var task map[string]interface{}
	if err := IToi(d, &task); err != nil {
		return nil, fmt.Errorf("convert ArtifactTask to interface error: %v", err)
	}
	return task, nil
}
