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

package service

import (
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

// ScheduleType 触发模式
type ScheduleType string

const (
	// TimingSchedule 定时循环
	TimingSchedule ScheduleType = "timing"
	// GapSchedule 间隔循环
	GapSchedule ScheduleType = "gap"
)

type PipelineResource struct {
	Version         string           `bson:"version" json:"version"`
	Kind            string           `bson:"kind" json:"kind"`
	Metadata        PipelineMetadata `bson:"metadata" json:"metadata"`
	Spec            PipelineSpec     `bson:"spec" json:"spec"`
	LastExecuteTime *time.Time       `json:"lastExecuteTime,omitempty" bson:"lastExecuteTime"`
	HookPayload     *HookPayload     `bson:"hook_payload" json:"hook_payload"`

	Statistic      *PipelineStatistic `json:"statistic,omitempty" bson:"-"`
	NotificationID string             `bson:"notification_id" json:"notification_id"`
}

type PipelineMetadata struct {
	Name             string `bson:"name" json:"name"`
	Project          string `bson:"project" json:"project"`
	ProjectID        string `bson:"projectId" json:"projectId"`
	Revision         int    `bson:"revision" json:"revision"`
	AccountID        string `bson:"accountId" json:"accountId"`
	OriginYamlString string `bson:"originYamlString" json:"originYamlString"`
	ID               string `bson:"id" json:"id"`

	CreatedAt string `bson:"created_at" json:"created_at"`
	UpdatedAt string `bson:"updated_at" json:"updated_at"`
}

type PipelineSpec struct {
	Schedules        []*Schedule       `bson:"schedules" json:"schedules"`
	Triggers         []PipelineTrigger `bson:"triggers" json:"triggers"`
	Steps            map[string]Dict   `bson:"steps" json:"steps"`
	Stages           []string          `bson:"stages" json:"stages"`
	Variables        []Variable        `bson:"variables" json:"variables"`
	Contexts         []interface{}     `bson:"contexts" json:"contexts"`
	Services         Dict              `bson:"services" json:"services"`
	ExtractVariables []string          `bson:"-" json:"extractVariables"`
	StepNames        []string          `bson:"-" json:"-" `
	OrderedSteps     []Dict            `bson:"-" json:"-" `
}

type Schedule struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"                 json:"id,omitempty"`
	Number       uint64             `bson:"number"                        json:"number"`
	Frequency    string             `bson:"frequency"                     json:"frequency"`
	Time         string             `bson:"time"                          json:"time"`
	MaxFailures  int                `bson:"max_failures,omitempty"        json:"max_failures,omitempty"`
	TaskArgs     *TaskArgs          `bson:"task_args,omitempty"           json:"task_args,omitempty"`
	WorkflowArgs *WorkflowTaskArgs  `bson:"workflow_args,omitempty"       json:"workflow_args,omitempty"`
	TestArgs     *TestTaskArgs      `bson:"test_args,omitempty"           json:"test_args,omitempty"`
	Type         ScheduleType       `bson:"type"                          json:"type"`
	Cron         string             `bson:"cron"                          json:"cron"`
	IsModified   bool               `bson:"-"                             json:"-"`
	// 自由编排工作流的开关是放在schedule里面的
	Enabled bool `bson:"enabled"                       json:"enabled"`
}

//Validate validate schedule setting
func (schedule *Schedule) Validate() error {
	switch schedule.Type {
	case TimingSchedule:
		// 默认间隔循环间隔是1
		if schedule.Number <= 0 {
			schedule.Number = 1
		}
		if err := isValidJobTime(schedule.Time); err != nil {
			return err
		}
		if schedule.Frequency != setting.FrequencyDay &&
			schedule.Frequency != setting.FrequencyMondy &&
			schedule.Frequency != setting.FrequencyTuesday &&
			schedule.Frequency != setting.FrequencyWednesday &&
			schedule.Frequency != setting.FrequencyThursday &&
			schedule.Frequency != setting.FrequencyFriday &&
			schedule.Frequency != setting.FrequencySaturday &&
			schedule.Frequency != setting.FrequencySunday {
			return fmt.Errorf("%s 定时任务频率错误", e.InvalidFormatErrMsg)
		}
		return nil

	case GapSchedule:
		if schedule.Frequency != setting.FrequencyHours &&
			schedule.Frequency != setting.FrequencyMinutes {
			return fmt.Errorf("%s 间隔任务频率错误", e.InvalidFormatErrMsg)
		}
		if schedule.Number <= 0 {
			return fmt.Errorf("%s 间隔循环时间间隔不能小于等于0", e.InvalidFormatErrMsg)
		}
		//if schedule.Frequency == FrequencyMinutes && schedule.Number < 1 {
		//	log.Info("minimum schedule minutes must >= 30")
		//	return fmt.Errorf("%s 定时任务最短为30分钟", e.InvalidFormatErrMsg)
		//
		//}
		return nil

	default:
		return fmt.Errorf("%s 间隔任务模式未设置", e.InvalidFormatErrMsg)
	}
}

