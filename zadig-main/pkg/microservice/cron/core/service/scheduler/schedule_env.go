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
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/jasonlvhit/gocron"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/cron/core/service"
	"github.com/koderover/zadig/pkg/setting"
)

// UpsertEnvServiceScheduler ...
func (c *CronClient) UpsertEnvServiceScheduler(log *zap.SugaredLogger) {
	envs, err := c.AslanCli.ListEnvs(log)
	if err != nil {
		log.Error(err)
		return
	}
	//当前的环境数据和上次做比较，如果环境有删除或者环境中的服务有删除，要清理掉定时器
	c.compareProductRevision(envs, log)

	log.Info("start init env scheduler..")
	taskMap := make(map[string]bool)
	for _, env := range envs {
		for _, serviceRevision := range env.ServiceRevisions {
			if serviceRevision.Type != setting.PMDeployType {
				continue
			}
			envObj, err := c.AslanCli.GetEnvService(env.ProductName, env.EnvName, log)
			if err != nil {
				log.Error("GetEnvService productName:%s envName:%s err:%v", env.ProductName, env.EnvName, err)
				continue
			}
			envServiceNames := sets.String{}
			for _, serviceNames := range envObj.Services {
				envServiceNames.Insert(serviceNames...)
			}
			svc, _ := c.AslanCli.GetService(serviceRevision.ServiceName, env.ProductName, setting.PMDeployType, serviceRevision.CurrentRevision, log)
			if svc == nil || len(svc.HealthChecks) == 0 || len(svc.EnvConfigs) == 0 || !envServiceNames.Has(serviceRevision.ServiceName) {
				key := "service-" + serviceRevision.ServiceName + "-" + setting.PMDeployType + "-" +
					env.EnvName
				for scheduleKey := range c.Schedulers {
					if strings.Contains(scheduleKey, key) {
						c.Schedulers[scheduleKey].Clear()
						delete(c.Schedulers, scheduleKey)
					}
				}

				for lastScheduleKey := range c.lastSchedulers {
					if strings.Contains(lastScheduleKey, key) {
						delete(c.lastSchedulers, lastScheduleKey)
					}
				}
				continue
			}
			for _, envStatus := range svc.EnvStatuses {
				if envStatus.EnvName != env.EnvName {
					continue
				}

				for _, healthCheck := range svc.HealthChecks {
					key := "service-" + serviceRevision.ServiceName + "-" + setting.PMDeployType + "-" +
						env.EnvName + "-" + envStatus.HostID + "-" + healthCheck.Protocol + "-" + strconv.Itoa(healthCheck.Port) + "-" + healthCheck.Path
					taskMap[key] = true
					if _, ok := c.lastServiceSchedulers[key]; ok && reflect.DeepEqual(serviceRevision, c.lastServiceSchedulers[key]) {
						continue
					}
					c.lastServiceSchedulers[key] = serviceRevision

					newScheduler := gocron.NewScheduler()
					BuildScheduledEnvJob(newScheduler, healthCheck).Do(c.RunScheduledService, svc, healthCheck, envStatus.Address, env.EnvName, envStatus.HostID, log)
					c.Schedulers[key] = newScheduler
					log.Infof("[%s] service schedulers..", key)
					c.Schedulers[key].Start()
				}
			}
			break
		}
	}
}

