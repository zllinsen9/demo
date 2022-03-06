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
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"
	"k8s.io/client-go/informers"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/pm"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

type PMService struct {
	log *zap.SugaredLogger
}

func (p *PMService) queryServiceStatus(namespace, envName, productName string, serviceTmpl *commonmodels.Service, informer informers.SharedInformerFactory) (string, string, []string) {
	p.log.Infof("queryServiceStatus of service: %s of product: %s in namespace %s", serviceTmpl.ServiceName, productName, namespace)
	pipelineName := fmt.Sprintf("%s-%s-%s", serviceTmpl.ServiceName, envName, "job")
	taskObj, err := commonrepo.NewTaskColl().FindTask(pipelineName, config.ServiceType)
	if err != nil {
		return setting.PodError, setting.PodNotReady, []string{}
	}
	if taskObj.Status == setting.PodCreated {
		return setting.PodPending, setting.PodNotReady, []string{}
	}

	return queryPodsStatus(namespace, "", serviceTmpl.ServiceName, informer, p.log)
}

func (p *PMService) updateService(args *SvcOptArgs) error {
	svc := &commonmodels.ProductService{
		ServiceName: args.ServiceName,
		ProductName: args.ProductName,
		Type:        args.ServiceType,
		Revision:    args.ServiceRev.NextRevision,
		Containers:  args.ServiceRev.Containers,
	}
	opt := &commonrepo.ProductFindOptions{Name: args.ProductName, EnvName: args.EnvName}
	exitedProd, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		p.log.Error(err)
		return errors.New(e.UpsertServiceErrMsg)
	}

	// 更新产品服务
	for _, group := range exitedProd.Services {
		for i, service := range group {
			if service.ServiceName == args.ServiceName && service.Type == args.ServiceType {
				group[i] = svc
			}
		}
	}

	if err := commonrepo.NewProductColl().Update(exitedProd); err != nil {
		p.log.Errorf("[%s][%s] Product.Update error: %v", args.EnvName, args.ProductName, err)
		return e.ErrUpdateProduct
	}
	return nil
}

func (p *PMService) listGroupServices(allServices []*commonmodels.ProductService, envName, productName string, informer informers.SharedInformerFactory, productInfo *commonmodels.Product) []*commonservice.ServiceResp {
	var wg sync.WaitGroup
	var resp []*commonservice.ServiceResp
	var mutex sync.RWMutex

	for _, service := range allServices {
		wg.Add(1)
		go func(service *commonmodels.ProductService) {
			defer wg.Done()
			gp := &commonservice.ServiceResp{
				ServiceName: service.ServiceName,
				Type:        service.Type,
				EnvName:     envName,
				Revision:    service.Revision,
			}
			serviceTmpl, err := commonservice.GetServiceTemplate(
				service.ServiceName, setting.PMDeployType, service.ProductName, "", service.Revision, p.log,
			)

			for _, envconfig := range serviceTmpl.EnvConfigs {
				if envconfig.EnvName == envName {
					gp.EnvConfigs = []*models.EnvConfig{envconfig}
					break
				}
			}

			if err != nil {
				gp.Status = setting.PodFailed
				mutex.Lock()
				resp = append(resp, gp)
				mutex.Unlock()
				return
			}

			gp.ProductName = serviceTmpl.ProductName
			if len(serviceTmpl.EnvStatuses) > 0 {
				envStatuses := make([]*commonmodels.EnvStatus, 0)
				for _, envStatus := range serviceTmpl.EnvStatuses {
					if envStatus.EnvName == envName {
						envStatuses = append(envStatuses, envStatus)
					}
				}
				if len(envStatuses) > 0 {
					gp.EnvStatuses = envStatuses
					mutex.Lock()
					resp = append(resp, gp)
					mutex.Unlock()
					return
				}
			}

			mutex.Lock()
			resp = append(resp, gp)
			mutex.Unlock()
		}(service)
	}

	wg.Wait()

	//把数据按照名称排序
	sort.SliceStable(resp, func(i, j int) bool { return resp[i].ServiceName < resp[j].ServiceName })

	return resp
}

