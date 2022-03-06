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

package setting

import "time"

const LocalConfig = "local.env"

// envs
const (
	// common
	ENVSystemAddress           = "ADDRESS"
	ENVEnterprise              = "ENTERPRISE"
	ENVMode                    = "MODE"
	ENVNsqLookupAddrs          = "NSQLOOKUP_ADDRS"
	ENVMongoDBConnectionString = "MONGODB_CONNECTION_STRING"
	ENVAslanDBName             = "ASLAN_DB"
	ENVHubAgentImage           = "HUB_AGENT_IMAGE"
	ENVResourceServerImage     = "RESOURCE_SERVER_IMAGE"
	ENVMysqlUser               = "MYSQL_USER"
	ENVMysqlPassword           = "MYSQL_PASSWORD"
	ENVMysqlHost               = "MYSQL_HOST"
	ENVMysqlUserDb             = "MYSQL_USER_DB"

	// Aslan
	ENVPodName              = "BE_POD_NAME"
	ENVNamespace            = "BE_POD_NAMESPACE"
	ENVLogLevel             = "LOG_LEVEL"
	ENVServiceStartTimeout  = "SERVICE_START_TIMEOUT"
	ENVDefaultEnvRecycleDay = "DEFAULT_ENV_RECYCLE_DAY"
	ENVDefaultIngressClass  = "DEFAULT_INGRESS_CLASS"
	ENVAslanRegAddress      = "DEFAULT_REGISTRY"
	ENVAslanRegAccessKey    = "DEFAULT_REGISTRY_AK"
	ENVAslanRegSecretKey    = "DEFAULT_REGISTRY_SK"
	ENVAslanRegNamespace    = "DEFAULT_REGISTRY_NAMESPACE"

	ENVGithubSSHKey    = "GITHUB_SSH_KEY"
	ENVGithubKnownHost = "GITHUB_KNOWN_HOST"

	ENVReaperImage      = "REAPER_IMAGE"
	ENVReaperBinaryFile = "REAPER_BINARY_FILE"
	ENVPredatorImage    = "PREDATOR_IMAGE"
	EnvPackagerImage    = "PACKAGER_IMAGE"

	ENVDockerHosts = "DOCKER_HOSTS"

	ENVUseClassicBuild       = "USE_CLASSIC_BUILD"
	ENVCustomDNSNotSupported = "CUSTOM_DNS_NOT_SUPPORTED"

	ENVOldEnvSupported = "OLD_ENV_SUPPORTED"

	ENVS3StorageAK       = "S3STORAGE_AK"
	ENVS3StorageSK       = "S3STORAGE_SK"
	ENVS3StorageEndpoint = "S3STORAGE_ENDPOINT"
	ENVS3StorageBucket   = "S3STORAGE_BUCKET"
	ENVS3StorageProtocol = "S3STORAGE_PROTOCOL"
	ENVS3StoragePath     = "S3STORAGE_PATH"
	ENVKubeServerAddr    = "KUBE_SERVER_ADDR"

	// cron
	ENVRootToken = "ROOT_TOKEN"

	ENVKodespaceVersion = "KODESPACE_VERSION"

	// hubagent
	HubAgentToken         = "HUB_AGENT_TOKEN"
	HubServerBaseAddr     = "HUB_SERVER_BASE_ADDR"
	KubernetesServiceHost = "KUBERNETES_SERVICE_HOST"
	KubernetesServicePort = "KUBERNETES_SERVICE_PORT"
	Token                 = "X-API-Tunnel-Token"
	Params                = "X-API-Tunnel-Params"

	// warpdrive
	WarpDrivePodName    = "WD_POD_NAME"
	ReleaseImageTimeout = "RELEASE_IMAGE_TIMEOUT"
	DefaultRegistryAddr = "DEFAULT_REG_ADDRESS"
	DefaultRegistryAK   = "DEFAULT_REG_ACCESS_KEY"
	DefaultRegistrySK   = "DEFAULT_REG_SECRET_KEY"

	// reaper
	Home          = "HOME"
	PkgFile       = "PKG_FILE"
	JobConfigFile = "JOB_CONFIG_FILE"
	DockerAuthDir = "DOCKER_AUTH_DIR"
	Path          = "PATH"
	DockerHost    = "DOCKER_HOST"
	BuildURL      = "BUILD_URL"

	// jenkins
	JenkinsBuildImage = "JENKINS_BUILD_IMAGE"

	DebugMode   = "debug"
	ReleaseMode = "release"
	TestMode    = "test"

	// user
	ENVIssuerURL      = "ISSUER_URL"
	ENVClientID       = "CLIENT_ID"
	ENVClientSecret   = "CLIENT_SECRET"
	ENVRedirectURI    = "REDIRECT_URI"
	ENVSecretKey      = "SECRET_KEY"
	ENVMysqlUserDB    = "MYSQL_USER_DB"
	ENVScopes         = "SCOPES"
	ENVTokenExpiresAt = "TOKEN_EXPIRES_AT"
	ENVUserPort       = "USER_PORT"

	// initconfig
	ENVAdminEmail    = "ADMIN_EMAIL"
	ENVAdminPassword = "ADMIN_PASSWORD"
	PresetAccount    = "admin"
)

