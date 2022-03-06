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

	gin2 "github.com/koderover/zadig/pkg/middleware/gin"
)

type Router struct{}

func (*Router) Inject(router *gin.RouterGroup) {
	// ---------------------------------------------------------------------------------------
	// 对外公共接口
	// ---------------------------------------------------------------------------------------
	webhook := router.Group("webhook")
	{
		webhook.POST("", ProcessWebHook)
	}

	build := router.Group("build")
	{
		build.GET("/:name/:version/to/subtasks", BuildModuleToSubTasks)
	}

	// ---------------------------------------------------------------------------------------
	// Server Sent Events 接口
	// ---------------------------------------------------------------------------------------
	sse := router.Group("sse")
	{
		sse.GET("/workflows/id/:id/pipelines/:name", GetWorkflowTaskSSE)
		sse.GET("/tasks/running", RunningPipelineTasksSSE)
		sse.GET("/tasks/pending", PendingPipelineTasksSSE)
		sse.GET("/tasks/id/:id/pipelines/:name", GetPipelineTaskSSE)
		sse.GET("/workflowtask/v3/id/:id/name/:name", GetWorkflowTaskV3SSE)
	}

	// ---------------------------------------------------------------------------------------
	// Pipeline 管理接口
	// ---------------------------------------------------------------------------------------
	pipeline := router.Group("v2/pipelines")
	{
		pipeline.GET("", ListPipelines)
		pipeline.GET("/:name", GetPipeline)
		pipeline.POST("", GetPipelineProductName, gin2.UpdateOperationLogStatus, UpsertPipeline)
		pipeline.POST("/old/:old/new/:new", GetProductNameByPipeline, gin2.UpdateOperationLogStatus, CopyPipeline)
		pipeline.PUT("/rename/:old/:new", GetProductNameByPipeline, gin2.UpdateOperationLogStatus, RenamePipeline)
		pipeline.DELETE("/:name", GetProductNameByPipeline, gin2.UpdateOperationLogStatus, DeletePipeline)
	}

	// ---------------------------------------------------------------------------------------
	// Pipeline 状态接口
	// ---------------------------------------------------------------------------------------
	statusV2 := router.Group("v2/status")
	{
		statusV2.GET("/preview", ListPipelinesPreview)
		statusV2.GET("/task/info", FindTasks)
	}

	// ---------------------------------------------------------------------------------------
	// Pipeline 任务管理接口
	// ---------------------------------------------------------------------------------------
	taskV2 := router.Group("v2/tasks")
	{
		taskV2.POST("", GetProductNameByPipelineTask, gin2.UpdateOperationLogStatus, CreatePipelineTask)
		taskV2.GET("/max/:max/start/:start/pipelines/:name", ListPipelineTasksResult)
		taskV2.GET("/id/:id/pipelines/:name", GetPipelineTask)
		taskV2.POST("/id/:id/pipelines/:name/restart", GetProductNameByPipeline, gin2.UpdateOperationLogStatus, RestartPipelineTask)
		taskV2.DELETE("/id/:id/pipelines/:name", GetProductNameByPipeline, gin2.UpdateOperationLogStatus, CancelTaskV2)
		taskV2.GET("/pipelines/:name/products", ListPipelineUpdatableProductNames)
		taskV2.GET("/file", GetPackageFile)
		taskV2.GET("/workflow/:pipelineName/taskId/:taskId", GetArtifactFile)
	}

	// ---------------------------------------------------------------------------------------
	// Pipeline Favorite 接口
	// ---------------------------------------------------------------------------------------
	favorite := router.Group("favorite")
	{
		favorite.POST("", gin2.UpdateOperationLogStatus, CreateFavoritePipeline)
		favorite.DELETE("/:productName/:name/:type", gin2.UpdateOperationLogStatus, DeleteFavoritePipeline)
		favorite.GET("", ListFavoritePipelines)
	}

	// ---------------------------------------------------------------------------------------
	// 产品工作流模块接口
	// ---------------------------------------------------------------------------------------
	workflow := router.Group("workflow")
	{
		workflow.POST("/:productName/auto", AutoCreateWorkflow)
		workflow.POST("", GetWorkflowProductName, gin2.UpdateOperationLogStatus, CreateWorkflow)
		workflow.PUT("", GetWorkflowProductName, gin2.UpdateOperationLogStatus, UpdateWorkflow)
		workflow.GET("", ListWorkflows)
		workflow.GET("/testName/:testName", ListTestWorkflows)
		workflow.GET("/find/:name", FindWorkflow)
		workflow.DELETE("/:name", GetProductNameByWorkflow, gin2.UpdateOperationLogStatus, DeleteWorkflow)
		workflow.GET("/preset/:productName", PreSetWorkflow)

		workflow.PUT("/old/:old/new/:new", CopyWorkflow)
	}

	// ---------------------------------------------------------------------------------------
	// 产品工作流任务接口
	// ---------------------------------------------------------------------------------------
	workflowtask := router.Group("workflowtask")
	{
		//todo 修改权限的uuid
		workflowtask.GET("/targets/:productName/:namespace", GetWorkflowArgs)
		workflowtask.GET("/preset/:namespace/:workflowName", PresetWorkflowArgs)
		workflowtask.POST("", gin2.UpdateOperationLogStatus, CreateWorkflowTask)
		workflowtask.PUT("", gin2.UpdateOperationLogStatus, CreateArtifactWorkflowTask)
		workflowtask.GET("/max/:max/start/:start/pipelines/:name", ListWorkflowTasksResult)
		workflowtask.GET("/filters/pipelines/:name", GetFiltersPipeline)
		workflowtask.GET("/id/:id/pipelines/:name", GetWorkflowTask)
		workflowtask.POST("/id/:id/pipelines/:name/restart", gin2.UpdateOperationLogStatus, RestartWorkflowTask)
		workflowtask.DELETE("/id/:id/pipelines/:name", gin2.UpdateOperationLogStatus, CancelWorkflowTaskV2)
		workflowtask.GET("/callback/id/:id/name/:name", GetWorkflowTaskCallback)
	}

	serviceTask := router.Group("servicetask")
	{
		serviceTask.GET("/workflows/:productName/:envName/:serviceName/:serviceType", ListServiceWorkflows)
	}

	// ---------------------------------------------------------------------------------------
	// 新版本 通用工作流（暂命名） 接口
	// ---------------------------------------------------------------------------------------
	workflowV3 := router.Group("v3")
	{
		workflowV3.POST("", gin2.UpdateOperationLogStatus, CreateWorkflowV3)
		workflowV3.GET("", ListWorkflowsV3)
		workflowV3.GET("/:id", GetWorkflowV3Detail)
		workflowV3.PUT("/:id", gin2.UpdateOperationLogStatus, UpdateWorkflowV3)
		workflowV3.DELETE("/:id", DeleteWorkflowV3)
		workflowV3.GET("/:id/args", GetWorkflowV3Args)
	}

	// ---------------------------------------------------------------------------------------
	// workflow v3 任务接口
	// ---------------------------------------------------------------------------------------
	taskV3 := router.Group("v3/workflowtask")
	{
		taskV3.POST("", gin2.UpdateOperationLogStatus, CreateWorkflowTaskV3)
		taskV3.POST("/id/:id/name/:name/restart", gin2.UpdateOperationLogStatus, RestartWorkflowTaskV3)
		taskV3.DELETE("/id/:id/name/:name", gin2.UpdateOperationLogStatus, CancelWorkflowTaskV3)
		taskV3.GET("/max/:max/start/:start/name/:name", ListWorkflowV3TasksResult)
		taskV3.GET("/id/:id/name/:name", GetWorkflowTaskV3)
		taskV3.GET("/callback/id/:id/name/:name", GetWorkflowTaskV3Callback)
	}

	bundles := router.Group("bundle-resources")
	{
		bundles.GET("", GetBundleResources)
	}
}
