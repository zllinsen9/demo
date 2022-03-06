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

package taskplugin

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	helmclient "github.com/mittwald/go-helm-client"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/releaseutil"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/taskplugin/s3"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/shared/kube/resource"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	helmtool "github.com/koderover/zadig/pkg/tool/helmclient"
	"github.com/koderover/zadig/pkg/tool/httpclient"
	krkubeclient "github.com/koderover/zadig/pkg/tool/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
	"github.com/koderover/zadig/pkg/util"
	"github.com/koderover/zadig/pkg/util/converter"
	fsutil "github.com/koderover/zadig/pkg/util/fs"
	yamlutil "github.com/koderover/zadig/pkg/util/yaml"
)

// InitializeDeployTaskPlugin to initiate deploy task plugin and return ref
func InitializeDeployTaskPlugin(taskType config.TaskType) TaskPlugin {
	return &DeployTaskPlugin{
		Name:       taskType,
		kubeClient: krkubeclient.Client(),
		restConfig: krkubeclient.RESTConfig(),
		httpClient: httpclient.New(
			httpclient.SetHostURL(configbase.AslanServiceAddress()),
		),
	}
}

// DeployTaskPlugin Plugin name should be compatible with task type
type DeployTaskPlugin struct {
	Name         config.TaskType
	JobName      string
	kubeClient   client.Client
	restConfig   *rest.Config
	Task         *task.Deploy
	Log          *zap.SugaredLogger
	ReplaceImage string

	httpClient *httpclient.Client
}

func (p *DeployTaskPlugin) SetAckFunc(func()) {
}

const (
	imageUrlParseRegexString = `(?P<repo>.+/)?(?P<image>[^:]+){1}(:)?(?P<tag>.+)?`
)

var (
	imageParseRegex = regexp.MustCompile(imageUrlParseRegexString)
)

// Init ...
func (p *DeployTaskPlugin) Init(jobname, filename string, xl *zap.SugaredLogger) {
	p.JobName = jobname
	// SetLogger ...
	p.Log = xl
}

// Type ...
func (p *DeployTaskPlugin) Type() config.TaskType {
	return p.Name
}

// Status ...
func (p *DeployTaskPlugin) Status() config.Status {
	return p.Task.TaskStatus
}

// SetStatus ...
func (p *DeployTaskPlugin) SetStatus(status config.Status) {
	p.Task.TaskStatus = status
}

// TaskTimeout ...
func (p *DeployTaskPlugin) TaskTimeout() int {
	if p.Task.Timeout == 0 {
		p.Task.Timeout = setting.DeployTimeout
	} else {
		if !p.Task.IsRestart {
			p.Task.Timeout = p.Task.Timeout * 60
		}
	}

	return p.Task.Timeout
}

type EnvArgs struct {
	EnvName     string `json:"env_name"`
	ProductName string `json:"product_name"`
}

type ResourceComponentSet interface {
	GetName() string
	GetAnnotations() map[string]string
	GetContainers() []*resource.ContainerImage
	GetKind() string
}

func RcsListFromDeployments(source []*appsv1.Deployment) []ResourceComponentSet {
	rcsList := make([]ResourceComponentSet, 0, len(source))
	for _, deploy := range source {
		rcsList = append(rcsList, wrapper.Deployment(deploy))
	}
	return rcsList
}

func RcsListFromStatefulSets(source []*appsv1.StatefulSet) []ResourceComponentSet {
	rcsList := make([]ResourceComponentSet, 0, len(source))
	for _, sfs := range source {
		rcsList = append(rcsList, wrapper.StatefulSet(sfs))
	}
	return rcsList
}

