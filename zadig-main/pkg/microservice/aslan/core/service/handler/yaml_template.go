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

package handler

import (
	"github.com/gin-gonic/gin"

	svcservice "github.com/koderover/zadig/pkg/microservice/aslan/core/service/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

type LoadServiceFromYamlTemplateReq struct {
	ServiceName string                 `json:"service_name"`
	ProjectName string                 `json:"project_name"`
	TemplateID  string                 `json:"template_id"`
	Variables   []*svcservice.Variable `json:"variables"`
}

func LoadServiceFromYamlTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := new(LoadServiceFromYamlTemplateReq)

	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = svcservice.LoadServiceFromYamlTemplate(ctx.UserName, req.ProjectName, req.ServiceName, req.TemplateID, req.Variables, ctx.Logger)
}

func ReloadServiceFromYamlTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := new(LoadServiceFromYamlTemplateReq)
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = svcservice.ReloadServiceFromYamlTemplate(ctx.UserName, req.ProjectName, req.ServiceName, req.Variables, ctx.Logger)
}