func (c *CronClient) RunScheduledService(svc *service.Service, healthCheck *service.PmHealthCheck, address, envName, hostID string, log *zap.SugaredLogger) {
	log.Infof("[%s]start to Run ScheduledService...", svc.ServiceName)
	var (
		message   string
		err       error
		envStatus = new(service.EnvStatus)
	)

	for i := 0; i < MaxProbeRetries; i++ {
		if message, err = runProbe(healthCheck, address, log); err == nil {
			log.Infof("runProbe message:[%s]", message)
			break
		}
	}

	switch message {
	case Success:
		healthCheck.CurrentHealthyNum++
		healthCheck.CurrentUnhealthyNum = 0
	case Failure:
		healthCheck.CurrentUnhealthyNum++
		healthCheck.CurrentHealthyNum = 0
	}

	envStatus.EnvName = envName
	envStatus.Address = address
	envStatus.HostID = hostID
	envStatus.PmHealthCheck = healthCheck
	if healthCheck.CurrentHealthyNum >= healthCheck.HealthyThreshold && healthCheck.CurrentHealthyNum > 0 {
		healthCheck.CurrentHealthyNum = 0
		healthCheck.CurrentUnhealthyNum = 0
		envStatus.Status = setting.PodRunning
	}

	if healthCheck.CurrentUnhealthyNum >= healthCheck.UnhealthyThreshold && healthCheck.CurrentUnhealthyNum > 0 {
		healthCheck.CurrentHealthyNum = 0
		healthCheck.CurrentUnhealthyNum = 0
		envStatus.Status = setting.PodError
	}

	if len(svc.EnvStatuses) == 0 {
		svc.EnvStatuses = []*service.EnvStatus{envStatus}
	} else {
		envStatusKeys := sets.String{}
		for _, tmpEnvStatus := range svc.EnvStatuses {
			//envStatusKeys = append(envStatusKeys, key)
			//if tmpEnvStatus.PmHealthCheck.Protocol == healthCheck.Protocol && tmpEnvStatus.Address == envStatus.Address && tmpEnvStatus.PmHealthCheck.Port == healthCheck.Port &&
			//	tmpEnvStatus.PmHealthCheck.Path == healthCheck.Path && tmpEnvStatus.EnvName == envStatus.EnvName {
			//	tmpEnvStatus.Status = envStatus.Status
			//}
			if tmpEnvStatus.HostID != hostID {
				continue
			}
			key := fmt.Sprintf("%s-%s-%d-%s-%s", healthCheck.Protocol, tmpEnvStatus.Address, healthCheck.Port, healthCheck.Path, tmpEnvStatus.EnvName)
			envStatusKeys.Insert(key)
			if tmpEnvStatus.Address == "" {
				tmpEnvStatus.Address = envStatus.Address
			}

			tmpEnvStatus.Status = envStatus.Status
		}
		currentEnvStatusKey := fmt.Sprintf("%s-%s-%d-%s-%s", healthCheck.Protocol, envStatus.Address, healthCheck.Port, healthCheck.Path, envStatus.EnvName)
		if !envStatusKeys.Has(currentEnvStatusKey) {
			svc.EnvStatuses = append(svc.EnvStatuses, envStatus)
		}
	}

	if err = c.AslanCli.UpdateService(&service.ServiceTmplObject{
		ProductName: svc.ProductName,
		ServiceName: svc.ServiceName,
		Revision:    svc.Revision,
		Type:        setting.PMDeployType,
		EnvStatuses: svc.EnvStatuses,
		Username:    "system",
	}, log); err != nil {
		log.Errorf("UpdateService err:%v", err)
	} else {
		log.Infof("ready to UpdateService serviceName:%s，revision:%d，envName:%s，address:%s，status:%s", svc.ServiceName, svc.Revision, envStatus.EnvName, envStatus.Address, envStatus.Status)
	}
}

// BuildScheduledEnvJob ...
func BuildScheduledEnvJob(scheduler *gocron.Scheduler, healthCheck *service.PmHealthCheck) *gocron.Job {
	interval := healthCheck.Interval
	if interval < 2 {
		interval = 2
	}
	return scheduler.Every(interval).Seconds()
}
func runProbe(healthCheck *service.PmHealthCheck, address string, log *zap.SugaredLogger) (string, error) {
	var (
		message string
		err     error
	)
	timeout := time.Duration(healthCheck.TimeOut) * time.Second
	switch healthCheck.Protocol {
	case setting.ProtocolHTTP, setting.ProtocolHTTPS:
		if message, err = doHTTPProbe(healthCheck.Protocol, address, healthCheck.Path, healthCheck.Port, timeout, log); err != nil {
			log.Errorf("doHttpProbe err:%v", err)
			return Failure, err
		}
	case setting.ProtocolTCP:
		if message, err = doTCPProbe(address, healthCheck.Port, timeout, log); err != nil {
			log.Errorf("doTCPProbe err:%v", err)
			return Failure, err
		}
	}

	return message, nil
}
func doTCPProbe(addr string, port int, timeout time.Duration, log *zap.SugaredLogger) (string, error) {
	var (
		conn net.Conn
		err  error
	)
	if port == 0 {
		conn, err = net.DialTimeout("tcp", addr, timeout)
	} else {
		conn, err = net.DialTimeout("tcp", fmt.Sprintf("%s:%d", addr, port), timeout)
	}

	if err != nil {
		return Failure, err
	}
	err = conn.Close()
	if err != nil {
		log.Errorf("Unexpected error closing TCP socket: %v (%#v)", err, err)
		return Failure, err
	}
	return Success, nil
}