func (p *PMService) createGroup(envName, productName, username string, group []*commonmodels.ProductService, renderSet *commonmodels.RenderSet, inf informers.SharedInformerFactory, kubeClient client.Client) error {
	p.log.Infof("[Namespace:%s][Product:%s] createGroup", envName, productName)

	// 异步创建无依赖的服务
	errList := &multierror.Error{}

	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		errList = multierror.Append(errList, err)
	}
	for _, productService := range group {
		//更新非k8s服务
		if len(productService.EnvConfigs) > 0 {
			serviceTempl, err := commonservice.GetServiceTemplate(productService.ServiceName, setting.PMDeployType, productName, setting.ProductStatusDeleting, productService.Revision, p.log)
			if err != nil {
				errList = multierror.Append(errList, err)
			}
			if serviceTempl != nil {
				oldEnvConfigs := serviceTempl.EnvConfigs
				newEnvConfigs := []*commonmodels.EnvConfig{}
				// rm not exist env
				for _, v := range oldEnvConfigs {
					if _, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
						Name:    productName,
						EnvName: v.EnvName,
					}); err == nil {
						if envName != v.EnvName {
							newEnvConfigs = append(newEnvConfigs, v)
						}
					}
				}
				for _, currentEnvConfig := range productService.EnvConfigs {
					envConfig := &commonmodels.EnvConfig{
						EnvName: currentEnvConfig.EnvName,
						HostIDs: currentEnvConfig.HostIDs,
						Labels:  currentEnvConfig.Labels,
					}
					newEnvConfigs = append(newEnvConfigs, envConfig)
				}

				changeEnvStatus, err := pm.GenerateEnvStatus(newEnvConfigs, log.NopSugaredLogger())
				if err != nil {
					log.Errorf("GenerateEnvStatus err:%s", err)
					return err
				}
				args := &commonservice.ServiceTmplBuildObject{
					ServiceTmplObject: &commonservice.ServiceTmplObject{
						ProductName:  serviceTempl.ProductName,
						ServiceName:  serviceTempl.ServiceName,
						Visibility:   serviceTempl.Visibility,
						Revision:     serviceTempl.Revision,
						Type:         serviceTempl.Type,
						Username:     username,
						HealthChecks: serviceTempl.HealthChecks,
						EnvConfigs:   newEnvConfigs,
						EnvStatuses:  changeEnvStatus,
						From:         "createEnv",
					},
					Build: &commonmodels.Build{Name: serviceTempl.BuildName},
				}

				if err := commonservice.UpdatePmServiceTemplate(username, args, p.log); err != nil {
					errList = multierror.Append(errList, err)
				}
			}
		}
		var latestRevision int64 = productService.Revision
		// 获取最新版本的服务
		if latestServiceTempl, _ := commonservice.GetServiceTemplate(productService.ServiceName, setting.PMDeployType, productName, setting.ProductStatusDeleting, 0, p.log); latestServiceTempl != nil {
			latestRevision = latestServiceTempl.Revision
		}
		// 更新环境
		if latestRevision > productService.Revision {
			// 更新产品服务
			for _, serviceGroup := range prod.Services {
				for j, service := range serviceGroup {
					if service.ServiceName == productService.ServiceName && service.Type == setting.PMDeployType {
						serviceGroup[j].Revision = latestRevision
					}
				}
			}
			if err := commonrepo.NewProductColl().Update(prod); err != nil {
				log.Errorf("[%s][%s] Product.Update error: %v", envName, productName, err)
				errList = multierror.Append(errList, err)
			}
		}
		if _, err = workflow.CreateServiceTask(&commonmodels.ServiceTaskArgs{
			ProductName:        productName,
			ServiceName:        productService.ServiceName,
			Revision:           latestRevision,
			EnvNames:           []string{envName},
			ServiceTaskCreator: username,
		}, p.log); err != nil {
			_, messageMap := e.ErrorMessage(err)
			if description, ok := messageMap["description"]; ok {
				log.Errorf("description :%s", description)
			}
			errList = multierror.Append(errList, err)
		}
	}
	// 如果创建依赖服务组有返回错误, 停止等待
	if err := errList.ErrorOrNil(); err != nil {
		return err
	}

	return nil
}