// k8s concepts
const (
	Secret             = "Secret"
	ConfigMap          = "ConfigMap"
	Ingress            = "Ingress"
	Service            = "Service"
	Deployment         = "Deployment"
	StatefulSet        = "StatefulSet"
	Pod                = "Pod"
	ReplicaSet         = "ReplicaSet"
	Job                = "Job"
	CronJob            = "CronJob"
	ClusterRoleBinding = "ClusterRoleBinding"
	ServiceAccount     = "ServiceAccount"
	ClusterRole        = "ClusterRole"
	Role               = "Role"
	RoleBinding        = "RoleBinding"

	// labels
	TaskLabel                       = "s-task"
	TypeLabel                       = "s-type"
	PipelineTypeLable               = "p-type"
	ProductLabel                    = "s-product"
	GroupLabel                      = "s-group"
	ServiceLabel                    = "s-service"
	ConfigBackupLabel               = "config-backup"
	EnvNameLabel                    = "s-env"
	UpdateBy                        = "update-by"
	UpdateByID                      = "update-by-id"
	UpdateTime                      = "update-time"
	UpdatedByLabel                  = "updated-by-koderover"
	IngressClassLabel               = "kubernetes.io/ingress.class"
	IngressProxyConnectTimeoutLabel = "nginx.ingress.kubernetes.io/proxy-connect-timeout"
	IngressProxySendTimeoutLabel    = "nginx.ingress.kubernetes.io/proxy-send-timeout"
	IngressProxyReadTimeoutLabel    = "nginx.ingress.kubernetes.io/proxy-read-timeout"
	ComponentLabel                  = "app.kubernetes.io/component"
	companyLabel                    = "koderover.io"
	DirtyLabel                      = companyLabel + "/" + "modified-since-last-update"
	OwnerLabel                      = companyLabel + "/" + "owner"
	InactiveConfigLabel             = companyLabel + "/" + "inactive"
	ModifiedByAnnotation            = companyLabel + "/" + "last-modified-by"
	EditorIDAnnotation              = companyLabel + "/" + "editor-id"
	LastUpdateTimeAnnotation        = companyLabel + "/" + "last-update-time"

	LabelValueTrue = "true"

	// Pod status
	PodRunning    = "Running"
	PodError      = "Error"
	PodUnstable   = "Unstable"
	PodCreating   = "Creating"
	PodCreated    = "created"
	PodUpdating   = "Updating"
	PodDeleting   = "Deleting"
	PodSucceeded  = "Succeeded"
	PodFailed     = "Failed"
	PodPending    = "Pending"
	PodNonStarted = "Unstart"
	PodCompleted  = "Completed"

	// cluster status
	ClusterUnknown      = "Unknown"
	ClusterNotFound     = "NotFound"
	ClusterDisconnected = "Disconnected"

	// annotations
	HelmReleaseNameAnnotation = "meta.helm.sh/release-name"

	EnvCreatedBy              = "createdBy"
	EnvCreator                = "koderover"
	PodReady                  = "ready"
	JobReady                  = "Completed"
	PodNotReady               = "not ready"
	HelmReleaseStatusPending  = "Pending"
	HelmReleaseStatusDeployed = "Deployed"

	APIVersionAppsV1 = "apps/v1"

	DefaultImagePullSecret = "default-registry-secret"
)

