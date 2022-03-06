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
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"

	templateservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/template"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func CreateYamlTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := &templateservice.YamlTemplate{}

	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = templateservice.CreateYamlTemplate(req, ctx.Logger)
}

func UpdateYamlTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := &templateservice.YamlTemplate{}

	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}

	ctx.Err = templateservice.UpdateYamlTemplate(c.Param("id"), req, ctx.Logger)
}

type listYamlQuery struct {
	PageSize int `json:"page_size" form:"page_size,default=100"`
	PageNum  int `json:"page_num"  form:"page_num,default=1"`
}

type ListYamlResp struct {
	SystemVariables []*commonmodels.ChartVariable     `json:"system_variables"`
	YamlTemplates   []*templateservice.YamlListObject `json:"yaml_template"`
	Total           int                               `json:"total"`
}

func ListYamlTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// Query Verification
	args := &listYamlQuery{}
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = err
		return
	}

	systemVariables := templateservice.GetSystemDefaultVariables()
	YamlTemplateList, total, err := templateservice.ListYamlTemplate(args.PageNum, args.PageSize, ctx.Logger)
	resp := ListYamlResp{
		SystemVariables: systemVariables,
		YamlTemplates:   YamlTemplateList,
		Total:           total,
	}
	ctx.Resp = resp
	ctx.Err = err
}

func GetYamlTemplateDetail(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = templateservice.GetYamlTemplateDetail(c.Param("id"), ctx.Logger)
}

func DeleteYamlTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Err = templateservice.DeleteYamlTemplate(c.Param("id"), ctx.Logger)
}

func GetYamlTemplateReference(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = templateservice.GetYamlTemplateReference(c.Param("id"), ctx.Logger)
}

type getYamlTemplateVariablesReq struct {
	Content string `json:"content"`
}

func GetYamlTemplateVariables(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := &getYamlTemplateVariablesReq{}
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}

	ctx.Resp, ctx.Err = templateservice.GetYamlVariables(req.Content, ctx.Logger)
}