func isValidJobTime(t string) error {

	hour := int((t[0]-'0')*10 + (t[1] - '0'))
	min := int((t[3]-'0')*10 + (t[4] - '0'))

	if hour < 0 || hour > 23 || min < 0 || min > 59 {
		return fmt.Errorf("%s 时间范围: 小时[0-24] 分钟[0-59]", e.InvalidFormatErrMsg)
	}

	return nil
}

type PipelineTrigger struct {
	Owner        string   `bson:"repo_owner"               json:"repo_owner"`
	Repo         string   `bson:"repo"                     json:"repo"`
	Branch       string   `bson:"branch"                   json:"branch"`
	Events       []string `bson:"events"                   json:"events"`
	Disabled     bool     `bson:"disabled"                 json:"disabled"`
	Provider     string   `bson:"provider"                 json:"provider"`
	CodehostID   int      `bson:"codehost_id"              json:"codehost_id"`
	MatchFolders []string `bson:"match_folders"            json:"match_folders"`
}

type Variable struct {
	Key       string `bson:"key" json:"key"`
	Value     string `bson:"value" json:"value"`
	Encrypted bool   `bson:"encrypted" json:"encrypted"`
}

type Dict map[string]interface{}

// TaskArgs 单服务工作流任务参数
type TaskArgs struct {
	ProductName    string        `bson:"product_name"            json:"product_name"`
	PipelineName   string        `bson:"pipeline_name"           json:"pipeline_name"`
	Builds         []*Repository `bson:"builds"                  json:"builds"`
	BuildArgs      []*KeyVal     `bson:"build_args"              json:"build_args"`
	Deploy         DeployArgs    `bson:"deploy"                  json:"deploy"`
	Test           TestArgs      `bson:"test"                    json:"test,omitempty"`
	HookPayload    *HookPayload  `bson:"hook_payload"            json:"hook_payload,omitempty"`
	TaskCreator    string        `bson:"task_creator"            json:"task_creator,omitempty"`
	ReqID          string        `bson:"req_id"                  json:"req_id"`
	IsQiNiu        bool          `bson:"is_qiniu"                json:"is_qiniu"`
	NotificationID string        `bson:"notification_id"         json:"notification_id"`
}

type Repository struct {
	// Source is github, gitlab
	Source        string `bson:"source,omitempty"          json:"source,omitempty"`
	RepoOwner     string `bson:"repo_owner"                json:"repo_owner"`
	RepoName      string `bson:"repo_name"                 json:"repo_name"`
	RemoteName    string `bson:"remote_name,omitempty"     json:"remote_name,omitempty"`
	Branch        string `bson:"branch"                    json:"branch"`
	PR            int    `bson:"pr,omitempty"              json:"pr,omitempty"`
	Tag           string `bson:"tag,omitempty"             json:"tag,omitempty"`
	CommitID      string `bson:"commit_id,omitempty"       json:"commit_id,omitempty"`
	CommitMessage string `bson:"commit_message,omitempty"  json:"commit_message,omitempty"`
	CheckoutPath  string `bson:"checkout_path,omitempty"   json:"checkout_path,omitempty"`
	SubModules    bool   `bson:"submodules,omitempty"      json:"submodules,omitempty"`
	// UseDefault defines if the repo can be configured in start pipeline task page
	UseDefault bool `bson:"use_default,omitempty"          json:"use_default,omitempty"`
	// IsPrimary used to generated image and package name, each build has one primary repo
	IsPrimary  bool `bson:"is_primary"                     json:"is_primary"`
	CodehostID int  `bson:"codehost_id"                    json:"codehost_id"`
	// add
	OauthToken  string `bson:"oauth_token"                  json:"oauth_token"`
	Address     string `bson:"address"                      json:"address"`
	AuthorName  string `bson:"author_name,omitempty"        json:"author_name,omitempty"`
	CheckoutRef string `bson:"checkout_ref,omitempty"       json:"checkout_ref,omitempty"`
}

