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

	"github.com/koderover/zadig/pkg/microservice/policy/core/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func GetUserPermission(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	projectName := c.Query("projectName")

	ctx.Resp, ctx.Err = service.GetPermission(projectName, c.Param("uid"), ctx.Logger)
}

type GetUserResourcesPermissionReq struct {
	ProjectName  string   `json:"project_name"      form:"project_name"`
	Uid          string   `json:"uid"               form:"uid"`
	Resources    []string `json:"resources"         form:"resources"`
	ResourceType string   `json:"resource_type"     form:"resource_type"`
}

func GetUserResourcesPermission(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := new(GetUserResourcesPermissionReq)

	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = e.ErrInvalidParam.AddErr(err)
		return
	}
	ctx.Resp, ctx.Err = service.GetResourcesPermission(req.Uid, req.ProjectName, req.ResourceType, req.Resources, ctx.Logger)
}
