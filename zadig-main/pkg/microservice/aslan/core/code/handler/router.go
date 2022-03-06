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
	codehost := router.Group("codehost")
	{
		codehost.GET("", GetCodeHostList)
		codehost.GET("/:codehostId/namespaces", CodeHostGetNamespaceList)
		codehost.GET("/:codehostId/namespaces/:namespace/projects", CodeHostGetProjectsList)
		codehost.GET("/:codehostId/namespaces/:namespace/projects/:projectName/branches", CodeHostGetBranchList)
		codehost.GET("/:codehostId/namespaces/:namespace/projects/:projectName/tags", CodeHostGetTagList)
		codehost.GET("/:codehostId/namespaces/:namespace/projects/:projectName/prs", CodeHostGetPRList)
		codehost.PUT("/infos", ListRepoInfos)
		codehost.POST("/branches/regular/check", MatchBranchesList)
	}

	// ---------------------------------------------------------------------------------------
	// Pipeline workspace 管理接口
	// ---------------------------------------------------------------------------------------
	workspace := router.Group("workspace")
	{
		workspace.DELETE("", GetProductNameByWorkspacePipeline, gin2.UpdateOperationLogStatus, CleanWorkspace)
		workspace.GET("/file", GetWorkspaceFile)
		workspace.GET("/git/:codehostId/:repoName/:branchName/:remoteName", GetGitRepoInfo)

		workspace.GET("/tree", GetRepoTree)
		workspace.GET("/codehub/:codehostId/:repoUUID/:branchName", GetCodehubRepoInfo)
	}

}
