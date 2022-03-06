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

package scheduler

import (
	"fmt"
	stdlog "log"
	"os"
	"time"

	"github.com/jasonlvhit/gocron"
	"github.com/nsqio/go-nsq"
	"github.com/rfyiamcool/cronlib"
	"go.uber.org/zap"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/cron/config"
	"github.com/koderover/zadig/pkg/microservice/cron/core/service"
	"github.com/koderover/zadig/pkg/microservice/cron/core/service/client"
	"github.com/koderover/zadig/pkg/setting"
	configclient "github.com/koderover/zadig/pkg/shared/config"
	"github.com/koderover/zadig/pkg/tool/log"
)

// CronClient ...
type CronClient struct {
	AslanCli              *client.Client
	CollieCli             *client.CollieClient
	Schedulers            map[string]*gocron.Scheduler
	SchedulerController   map[string]chan bool
	lastSchedulers        map[string][]*service.Schedule
	lastServiceSchedulers map[string]*service.SvcRevision
	enabledMap            map[string]bool
	lastProductRevisions  []*service.ProductRevision
	log                   *zap.SugaredLogger
}

const (
	CleanJobScheduler = "CleanJobScheduler"

	UpsertWorkflowScheduler = "UpsertWorkflowScheduler"

	UpsertTestScheduler = "UpsertTestScheduler"

	UpsertColliePipelineScheduler = "UpsertColliePipelineScheduler"

	CleanProductScheduler = "CleanProductScheduler"

	CleanCIResourcesScheduler = "CleanCIResourcesScheduler"

	InitStatScheduler = "InitStatScheduler"

	InitOperationStatScheduler = "InitOperationStatScheduler"

	InitPullSonarStatScheduler = "InitPullSonarStatScheduler"

	// SystemCapacityGC periodically triggers  garbage collection for system data based on its retention policy.
	SystemCapacityGC = "SystemCapacityGC"

	InitHealthCheckScheduler = "InitHealthCheckScheduler"
)

// NewCronClient ...
// 服务初始化
func NewCronClient() *CronClient {
	nsqLookupAddrs := config.NsqLookupAddrs()

	aslanCli := client.NewAslanClient(fmt.Sprintf("%s/api", configbase.AslanServiceAddress()))
	collieCli := client.NewCollieClient(config.CollieAPI())
	//初始化nsq
	config := nsq.NewConfig()
	// 注意 WD_POD_NAME 必须使用 Downward API 配置环境变量
	config.UserAgent = "ASLAN_CRONJOB"
	config.MaxAttempts = 50
	config.LookupdPollInterval = 1 * time.Second

	//nsqClient := nsqcli.NewNsqClient(nsqLookupAddrs, "127.0.0.1:4151")
	//// 初始化nsq topic
	//err := nsqClient.EnsureNsqdTopics([]string{setting.TopicAck, setting.TopicItReport, setting.TopicNotification})
	//if err != nil {
	//	//FIXME
	//	log.Fatalf("cannot ensure nsq topic, the error is %v", err)
	//}

	//Cronjob Client
	cronjobClient, err := nsq.NewConsumer(setting.TopicCronjob, "cronjob", config)
	if err != nil {
		log.Fatalf("failed to init nsq consumer cronjob, error is %v", err)
	}
	cronjobClient.SetLogger(stdlog.New(os.Stdout, "nsq consumer:", 0), nsq.LogLevelError)

	cronjobScheduler := cronlib.New()
	cronjobScheduler.Start()

	cronjobHandler := NewCronjobHandler(aslanCli, cronjobScheduler)
	cronjobClient.AddConcurrentHandlers(cronjobHandler, 10)

	if err := cronjobClient.ConnectToNSQLookupds(nsqLookupAddrs); err != nil {
		errInfo := fmt.Sprintf("nsq consumer for cron job failed to start, the error is: %s", err)
		panic(errInfo)
	}

	return &CronClient{
		AslanCli:              aslanCli,
		CollieCli:             collieCli,
		Schedulers:            make(map[string]*gocron.Scheduler),
		lastSchedulers:        make(map[string][]*service.Schedule),
		lastServiceSchedulers: make(map[string]*service.SvcRevision),
		SchedulerController:   make(map[string]chan bool),
		enabledMap:            make(map[string]bool),
		log:                   log.SugaredLogger(),
	}
}