const (
	ProductName = "zadig"
	RequestID   = "requestID"

	ProtocolHTTP  string = "http"
	ProtocolHTTPS string = "https"
	ProtocolTCP   string = "tcp"

	DefaultIngressClass = "koderover-nginx"

	// K8SDeployType 容器化部署方式
	K8SDeployType = "k8s"
	// helm 部署
	HelmDeployType = "helm"
	// PMDeployType physical machine deploy 脚本物理机部署方式
	PMDeployType = "pm"

	// 基础设施 k8s类型
	BasicFacilityK8S = "kubernetes"
	// 基础设施 云主机
	BasicFacilityCVM = "cloud_host"

	// SourceFromZadig 配置来源由平台管理
	SourceFromZadig = "spock"
	// SourceFromGitlab 配置来源为gitlab
	SourceFromGitlab = "gitlab"
	// SourceFromGithub 配置来源为github
	SourceFromGithub = "github"
	// SourceFromGitlab 配置来源为gerrit
	SourceFromGerrit = "gerrit"
	// SourceFromCodeHub 配置来源为codehub
	SourceFromCodeHub = "codehub"
	// SourceFromChartTemplate 配置来源为helmTemplate
	SourceFromChartTemplate = "chartTemplate"
	// SourceFromPublicRepo 配置来源为publicRepo
	SourceFromPublicRepo = "publicRepo"
	SourceFromChartRepo  = "chartRepo"

	// SourceFromGUI 配置来源为gui
	SourceFromGUI = "gui"
	//SourceFromHelm
	SourceFromHelm = "helm"
	//SourceFromExternal
	SourceFromExternal = "external"
	// service from yaml template
	ServiceSourceTemplate = "template"
	SourceFromPM          = "pm"

	ProdENV = "prod"
	TestENV = "test"
	AllENV  = "all"

	// action type
	TypeEnableCronjob  = "enable"
	TypeDisableCronjob = "disable"

	PublicService = "public"

	// onboarding流程第二步
	OnboardingStatusSecond = 2

	Unset            = "UNSET"
	CleanSkippedList = "CLEAN_WHITE_LIST"
	PerPage          = 20

	BuildType   = "build"
	DeployType  = "deploy"
	TestType    = "test"
	PublishType = "publish"

	FunctionTestType = "function"
)

const (
	DeliveryVersionTypeChart       = "HelmChart"
	DeliveryVersionTypeK8SWorkflow = "K8SWorkflow"
)

const (
	DeliveryDeployTypeImage = "image"
	DeliveryDeployTypeChart = "chart"
)

const (
	AuthorizationHeader = "Authorization"
)

//install script constants
const (
	StandardScriptName   = "install.sh"
	AllInOneScriptName   = "install_with_k8s.sh"
	UninstallScriptName  = "uninstall.sh"
	StandardInstallType  = "standard"
	AllInOneInstallType  = "all-in-one"
	UninstallInstallType = "uninstall"
)

// Pod Status
const (
	StatusRunning   = "Running"
	StatusSucceeded = "Succeeded"
)

//build image consts
const (
	// BuildImageJob ...
	BuildImageJob = "docker-build"
	// ReleaseImageJob ...
	ReleaseImageJob = "docker-release"
)

const (
	BuildChartPackage = "chart-package"
)

const (
	JenkinsBuildJob = "jenkins-build"
)

// counter prefix
const (
	PipelineTaskFmt   = "PipelineTask:%s"
	WorkflowTaskFmt   = "WorkflowTask:%s"
	WorkflowTaskV3Fmt = "WorkflowTaskV3:%s"
	TestTaskFmt       = "TestTask:%s"
	ServiceTaskFmt    = "ServiceTask:%s"
)