// find affected resources(deployment+statefulSet) for helm install or upgrade
// resource type: deployment statefulSet
func (p *DeployTaskPlugin) findHelmAffectedResources(namespace, serviceName string, resList []ResourceComponentSet) {
	for _, res := range resList {
		annotation := res.GetAnnotations()
		if len(annotation) == 0 {
			continue
		}
		// filter by services
		if chartRelease, ok := annotation[setting.HelmReleaseNameAnnotation]; ok {
			extractedServiceName := util.ExtraServiceName(chartRelease, namespace)
			if extractedServiceName != serviceName {
				continue
			}
		}
		for _, container := range res.GetContainers() {
			resolvedImageUrl := resolveImageUrl(container.Image)
			if resolvedImageUrl[setting.PathSearchComponentImage] == p.Task.ContainerName {
				p.Log.Infof("%s find match container.name:%s container.image:%s", res.GetKind(), container.Name, container.Image)
				p.Task.ReplaceResources = append(p.Task.ReplaceResources, task.Resource{
					Kind:      res.GetKind(),
					Container: container.Name,
					Origin:    container.Image,
					Name:      res.GetName(),
				})
			}
		}
	}
}

func (p *DeployTaskPlugin) Run(ctx context.Context, pipelineTask *task.Task, _ *task.PipelineCtx, _ string) {
	var (
		err      error
		replaced = false
	)

	defer func() {
		if err != nil {
			p.Log.Error(err)
			p.Task.TaskStatus = config.StatusFailed
			p.Task.Error = err.Error()
			return
		}
	}()

	if pipelineTask.ConfigPayload.DeployClusterID != "" {
		p.restConfig, err = kubeclient.GetRESTConfig(pipelineTask.ConfigPayload.HubServerAddr, pipelineTask.ConfigPayload.DeployClusterID)
		if err != nil {
			err = errors.WithMessage(err, "can't get k8s rest config")
			return
		}

		p.kubeClient, err = kubeclient.GetKubeClient(pipelineTask.ConfigPayload.HubServerAddr, pipelineTask.ConfigPayload.DeployClusterID)
		if err != nil {
			err = errors.WithMessage(err, "can't init k8s client")
			return
		}
	}
	containerName := p.Task.ContainerName
	containerName = strings.TrimSuffix(containerName, "_"+p.Task.ServiceName)
	if p.Task.ServiceType != setting.HelmDeployType {
		// get servcie info
		var (
			serviceInfo *types.ServiceTmpl
			selector    labels.Selector
		)
		serviceInfo, err = p.getService(ctx, p.Task.ServiceName, p.Task.ServiceType, p.Task.ProductName, 0)
		if err != nil {
			// Maybe it is a share service, the entity is not under the project
			serviceInfo, err = p.getService(ctx, p.Task.ServiceName, p.Task.ServiceType, "", 0)
			if err != nil {
				return
			}
		}
		if serviceInfo.WorkloadType == "" {
			selector := labels.Set{setting.ProductLabel: p.Task.ProductName, setting.ServiceLabel: p.Task.ServiceName}.AsSelector()

			var deployments []*appsv1.Deployment
			deployments, err = getter.ListDeployments(p.Task.Namespace, selector, p.kubeClient)
			if err != nil {
				return
			}

			var statefulSets []*appsv1.StatefulSet
			statefulSets, err = getter.ListStatefulSets(p.Task.Namespace, selector, p.kubeClient)
			if err != nil {
				return
			}

		L:
			for _, deploy := range deployments {
				for _, container := range deploy.Spec.Template.Spec.Containers {
					if container.Name == containerName {
						err = updater.UpdateDeploymentImage(deploy.Namespace, deploy.Name, containerName, p.Task.Image, p.kubeClient)
						if err != nil {
							err = errors.WithMessagef(
								err,
								"failed to update container image in %s/deployments/%s/%s",
								p.Task.Namespace, deploy.Name, container.Name)
							return
						}
						p.Task.ReplaceResources = append(p.Task.ReplaceResources, task.Resource{
							Kind:      setting.Deployment,
							Container: container.Name,
							Origin:    container.Image,
							Name:      deploy.Name,
						})
						replaced = true
						break L
					}
				}
			}
		Loop:
			for _, sts := range statefulSets {
				for _, container := range sts.Spec.Template.Spec.Containers {
					if container.Name == containerName {
						err = updater.UpdateStatefulSetImage(sts.Namespace, sts.Name, containerName, p.Task.Image, p.kubeClient)
						if err != nil {
							err = errors.WithMessagef(
								err,
								"failed to update container image in %s/statefulsets/%s/%s",
								p.Task.Namespace, sts.Name, container.Name)
							return
						}
						p.Task.ReplaceResources = append(p.Task.ReplaceResources, task.Resource{
							Kind:      setting.StatefulSet,
							Container: container.Name,
							Origin:    container.Image,
							Name:      sts.Name,
						})
						replaced = true
						break Loop
					}
				}
			}
		} else {
			switch serviceInfo.WorkloadType {
			case setting.StatefulSet:
				var statefulSet *appsv1.StatefulSet
				statefulSet, _, err = getter.GetStatefulSet(p.Task.Namespace, p.Task.ServiceName, p.kubeClient)
				if err != nil {
					return
				}
				for _, container := range statefulSet.Spec.Template.Spec.Containers {
					if container.Name == containerName {
						err = updater.UpdateStatefulSetImage(statefulSet.Namespace, statefulSet.Name, containerName, p.Task.Image, p.kubeClient)
						if err != nil {
							err = errors.WithMessagef(
								err,
								"failed to update container image in %s/statefulsets/%s/%s",
								p.Task.Namespace, statefulSet.Name, container.Name)
							return
						}
						p.Task.ReplaceResources = append(p.Task.ReplaceResources, task.Resource{
							Kind:      setting.StatefulSet,
							Container: container.Name,
							Origin:    container.Image,
							Name:      statefulSet.Name,
						})
						replaced = true
						break
					}
				}
			case setting.Deployment:
				var deployment *appsv1.Deployment
				deployment, _, err = getter.GetDeployment(p.Task.Namespace, p.Task.ServiceName, p.kubeClient)
				if err != nil {
					return
				}
				for _, container := range deployment.Spec.Template.Spec.Containers {
					if container.Name == containerName {
						err = updater.UpdateDeploymentImage(deployment.Namespace, deployment.Name, containerName, p.Task.Image, p.kubeClient)
						if err != nil {
							err = errors.WithMessagef(
								err,
								"failed to update container image in %s/deployments/%s/%s",
								p.Task.Namespace, deployment.Name, container.Name)
							return
						}
						p.Task.ReplaceResources = append(p.Task.ReplaceResources, task.Resource{
							Kind:      setting.Deployment,
							Container: container.Name,
							Origin:    container.Image,
							Name:      deployment.Name,
						})
						replaced = true
						break
					}
				}
			}
		}
		if !replaced {
			err = errors.Errorf(
				"container %s is not found in resources with label %s", containerName, selector)
			return
		}
	} else if p.Task.ServiceType == setting.HelmDeployType {
		var (
			productInfo              *types.Product
			renderChart              *types.RenderChart
			replacedValuesYaml       string
			mergedValuesYaml         string
			replacedMergedValuesYaml string
			servicePath              string
			chartPath                string
			replaceValuesMap         map[string]interface{}
			renderInfo               *types.RenderSet
			helmClient               helmclient.Client
		)

		rcsList := make([]ResourceComponentSet, 0)
		deployments, errFindDeploy := getter.ListDeployments(p.Task.Namespace, nil, p.kubeClient)
		if errFindDeploy != nil {
			p.Log.Errorf("failed to list deployments in namespace %s, productName %s, err %s", p.Task.Namespace, p.Task.ProductName, errFindDeploy)
		} else {
			rcsList = append(rcsList, RcsListFromDeployments(deployments)...)
		}
		statefulSets, errFindSts := getter.ListStatefulSets(p.Task.Namespace, nil, p.kubeClient)
		if errFindSts != nil {
			p.Log.Errorf("failed to list statefulsets in namespace %s, productName %s, err %s", p.Task.Namespace, p.Task.ProductName, errFindSts)
		} else {
			rcsList = append(rcsList, RcsListFromStatefulSets(statefulSets)...)
		}
		p.findHelmAffectedResources(p.Task.Namespace, p.Task.ServiceName, rcsList)

		p.Log.Infof("start helm deploy, productName %s serviceName %s containerName %s namespace %s", p.Task.ProductName,
			p.Task.ServiceName, containerName, p.Task.Namespace)

		productInfo, err = p.getProductInfo(ctx, &EnvArgs{EnvName: p.Task.EnvName, ProductName: p.Task.ProductName})
		if err != nil {
			err = errors.WithMessagef(
				err,
				"failed to get product %s/%s",
				p.Task.Namespace, p.Task.ServiceName)
			return
		}

		renderInfo, err = p.getRenderSet(ctx, productInfo.Render.Name, productInfo.Render.Revision)
		if err != nil {
			err = errors.WithMessagef(
				err,
				"failed to get getRenderSet %s/%d",
				productInfo.Render.Name, productInfo.Render.Revision)
			return
		}

		serviceRevisionInProduct := int64(0)
		var targetContainer *types.Container
		for _, service := range productInfo.GetServiceMap() {
			if service.ServiceName == p.Task.ServiceName {
				serviceRevisionInProduct = service.Revision
				for _, container := range service.Containers {
					if container.Name == containerName {
						targetContainer = container
						break
					}
				}
				break
			}
		}

		if targetContainer == nil {
			err = errors.Errorf("failed to find target container %s from service %s", containerName, p.Task.ServiceName)
			return
		}

		if targetContainer.ImagePath == nil {
			err = errors.Errorf("failed to get image path of  %s from service %s", containerName, p.Task.ServiceName)
			return
		}

		for _, chartInfo := range renderInfo.ChartInfos {
			if chartInfo.ServiceName == p.Task.ServiceName {
				renderChart = chartInfo
				break
			}
		}

		if renderChart == nil {
			err = errors.Errorf("failed to update container image in %s/%s，chart not found",
				p.Task.Namespace, p.Task.ServiceName)
			return
		}

		// use revision of service currently applied in environment instead of the latest revision
		path, errDownload := p.downloadService(pipelineTask.ProductName, p.Task.ServiceName,
			pipelineTask.StorageURI, serviceRevisionInProduct)
		if errDownload != nil {
			p.Log.Warnf("failed to get chart of revision: %d for service: %s, use latest version",
				serviceRevisionInProduct, p.Task.ServiceName)
			path, errDownload = p.downloadService(pipelineTask.ProductName, p.Task.ServiceName,
				pipelineTask.StorageURI, 0)
			if errDownload != nil {
				err = errors.WithMessagef(
					errDownload,
					"failed to download service %s/%s",
					p.Task.Namespace, p.Task.ServiceName)
				return
			}
		}

		chartPath, err = fsutil.RelativeToCurrentPath(path)
		if err != nil {
			err = errors.WithMessagef(
				err,
				"failed to get relative path %s",
				servicePath,
			)
			return
		}

		serviceValuesYaml := renderChart.ValuesYaml

		// prepare image replace info
		validMatchData := getValidMatchData(targetContainer.ImagePath)

		replaceValuesMap, err = assignImageData(p.Task.Image, validMatchData)
		if err != nil {
			err = errors.WithMessagef(
				err,
				"failed to pase image uri %s/%s",
				p.Task.Namespace, p.Task.ServiceName)
			return
		}

		// replace image into service's values.yaml
		replacedValuesYaml, err = replaceImage(serviceValuesYaml, replaceValuesMap)
		if err != nil {
			err = errors.WithMessagef(
				err,
				"failed to replace image uri %s/%s",
				p.Task.Namespace, p.Task.ServiceName)
			return
		}
		if replacedValuesYaml == "" {
			err = errors.Errorf("failed to set new image uri into service's values.yaml %s/%s",
				p.Task.Namespace, p.Task.ServiceName)
			return
		}

		// merge override values and kvs into service's yaml
		mergedValuesYaml, err = helmtool.MergeOverrideValues(serviceValuesYaml, renderInfo.DefaultValues, renderChart.GetOverrideYaml(), renderChart.OverrideValues)
		if err != nil {
			err = errors.WithMessagef(
				err,
				"failed to merge override values %s",
				renderChart.OverrideValues,
			)
			return
		}

		// replace image into final merged values.yaml
		replacedMergedValuesYaml, err = replaceImage(mergedValuesYaml, replaceValuesMap)
		if err != nil {
			err = errors.WithMessagef(
				err,
				"failed to replace image uri into helm values %s/%s",
				p.Task.Namespace, p.Task.ServiceName)
			return
		}
		if replacedMergedValuesYaml == "" {
			err = errors.Errorf("failed to set image uri into mreged values.yaml in %s/%s",
				p.Task.Namespace, p.Task.ServiceName)
			return
		}

		p.Log.Infof("final replaced merged values: \n%s", replacedMergedValuesYaml)

		helmClient, err = helmtool.NewClientFromRestConf(p.restConfig, p.Task.Namespace)
		if err != nil {
			err = errors.WithMessagef(
				err,
				"failed to create helm client %s/%s",
				p.Task.Namespace, p.Task.ServiceName)
			return
		}

		releaseName := util.GeneHelmReleaseName(p.Task.Namespace, p.Task.ServiceName)

		ensureUpgrade := func() error {
			hrs, errHistory := helmClient.ListReleaseHistory(releaseName, 10)
			if errHistory != nil {
				// list history should not block deploy operation, error will be logged instead of returned
				p.Log.Errorf("failed to list release history, release: %s, err: %s", releaseName, errHistory)
				return nil
			}
			if len(hrs) == 0 {
				return nil
			}
			releaseutil.Reverse(hrs, releaseutil.SortByRevision)
			rel := hrs[0]

			if rel.Info.Status.IsPending() {
				return fmt.Errorf("failed to upgrade release: %s with exceptional status: %s", releaseName, rel.Info.Status)
			}
			return nil
		}

		err = ensureUpgrade()
		if err != nil {
			return
		}

		chartSpec := helmclient.ChartSpec{
			ReleaseName: releaseName,
			ChartName:   chartPath,
			Namespace:   p.Task.Namespace,
			ReuseValues: true,
			Version:     renderChart.ChartVersion,
			ValuesYaml:  replacedMergedValuesYaml,
			SkipCRDs:    false,
			UpgradeCRDs: true,
			Timeout:     time.Second * setting.DeployTimeout,
			Wait:        true,
			Replace:     true,
			MaxHistory:  10,
		}

		done := make(chan bool)
		go func(chan bool) {
			if _, err = helmClient.InstallOrUpgradeChart(context.TODO(), &chartSpec); err != nil {
				err = errors.WithMessagef(
					err,
					"failed to Install helm chart %s/%s",
					p.Task.Namespace, p.Task.ServiceName)
				done <- false
			} else {
				done <- true
			}
		}(done)

		select {
		case <-done:
			break
		case <-time.After(chartSpec.Timeout + 30*time.Second):
			err = fmt.Errorf("failed to upgrade relase: %s, timeout", chartSpec.ReleaseName)
		}
		if err != nil {
			return
		}

		//替换环境变量中的chartInfos
		for _, chartInfo := range renderInfo.ChartInfos {
			if chartInfo.ServiceName == p.Task.ServiceName {
				chartInfo.ValuesYaml = replacedValuesYaml
				break
			}
		}

		// TODO too dangerous to override entire renderset!
		err = p.updateRenderSet(ctx, &types.RenderSet{
			Name:          renderInfo.Name,
			Revision:      renderInfo.Revision,
			DefaultValues: renderInfo.DefaultValues,
			ChartInfos:    renderInfo.ChartInfos,
		})
		if err != nil {
			err = errors.WithMessagef(
				err,
				"failed to update renderset info %s/%s, renderset %s",
				p.Task.Namespace, p.Task.ServiceName, renderInfo.Name)
		}
	}
}