type KeyVal struct {
	Key          string `bson:"key"                 json:"key"`
	Value        string `bson:"value"               json:"value"`
	IsCredential bool   `bson:"is_credential"       json:"is_credential"`
}

type DeployArgs struct {
	// 目标部署环境
	Namespace string `json:"namespace"`
	// 镜像或者二进制名称后缀, 一般为branch或者PR
	Tag string `json:"suffix"`
	// 部署镜像名称
	// 格式: registy.xxx.com/{namespace}/{service name}:{timestamp}-{suffix}}
	// timestamp format: 20060102150405
	Image string `json:"image"`
	// 部署二进制包名称
	// 格式: {service name}-{timestamp}-{suffix}}.tar.gz
	// timestamp format: 20060102150405
	PackageFile string `json:"package_file"`
}

// TestArgs ...
type TestArgs struct {
	Namespace      string        `bson:"namespace" json:"namespace"`
	TestModuleName string        `bson:"test_module_name" json:"test_module_name"`
	Envs           []*KeyVal     `bson:"envs" json:"envs"`
	Builds         []*Repository `bson:"builds" json:"builds"`
}

// HookPayload ...
type HookPayload struct {
	Owner      string `bson:"owner"          json:"owner,omitempty"`
	Repo       string `bson:"repo"           json:"repo,omitempty"`
	Branch     string `bson:"branch"         json:"branch,omitempty"`
	Ref        string `bson:"ref"            json:"ref,omitempty"`
	IsPr       bool   `bson:"is_pr"          json:"is_pr,omitempty"`
	CheckRunID int64  `bson:"check_run_id"   json:"check_run_id,omitempty"`
	DeliveryID string `bson:"delivery_id"    json:"delivery_id,omitempty"`
}

// WorkflowTaskArgs 多服务工作流任务参数
type WorkflowTaskArgs struct {
	WorkflowName    string `bson:"workflow_name"                json:"workflow_name"`
	ProductTmplName string `bson:"product_tmpl_name"            json:"product_tmpl_name"`
	Description     string `bson:"description,omitempty"        json:"description,omitempty"`
	//为了兼容老数据，namespace可能会存多个环境名称，用逗号隔开
	Namespace          string          `bson:"namespace"                    json:"namespace"`
	BaseNamespace      string          `bson:"base_namespace,omitempty"     json:"base_namespace,omitempty"`
	EnvRecyclePolicy   string          `bson:"env_recycle_policy,omitempty" json:"env_recycle_policy,omitempty"`
	EnvUpdatePolicy    string          `bson:"env_update_policy,omitempty"  json:"env_update_policy,omitempty"`
	Target             []*TargetArgs   `bson:"targets"                      json:"targets"`
	Artifact           []*ArtifactArgs `bson:"artifact_args"                json:"artifact_args"`
	Tests              []*TestArgs     `bson:"tests"                        json:"tests"`
	VersionArgs        *VersionArgs    `bson:"version_args,omitempty"       json:"version_args,omitempty"`
	ReqID              string          `bson:"req_id"                       json:"req_id"`
	RegistryID         string          `bson:"registry_id,omitempty"        json:"registry_id,omitempty"`
	StorageID          string          `bson:"storage_id,omitempty"         json:"storage_id,omitempty"`
	DistributeEnabled  bool            `bson:"distribute_enabled"           json:"distribute_enabled"`
	WorklowTaskCreator string          `bson:"workflow_task_creator"        json:"workflow_task_creator"`
	// Ignore docker build cache
	IgnoreCache bool `json:"ignore_cache" bson:"ignore_cache"`
	// Ignore workspace cache and reset volume
	ResetCache bool `json:"reset_cache" bson:"reset_cache"`

	// NotificationID is the id of scmnotify.Notification
	NotificationID string `bson:"notification_id" json:"notification_id"`

	// webhook触发工作流任务时，触发任务的repo信息、prID和commitID
	MergeRequestID string `bson:"merge_request_id" json:"merge_request_id"`
	CommitID       string `bson:"commit_id"        json:"commit_id"`
	Source         string `bson:"source"           json:"source"`
	CodehostID     int    `bson:"codehost_id"      json:"codehost_id"`
	RepoOwner      string `bson:"repo_owner"       json:"repo_owner"`
	RepoName       string `bson:"repo_name"        json:"repo_name"`

	//github check run
	HookPayload *HookPayload `bson:"hook_payload"            json:"hook_payload,omitempty"`
	// 请求模式，openAPI表示外部客户调用
	RequestMode string `json:"request_mode,omitempty"`
	IsParallel  bool   `json:"is_parallel" bson:"is_parallel"`
}