func doHTTPProbe(protocol, address, path string, port int, timeout time.Duration, log *zap.SugaredLogger) (string, error) {
	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	transport := &http.Transport{
		TLSClientConfig:   tlsConfig,
		DisableKeepAlives: true,
		Proxy:             http.ProxyURL(nil),
	}
	client := &http.Client{
		Timeout:       timeout,
		Transport:     transport,
		CheckRedirect: redirectChecker(false),
	}
	url, err := formatURL(protocol, address, path, port)
	if err != nil {
		return Failure, err
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return Failure, err
	}
	res, err := client.Do(req)
	if err != nil {
		return Failure, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return Failure, err
	}

	if res.StatusCode >= http.StatusOK && res.StatusCode < http.StatusBadRequest {
		log.Infof("Probe succeeded for %s, Response: %v", url, *res)
		return Success, nil
	}
	log.Infof("Probe failed for %s, response body: %v", url, string(body))
	return Failure, fmt.Errorf("HTTP probe failed with statuscode: %d", res.StatusCode)
}

func redirectChecker(followNonLocalRedirects bool) func(*http.Request, []*http.Request) error {
	if followNonLocalRedirects {
		return nil // Use the default http client checker.
	}

	return func(req *http.Request, via []*http.Request) error {
		if req.URL.Hostname() != via[0].URL.Hostname() {
			return http.ErrUseLastResponse
		}
		// Default behavior: stop after 10 redirects.
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
}

func formatURL(protocol, address, path string, port int) (string, error) {
	if len(strings.Split(address, ":")) > 2 {
		return "", fmt.Errorf("illegal address")
	}
	if path == "" && port == 0 {
		return fmt.Sprintf("%s://%s", protocol, address), nil
	}

	path = strings.TrimPrefix(path, "/")

	if port == 0 {
		return fmt.Sprintf("%s://%s/%s", protocol, address, path), nil
	}
	return fmt.Sprintf("%s://%s:%d/%s", protocol, address, port, path), nil
}

// 先比较环境，再比较服务
func (c *CronClient) compareProductRevision(currentProductRevisions []*service.ProductRevision, log *zap.SugaredLogger) {
	if len(c.lastProductRevisions) == 0 {
		c.lastProductRevisions = currentProductRevisions
		return
	}
	deleteProductRevisions := make([]*service.ProductRevision, 0)
	lastProductSvcRevisionMap := make(map[string][]*service.SvcRevision)
	currentProductSvcRevisionMap := make(map[string][]*service.SvcRevision)
	for _, lastProductRevision := range c.lastProductRevisions {
		isContain := false
		for _, currentProductRevision := range currentProductRevisions {
			if currentProductRevision.ProductName == lastProductRevision.ProductName &&
				currentProductRevision.EnvName == lastProductRevision.EnvName {

				currentProductSvcRevisionMap[currentProductRevision.ProductName+"-"+currentProductRevision.EnvName] = currentProductRevision.ServiceRevisions
				lastProductSvcRevisionMap[lastProductRevision.ProductName+"-"+lastProductRevision.EnvName] = lastProductRevision.ServiceRevisions
				isContain = true
				break
			}
		}

		if !isContain {
			deleteProductRevisions = append(deleteProductRevisions, lastProductRevision)
		}
	}
	//已经删除的环境，如果包含非k8s服务，则清除定时器
	for _, env := range deleteProductRevisions {
		for _, serviceRevision := range env.ServiceRevisions {
			if serviceRevision.Type != setting.PMDeployType {
				continue
			}
			key := "service-" + serviceRevision.ServiceName + "-" + setting.PMDeployType + "-" +
				env.EnvName
			for scheduleKey := range c.Schedulers {
				if strings.Contains(scheduleKey, key) {
					c.Schedulers[scheduleKey].Clear()
					delete(c.Schedulers, scheduleKey)
				}
			}

			for lastScheduleKey := range c.lastSchedulers {
				if strings.Contains(lastScheduleKey, key) {
					delete(c.lastSchedulers, lastScheduleKey)
				}
			}

			for lastServiceSchedulerKey := range c.lastServiceSchedulers {
				if strings.Contains(lastServiceSchedulerKey, key) {
					delete(c.lastServiceSchedulers, lastServiceSchedulerKey)
				}
			}
		}
	}
	// 已经删除的服务，如果是非k8s，则清除定时器
	deleteSvcRevisionsMap := make(map[string][]*service.SvcRevision)
	lastServiceMap := make(map[string]*service.SvcRevision)
	currentServicesMap := make(map[string]*service.SvcRevision)
	for key, lastSvcRevisions := range lastProductSvcRevisionMap {
		currentSvcRevisions := currentProductSvcRevisionMap[key]
		for _, lastSvcRevision := range lastSvcRevisions {
			isContain := false
			for _, currentSvcRevision := range currentSvcRevisions {
				if lastSvcRevision.ServiceName == currentSvcRevision.ServiceName {
					lastServiceMap[key+"-"+lastSvcRevision.ServiceName] = lastSvcRevision
					currentServicesMap[key+"-"+currentSvcRevision.ServiceName] = currentSvcRevision
					isContain = true
					break
				}
			}
			if !isContain {
				deleteSvcRevisionsMap[key] = append(deleteSvcRevisionsMap[key], lastSvcRevision)
			}
		}
	}

	for key, deleteSvcRevisions := range deleteSvcRevisionsMap {
		envName := strings.Split(key, "-")[1]
		for _, deleteSvcRevision := range deleteSvcRevisions {
			if deleteSvcRevision.Type != setting.PMDeployType {
				continue
			}
			key := "service-" + deleteSvcRevision.ServiceName + "-" + setting.PMDeployType + "-" +
				envName
			for scheduleKey := range c.Schedulers {
				if strings.Contains(scheduleKey, key) {
					c.Schedulers[scheduleKey].Clear()
					delete(c.Schedulers, scheduleKey)
				}
			}

			for lastScheduleKey := range c.lastSchedulers {
				if strings.Contains(lastScheduleKey, key) {
					delete(c.lastSchedulers, lastScheduleKey)
				}
			}

			for lastServiceSchedulerKey := range c.lastServiceSchedulers {
				if strings.Contains(lastServiceSchedulerKey, key) {
					delete(c.lastServiceSchedulers, lastServiceSchedulerKey)
				}
			}
		}
	}

	//判断相同的服务，revision是否相同，如果revision相同在判断env_configs和health_checks是否相同
	//找出老的revision的service
	oldRevisionServices := make(map[string]*service.SvcRevision)
	for lastKey, lastServiceRevison := range lastServiceMap {
		currentRevison := currentServicesMap[lastKey]
		if lastServiceRevison.CurrentRevision < currentRevison.CurrentRevision {
			oldRevisionServices[lastKey] = lastServiceRevison
		}
	}
	//清理掉老版本的探活定时器
	for key, oldRevisionService := range oldRevisionServices {
		if oldRevisionService.Type != setting.PMDeployType {
			continue
		}
		envName := strings.Split(key, "-")[1]
		key := "service-" + oldRevisionService.ServiceName + "-" + setting.PMDeployType + "-" +
			envName
		for scheduleKey := range c.Schedulers {
			if strings.Contains(scheduleKey, key) {
				c.Schedulers[scheduleKey].Clear()
				delete(c.Schedulers, scheduleKey)
			}
		}

		for lastScheduleKey := range c.lastSchedulers {
			if strings.Contains(lastScheduleKey, key) {
				delete(c.lastSchedulers, lastScheduleKey)
			}
		}

		for lastServiceSchedulerKey := range c.lastServiceSchedulers {
			if strings.Contains(lastServiceSchedulerKey, key) {
				delete(c.lastServiceSchedulers, lastServiceSchedulerKey)
			}
		}
	}

	c.lastProductRevisions = currentProductRevisions
}

const (
	// Success Result
	Success string = "success"
	// Failure Result
	Failure string = "failure"
	// maxProbeRetries
	MaxProbeRetries = 3
)