func (p *DeployTaskPlugin) getProductInfo(ctx context.Context, args *EnvArgs) (*types.Product, error) {
	url := fmt.Sprintf("/api/environment/environments/%s/productInfo", args.EnvName)

	prod := &types.Product{}
	_, err := p.httpClient.Get(url, httpclient.SetResult(prod), httpclient.SetQueryParam("projectName", args.ProductName))
	if err != nil {
		return nil, err
	}
	return prod, nil
}

func (p *DeployTaskPlugin) getService(ctx context.Context, name, serviceType, productName string, revision int64) (*types.ServiceTmpl, error) {
	url := fmt.Sprintf("/api/service/services/%s/%s", name, serviceType)

	s := &types.ServiceTmpl{}
	_, err := p.httpClient.Get(url, httpclient.SetResult(s), httpclient.SetQueryParams(map[string]string{
		"projectName": productName,
		"revision":    fmt.Sprintf("%d", revision),
	}))
	if err != nil {
		return nil, err
	}
	return s, nil
}

// download chart info of specific version, use the latest version if fails
func (p *DeployTaskPlugin) downloadService(productName, serviceName, storageURI string, revision int64) (string, error) {
	logger := p.Log

	fileName := serviceName
	if revision > 0 {
		fileName = fmt.Sprintf("%s-%d", serviceName, revision)
	}
	tarball := fmt.Sprintf("%s.tar.gz", fileName)
	localBase := configbase.LocalServicePath(productName, serviceName)
	tarFilePath := filepath.Join(localBase, tarball)

	exists, err := fsutil.FileExists(tarFilePath)
	if err != nil {
		return "", err
	}
	if exists {
		return tarFilePath, nil
	}

	s3Storage, err := s3.NewS3StorageFromEncryptedURI(storageURI)
	if err != nil {
		return "", err
	}

	s3Storage.Subfolder = filepath.Join(s3Storage.Subfolder, configbase.ObjectStorageServicePath(productName, serviceName))
	forcedPathStyle := true
	if s3Storage.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}
	s3Client, err := s3tool.NewClient(s3Storage.Endpoint, s3Storage.Ak, s3Storage.Sk, s3Storage.Insecure, forcedPathStyle)
	if err != nil {
		p.Log.Errorf("failed to create s3 client, err: %s", err)
		return "", err
	}
	if err = s3Client.Download(s3Storage.Bucket, s3Storage.GetObjectPath(tarball), tarFilePath); err != nil {
		logger.Errorf("failed to download file from s3, err: %s", err)
		return "", err
	}

	exists, err = fsutil.FileExists(tarFilePath)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", fmt.Errorf("file %s on s3 not found", s3Storage.GetObjectPath(tarball))
	}

	return tarFilePath, nil
}

