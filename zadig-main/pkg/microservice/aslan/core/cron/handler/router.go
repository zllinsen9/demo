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
)

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {

	// ---------------------------------------------------------------------------------------
	// 定时任务管理接口
	// ---------------------------------------------------------------------------------------
	cron := router.Group("cron")
	{
		cron.GET("/cleanjob", CleanJobCronJob)
		cron.GET("/cleanconfigmap", CleanConfigmapCronJob)
	}

	cronjob := router.Group("cronjob")
	{
		cronjob.POST("/disable", DisableCronjob)
		cronjob.GET("/failsafe", ListActiveCronjobFailsafe)
		cronjob.GET("", ListActiveCronjob)
		cronjob.GET("/type/:type/name/:name", ListCronjob)
	}
}
