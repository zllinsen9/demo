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
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/27149chen/afero"
	"github.com/otiai10/copy"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/yaml"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	fsservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/fs"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/helmclient"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/util"
	yamlutil "github.com/koderover/zadig/pkg/util/yaml"
)

type HelmService struct {
	ServiceInfos []*commonmodels.Service `json:"service_infos"`
	FileInfos    []*types.FileInfo       `json:"file_infos"`
	Services     [][]string              `json:"services"`
}

type HelmServiceArgs struct {
	ProductName      string             `json:"product_name"`
	CreateBy         string             `json:"create_by"`
	HelmServiceInfos []*HelmServiceInfo `json:"helm_service_infos"`
}

type HelmServiceInfo struct {
	ServiceName string `json:"service_name"`
	FilePath    string `json:"file_path"`
	FileName    string `json:"file_name"`
	FileContent string `json:"file_content"`
}

type HelmServiceModule struct {
	ServiceModules []*ServiceModule      `json:"service_modules"`
	Service        *commonmodels.Service `json:"service,omitempty"`
}

type Chart struct {
	APIVersion string `json:"apiVersion"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	AppVersion string `json:"appVersion"`
}

type helmServiceCreationArgs struct {
	ChartName        string
	ChartVersion     string
	ServiceRevision  int64
	MergedValues     string
	ServiceName      string
	FilePath         string
	ProductName      string
	CreateBy         string
	CodehostID       int
	Owner            string
	Repo             string
	Branch           string
	RepoLink         string
	Source           string
	HelmTemplateName string
	ValuePaths       []string
	ValuesYaml       string
	Variables        []*Variable
	GerritRepoName   string
	GerritBranchName string
	GerritRemoteName string
	GerritPath       string
	GerritCodeHostID int
	ChartRepoName    string
}

type ChartTemplateData struct {
	TemplateName      string
	TemplateData      *commonmodels.Chart
	ChartName         string
	ChartVersion      string
	DefaultValuesYAML []byte // content of values.yaml in template
}

type GetFileContentParam struct {
	FilePath        string `json:"filePath"        form:"filePath"`
	FileName        string `json:"fileName"        form:"fileName"`
	Revision        int64  `json:"revision"        form:"revision"`
	DeliveryVersion bool   `json:"deliveryVersion" form:"deliveryVersion"`
}

func ListHelmServices(productName string, log *zap.SugaredLogger) (*HelmService, error) {
	helmService := &HelmService{
		ServiceInfos: []*commonmodels.Service{},
		FileInfos:    []*types.FileInfo{},
		Services:     [][]string{},
	}

	opt := &commonrepo.ServiceListOption{
		ProductName: productName,
		Type:        setting.HelmDeployType,
	}

	services, err := commonrepo.NewServiceColl().ListMaxRevisions(opt)
	if err != nil {
		log.Errorf("[helmService.list] err:%v", err)
		return nil, e.ErrListTemplate.AddErr(err)
	}
	helmService.ServiceInfos = services

	if len(services) > 0 {
		fis, err := loadServiceFileInfos(services[0].ProductName, services[0].ServiceName, 0, "")
		if err != nil {
			log.Errorf("Failed to load service file info, err: %s", err)
			return nil, e.ErrListTemplate.AddErr(err)
		}
		helmService.FileInfos = fis
	}
	project, err := templaterepo.NewProductColl().Find(productName)
	if err != nil {
		log.Errorf("Failed to find project info, err: %s", err)
		return nil, e.ErrListTemplate.AddErr(err)
	}
	helmService.Services = project.Services

	return helmService, nil
}

func GetHelmServiceModule(serviceName, productName string, revision int64, log *zap.SugaredLogger) (*HelmServiceModule, error) {

	serviceTemplate, err := commonservice.GetServiceTemplate(serviceName, setting.HelmDeployType, productName, setting.ProductStatusDeleting, revision, log)
	if err != nil {
		return nil, err
	}
	helmServiceModule := new(HelmServiceModule)
	serviceModules := make([]*ServiceModule, 0)
	for _, container := range serviceTemplate.Containers {
		serviceModule := new(ServiceModule)
		serviceModule.Container = container
		buildObj, _ := commonrepo.NewBuildColl().Find(&commonrepo.BuildFindOption{ProductName: productName, ServiceName: serviceName, Targets: []string{container.Name}})
		if buildObj != nil {
			serviceModule.BuildName = buildObj.Name
		}
		serviceModules = append(serviceModules, serviceModule)
	}
	helmServiceModule.Service = serviceTemplate
	helmServiceModule.ServiceModules = serviceModules
	return helmServiceModule, err
}

func GetFilePath(serviceName, productName string, revision int64, dir string, _ *zap.SugaredLogger) ([]*types.FileInfo, error) {
	return loadServiceFileInfos(productName, serviceName, revision, dir)
}

func GetFileContent(serviceName, productName string, param *GetFileContentParam, log *zap.SugaredLogger) (string, error) {
	filePath, fileName, revision, forDelivery := param.FilePath, param.FileName, param.Revision, param.DeliveryVersion
	svc, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
		ProductName: productName,
		ServiceName: serviceName,
		Revision:    revision,
	})
	if err != nil {
		return "", e.ErrFileContent.AddDesc(err.Error())
	}

	base := config.LocalServicePath(productName, serviceName)
	if revision > 0 {
		base = config.LocalServicePathWithRevision(productName, serviceName, revision)
		if err = commonservice.PreloadServiceManifestsByRevision(base, svc); err != nil {
			log.Warnf("failed to get chart of revision: %d for service: %s, use latest version",
				svc.Revision, svc.ServiceName)
		}
	}
	if err != nil || revision == 0 {
		base = config.LocalServicePath(productName, serviceName)
		err = commonservice.PreLoadServiceManifests(base, svc)
		if err != nil {
			return "", e.ErrFileContent.AddDesc(err.Error())
		}
	}

	if forDelivery {
		base = config.LocalDeliveryChartPathWithRevision(productName, serviceName, revision)
	}

	file := filepath.Join(base, serviceName, filePath, fileName)
	fileContent, err := os.ReadFile(file)
	if err != nil {
		log.Errorf("Failed to read file %s, err: %s", file, err)
		return "", e.ErrFileContent.AddDesc(err.Error())
	}

	return string(fileContent), nil
}

func prepareChartTemplateData(templateName string, logger *zap.SugaredLogger) (*ChartTemplateData, error) {
	templateChart, err := commonrepo.NewChartColl().Get(templateName)
	if err != nil {
		logger.Errorf("Failed to get chart template %s, err: %s", templateName, err)
		return nil, fmt.Errorf("failed to get chart template: %s", templateName)
	}

	// get chart template from local disk
	localBase := configbase.LocalChartTemplatePath(templateName)
	s3Base := configbase.ObjectStorageChartTemplatePath(templateName)
	if err = fsservice.PreloadFiles(templateName, localBase, s3Base, logger); err != nil {
		logger.Errorf("Failed to download template %s, err: %s", templateName, err)
		return nil, err
	}

	base := filepath.Base(templateChart.Path)
	defaultValuesFile := filepath.Join(localBase, base, setting.ValuesYaml)
	defaultValues, _ := os.ReadFile(defaultValuesFile)

	chartFilePath := filepath.Join(localBase, base, setting.ChartYaml)
	chartFileContent, err := os.ReadFile(chartFilePath)
	if err != nil {
		logger.Errorf("Failed to read chartfile template %s, err: %s", templateName, err)
		return nil, err
	}
	chart := new(Chart)
	if err = yaml.Unmarshal(chartFileContent, chart); err != nil {
		logger.Errorf("Failed to unmarshal chart yaml %s, err: %s", setting.ChartYaml, err)
		return nil, err
	}

	return &ChartTemplateData{
		TemplateName:      templateName,
		TemplateData:      templateChart,
		ChartName:         chart.Name,
		ChartVersion:      chart.Version,
		DefaultValuesYAML: defaultValues,
	}, nil
}

func getNextServiceRevision(productName, serviceName string) (int64, error) {
	serviceTemplate := fmt.Sprintf(setting.ServiceTemplateCounterName, serviceName, productName)
	rev, err := commonrepo.NewCounterColl().GetNextSeq(serviceTemplate)
	if err != nil {
		log.Errorf("Failed to get next revision for service %s, err: %s", serviceName, err)
		return 0, err
	}
	if err = commonrepo.NewServiceColl().Delete(serviceName, setting.HelmDeployType, serviceName, setting.ProductStatusDeleting, rev); err != nil {
		log.Warnf("Failed to delete stale service %s with revision %d, err: %s", serviceName, rev, err)
	}
	return rev, err
}

// make local chart info copy with revision
func copyChartRevision(projectName, serviceName string, revision int64) error {
	sourceChartPath := config.LocalServicePath(projectName, serviceName)
	revisionChartLocalPath := config.LocalServicePathWithRevision(projectName, serviceName, revision)

	err := os.RemoveAll(revisionChartLocalPath)
	if err != nil {
		log.Errorf("failed to remove old chart revision data, projectName %s serviceName %s revision %d, err %s", projectName, serviceName, revision, err)
		return err
	}

	err = copy.Copy(sourceChartPath, revisionChartLocalPath)
	if err != nil {
		log.Errorf("failed to copy chart info, projectName %s serviceName %s revision %d, err %s", projectName, serviceName, revision, err)
		return err
	}
	return nil
}

func clearChartFiles(projectName, serviceName string, revision int64, logger *zap.SugaredLogger, source ...string) {
	clearChartFilesInS3Storage(projectName, serviceName, revision, logger)
	if len(source) == 0 {
		clearLocalChartFiles(projectName, serviceName, revision, logger)
	}
}

// clear chart files in s3 storage
func clearChartFilesInS3Storage(projectName, serviceName string, revision int64, logger *zap.SugaredLogger) {
	s3FileNames := []string{serviceName, fmt.Sprintf("%s-%d", serviceName, revision)}
	errRemoveFile := fsservice.DeleteArchivedFileFromS3(s3FileNames, config.ObjectStorageServicePath(projectName, serviceName), logger)
	if errRemoveFile != nil {
		logger.Errorf("Failed to remove files: %v from s3 strorage, err: %s", s3FileNames, errRemoveFile)
	}
}

// clear local chart infos
func clearLocalChartFiles(projectName, serviceName string, revision int64, logger *zap.SugaredLogger) {
	latestChartPath := config.LocalServicePath(projectName, serviceName)
	revisionChartLocalPath := config.LocalServicePathWithRevision(projectName, serviceName, revision)
	for _, path := range []string{latestChartPath, revisionChartLocalPath} {
		err := os.RemoveAll(path)
		if err != nil {
			logger.Errorf("failed to remove local chart data, path: %s, err: %s", path, err)
		}
	}
}

func CreateOrUpdateHelmService(projectName string, args *HelmServiceCreationArgs, logger *zap.SugaredLogger) (*BulkHelmServiceCreationResponse, error) {
	switch args.Source {
	case LoadFromRepo, LoadFromPublicRepo:
		return CreateOrUpdateHelmServiceFromGitRepo(projectName, args, logger)
	case LoadFromChartTemplate:
		return CreateOrUpdateHelmServiceFromChartTemplate(projectName, args, logger)
	case LoadFromGerrit:
		return CreateOrUpdateHelmServiceFromGerrit(projectName, args, logger)
	case LoadFromChartRepo:
		return CreateOrUpdateHelmServiceFromChartRepo(projectName, args, logger)
	default:
		return nil, fmt.Errorf("invalid source")
	}
}

func CreateOrUpdateHelmServiceFromChartRepo(projectName string, args *HelmServiceCreationArgs, log *zap.SugaredLogger) (*BulkHelmServiceCreationResponse, error) {
	chartRepoArgs, ok := args.CreateFrom.(*CreateFromChartRepo)
	if !ok {
		return nil, e.ErrCreateTemplate.AddDesc("invalid argument")
	}

	chartRepo, err := commonrepo.NewHelmRepoColl().Find(&commonrepo.HelmRepoFindOption{RepoName: chartRepoArgs.ChartRepoName})
	if err != nil {
		log.Errorf("failed to query chart-repo info, productName: %s, err: %s", projectName, err)
		return nil, e.ErrCreateTemplate.AddDesc(fmt.Sprintf("failed to query chart-repo info, productName: %s, repoName: %s", projectName, chartRepoArgs.ChartRepoName))
	}

	chartClient, err := helmclient.NewHelmChartRepoClient(chartRepo.URL, chartRepo.Username, chartRepo.Password)
	if err != nil {
		return nil, e.ErrCreateTemplate.AddErr(errors.Wrapf(err, "failed to init chart client for repo: %s", chartRepo.RepoName))
	}

	index, err := chartClient.FetchIndexYaml()
	if err != nil {
		return nil, e.ErrCreateTemplate.AddErr(err)
	}

	// validate chart with specific version exists in repo
	foundChart := false
	for name, entries := range index.Entries {
		if name != chartRepoArgs.ChartName {
			continue
		}
		for _, entry := range entries {
			if entry.Version == chartRepoArgs.ChartVersion {
				foundChart = true
				break
			}
		}
		break
	}
	if !foundChart {
		return nil, e.ErrCreateTemplate.AddDesc(fmt.Sprintf("failed to find chart: %s-%s from chart repo", chartRepoArgs.ChartName, chartRepoArgs.ChartVersion))
	}

	localPath := config.LocalServicePath(projectName, chartRepoArgs.ChartName)
	err = chartClient.DownloadAndExpand(chartRepoArgs.ChartName, chartRepoArgs.ChartVersion, localPath)
	if err != nil {
		return nil, e.ErrCreateTemplate.AddErr(err)
	}

	serviceName := chartRepoArgs.ChartName
	rev, err := getNextServiceRevision(projectName, serviceName)
	if err != nil {
		log.Errorf("Failed to get next revision for service %s, err: %s", serviceName, err)
		return nil, e.ErrCreateTemplate.AddErr(err)
	}

	var finalErr error
	// clear files from both s3 and local when error occurred in next stages
	defer func() {
		if finalErr != nil {
			clearChartFiles(projectName, serviceName, rev, log)
		}
	}()

	// read values.yaml
	fsTree := os.DirFS(config.LocalServicePath(projectName, chartRepoArgs.ChartName))
	valuesYAML, err := readValuesYAML(fsTree, chartRepoArgs.ChartName, log)
	if err != nil {
		finalErr = e.ErrCreateTemplate.AddErr(err)
		return nil, finalErr
	}

	// upload to s3 storage
	s3Base := config.ObjectStorageServicePath(projectName, serviceName)
	err = fsservice.ArchiveAndUploadFilesToS3(fsTree, []string{serviceName, fmt.Sprintf("%s-%d", serviceName, rev)}, s3Base, log)
	if err != nil {
		finalErr = e.ErrCreateTemplate.AddErr(err)
		return nil, finalErr
	}

	// copy service revision data from latest
	err = copyChartRevision(projectName, serviceName, rev)
	if err != nil {
		log.Errorf("Failed to copy file %s, err: %s", serviceName, err)
		finalErr = errors.Wrapf(err, "Failed to copy chart info, service %s", serviceName)
		return nil, finalErr
	}

	svc, err := createOrUpdateHelmService(
		fsTree,
		&helmServiceCreationArgs{
			ChartName:       chartRepoArgs.ChartName,
			ChartVersion:    chartRepoArgs.ChartVersion,
			ChartRepoName:   chartRepoArgs.ChartRepoName,
			ServiceRevision: rev,
			MergedValues:    string(valuesYAML),
			ServiceName:     serviceName,
			ProductName:     projectName,
			CreateBy:        args.CreatedBy,
			Source:          setting.SourceFromChartRepo,
		},
		log,
	)
	if err != nil {
		log.Errorf("Failed to create service %s in project %s, error: %s", serviceName, projectName, err)
		finalErr = e.ErrCreateTemplate.AddErr(err)
		return nil, finalErr
	}

	compareHelmVariable([]*templatemodels.RenderChart{
		{
			ServiceName:  chartRepoArgs.ChartName,
			ChartVersion: svc.HelmChart.Version,
			ValuesYaml:   svc.HelmChart.ValuesYaml,
		},
	}, projectName, args.CreatedBy, log)

	return &BulkHelmServiceCreationResponse{
		SuccessServices: []string{serviceName},
	}, nil
}

func CreateOrUpdateHelmServiceFromChartTemplate(projectName string, args *HelmServiceCreationArgs, logger *zap.SugaredLogger) (*BulkHelmServiceCreationResponse, error) {
	templateArgs, ok := args.CreateFrom.(*CreateFromChartTemplate)
	if !ok {
		return nil, fmt.Errorf("invalid argument")
	}

	templateChartInfo, err := prepareChartTemplateData(templateArgs.TemplateName, logger)
	if err != nil {
		return nil, err
	}

	var values [][]byte
	if len(templateChartInfo.DefaultValuesYAML) > 0 {
		//render variables
		renderedYaml, err := renderVariablesToYaml(string(templateChartInfo.DefaultValuesYAML), projectName, args.Name, templateArgs.Variables)
		if err != nil {
			return nil, err
		}
		values = append(values, []byte(renderedYaml))
	}

	if len(templateArgs.ValuesYAML) > 0 {
		values = append(values, []byte(templateArgs.ValuesYAML))
	}

	localBase := configbase.LocalChartTemplatePath(templateArgs.TemplateName)
	base := filepath.Base(templateChartInfo.TemplateData.Path)

	// copy template to service path and update the values.yaml
	from := filepath.Join(localBase, base)
	to := filepath.Join(config.LocalServicePath(projectName, args.Name), args.Name)
	// remove old files
	if err = os.RemoveAll(to); err != nil {
		logger.Errorf("Failed to remove dir %s, err: %s", to, err)
		return nil, err
	}
	if err = copy.Copy(from, to); err != nil {
		logger.Errorf("Failed to copy file from %s to %s, err: %s", from, to, err)
		return nil, err
	}

	merged, err := yamlutil.Merge(values)
	if err != nil {
		logger.Errorf("Failed to merge values, err: %s", err)
		return nil, err
	}

	if err = os.WriteFile(filepath.Join(to, setting.ValuesYaml), merged, 0644); err != nil {
		logger.Errorf("Failed to write values, err: %s", err)
		return nil, err
	}

	rev, err := getNextServiceRevision(projectName, args.Name)
	if err != nil {
		logger.Errorf("Failed to get next revision for service %s, err: %s", args.Name, err)
		return nil, errors.Wrapf(err, "Failed to get service next revision, service %s", args.Name)
	}

	err = copyChartRevision(projectName, args.Name, rev)
	if err != nil {
		logger.Errorf("Failed to copy file %s, err: %s", args.Name, err)
		return nil, errors.Wrapf(err, "Failed to copy chart info, service %s", args.Name)
	}

	// clear files from both s3 and local when error occurred in next stages
	defer func() {
		if err != nil {
			clearChartFiles(projectName, args.Name, rev, logger)
		}
	}()

	fsTree := os.DirFS(config.LocalServicePath(projectName, args.Name))
	serviceS3Base := config.ObjectStorageServicePath(projectName, args.Name)
	if err = fsservice.ArchiveAndUploadFilesToS3(fsTree, []string{args.Name, fmt.Sprintf("%s-%d", args.Name, rev)}, serviceS3Base, logger); err != nil {
		logger.Errorf("Failed to upload files for service %s in project %s, err: %s", args.Name, projectName, err)
		return nil, err
	}

	svc, errCreate := createOrUpdateHelmService(
		fsTree,
		&helmServiceCreationArgs{
			ChartName:        templateChartInfo.ChartName,
			ChartVersion:     templateChartInfo.ChartVersion,
			ServiceRevision:  rev,
			MergedValues:     string(merged),
			ServiceName:      args.Name,
			FilePath:         to,
			ProductName:      projectName,
			CreateBy:         args.CreatedBy,
			Source:           setting.SourceFromChartTemplate,
			HelmTemplateName: templateArgs.TemplateName,
			ValuesYaml:       templateArgs.ValuesYAML,
			Variables:        templateArgs.Variables,
		},
		logger,
	)

	if errCreate != nil {
		err = errCreate
		logger.Errorf("Failed to create service %s in project %s, error: %s", args.Name, projectName, err)
		return nil, err
	}

	compareHelmVariable([]*templatemodels.RenderChart{
		{ServiceName: args.Name,
			ChartVersion: svc.HelmChart.Version,
			ValuesYaml:   svc.HelmChart.ValuesYaml,
		},
	}, projectName, args.CreatedBy, logger)

	return &BulkHelmServiceCreationResponse{
		SuccessServices: []string{args.Name},
	}, nil
}

func getCodehostType(repoArgs *CreateFromRepo, repoLink string) (string, *systemconfig.CodeHost, error) {
	if repoLink != "" {
		return setting.SourceFromPublicRepo, nil, nil
	}
	ch, err := systemconfig.New().GetCodeHost(repoArgs.CodehostID)
	if err != nil {
		log.Errorf("Failed to get codeHost by id %d, err: %s", repoArgs.CodehostID, err.Error())
		return "", ch, err
	}
	return ch.Type, ch, nil
}

func CreateOrUpdateHelmServiceFromGerrit(projectName string, args *HelmServiceCreationArgs, log *zap.SugaredLogger) (*BulkHelmServiceCreationResponse, error) {
	var (
		filePaths []string
		response  = &BulkHelmServiceCreationResponse{}
		base      string
	)
	resByte, resByteErr := json.Marshal(args.CreateFrom)
	if resByteErr != nil {
		log.Errorf("failed to json.Marshal err:%s", resByteErr)
		return nil, resByteErr
	}
	var createFromRepo CreateFromRepo
	jsonResErr := json.Unmarshal(resByte, &createFromRepo)
	if jsonResErr != nil {
		log.Errorf("failed to json.Unmarshal err:%s", resByteErr)
		return nil, jsonResErr
	}

	filePaths = createFromRepo.Paths
	base = path.Join(config.S3StoragePath(), createFromRepo.Repo)
	helmRenderCharts := make([]*templatemodels.RenderChart, 0, len(filePaths))
	var wg wait.Group
	var mux sync.RWMutex
	for _, p := range filePaths {
		filePath := strings.TrimLeft(p, "/")
		wg.Start(func() {
			var (
				serviceName  string
				chartVersion string
				valuesYAML   []byte
				finalErr     error
			)
			defer func() {
				mux.Lock()
				if finalErr != nil {
					response.FailedServices = append(response.FailedServices, &FailedService{
						Path:  filePath,
						Error: finalErr.Error(),
					})
				} else {
					response.SuccessServices = append(response.SuccessServices, serviceName)
				}
				mux.Unlock()
			}()

			currentFilePath := path.Join(base, filePath)
			log.Infof("Loading chart under path %s", currentFilePath)
			serviceName, chartVersion, finalErr = readChartYAMLFromLocal(currentFilePath, log)
			if finalErr != nil {
				return
			}
			valuesYAML, finalErr = readValuesYAMLFromLocal(currentFilePath, log)
			if finalErr != nil {
				return
			}

			log.Info("Found valid chart, Starting to save and upload files")
			rev, err := getNextServiceRevision(projectName, serviceName)
			if err != nil {
				log.Errorf("Failed to get next revision for service %s, err: %s", serviceName, err)
				finalErr = e.ErrCreateTemplate.AddErr(err)
				return
			}

			// clear files from s3 when error occurred in next stages
			defer func() {
				if finalErr != nil {
					clearChartFiles(projectName, serviceName, rev, log, string(args.Source))
				}
			}()

			// copy to latest dir and upload to s3
			if err = commonservice.CopyAndUploadService(projectName, serviceName, currentFilePath, []string{fmt.Sprintf("%s-%d", serviceName, rev)}); err != nil {
				log.Errorf("Failed to save or upload files for service %s in project %s, error: %s", serviceName, projectName, err)
				finalErr = e.ErrCreateTemplate.AddErr(err)
				return
			}

			err = copyChartRevision(projectName, serviceName, rev)
			if err != nil {
				log.Errorf("Failed to copy file %s, err: %s", serviceName, err)
				finalErr = errors.Wrapf(err, "Failed to copy chart info, service %s", serviceName)
				return
			}

			svc, err := createOrUpdateHelmService(
				nil,
				&helmServiceCreationArgs{
					ChartName:        serviceName,
					ChartVersion:     chartVersion,
					ServiceRevision:  rev,
					MergedValues:     string(valuesYAML),
					ServiceName:      serviceName,
					FilePath:         filePath,
					ProductName:      projectName,
					CreateBy:         args.CreatedBy,
					CodehostID:       createFromRepo.CodehostID,
					Owner:            createFromRepo.Owner,
					Repo:             createFromRepo.Repo,
					Branch:           createFromRepo.Branch,
					Source:           string(args.Source),
					GerritCodeHostID: createFromRepo.CodehostID,
					GerritPath:       currentFilePath,
					GerritRepoName:   createFromRepo.Repo,
					GerritBranchName: createFromRepo.Branch,
					GerritRemoteName: "origin",
				},
				log,
			)
			if err != nil {
				log.Errorf("Failed to create service %s in project %s, error: %s", serviceName, projectName, err)
				finalErr = e.ErrCreateTemplate.AddErr(err)
				return
			}

			helmRenderCharts = append(helmRenderCharts, &templatemodels.RenderChart{
				ServiceName:  serviceName,
				ChartVersion: svc.HelmChart.Version,
				ValuesYaml:   svc.HelmChart.ValuesYaml,
			})
		})
	}

	wg.Wait()

	compareHelmVariable(helmRenderCharts, projectName, args.CreatedBy, log)
	return response, nil
}

func CreateOrUpdateHelmServiceFromGitRepo(projectName string, args *HelmServiceCreationArgs, log *zap.SugaredLogger) (*BulkHelmServiceCreationResponse, error) {
	var err error
	var repoLink string
	repoArgs, ok := args.CreateFrom.(*CreateFromRepo)
	if !ok {
		publicArgs, ok := args.CreateFrom.(*CreateFromPublicRepo)
		if !ok {
			return nil, fmt.Errorf("invalid argument")
		}

		repoArgs, err = PublicRepoToPrivateRepoArgs(publicArgs)
		if err != nil {
			log.Errorf("Failed to parse repo args %+v, err: %s", publicArgs, err)
			return nil, err
		}

		repoLink = publicArgs.RepoLink
	}

	response := &BulkHelmServiceCreationResponse{}
	source, codehostInfo, err := getCodehostType(repoArgs, repoLink)
	if err != nil {
		log.Errorf("Failed to get source form repo data %+v, err: %s", *repoArgs, err.Error())
		return nil, err
	}

	helmRenderCharts := make([]*templatemodels.RenderChart, 0, len(repoArgs.Paths))

	var wg wait.Group
	var mux sync.RWMutex
	for _, p := range repoArgs.Paths {
		filePath := strings.TrimLeft(p, "/")
		wg.Start(func() {
			var (
				serviceName  string
				chartVersion string
				valuesYAML   []byte
				finalErr     error
			)
			defer func() {
				mux.Lock()
				if finalErr != nil {
					response.FailedServices = append(response.FailedServices, &FailedService{
						Path:  filePath,
						Error: finalErr.Error(),
					})
				} else {
					response.SuccessServices = append(response.SuccessServices, serviceName)
				}
				mux.Unlock()
			}()

			log.Infof("Loading chart under path %s", filePath)

			fsTree, err := fsservice.DownloadFilesFromSource(
				&fsservice.DownloadFromSourceArgs{CodehostID: repoArgs.CodehostID, Owner: repoArgs.Owner, Repo: repoArgs.Repo, Path: filePath, Branch: repoArgs.Branch, RepoLink: repoLink},
				func(chartTree afero.Fs) (string, error) {
					var err error
					serviceName, chartVersion, err = readChartYAML(afero.NewIOFS(chartTree), filepath.Base(filePath), log)
					if err != nil {
						return serviceName, err
					}
					valuesYAML, err = readValuesYAML(afero.NewIOFS(chartTree), filepath.Base(filePath), log)
					return serviceName, err
				})
			if err != nil {
				log.Errorf("Failed to download files from source, err %s", err)
				finalErr = e.ErrCreateTemplate.AddErr(err)
				return
			}

			log.Info("Found valid chart, Starting to save and upload files")

			rev, err := getNextServiceRevision(projectName, serviceName)
			if err != nil {
				log.Errorf("Failed to get next revision for service %s, err: %s", serviceName, err)
				finalErr = e.ErrCreateTemplate.AddErr(err)
				return
			}

			// clear files from both s3 and local when error occurred in next stages
			defer func() {
				if finalErr != nil {
					clearChartFiles(projectName, serviceName, rev, log)
				}
			}()

			// save files to disk and upload them to s3
			if err = commonservice.SaveAndUploadService(projectName, serviceName, []string{fmt.Sprintf("%s-%d", serviceName, rev)}, fsTree); err != nil {
				log.Errorf("Failed to save or upload files for service %s in project %s, error: %s", serviceName, projectName, err)
				finalErr = e.ErrCreateTemplate.AddErr(err)
				return
			}

			err = copyChartRevision(projectName, serviceName, rev)
			if err != nil {
				log.Errorf("Failed to copy file %s, err: %s", serviceName, err)
				finalErr = errors.Wrapf(err, "Failed to copy chart info, service %s", serviceName)
				return
			}

			if source != setting.SourceFromPublicRepo && codehostInfo != nil {
				repoLink = fmt.Sprintf("%s/%s/%s/%s/%s/%s", codehostInfo.Address, repoArgs.Owner, repoArgs.Repo, "tree", repoArgs.Branch, filePath)
			}

			svc, err := createOrUpdateHelmService(
				fsTree,
				&helmServiceCreationArgs{
					ChartName:       serviceName,
					ChartVersion:    chartVersion,
					ServiceRevision: rev,
					MergedValues:    string(valuesYAML),
					ServiceName:     serviceName,
					FilePath:        filePath,
					ProductName:     projectName,
					CreateBy:        args.CreatedBy,
					CodehostID:      repoArgs.CodehostID,
					Owner:           repoArgs.Owner,
					Repo:            repoArgs.Repo,
					Branch:          repoArgs.Branch,
					RepoLink:        repoLink,
					Source:          source,
				},
				log,
			)
			if err != nil {
				log.Errorf("Failed to create service %s in project %s, error: %s", serviceName, projectName, err)
				finalErr = e.ErrCreateTemplate.AddErr(err)
				return
			}

			helmRenderCharts = append(helmRenderCharts, &templatemodels.RenderChart{
				ServiceName:  serviceName,
				ChartVersion: svc.HelmChart.Version,
				ValuesYaml:   svc.HelmChart.ValuesYaml,
			})
		})
	}

	wg.Wait()

	compareHelmVariable(helmRenderCharts, projectName, args.CreatedBy, log)
	return response, nil
}

func CreateOrUpdateBulkHelmService(projectName string, args *BulkHelmServiceCreationArgs, logger *zap.SugaredLogger) (*BulkHelmServiceCreationResponse, error) {
	switch args.Source {
	case LoadFromChartTemplate:
		return CreateOrUpdateBulkHelmServiceFromTemplate(projectName, args, logger)
	default:
		return nil, fmt.Errorf("invalid source")
	}
}

func CreateOrUpdateBulkHelmServiceFromTemplate(projectName string, args *BulkHelmServiceCreationArgs, logger *zap.SugaredLogger) (*BulkHelmServiceCreationResponse, error) {
	templateArgs, ok := args.CreateFrom.(*CreateFromChartTemplate)
	if !ok {
		return nil, fmt.Errorf("invalid argument")
	}

	if args.ValuesData == nil || args.ValuesData.GitRepoConfig == nil || len(args.ValuesData.GitRepoConfig.ValuesPaths) == 0 {
		return nil, fmt.Errorf("invalid argument, missing values")
	}

	templateChartData, err := prepareChartTemplateData(templateArgs.TemplateName, logger)
	if err != nil {
		return nil, err
	}

	localBase := configbase.LocalChartTemplatePath(templateArgs.TemplateName)
	base := filepath.Base(templateChartData.TemplateData.Path)
	// copy template to service path and update the values.yaml
	from := filepath.Join(localBase, base)

	//record errors for every service
	failedServiceMap := &sync.Map{}
	renderChartMap := &sync.Map{}

	wg := sync.WaitGroup{}
	// run goroutines to speed up
	for _, singlePath := range args.ValuesData.GitRepoConfig.ValuesPaths {
		wg.Add(1)
		go func(repoConfig *commonservice.RepoConfig, path string) {
			defer wg.Done()
			renderChart, err := handleSingleService(projectName, repoConfig, path, from, args.CreatedBy, templateChartData, logger)
			if err != nil {
				failedServiceMap.Store(path, err.Error())
			} else {
				renderChartMap.Store(renderChart.ServiceName, renderChart)
			}
		}(args.ValuesData.GitRepoConfig, singlePath)
	}

	wg.Wait()

	resp := &BulkHelmServiceCreationResponse{
		SuccessServices: make([]string, 0),
		FailedServices:  make([]*FailedService, 0),
	}

	renderChars := make([]*templatemodels.RenderChart, 0)

	renderChartMap.Range(func(key, value interface{}) bool {
		resp.SuccessServices = append(resp.SuccessServices, key.(string))
		renderChars = append(renderChars, value.(*templatemodels.RenderChart))
		return true
	})

	failedServiceMap.Range(func(key, value interface{}) bool {
		resp.FailedServices = append(resp.FailedServices, &FailedService{
			Path:  key.(string),
			Error: value.(string),
		})
		return true
	})

	compareHelmVariable(renderChars, projectName, args.CreatedBy, logger)

	return resp, nil
}

func handleSingleService(projectName string, repoConfig *commonservice.RepoConfig, path, fromPath, createBy string,
	templateChartData *ChartTemplateData, logger *zap.SugaredLogger) (*templatemodels.RenderChart, error) {

	valuesYAML, err := fsservice.DownloadFileFromSource(&fsservice.DownloadFromSourceArgs{
		CodehostID: repoConfig.CodehostID,
		Owner:      repoConfig.Owner,
		Repo:       repoConfig.Repo,
		Path:       path,
		Branch:     repoConfig.Branch,
	})
	if err != nil {
		return nil, err
	}

	if len(valuesYAML) == 0 {
		return nil, fmt.Errorf("values.yaml is empty")
	}

	values := [][]byte{templateChartData.DefaultValuesYAML, valuesYAML}
	mergedValues, err := yamlutil.Merge(values)
	if err != nil {
		logger.Errorf("Failed to merge values, err: %s", err)
		return nil, err
	}

	serviceName := filepath.Base(path)
	serviceName = strings.TrimSuffix(serviceName, filepath.Ext(serviceName))

	to := filepath.Join(config.LocalServicePath(projectName, serviceName), serviceName)
	// remove old files
	if err = os.RemoveAll(to); err != nil {
		logger.Errorf("Failed to remove dir %s, err: %s", to, err)
		return nil, err
	}
	if err = copy.Copy(fromPath, to); err != nil {
		logger.Errorf("Failed to copy file from %s to %s, err: %s", fromPath, to, err)
		return nil, err
	}

	// write values.yaml file
	if err = os.WriteFile(filepath.Join(to, setting.ValuesYaml), mergedValues, 0644); err != nil {
		logger.Errorf("Failed to write values, err: %s", err)
		return nil, err
	}

	rev, err := getNextServiceRevision(projectName, serviceName)
	if err != nil {
		log.Errorf("Failed to get next revision for service %s, err: %s", serviceName, err)
		return nil, errors.Wrapf(err, "Failed to get service next revision, service %s", serviceName)
	}

	err = copyChartRevision(projectName, serviceName, rev)
	if err != nil {
		log.Errorf("Failed to copy file %s, err: %s", serviceName, err)
		return nil, errors.Wrapf(err, "Failed to copy chart info, service %s", serviceName)
	}

	fsTree := os.DirFS(config.LocalServicePath(projectName, serviceName))
	serviceS3Base := config.ObjectStorageServicePath(projectName, serviceName)

	// clear files from both s3 and local when error occurred in next stages
	defer func() {
		if err != nil {
			clearChartFiles(projectName, serviceName, rev, logger)
		}
	}()

	if err = fsservice.ArchiveAndUploadFilesToS3(fsTree, []string{serviceName, fmt.Sprintf("%s-%d", serviceName, rev)}, serviceS3Base, logger); err != nil {
		logger.Errorf("Failed to upload files for service %s in project %s, err: %s", serviceName, projectName, err)
		return nil, err
	}

	_, err = createOrUpdateHelmService(
		fsTree,
		&helmServiceCreationArgs{
			ChartName:        templateChartData.ChartName,
			ChartVersion:     templateChartData.ChartVersion,
			ServiceRevision:  rev,
			MergedValues:     string(mergedValues),
			ServiceName:      serviceName,
			FilePath:         to,
			ProductName:      projectName,
			CreateBy:         createBy,
			CodehostID:       repoConfig.CodehostID,
			Source:           setting.SourceFromChartTemplate,
			HelmTemplateName: templateChartData.TemplateName,
			ValuePaths:       []string{path},
			ValuesYaml:       string(valuesYAML),
		},
		logger,
	)
	if err != nil {
		logger.Errorf("Failed to create service %s in project %s, error: %s", serviceName, projectName, err)
		return nil, err
	}

	return &templatemodels.RenderChart{
		ServiceName:  serviceName,
		ChartVersion: templateChartData.ChartVersion,
		ValuesYaml:   string(mergedValues),
	}, nil
}

func readChartYAML(chartTree fs.FS, base string, logger *zap.SugaredLogger) (string, string, error) {
	chartFile, err := fs.ReadFile(chartTree, filepath.Join(base, setting.ChartYaml))
	if err != nil {
		logger.Errorf("Failed to read %s, err: %s", setting.ChartYaml, err)
		return "", "", err
	}
	chart := new(Chart)
	if err = yaml.Unmarshal(chartFile, chart); err != nil {
		log.Errorf("Failed to unmarshal yaml %s, err: %s", setting.ChartYaml, err)
		return "", "", err
	}

	return chart.Name, chart.Version, nil
}

func readChartYAMLFromLocal(base string, logger *zap.SugaredLogger) (string, string, error) {
	chartFile, err := util.ReadFile(filepath.Join(base, setting.ChartYaml))
	if err != nil {
		logger.Errorf("Failed to read %s, err: %s", setting.ChartYaml, err)
		return "", "", err
	}
	chart := new(Chart)
	if err = yaml.Unmarshal(chartFile, chart); err != nil {
		log.Errorf("Failed to unmarshal yaml %s, err: %s", setting.ChartYaml, err)
		return "", "", err
	}

	return chart.Name, chart.Version, nil
}

func readValuesYAML(chartTree fs.FS, base string, logger *zap.SugaredLogger) ([]byte, error) {
	content, err := fs.ReadFile(chartTree, filepath.Join(base, setting.ValuesYaml))
	if err != nil {
		logger.Errorf("Failed to read %s, err: %s", setting.ValuesYaml, err)
		return nil, err
	}
	return content, nil
}

func readValuesYAMLFromLocal(base string, logger *zap.SugaredLogger) ([]byte, error) {
	content, err := util.ReadFile(filepath.Join(base, setting.ValuesYaml))
	if err != nil {
		logger.Errorf("Failed to read %s, err: %s", setting.ValuesYaml, err)
		return nil, err
	}
	return content, nil
}

func geneCreationDetail(args *helmServiceCreationArgs) interface{} {
	switch args.Source {
	case setting.SourceFromGitlab,
		setting.SourceFromGithub,
		setting.SourceFromGerrit,
		setting.SourceFromCodeHub:
		return &models.CreateFromRepo{
			GitRepoConfig: &templatemodels.GitRepoConfig{
				CodehostID: args.CodehostID,
				Owner:      args.Owner,
				Repo:       args.Repo,
				Branch:     args.Branch,
			},
			LoadPath: args.FilePath,
		}
	case setting.SourceFromPublicRepo:
		return &models.CreateFromPublicRepo{
			RepoLink: args.RepoLink,
			LoadPath: args.FilePath,
		}
	case setting.SourceFromChartTemplate:
		yamlData := &templatemodels.CustomYaml{
			YamlContent: args.ValuesYaml,
		}
		variables := make([]*models.Variable, 0, len(args.Variables))
		for _, variable := range args.Variables {
			variables = append(variables, &models.Variable{
				Key:   variable.Key,
				Value: variable.Value,
			})
		}
		return &models.CreateFromChartTemplate{
			YamlData:     yamlData,
			TemplateName: args.HelmTemplateName,
			ServiceName:  args.ServiceName,
			Variables:    variables,
		}
	case setting.SourceFromChartRepo:
		return models.CreateFromChartRepo{
			ChartRepoName: args.ChartRepoName,
			ChartName:     args.ChartName,
			ChartVersion:  args.ChartVersion,
		}
	}
	return nil
}

func renderVariablesToYaml(valuesYaml string, productName, serviceName string, variables []*Variable) (string, error) {
	valuesYaml = strings.Replace(valuesYaml, setting.TemplateVariableProduct, productName, -1)
	valuesYaml = strings.Replace(valuesYaml, setting.TemplateVariableService, serviceName, -1)

	// build replace data
	valuesMap := make(map[string]interface{})
	for _, variable := range variables {
		valuesMap[variable.Key] = variable.Value
	}

	tmpl, err := template.New("values").Parse(valuesYaml)
	if err != nil {
		log.Errorf("failed to parse template, err %s valuesYaml %s", err, valuesYaml)
		return "", errors.Wrapf(err, "failed to parse template, err %s", err)
	}

	buf := bytes.NewBufferString("")
	err = tmpl.Execute(buf, valuesMap)
	if err != nil {
		log.Errorf("failed to render values content, err %s", err)
		return "", fmt.Errorf("failed to render variables")
	}
	valuesYaml = buf.String()
	return valuesYaml, nil
}

func createOrUpdateHelmService(fsTree fs.FS, args *helmServiceCreationArgs, logger *zap.SugaredLogger) (*commonmodels.Service, error) {
	var (
		chartName, chartVersion string
		err                     error
	)
	switch args.Source {
	case string(LoadFromGerrit):
		base := path.Join(config.S3StoragePath(), args.GerritRepoName)
		chartName, chartVersion, err = readChartYAMLFromLocal(filepath.Join(base, args.FilePath), logger)
	default:
		chartName, chartVersion, err = readChartYAML(fsTree, args.ServiceName, logger)
	}

	if err != nil {
		logger.Errorf("Failed to read chart.yaml, err %s", err)
		return nil, err
	}

	valuesYaml := args.MergedValues
	valuesMap := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(valuesYaml), &valuesMap)
	if err != nil {
		logger.Errorf("Failed to unmarshall yaml, err %s", err)
		return nil, err
	}

	containerList, err := commonservice.ParseImagesForProductService(valuesMap, args.ServiceName, args.ProductName)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to parse service from yaml")
	}

	serviceObj := &commonmodels.Service{
		ServiceName: args.ServiceName,
		Type:        setting.HelmDeployType,
		Revision:    args.ServiceRevision,
		ProductName: args.ProductName,
		Visibility:  setting.PrivateVisibility,
		CreateTime:  time.Now().Unix(),
		CreateBy:    args.CreateBy,
		Containers:  containerList,
		CodehostID:  args.CodehostID,
		RepoOwner:   args.Owner,
		RepoName:    args.Repo,
		BranchName:  args.Branch,
		LoadPath:    args.FilePath,
		SrcPath:     args.RepoLink,
		Source:      args.Source,
		HelmChart: &commonmodels.HelmChart{
			Name:       chartName,
			Version:    chartVersion,
			ValuesYaml: valuesYaml,
		},
		CreateFrom: geneCreationDetail(args),
	}

	switch args.Source {
	case string(LoadFromGerrit):
		serviceObj.GerritPath = args.GerritPath
		serviceObj.GerritCodeHostID = args.GerritCodeHostID
		serviceObj.GerritRepoName = args.GerritRepoName
		serviceObj.GerritBranchName = args.GerritBranchName
		serviceObj.GerritRemoteName = args.GerritRemoteName
	}

	log.Infof("Starting to create service %s with revision %d", args.ServiceName, args.ServiceRevision)
	currentSvcTmpl, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
		ProductName:         args.ProductName,
		ServiceName:         args.ServiceName,
		ExcludeStatus:       setting.ProductStatusDeleting,
		IgnoreNoDocumentErr: true,
	})
	if err != nil {
		log.Errorf("Failed to find current service template %s error: %s", args.ServiceName, err)
		return nil, err
	}

	// update status of current service template to deleting
	if currentSvcTmpl != nil {
		err = commonrepo.NewServiceColl().UpdateStatus(args.ServiceName, args.ProductName, setting.ProductStatusDeleting)
		if err != nil {
			log.Errorf("Failed to set status of current service templates, serviceName: %s, err: %s", args.ServiceName, err)
			return nil, err
		}
	}

	// create new service template
	if err = commonrepo.NewServiceColl().Create(serviceObj); err != nil {
		log.Errorf("Failed to create service %s error: %s", args.ServiceName, err)
		return nil, err
	}

	switch args.Source {
	case string(LoadFromGerrit):
		if err := createGerritWebhookByService(args.CodehostID, args.ServiceName, args.Repo, args.Branch); err != nil {
			log.Errorf("Failed to create gerrit webhook, err: %s", err)
			return nil, err
		}
	default:
		commonservice.ProcessServiceWebhook(serviceObj, currentSvcTmpl, args.ServiceName, logger)
	}

	if err = templaterepo.NewProductColl().AddService(args.ProductName, args.ServiceName); err != nil {
		log.Errorf("Failed to add service %s to project %s, err: %s", args.ProductName, args.ServiceName, err)
		return nil, err
	}

	return serviceObj, nil
}

func loadServiceFileInfos(productName, serviceName string, revision int64, dir string) ([]*types.FileInfo, error) {
	svc, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
		ProductName: productName,
		ServiceName: serviceName,
	})
	if err != nil {
		return nil, e.ErrFilePath.AddDesc(err.Error())
	}

	base := config.LocalServicePath(productName, serviceName)
	if revision > 0 {
		base = config.LocalServicePathWithRevision(productName, serviceName, revision)
		if err = commonservice.PreloadServiceManifestsByRevision(base, svc); err != nil {
			log.Warnf("failed to get chart of revision: %d for service: %s, use latest version",
				svc.Revision, svc.ServiceName)
		}
	}
	if err != nil || revision == 0 {
		base = config.LocalServicePath(productName, serviceName)
		err = commonservice.PreLoadServiceManifests(base, svc)
		if err != nil {
			return nil, e.ErrFilePath.AddDesc(err.Error())
		}
	}

	err = commonservice.PreLoadServiceManifests(base, svc)
	if err != nil {
		return nil, e.ErrFilePath.AddDesc(err.Error())
	}

	var fis []*types.FileInfo

	files, err := os.ReadDir(filepath.Join(base, serviceName, dir))
	if err != nil {
		return nil, e.ErrFilePath.AddDesc(err.Error())
	}

	for _, file := range files {
		info, _ := file.Info()
		if info == nil {
			continue
		}
		fi := &types.FileInfo{
			Parent:  dir,
			Name:    file.Name(),
			Size:    info.Size(),
			Mode:    file.Type(),
			ModTime: info.ModTime().Unix(),
			IsDir:   file.IsDir(),
		}

		fis = append(fis, fi)
	}
	return fis, nil
}

// UpdateHelmService TODO need to be deprecated
func UpdateHelmService(args *HelmServiceArgs, log *zap.SugaredLogger) error {
	serviceMap := make(map[string]int64)
	for _, helmServiceInfo := range args.HelmServiceInfos {

		opt := &commonrepo.ServiceFindOption{
			ProductName: args.ProductName,
			ServiceName: helmServiceInfo.ServiceName,
			Type:        setting.HelmDeployType,
		}
		preServiceTmpl, err := commonrepo.NewServiceColl().Find(opt)
		if err != nil {
			return e.ErrUpdateTemplate.AddDesc(err.Error())
		}

		base := config.LocalServicePath(args.ProductName, helmServiceInfo.ServiceName)
		if err = commonservice.PreLoadServiceManifests(base, preServiceTmpl); err != nil {
			return e.ErrUpdateTemplate.AddDesc(err.Error())
		}

		filePath := filepath.Join(base, helmServiceInfo.ServiceName, helmServiceInfo.FilePath, helmServiceInfo.FileName)
		if err = os.WriteFile(filePath, []byte(helmServiceInfo.FileContent), 0644); err != nil {
			log.Errorf("Failed to write file %s, err: %s", filePath, err)
			return e.ErrUpdateTemplate.AddDesc(err.Error())
		}

		// TODO：use yaml compare instead of just comparing the characters
		// TODO service variables
		if helmServiceInfo.FileName == setting.ValuesYaml && preServiceTmpl.HelmChart.ValuesYaml != helmServiceInfo.FileContent {
			var valuesMap map[string]interface{}
			if err = yaml.Unmarshal([]byte(helmServiceInfo.FileContent), &valuesMap); err != nil {
				return e.ErrCreateTemplate.AddDesc("values.yaml解析失败")
			}

			containerList, err := commonservice.ParseImagesForProductService(valuesMap, preServiceTmpl.ServiceName, preServiceTmpl.ProductName)
			if err != nil {
				return e.ErrUpdateTemplate.AddErr(errors.Wrapf(err, "failed to parse images from yaml"))
			}

			preServiceTmpl.Containers = containerList
			preServiceTmpl.HelmChart.ValuesYaml = helmServiceInfo.FileContent

			//修改helm renderset
			renderOpt := &commonrepo.RenderSetFindOption{Name: args.ProductName}
			if rs, err := commonrepo.NewRenderSetColl().Find(renderOpt); err == nil {
				for _, chartInfo := range rs.ChartInfos {
					if chartInfo.ServiceName == helmServiceInfo.ServiceName {
						chartInfo.ValuesYaml = helmServiceInfo.FileContent
						break
					}
				}
				if err = commonrepo.NewRenderSetColl().Update(rs); err != nil {
					log.Errorf("[renderset.update] err:%v", err)
				}
			}
		} else if helmServiceInfo.FileName == setting.ChartYaml {
			chart := new(Chart)
			if err = yaml.Unmarshal([]byte(helmServiceInfo.FileContent), chart); err != nil {
				return e.ErrCreateTemplate.AddDesc(fmt.Sprintf("解析%s失败", setting.ChartYaml))
			}
			if preServiceTmpl.HelmChart.Version != chart.Version {
				preServiceTmpl.HelmChart.Version = chart.Version

				//修改helm renderset
				renderOpt := &commonrepo.RenderSetFindOption{Name: args.ProductName}
				if rs, err := commonrepo.NewRenderSetColl().Find(renderOpt); err == nil {
					for _, chartInfo := range rs.ChartInfos {
						if chartInfo.ServiceName == helmServiceInfo.ServiceName {
							chartInfo.ChartVersion = chart.Version
							break
						}
					}
					if err = commonrepo.NewRenderSetColl().Update(rs); err != nil {
						log.Errorf("[renderset.update] err:%v", err)
					}
				}
			}
		}

		preServiceTmpl.CreateBy = args.CreateBy
		serviceTemplate := fmt.Sprintf(setting.ServiceTemplateCounterName, helmServiceInfo.ServiceName, preServiceTmpl.ProductName)
		rev, err := commonrepo.NewCounterColl().GetNextSeq(serviceTemplate)
		if err != nil {
			return fmt.Errorf("get next helm service revision error: %v", err)
		}

		serviceMap[helmServiceInfo.ServiceName] = rev
		preServiceTmpl.Revision = rev
		if err := commonrepo.NewServiceColl().Delete(helmServiceInfo.ServiceName, setting.HelmDeployType, args.ProductName, setting.ProductStatusDeleting, preServiceTmpl.Revision); err != nil {
			log.Errorf("helmService.update delete %s error: %v", helmServiceInfo.ServiceName, err)
		}

		if err := commonrepo.NewServiceColl().Create(preServiceTmpl); err != nil {
			log.Errorf("helmService.update serviceName:%s error:%v", helmServiceInfo.ServiceName, err)
			return e.ErrUpdateTemplate.AddDesc(err.Error())
		}
	}

	for serviceName, rev := range serviceMap {
		s3Base := config.ObjectStorageServicePath(args.ProductName, serviceName)
		if err := fsservice.ArchiveAndUploadFilesToS3(os.DirFS(config.LocalServicePath(args.ProductName, serviceName)), []string{serviceName, fmt.Sprintf("%s-%d", serviceName, rev)}, s3Base, log); err != nil {
			return e.ErrUpdateTemplate.AddDesc(err.Error())
		}
	}

	return nil
}

// compareHelmVariable 比较helm变量是否有改动，是否需要添加新的renderSet
func compareHelmVariable(chartInfos []*templatemodels.RenderChart, productName, createdBy string, log *zap.SugaredLogger) {
	// 对比上个版本的renderset，新增一个版本
	latestChartInfos := make([]*templatemodels.RenderChart, 0)
	renderOpt := &commonrepo.RenderSetFindOption{Name: productName}
	if latestDefaultRenderSet, err := commonrepo.NewRenderSetColl().Find(renderOpt); err == nil {
		latestChartInfos = latestDefaultRenderSet.ChartInfos
	}

	currentChartInfoMap := make(map[string]*templatemodels.RenderChart)
	for _, chartInfo := range chartInfos {
		currentChartInfoMap[chartInfo.ServiceName] = chartInfo
	}

	mixtureChartInfos := make([]*templatemodels.RenderChart, 0)
	for _, latestChartInfo := range latestChartInfos {
		//如果新的里面存在就拿新的数据替换，不存在就还使用老的数据
		if currentChartInfo, isExist := currentChartInfoMap[latestChartInfo.ServiceName]; isExist {
			mixtureChartInfos = append(mixtureChartInfos, currentChartInfo)
			delete(currentChartInfoMap, latestChartInfo.ServiceName)
			continue
		}
		mixtureChartInfos = append(mixtureChartInfos, latestChartInfo)
	}

	//把新增的服务添加到新的slice里面
	for _, chartInfo := range currentChartInfoMap {
		mixtureChartInfos = append(mixtureChartInfos, chartInfo)
	}

	//添加renderset
	if err := commonservice.CreateHelmRenderSet(
		&models.RenderSet{
			Name:        productName,
			Revision:    0,
			ProductTmpl: productName,
			UpdateBy:    createdBy,
			ChartInfos:  mixtureChartInfos,
		}, log,
	); err != nil {
		log.Errorf("helmService.Create CreateHelmRenderSet error: %v", err)
	}
}