// TargetArgs ...
type TargetArgs struct {
	// 服务组件名称
	Name string `bson:"name"                      json:"name"`
	// 服务名称
	ServiceName      string            `bson:"service_name"              json:"service_name"`
	ServiceType      string            `bson:"service_type,omitempty"    json:"service_type,omitempty"`
	Build            *BuildArgs        `bson:"build"                     json:"build"`
	Version          string            `bson:"version"                   json:"version"`
	Deploy           []DeployEnv       `bson:"deloy"                     json:"deploy"`
	Image            string            `bson:"image"                     json:"image"`
	BinFile          string            `bson:"bin_file"                  json:"bin_file"`
	Envs             []*KeyVal         `bson:"envs"                      json:"envs"`
	HasBuild         bool              `bson:"has_build"                 json:"has_build"`
	JenkinsBuildArgs *JenkinsBuildArgs `bson:"jenkins_build_args"        json:"jenkins_build_args"`
}

type JenkinsBuildArgs struct {
	JobName            string               `bson:"job_name"            json:"job_name"`
	JenkinsBuildParams []*JenkinsBuildParam `bson:"jenkins_build_param" json:"jenkins_build_params"`
}

type JenkinsBuildParam struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

type BuildArgs struct {
	Repos []*Repository `bson:"repos"               json:"repos"`
}

type ArtifactArgs struct {
	Name        string      `bson:"name"                      json:"name"`
	ServiceName string      `bson:"service_name"              json:"service_name"`
	Image       string      `bson:"image"                     json:"image"`
	Deploy      []DeployEnv `bson:"deloy"                     json:"deploy"`
}

type DeployEnv struct {
	Env         string `json:"env"`
	Type        string `json:"type"`
	ProductName string `json:"product_name,omitempty"`
}

type PipelineStatistic struct {
	AvgTime      int                `json:"avgTime" bson:"avgTime"`
	MaxTime      int                `json:"maxTime" bson:"maxTime"`
	Workflows    []*WorkflowSummary `json:"workflows" bson:"workflows"`
	SuccessCount int                `json:"successCount" bson:"successCount"`
	ID           string             `json:"-" bson:"_id"`
}

type WorkflowSummary struct {
	Status   string    `json:"status" bson:"status"`
	ID       string    `json:"id" bson:"id"`
	Created  time.Time `json:"created" bson:"created"`
	Finished time.Time `json:"finished" bson:"finished"`
}

type VersionArgs struct {
	Enabled bool     `bson:"enabled" json:"enabled"`
	Version string   `bson:"version" json:"version"`
	Desc    string   `bson:"desc"    json:"desc"`
	Labels  []string `bson:"labels"  json:"labels"`
}

type TestTaskArgs struct {
	ProductName     string `bson:"product_name"            json:"product_name"`
	TestName        string `bson:"test_name"               json:"test_name"`
	TestTaskCreator string `bson:"test_task_creator"       json:"test_task_creator"`
	NotificationID  string `bson:"notification_id"         json:"notification_id"`
	ReqID           string `bson:"req_id"                  json:"req_id"`
	// webhook触发测试任务时，触发任务的repo、prID和commitID
	MergeRequestID string `bson:"merge_request_id" json:"merge_request_id"`
	CommitID       string `bson:"commit_id"        json:"commit_id"`
	Source         string `bson:"source"           json:"source"`
	CodehostID     int    `bson:"codehost_id"      json:"codehost_id"`
	RepoOwner      string `bson:"repo_owner"       json:"repo_owner"`
	RepoName       string `bson:"repo_name"        json:"repo_name"`
}

type CreateBuildRequest struct {
	UserID       string     `json:"userId" bson:"userId"`
	UserName     string     `json:"userName" bson:"userName"`
	NoCache      bool       `json:"noCache" bson:"noCache"`
	ResetVolume  bool       `json:"cleanVolume" bson:"cleanVolume"`
	Variables    []Variable `json:"variables" bson:"variables"`
	PipelineName string     `json:"pipeline_name" bson:"pipeline_name"`
	ProductName  string     `json:"product_name" bson:"product_name"`
}