func (p *DeployTaskPlugin) getRenderSet(ctx context.Context, name string, revision int64) (*types.RenderSet, error) {
	url := fmt.Sprintf("/api/project/renders/render/%s/revision/%d", name, revision)

	rs := &types.RenderSet{}
	_, err := p.httpClient.Get(url, httpclient.SetResult(rs))
	if err != nil {
		return nil, err
	}

	return rs, nil
}

func (p *DeployTaskPlugin) updateRenderSet(ctx context.Context, args *types.RenderSet) error {
	url := "/api/project/renders"

	_, err := p.httpClient.Put(url, httpclient.SetBody(args))

	return err
}

func getValidMatchData(spec *types.ImagePathSpec) map[string]string {
	ret := make(map[string]string)
	if spec.Repo != "" {
		ret[setting.PathSearchComponentRepo] = spec.Repo
	}
	if spec.Image != "" {
		ret[setting.PathSearchComponentImage] = spec.Image
	}
	if spec.Tag != "" {
		ret[setting.PathSearchComponentTag] = spec.Tag
	}
	return ret
}

// parse image url to map: repo=>xxx/xx/xx image=>xx tag=>xxx
func resolveImageUrl(imageUrl string) map[string]string {
	subMatchAll := imageParseRegex.FindStringSubmatch(imageUrl)
	result := make(map[string]string)
	exNames := imageParseRegex.SubexpNames()
	for i, matchedStr := range subMatchAll {
		if i != 0 && matchedStr != "" && matchedStr != ":" {
			result[exNames[i]] = matchedStr
		}
	}
	return result
}