// Product Status
const (
	ProductStatusSuccess  = "success"
	ProductStatusFailed   = "failed"
	ProductStatusCreating = "creating"
	ProductStatusUpdating = "updating"
	ProductStatusDeleting = "deleting"
	ProductStatusUnknown  = "unknown"
	ProductStatusUnstable = "Unstable"
)

// DeliveryVersion status
const (
	DeliveryVersionStatusSuccess  = "success"
	DeliveryVersionStatusFailed   = "failed"
	DeliveryVersionStatusCreating = "creating"
	DeliveryVersionStatusRetrying = "retrying"
)

const (
	DeliveryVersionPackageStatusSuccess   = "success"
	DeliveryVersionPackageStatusFailed    = "failed"
	DeliveryVersionPackageStatusWaiting   = "waiting"
	DeliveryVersionPackageStatusUploading = "uploading"
)

const (
	NormalModeProduct = "normal"
)

const (
	SystemUser = "system"
)

// events
const (
	CreateProductEvent        = "CreateProduct"
	UpdateProductEvent        = "UpdateProduct"
	DeleteProductEvent        = "DeleteProduct"
	UpdateContainerImageEvent = "UpdateContainerImage"
)

// Service Related
const (
	// PrivateVisibility ...
	PrivateVisibility = "private"
	// PublicAccess ...
	PublicAccess = "public"
	// ChartYaml
	ChartYaml = "Chart.yaml"
	// ValuesYaml
	ValuesYaml = "values.yaml"
	// TemplatesDir
	TemplatesDir = "templates"
	// ServiceTemplateCounterName 服务模板counter name
	ServiceTemplateCounterName = "service:%s&project:%s"
	// GerritDefaultOwner
	GerritDefaultOwner = "dafault"
	// YamlFileSeperator ...
	YamlFileSeperator = "\n---\n"
)

const MaskValue = "********"

// proxy
const (
	ProxyAPIAddr    = "PROXY_API_ADDR"
	ProxyHTTPSAddr  = "PROXY_HTTPS_ADDR"
	ProxyHTTPAddr   = "PROXY_HTTP_ADDR"
	ProxySocks5Addr = "PROXY_SOCKS_ADDR"
)

const (
	// WebhookTaskCreator ...
	WebhookTaskCreator = "webhook"
	// CronTaskCreator ...
	CronTaskCreator = "timer"
	// DefaultTaskRevoker ...
	DefaultTaskRevoker = "system" // default task revoker
)

const (
	// DefaultMaxFailures ...
	DefaultMaxFailures = 10

	// FrequencySeconds ...
	FrequencySeconds = "seconds"
	// FrequencyMinutes ...
	FrequencyMinutes = "minutes"
	// FrequencyHour ...
	FrequencyHour = "hour"
	// FrequencyHours ...
	FrequencyHours = "hours"
	// FrequencyDay ...
	FrequencyDay = "day"
	// FrequencyDays ...
	FrequencyDays = "days"
	// FrequencyMondy ...
	FrequencyMondy = "monday"
	// FrequencyTuesday ...
	FrequencyTuesday = "tuesday"
	// FrequencyWednesday ...
	FrequencyWednesday = "wednesday"
	// FrequencyThursday ...
	FrequencyThursday = "thursday"
	// FrequencyFriday ...
	FrequencyFriday = "friday"
	// FrequencySaturday ...
	FrequencySaturday = "saturday"
	// FrequencySunday ...
	FrequencySunday = "sunday"
)

const (
	// FunctionTest 功能测试
	FunctionTest = "function"
	// PerformanceTest 性能测试
	PerformanceTest = "performance"
)

const (
	// UbuntuPrecis ...
	UbuntuPrecis = "precise"
	// UbuntuTrusty ...
	UbuntuTrusty = "trusty"
	// UbuntuXenial ...
	UbuntuXenial = "xenial"
	// UbuntuBionic ...
	UbuntuBionic = "bionic"
	// TestOnly ...
	TestOnly = "test"
)

const (
	Version = "stable"

	EnvRecyclePolicyAlways     = "always"
	EnvRecyclePolicyTaskStatus = "success"
	EnvRecyclePolicyNever      = "never"
)

