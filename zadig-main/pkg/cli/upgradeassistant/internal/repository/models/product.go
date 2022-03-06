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

package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
)

type ProductAuthType string
type ProductPermission string

// Vars do not save, only input parameters
type Product struct {
	ID              primitive.ObjectID            `bson:"_id,omitempty"             json:"id,omitempty"`
	ProductName     string                        `bson:"product_name"              json:"product_name"`
	CreateTime      int64                         `bson:"create_time"               json:"create_time"`
	UpdateTime      int64                         `bson:"update_time"               json:"update_time"`
	Namespace       string                        `bson:"namespace,omitempty"       json:"namespace,omitempty"`
	Status          string                        `bson:"status"                    json:"status"`
	Revision        int64                         `bson:"revision"                  json:"revision"`
	Enabled         bool                          `bson:"enabled"                   json:"enabled"`
	EnvName         string                        `bson:"env_name"                  json:"env_name"`
	UpdateBy        string                        `bson:"update_by"                 json:"update_by"`
	Auth            []*ProductAuth                `bson:"auth"                      json:"auth"`
	Visibility      string                        `bson:"-"                         json:"visibility"`
	Services        [][]*ProductService           `bson:"services"                  json:"services"`
	Render          *RenderInfo                   `bson:"render"                    json:"render"`
	Error           string                        `bson:"error"                     json:"error"`
	Vars            []*templatemodels.RenderKV    `bson:"vars,omitempty"            json:"vars,omitempty"`
	ChartInfos      []*templatemodels.RenderChart `bson:"-"                         json:"chart_infos,omitempty"`
	IsPublic        bool                          `bson:"is_public"                 json:"isPublic"`
	RoleIDs         []int64                       `bson:"role_ids"                  json:"roleIds"`
	ClusterID       string                        `bson:"cluster_id,omitempty"      json:"cluster_id,omitempty"`
	RecycleDay      int                           `bson:"recycle_day"               json:"recycle_day"`
	Source          string                        `bson:"source"                    json:"source"`
	IsOpenSource    bool                          `bson:"is_opensource"             json:"is_opensource"`
	RegistryID      string                        `bson:"registry_id"               json:"registry_id"`
	IsForkedProduct bool                          `bson:"-"                         json:"-"`
}

type RenderInfo struct {
	Name        string `bson:"name"                     json:"name"`
	Revision    int64  `bson:"revision"                 json:"revision"`
	ProductTmpl string `bson:"product_tmpl"             json:"product_tmpl"`
	Description string `bson:"description"              json:"description"`
}

type ProductAuth struct {
	Type        ProductAuthType     `bson:"type"          json:"type"`
	Name        string              `bson:"name"          json:"name"`
	Permissions []ProductPermission `bson:"permissions"   json:"permissions"`
}

// ImagePathSpec paths in yaml used to parse image
type ImagePathSpec struct {
	Repo  string `bson:"repo,omitempty"           json:"repo,omitempty"`
	Image string `bson:"image,omitempty"           json:"image,omitempty"`
	Tag   string `bson:"tag,omitempty"           json:"tag,omitempty"`
}

type Container struct {
	Name      string         `bson:"name"           json:"name"`
	Image     string         `bson:"image"          json:"image"`
	ImagePath *ImagePathSpec `bson:"image_path,omitempty"          json:"imagePath,omitempty"`
}

type EnvConfig struct {
	EnvName string   `bson:"env_name,omitempty" json:"env_name"`
	HostIDs []string `bson:"host_ids,omitempty" json:"host_ids"`
	Labels  []string `bson:"labels,omitempty"   json:"labels"`
}

type ProductService struct {
	ServiceName string       `bson:"service_name"               json:"service_name"`
	ProductName string       `bson:"product_name"               json:"product_name"`
	Type        string       `bson:"type"                       json:"type"`
	Revision    int64        `bson:"revision"                   json:"revision"`
	Containers  []*Container `bson:"containers"                 json:"containers,omitempty"`
	Render      *RenderInfo  `bson:"render,omitempty"           json:"render,omitempty"` // Record the render information of each service to facilitate the update of a single service
	EnvConfigs  []*EnvConfig `bson:"-"                          json:"env_configs,omitempty"`
}

type ServiceConfig struct {
	ConfigName string `bson:"config_name"           json:"config_name"`
	Revision   int64  `bson:"revision"              json:"revision"`
}

func (Product) TableName() string {
	return "product"
}

func (p *Product) GetNamespace() string {
	return p.ProductName + "-env-" + p.EnvName
}

func (p *Product) GetGroupServiceNames() [][]string {
	var resp [][]string
	for _, group := range p.Services {
		services := make([]string, 0, len(group))
		for _, service := range group {
			services = append(services, service.ServiceName)
		}
		resp = append(resp, services)
	}
	return resp
}

func (p *Product) GetServiceMap() map[string]*ProductService {
	ret := make(map[string]*ProductService)
	for _, group := range p.Services {
		for _, svc := range group {
			ret[svc.ServiceName] = svc
		}
	}

	return ret
}