// replace image defines in yaml by new version
func replaceImage(sourceYaml string, imageValuesMap map[string]interface{}) (string, error) {
	nestedMap, err := converter.Expand(imageValuesMap)
	if err != nil {
		return "", err
	}
	bs, err := yaml.Marshal(nestedMap)
	if err != nil {
		return "", err
	}
	mergedBs, err := yamlutil.Merge([][]byte{[]byte(sourceYaml), bs})
	if err != nil {
		return "", err
	}
	return string(mergedBs), nil
}

// assignImageData assign image url data into match data
// matchData: image=>absolute-path repo=>absolute-path tag=>absolute-path
// return: absolute-image-path=>image-value  absolute-repo-path=>repo-value absolute-tag-path=>tag-value
func assignImageData(imageUrl string, matchData map[string]string) (map[string]interface{}, error) {
	ret := make(map[string]interface{})
	// total image url assigned into one single value
	if len(matchData) == 1 {
		for _, v := range matchData {
			ret[v] = imageUrl
		}
		return ret, nil
	}

	resolvedImageUrl := resolveImageUrl(imageUrl)

	// image url assigned into repo/image+tag
	if len(matchData) == 3 {
		ret[matchData[setting.PathSearchComponentRepo]] = strings.TrimSuffix(resolvedImageUrl[setting.PathSearchComponentRepo], "/")
		ret[matchData[setting.PathSearchComponentImage]] = resolvedImageUrl[setting.PathSearchComponentImage]
		ret[matchData[setting.PathSearchComponentTag]] = resolvedImageUrl[setting.PathSearchComponentTag]
		return ret, nil
	}

	if len(matchData) == 2 {
		// image url assigned into repo/image + tag
		if tagPath, ok := matchData[setting.PathSearchComponentTag]; ok {
			ret[tagPath] = resolvedImageUrl[setting.PathSearchComponentTag]
			for k, imagePath := range matchData {
				if k == setting.PathSearchComponentTag {
					continue
				}
				ret[imagePath] = fmt.Sprintf("%s%s", resolvedImageUrl[setting.PathSearchComponentRepo], resolvedImageUrl[setting.PathSearchComponentImage])
				break
			}
			return ret, nil
		}
		// image url assigned into repo + image(tag)
		ret[matchData[setting.PathSearchComponentRepo]] = strings.TrimSuffix(resolvedImageUrl[setting.PathSearchComponentRepo], "/")
		ret[matchData[setting.PathSearchComponentImage]] = fmt.Sprintf("%s:%s", resolvedImageUrl[setting.PathSearchComponentImage], resolvedImageUrl[setting.PathSearchComponentTag])
		return ret, nil
	}

	return nil, errors.Errorf("match data illegal, expect length: 1-3, actual length: %d", len(matchData))
}