type DomainWebHookUser struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"   json:"id,omitempty"`
	Domain    string             `bson:"domain"          json:"domain"`
	UserCount int                `bson:"user_count"      json:"user_count"`
	CreatedAt int64              `bson:"created_at"      json:"created_at"`
}

type OperationRequest struct {
	Data string `json:"data"`
}

type PrivateKey struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"          json:"id"`
	Name       string             `bson:"name"                   json:"name"`
	UserName   string             `bson:"user_name"              json:"user_name"`
	IP         string             `bson:"ip"                     json:"ip"`
	Label      string             `bson:"label"                  json:"label"`
	IsProd     bool               `bson:"is_prod"                json:"is_prod"`
	PrivateKey string             `bson:"private_key"            json:"private_key"`
	CreateTime int64              `bson:"create_time"            json:"create_time"`
	UpdateTime int64              `bson:"update_time"            json:"update_time"`
	UpdateBy   string             `bson:"update_by"              json:"update_by"`
}

type Container struct {
	Name  string `bson:"name"           json:"name"`
	Image string `bson:"image"          json:"image"`
}

type RenderInfo struct {
	Name        string `bson:"name"                     json:"name"`
	Revision    int64  `bson:"revision"                 json:"revision"`
	ProductTmpl string `bson:"product_tmpl"             json:"product_tmpl"`
	Description string `bson:"description"              json:"description"`
}

type RenderKV struct {
	Key      string   `bson:"key"               json:"key"`
	Value    string   `bson:"value"             json:"value"`
	Alias    string   `bson:"alias"             json:"alias"`
	State    KeyState `bson:"state"             json:"state"`
	Services []string `bson:"services"          json:"services"`
}

// KeyState can be new, deleted, normal
type KeyState string

type Pipeline struct {
	Name        string       `bson:"name"                         json:"name"`
	Type        PipelineType `bson:"type"                         json:"type"`
	Enabled     bool         `bson:"enabled"                      json:"enabled"`
	TeamName    string       `bson:"team"                         json:"team"`
	ProductName string       `bson:"product_name"                 json:"product_name"`
	// target 服务名称, k8s为容器名称, 物理机为服务名
	Target string `bson:"target"                    json:"target"`
	// 使用预定义编译管理模块中的内容生成SubTasks,
	// 查询条件为 服务名称: Target, 版本: BuildModuleVer
	// 如果为空，则使用pipeline自定义SubTasks
	BuildModuleVer string                    `bson:"build_module_ver"             json:"build_module_ver"`
	SubTasks       []*map[string]interface{} `bson:"sub_tasks"                    json:"sub_tasks"`
	Description    string                    `bson:"description,omitempty"        json:"description,omitempty"`
	UpdateBy       string                    `bson:"update_by"                    json:"update_by,omitempty"`
	CreateTime     int64                     `bson:"create_time"                  json:"create_time,omitempty"`
	UpdateTime     int64                     `bson:"update_time"                  json:"update_time,omitempty"`
	Schedules      ScheduleCtrl              `bson:"schedules,omitempty"          json:"schedules,omitempty"`
	Notifiers      []string                  `bson:"notifiers,omitempty"          json:"notifiers,omitempty"`
	RunCount       int                       `bson:"run_count,omitempty"          json:"run_count,omitempty"`
	DailyRunCount  float32                   `bson:"daily_run_count,omitempty"    json:"daily_run_count,omitempty"`
	PassRate       float32                   `bson:"pass_rate,omitempty"          json:"pass_rate,omitempty"`
	IsDeleted      bool                      `bson:"is_deleted"                   json:"is_deleted"`
}

// ScheduleCtrl ...
type ScheduleCtrl struct {
	Enabled bool        `bson:"enabled"    json:"enabled"`
	Items   []*Schedule `bson:"items"      json:"items"`
}

// Type pipeline type
type PipelineType string

const (
	// SingleType 单服务工作流
	SingleType = PipelineType("single")
	// WorkflowType 多服务工作流
	WorkflowType = PipelineType("workflow")
	// FreestyleType 自由编排工作流
	FreestyleType = PipelineType("freestyle")
	// TestType 测试
	TestType = PipelineType("test")
	// ServiceType 服务
	ServiceType = PipelineType("service")
)

type Workflow struct {
	Name      string        `json:"name"`
	Schedules *ScheduleCtrl `json:"schedules,omitempty"`
}
