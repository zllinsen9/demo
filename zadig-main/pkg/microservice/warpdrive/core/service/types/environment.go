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

package types

import (
	"github.com/koderover/zadig/pkg/microservice/warpdrive/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types/task"
)

type Product struct {
	//ID          bson.ObjectId         `bson:"_id,omitempty"             json:"id"`
	ProductName string           `bson:"product_name"              json:"product_name"`
	CreateTime  int64            `bson:"create_time"               json:"create_time"`
	UpdateTime  int64            `bson:"update_time"               json:"update_time"`
	Namespace   string           `bson:"namespace,omitempty"       json:"namespace,omitempty"`
	Status      string           `bson:"status"                    json:"status"`
	Revision    int64            `bson:"revision"                  json:"revision"`
	Enabled     bool             `bson:"enabled"                   json:"enabled"`
	EnvName     string           `bson:"env_name"                  json:"env_name"`
	UpdateBy    string           `bson:"update_by"                 json:"update_by"`
	Auth        []*ProductAuth   `bson:"auth"                      json:"auth"`
	Visibility  string           `bson:"-"                         json:"visibility"`
	Services    [][]*Service     `bson:"services"                  json:"services"`
	Render      *task.RenderInfo `bson:"render"                    json:"render"`
	Error       string           `bson:"error"                     json:"error"`
	Vars        []*RenderKV      `bson:"vars,omitempty"            json:"vars,omitempty"`
	ChartInfos  []*RenderChart   `bson:"-"                         json:"chart_infos,omitempty"`
	IsPublic    bool             `bson:"is_public"                 json:"isPublic"`
	RoleIDs     []int64          `bson:"role_ids"                  json:"roleIds"`
	ClusterID   string           `bson:"cluster_id,omitempty"      json:"cluster_id,omitempty"`
	RecycleDay  int              `bson:"recycle_day"               json:"recycle_day"`
	Source      string           `bson:"source"                    json:"source"`

	// used for cache
	//KubeClient kubecli.Client `bson:"-" json:"-"`
	//HelmClient helmclient.Client `bson:"-" json:"-"`
	// TODO: temp flag
	IsForkedProduct bool `bson:"-" json:"-"`
}

type RenderKV struct {
	Key      string   `bson:"key"               json:"key"`
	Value    string   `bson:"value"             json:"value"`
	Alias    string   `bson:"alias"             json:"alias"`
	State    string   `bson:"state"             json:"state"`
	Services []string `bson:"services"          json:"services"`
}

type GitRepoConfig struct {
	CodehostID int    `bson:"codehost_id,omitempty"`
	Owner      string `bson:"owner,omitempty"`
	Repo       string `bson:"repo,omitempty"`
	Branch     string `bson:"branch,omitempty"`
}

type CustomYaml struct {
	YamlSource    string         `bson:"yaml_source,omitempty"     json:"yaml_source,omitempty"`
	YamlContent   string         `bson:"yaml_content,omitempty"    json:"yaml_content,omitempty"`
	GitRepoConfig *GitRepoConfig `bson:"git_repo_config,omitempty"   json:"git_repo_config,omitempty"`
	ValuesPaths   []string       `bson:"values_paths,omitempty"    json:"values_paths,omitempty"`
}

type RenderChart struct {
	ServiceName    string      `bson:"service_name,omitempty"    json:"service_name,omitempty"`
	ChartVersion   string      `bson:"chart_version,omitempty"   json:"chart_version,omitempty"`
	ValuesYaml     string      `bson:"values_yaml,omitempty"     json:"values_yaml,omitempty"`
	OverrideYaml   *CustomYaml `bson:"override_yaml,omitempty"   json:"override_yaml,omitempty"`
	OverrideValues string      `bson:"override_values,omitempty"   json:"override_values,omitempty"`
}

type ProductAuth struct {
	Type        config.ProductAuthType     `bson:"type"          json:"type"`
	Name        string                     `bson:"name"          json:"name"`
	Permissions []config.ProductPermission `bson:"permissions"   json:"permissions"`
}

type Service struct {
	ServiceName string           `bson:"service_name"               json:"service_name"`
	Type        string           `bson:"type"                       json:"type"`
	Revision    int64            `bson:"revision"                   json:"revision"`
	Containers  []*Container     `bson:"containers"                 json:"containers,omitempty"`
	Configs     []*Config        `bson:"configs,omitempty"          json:"configs,omitempty"`
	Render      *task.RenderInfo `bson:"render,omitempty"           json:"render,omitempty"` // 记录每个服务render信息 便于更新单个服务
	EnvConfigs  []*EnvConfig     `bson:"-"                          json:"env_configs,omitempty"`
}

// Config ...
type Config struct {
	ConfigName string `bson:"config_name"           json:"config_name"`
	Revision   int64  `bson:"revision"              json:"revision"`
}

func (rc *RenderChart) GetOverrideYaml() string {
	if rc.OverrideYaml == nil {
		return ""
	}
	return rc.OverrideYaml.YamlContent
}

func (p *Product) GetServiceMap() map[string]*Service {
	ret := make(map[string]*Service)
	for _, group := range p.Services {
		for _, svc := range group {
			ret[svc.ServiceName] = svc
		}
	}
	return ret
}
