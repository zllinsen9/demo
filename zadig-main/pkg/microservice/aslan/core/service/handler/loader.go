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
	"strconv"

	"github.com/gin-gonic/gin"

	svcservice "github.com/koderover/zadig/pkg/microservice/aslan/core/service/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func PreloadServiceTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	codehostIDStr := c.Param("codehostId")

	codehostID, err := strconv.Atoi(codehostIDStr)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("cannot convert codehost id to int")
		return
	}

	repoName := c.Query("repoName")
	repoUUID := c.Query("repoUUID")
	if repoName == "" && repoUUID == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("repoName and repoUUID cannot be empty at the same time")
		return
	}

	branchName := c.Param("branchName")

	path := c.Query("path")
	isDir := c.Query("isDir") == "true"
	remoteName := c.Query("remoteName")
	repoOwner := c.Query("repoOwner")

	ctx.Resp, ctx.Err = svcservice.PreloadServiceFromCodeHost(codehostID, repoOwner, repoName, repoUUID, branchName, remoteName, path, isDir, ctx.Logger)
}

func LoadServiceTemplate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	codehostIDStr := c.Param("codehostId")

	codehostID, err := strconv.Atoi(codehostIDStr)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("cannot convert codehost id to string")
		return
	}

	repoName := c.Query("repoName")
	repoUUID := c.Query("repoUUID")
	if repoName == "" && repoUUID == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("repoName and repoUUID cannot be empty at the same time")
		return
	}

	branchName := c.Param("branchName")

	args := new(svcservice.LoadServiceReq)
	if err := c.BindJSON(args); err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("invalid LoadServiceReq json args")
		return
	}

	remoteName := c.Query("remoteName")
	repoOwner := c.Query("repoOwner")

	ctx.Err = svcservice.LoadServiceFromCodeHost(ctx.UserName, codehostID, repoOwner, repoName, repoUUID, branchName, remoteName, args, ctx.Logger)
}

// ValidateServiceUpdate ...
func ValidateServiceUpdate(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	codehostIDStr := c.Param("codehostId")

	codehostID, err := strconv.Atoi(codehostIDStr)
	if err != nil {
		ctx.Err = e.ErrInvalidParam.AddDesc("cannot convert codehost id to string")
		return
	}

	repoName := c.Query("repoName")
	repoUUID := c.Query("repoUUID")
	if repoName == "" && repoUUID == "" {
		ctx.Err = e.ErrInvalidParam.AddDesc("repoName and repoUUID cannot be empty at the same time")
		return
	}

	branchName := c.Param("branchName")

	path := c.Query("path")
	isDir := c.Query("isDir") == "true"
	remoteName := c.Query("remoteName")
	repoOwner := c.Query("repoOwner")
	serviceName := c.Query("serviceName")

	ctx.Err = svcservice.ValidateServiceUpdate(codehostID, serviceName, repoOwner, repoName, repoUUID, branchName, remoteName, path, isDir, ctx.Logger)
}