// Wait ...
func (p *DeployTaskPlugin) Wait(ctx context.Context) {
	// skip waiting for reset image task
	if p.Task.SkipWaiting {
		p.Task.TaskStatus = config.StatusPassed
		return
	}

	timeout := time.After(time.Duration(p.TaskTimeout()) * time.Second)

	selector := labels.Set{setting.ProductLabel: p.Task.ProductName, setting.ServiceLabel: p.Task.ServiceName}.AsSelector()

	for {
		select {
		case <-ctx.Done():
			p.Task.TaskStatus = config.StatusCancelled
			return

		case <-timeout:
			p.Task.TaskStatus = config.StatusTimeout

			pods, err := getter.ListPods(p.Task.Namespace, selector, p.kubeClient)
			if err != nil {
				p.Task.Error = err.Error()
				return
			}

			var msg []string
			for _, pod := range pods {
				podResource := wrapper.Pod(pod).Resource()
				if podResource.Status != setting.StatusRunning && podResource.Status != setting.StatusSucceeded {
					for _, cs := range podResource.ContainerStatuses {
						// message为空不认为是错误状态，有可能还在waiting
						if cs.Message != "" {
							msg = append(msg, fmt.Sprintf("Status: %s, Reason: %s, Message: %s", cs.Status, cs.Reason, cs.Message))
						}
					}
				}
			}

			if len(msg) != 0 {
				p.Task.Error = strings.Join(msg, "\n")
			}

			return

		default:
			time.Sleep(time.Second * 2)
			ready := true
			var err error
		L:
			for _, resource := range p.Task.ReplaceResources {
				switch resource.Kind {
				case setting.Deployment:
					d, found, e := getter.GetDeployment(p.Task.Namespace, resource.Name, p.kubeClient)
					if e != nil {
						err = e
					}
					if e != nil || !found {
						p.Log.Errorf(
							"failed to check deployment ready status %s/%s/%s - %v",
							p.Task.Namespace,
							resource.Kind,
							resource.Name,
							e,
						)
						ready = false
					} else {
						ready = wrapper.Deployment(d).Ready()
					}

					if !ready {
						break L
					}
				case setting.StatefulSet:
					s, found, e := getter.GetStatefulSet(p.Task.Namespace, resource.Name, p.kubeClient)
					if e != nil {
						err = e
					}
					if err != nil || !found {
						p.Log.Errorf(
							"failed to check statefulSet ready status %s/%s/%s",
							p.Task.Namespace,
							resource.Kind,
							resource.Name,
							e,
						)
						ready = false
					} else {
						ready = wrapper.StatefulSet(s).Ready()
					}

					if !ready {
						break L
					}
				}
			}

			if ready {
				p.Task.TaskStatus = config.StatusPassed
			}

			if p.IsTaskDone() {
				return
			}
		}
	}
}