const (
	ImageFromCustom     = "custom"
	FixedDayTimeCronjob = "timing"
	FixedGapCronjob     = "gap"
	CrontabCronjob      = "crontab"

	WorkflowCronjob = "workflow"
	TestingCronjob  = "test"

	TopicProcess      = "task.process"
	TopicCancel       = "task.cancel"
	TopicAck          = "task.ack"
	TopicItReport     = "task.it.report"
	TopicNotification = "task.notification"
	TopicCronjob      = "cronjob"
)

// S3 related constants
const (
	S3DefaultRegion = "ap-shanghai"
)

// ALL provider mapping
const (
	ProviderSourceETC = iota
	ProviderSourceAli
	ProviderSourceTencent
	ProviderSourceQiniu
	ProviderSourceHuawei
	ProviderSourceSystemDefault
)

// helm related
const (
	ValuesYamlSourceFreeEdit = "freeEdit"
	ValuesYamlSourceGitRepo  = "gitRepo"

	// components used to search image paths from yaml
	PathSearchComponentRepo  = "repo"
	PathSearchComponentImage = "image"
	PathSearchComponentTag   = "tag"
)

// Aliyun specific stuff
const (
	AliyunHost = ".aliyuncs.com"
)

// Dockerfile parsing consts
const (
	DockerfileCmdArg = "ARG"
)

// Dockerfile template constant
const (
	DockerfileSourceLocal    = "local"
	DockerfileSourceTemplate = "template"

	ZadigDockerfilePath = "zadig-dockerfile"
)

// Yaml template constant
const (
	RegExpParameter = `{{.(\w)+}}`
)

// template common constant
const (
	TemplateVariableProduct            = "$T-Project$"
	TemplateVariableProductDescription = "项目名称"
	TemplateVariableService            = "$T-Service$"
	TemplateVariableServiceDescription = "服务名称"
)

const MaxTries = 1

const DogFood = "/var/run/koderover-dog-food"

const ProgressFile = "/var/log/job-progress"

const (
	ResponseError = "error"
	ResponseData  = "response"
)

const ChartTemplatesPath = "charts"

type RoleType string

const (
	Contributor     RoleType = "contributor"
	ReadOnly        RoleType = "read-only"
	ProjectAdmin    RoleType = "project-admin"
	SystemAdmin     RoleType = "admin"
	ReadProjectOnly RoleType = "read-project-only"
)

// ModernWorkflowType 自由编排工作流
const ModernWorkflowType = "ModernWorkflow"

const (
	Subresource        = "subresource"
	StatusSubresource  = "status"
	IngressSubresource = "ingress"
	ResourcesHeader    = "Resources"
)

type K8SClusterStatus string

const (
	Disconnected K8SClusterStatus = "disconnected"
	Pending      K8SClusterStatus = "pending"
	Normal       K8SClusterStatus = "normal"
	Abnormal     K8SClusterStatus = "abnormal"
)

type ResetImagePolicyType string

const (
	ResetImagePolicyTaskCompleted      ResetImagePolicyType = "taskCompleted"
	ResetImagePolicyTaskCompletedOrder ResetImagePolicyType = ""
	ResetImagePolicyDeployFailed       ResetImagePolicyType = "deployFailed"
	ResetImagePolicyTestFailed         ResetImagePolicyType = "testFailed"
)

const LocalClusterID = "0123456789abcdef12345678"

const RequestModeOpenAPI = "openAPI"

const DeployTimeout = 60 * 10 // 10 minutes

const UpdateEnvTimeout = 60 * 5 * time.Second

// list namespace type
const (
	ListNamespaceTypeCreate = "create"
)

const (
	InformerNamingConvention = "%s-%s"
)

type ResourceType string

const (
	ResourceTypeSystem ResourceType = "system"
	ResourceTypeCustom ResourceType = "custom"
)

// AttachedClusterNamespace is the namespace Zadig uses in attached cluster.
// Note: **Restricted because of product design since v1.9.0**.
const AttachedClusterNamespace = "koderover-agent"