// 初始化轮询任务
func (c *CronClient) Init() {
	// 每天1点清理跑过的jobs
	c.InitCleanJobScheduler()
	// 每天2点 根据系统配额策略 清理系统过期数据
	c.InitSystemCapacityGCScheduler()
	// 定时任务触发
	c.InitJobScheduler()
	// 测试管理的定时任务触发
	c.InitTestScheduler()

	// 自由编排工作流定时任务触发
	cl := configclient.New(configbase.ConfigServiceAddress())
	if enable, err := cl.CheckFeature(setting.ModernWorkflowType); err == nil && enable {
		c.InitColliePipelineScheduler()
	}

	// 定时清理环境
	c.InitCleanProductScheduler()
	// clean collaboration instance resource every 5 minutes
	c.InitCleanCIResourcesScheduler()
	// 定时初始化构建数据
	c.InitBuildStatScheduler()
	// 定时器初始化话运营统计数据
	c.InitOperationStatScheduler()
	// 定时更新质效看板的统计数据
	c.InitPullSonarStatScheduler()
	// 定时初始化健康检查
	c.InitHealthCheckScheduler()
}

func (c *CronClient) InitCleanJobScheduler() {

	c.Schedulers[CleanJobScheduler] = gocron.NewScheduler()

	c.Schedulers[CleanJobScheduler].Every(1).Day().At("01:00").Do(c.AslanCli.TriggerCleanjobs, c.log)

	c.Schedulers[CleanJobScheduler].Start()
}

func (c *CronClient) InitCleanProductScheduler() {

	c.Schedulers[CleanProductScheduler] = gocron.NewScheduler()

	c.Schedulers[CleanProductScheduler].Every(5).Minutes().Do(c.AslanCli.TriggerCleanProducts, c.log)

	c.Schedulers[CleanProductScheduler].Start()
}

func (c *CronClient) InitCleanCIResourcesScheduler() {

	c.Schedulers[CleanCIResourcesScheduler] = gocron.NewScheduler()

	c.Schedulers[CleanCIResourcesScheduler].Every(5).Minutes().Do(c.AslanCli.TriggerCleanCIResources, c.log)

	c.Schedulers[CleanCIResourcesScheduler].Start()
}

func (c *CronClient) InitJobScheduler() {

	c.Schedulers[UpsertWorkflowScheduler] = gocron.NewScheduler()

	c.Schedulers[UpsertWorkflowScheduler].Every(1).Minutes().Do(c.UpsertWorkflowScheduler, c.log)

	c.Schedulers[UpsertWorkflowScheduler].Start()
}

func (c *CronClient) InitTestScheduler() {

	c.Schedulers[UpsertTestScheduler] = gocron.NewScheduler()

	c.Schedulers[UpsertTestScheduler].Every(1).Minutes().Do(c.UpsertTestScheduler, c.log)

	c.Schedulers[UpsertTestScheduler].Start()
}

func (c *CronClient) InitColliePipelineScheduler() {

	c.Schedulers[UpsertColliePipelineScheduler] = gocron.NewScheduler()

	c.Schedulers[UpsertColliePipelineScheduler].Every(1).Minutes().Do(c.UpsertColliePipelineScheduler, c.log)

	c.Schedulers[UpsertColliePipelineScheduler].Start()
}

func (c *CronClient) InitBuildStatScheduler() {
	c.Schedulers[InitStatScheduler] = gocron.NewScheduler()

	c.Schedulers[InitStatScheduler].Every(1).Day().At("01:00").Do(c.AslanCli.InitStatData, c.log)

	c.Schedulers[InitStatScheduler].Start()
}

func (c *CronClient) InitOperationStatScheduler() {

	c.Schedulers[InitOperationStatScheduler] = gocron.NewScheduler()

	c.Schedulers[InitOperationStatScheduler].Every(1).Hour().Do(c.AslanCli.InitOperationStatData, c.log)

	c.Schedulers[InitOperationStatScheduler].Start()
}

func (c *CronClient) InitPullSonarStatScheduler() {

	c.Schedulers[InitPullSonarStatScheduler] = gocron.NewScheduler()

	c.Schedulers[InitPullSonarStatScheduler].Every(10).Minutes().Do(c.AslanCli.InitPullSonarStatScheduler, c.log)

	c.Schedulers[InitPullSonarStatScheduler].Start()
}

func (c *CronClient) InitSystemCapacityGCScheduler() {

	c.Schedulers[SystemCapacityGC] = gocron.NewScheduler()

	c.Schedulers[SystemCapacityGC].Every(1).Day().At("02:00").Do(c.AslanCli.TriggerCleanCache, c.log)

	c.Schedulers[SystemCapacityGC].Start()
}

func (c *CronClient) InitHealthCheckScheduler() {

	c.Schedulers[InitHealthCheckScheduler] = gocron.NewScheduler()

	c.Schedulers[InitHealthCheckScheduler].Every(10).Seconds().Do(c.UpsertEnvServiceScheduler, c.log)

	c.Schedulers[InitHealthCheckScheduler].Start()
}
