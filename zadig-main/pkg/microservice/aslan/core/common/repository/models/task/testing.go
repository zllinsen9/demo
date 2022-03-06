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

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/types"
)

type Testing struct {
	TaskType     config.TaskType   `bson:"type"                            json:"type"`
	Enabled      bool              `bson:"enabled"                         json:"enabled"`
	TaskStatus   config.Status     `bson:"status"                          json:"status"`
	TestName     string            `bson:"test_name"                       json:"test_name"`
	Timeout      int               `bson:"timeout,omitempty"               json:"timeout,omitempty"`
	Error        string            `bson:"error,omitempty"                 json:"error,omitempty"`
	StartTime    int64             `bson:"start_time,omitempty"            json:"start_time,omitempty"`
	EndTime      int64             `bson:"end_time,omitempty"              json:"end_time,omitempty"`
	JobCtx       JobCtx            `bson:"job_ctx"                         json:"job_ctx"`
	InstallItems []*models.Item    `bson:"install_items"                   json:"install_items"`
	InstallCtx   []*models.Install `bson:"-"                               json:"install_ctx"`
	BuildOS      string            `bson:"build_os"                        json:"build_os,omitempty"`
	ImageFrom    string            `bson:"image_from"                      json:"image_from,omitempty"`
	ImageID      string            `bson:"image_id"                        json:"image_id"`
	// ResReq defines job requested resources
	ResReq         setting.Request             `bson:"res_req"                         json:"res_req"`
	ResReqSpec     setting.RequestSpec         `bson:"res_req_spec"                    json:"res_req_spec"`
	LogFile        string                      `bson:"log_file"                        json:"log_file"`
	TestModuleName string                      `bson:"test_module_name"                json:"test_module_name"`
	ReportReady    bool                        `bson:"report_ready"                    json:"report_ready"`
	IsRestart      bool                        `bson:"is_restart"                      json:"is_restart"`
	Registries     []*models.RegistryNamespace `bson:"-"                               json:"registries"`
	ClusterID      string                      `bson:"cluster_id,omitempty"            json:"cluster_id,omitempty"`
	Namespace      string                      `bson:"namespace"                       json:"namespace"`

	// New since V1.10.0.
	Cache        types.Cache        `bson:"cache"               json:"cache"`
	CacheEnable  bool               `bson:"cache_enable"        json:"cache_enable"`
	CacheDirType types.CacheDirType `bson:"cache_dir_type"      json:"cache_dir_type"`
	CacheUserDir string             `bson:"cache_user_dir"      json:"cache_user_dir"`
}

func (t *Testing) ToSubTask() (map[string]interface{}, error) {
	var task map[string]interface{}
	if err := IToi(t, &task); err != nil {
		return nil, fmt.Errorf("convert TestingTaskV2 to interface error: %v", err)
	}
	return task, nil
}
