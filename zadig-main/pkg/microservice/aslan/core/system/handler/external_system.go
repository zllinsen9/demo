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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/system/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	"github.com/koderover/zadig/pkg/tool/log"
)

func CreateExternalSystem(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	args := new(service.ExternalSystemDetail)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("CreateExternalSystem GetRawData err : %s", err)
	}
	if err = json.Unmarshal(data, args); err != nil {
		log.Errorf("CreateExternalSystem Unmarshal err : %s", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "新增", "系统配置-外部系统", fmt.Sprintf("name:%s server:%s", args.Name, args.Server), string(data), ctx.Logger)

	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if args.Name == "" || args.Server == "" {
		ctx.Err = errors.New("name and server must be provided")
		return
	}

	ctx.Err = service.CreateExternalSystem(args, ctx.Logger)
}

type listQuery struct {
	PageSize int64 `json:"page_size" form:"page_size,default=100"`
	PageNum  int64 `json:"page_num"  form:"page_num,default=1"`
}

type listExternalResp struct {
	SystemList []*service.ExternalSystemDetail `json:"external_system"`
	Total      int64                           `json:"total"`
}

func ListExternalSystem(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	// Query Verification
	args := &listQuery{}
	if err := c.ShouldBindQuery(args); err != nil {
		ctx.Err = err
		return
	}

	systemList, length, err := service.ListExternalSystem(args.PageNum, args.PageSize, ctx.Logger)
	if err == nil {
		ctx.Resp = &listExternalResp{
			SystemList: systemList,
			Total:      length,
		}
		return
	}
	ctx.Err = err
}

func GetExternalSystemDetail(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	ctx.Resp, ctx.Err = service.GetExternalSystemDetail(c.Param("id"), ctx.Logger)
}

func UpdateExternalSystem(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	req := new(service.ExternalSystemDetail)
	data, err := c.GetRawData()
	if err != nil {
		log.Errorf("UpdateExternalSystem GetRawData err : %s", err)
	}
	if err = json.Unmarshal(data, req); err != nil {
		log.Errorf("UpdateExternalSystem Unmarshal err : %s", err)
	}
	internalhandler.InsertOperationLog(c, ctx.UserName, "", "更新", "系统配置-外部系统", fmt.Sprintf("name:%s server:%s", req.Name, req.Server), string(data), ctx.Logger)

	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data))

	if req.Name == "" || req.Server == "" {
		ctx.Err = errors.New("name and server must be provided")
		return
	}

	ctx.Err = service.UpdateExternalSystem(c.Param("id"), req, ctx.Logger)
}

func DeleteExternalSystem(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	internalhandler.InsertOperationLog(c, ctx.UserName, "", "删除", "系统配置-外部系统", fmt.Sprintf("id:%s", c.Param("id")), "", ctx.Logger)

	ctx.Err = service.DeleteExternalSystem(c.Param("id"), ctx.Logger)
}
