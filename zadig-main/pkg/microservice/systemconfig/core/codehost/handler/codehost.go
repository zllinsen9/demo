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
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/codehost/repository/models"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/codehost/service"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
)

func CreateCodeHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	rep := new(models.CodeHost)
	if err := c.ShouldBindJSON(rep); err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = service.CreateCodeHost(rep, ctx.Logger)
}

func ListCodeHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	ctx.Resp, ctx.Err = service.List(c.Query("address"), c.Query("owner"), c.Query("source"), ctx.Logger)
}

func DeleteCodeHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		ctx.Err = err
		return
	}
	ctx.Err = service.DeleteCodeHost(id, ctx.Logger)
}

func GetCodeHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		ctx.Err = err
		return
	}
	ctx.Resp, ctx.Err = service.GetCodeHost(id, ctx.Logger)
}

func AuthCodeHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	id := c.Param("id")
	idInt, err := strconv.Atoi(id)
	if err != nil {
		ctx.Err = err
		return
	}
	url, err := service.AuthCodeHost(c.Query("redirect_url"), idInt, ctx.Logger)
	if err != nil {
		ctx.Err = err
		ctx.Logger.Errorf("auth err,id:%d,err: %s", idInt, err)
		return
	}
	c.Redirect(http.StatusFound, url)
}

func Callback(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	state := c.Query("state")
	redirectURL, err := service.HandleCallback(state, c.Request, ctx.Logger)
	if err != nil {
		ctx.Logger.Errorf("Callback err:%s", err)
		ctx.Err = err
		return
	}
	c.Redirect(http.StatusFound, redirectURL)
}

func UpdateCodeHost(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		ctx.Err = err
		return
	}
	req := &models.CodeHost{}
	if err := c.ShouldBindJSON(req); err != nil {
		ctx.Err = err
		return
	}
	req.ID = id
	ctx.Resp, ctx.Err = service.UpdateCodeHost(req, ctx.Logger)
}