func (p *DeployTaskPlugin) Complete(ctx context.Context, pipelineTask *task.Task, serviceName string) {
}

func (p *DeployTaskPlugin) SetTask(t map[string]interface{}) error {
	task, err := ToDeployTask(t)
	if err != nil {
		return err
	}
	p.Task = task

	return nil
}

// GetTask ...
func (p *DeployTaskPlugin) GetTask() interface{} {
	return p.Task
}

// IsTaskDone ...
func (p *DeployTaskPlugin) IsTaskDone() bool {
	if p.Task.TaskStatus != config.StatusCreated && p.Task.TaskStatus != config.StatusRunning {
		return true
	}
	return false
}

// IsTaskFailed ...
func (p *DeployTaskPlugin) IsTaskFailed() bool {
	if p.Task.TaskStatus == config.StatusFailed || p.Task.TaskStatus == config.StatusTimeout || p.Task.TaskStatus == config.StatusCancelled {
		return true
	}
	return false
}

// SetStartTime ...
func (p *DeployTaskPlugin) SetStartTime() {
	p.Task.StartTime = time.Now().Unix()
}

// SetEndTime ...
func (p *DeployTaskPlugin) SetEndTime() {
	p.Task.EndTime = time.Now().Unix()
}

// IsTaskEnabled ...
func (p *DeployTaskPlugin) IsTaskEnabled() bool {
	return p.Task.Enabled
}

// ResetError ...
func (p *DeployTaskPlugin) ResetError() {
	p.Task.Error = ""
}
