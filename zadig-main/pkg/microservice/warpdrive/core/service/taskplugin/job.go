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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/warpdrive/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/taskplugin/s3"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	"github.com/koderover/zadig/pkg/tool/kube/containerlog"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/podexec"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"github.com/koderover/zadig/pkg/tool/log"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
	commontypes "github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/util"
)

const (
	defaultSecretEmail = "bot@koderover.com"
	PredatorPlugin     = "predator-plugin"
	JenkinsPlugin      = "jenkins-plugin"
	PackagerPlugin     = "packager-plugin"
	NormalSchedule     = "normal"
	RequiredSchedule   = "required"
	PreferredSchedule  = "preferred"

	registrySecretSuffix    = "-registry-secret"
	ResourceServer          = "resource-server"
	DindServer              = "dind"
	KoderoverAgentNamespace = "koderover-agent"
)

func saveFile(src io.Reader, localFile string) error {
	out, err := os.Create(localFile)
	if err != nil {
		return err
	}

	defer out.Close()

	_, err = io.Copy(out, src)
	return err
}

func saveContainerLog(pipelineTask *task.Task, namespace, clusterID, fileName string, jobLabel *JobLabel, kubeClient client.Client) error {
	selector := labels.Set(getJobLabels(jobLabel)).AsSelector()
	pods, err := getter.ListPods(namespace, selector, kubeClient)
	if err != nil {
		return err
	}

	if len(pods) < 1 {
		return fmt.Errorf("no pod found with selector: %s", selector)
	}

	if len(pods[0].Status.ContainerStatuses) < 1 {
		return fmt.Errorf("no cotainer statuses : %s", selector)
	}

	buf := new(bytes.Buffer)
	// 默认取第一个build job的第一个pod的第一个container的日志
	sort.SliceStable(pods, func(i, j int) bool {
		return pods[i].CreationTimestamp.Before(&pods[j].CreationTimestamp)
	})

	clientSet, err := kubeclient.GetClientset(pipelineTask.ConfigPayload.HubServerAddr, clusterID)
	if err != nil {
		log.Errorf("saveContainerLog, get client set error: %s", err)
		return err
	}

	if err := containerlog.GetContainerLogs(namespace, pods[0].Name, pods[0].Spec.Containers[0].Name, false, int64(0), buf, clientSet); err != nil {
		return err
	}

	if tempFileName, err := util.GenerateTmpFile(); err == nil {
		defer func() {
			_ = os.Remove(tempFileName)
		}()
		if err = saveFile(buf, tempFileName); err == nil {
			var store *s3.S3
			if store, err = s3.NewS3StorageFromEncryptedURI(pipelineTask.StorageURI); err != nil {
				log.Errorf("failed to NewS3StorageFromEncryptedURI ")
				return err
			}
			if store.Subfolder != "" {
				store.Subfolder = fmt.Sprintf("%s/%s/%d/%s", store.Subfolder, strings.ToLower(pipelineTask.PipelineName), pipelineTask.TaskID, "log")
			} else {
				store.Subfolder = fmt.Sprintf("%s/%d/%s", strings.ToLower(pipelineTask.PipelineName), pipelineTask.TaskID, "log")
			}
			forcedPathStyle := true
			if store.Provider == setting.ProviderSourceAli {
				forcedPathStyle = false
			}
			s3client, err := s3tool.NewClient(store.Endpoint, store.Ak, store.Sk, store.Insecure, forcedPathStyle)
			if err != nil {
				return fmt.Errorf("saveContainerLog s3 create client error: %v", err)
			}
			objectKey := store.GetObjectPath(fileName + ".log")
			if err = s3client.Upload(
				store.Bucket,
				tempFileName,
				objectKey,
			); err != nil {
				return fmt.Errorf("saveContainerLog s3 Upload error: %v", err)
			}
		} else {
			return fmt.Errorf("saveContainerLog saveFile error: %v", err)
		}
	} else {
		return fmt.Errorf("saveContainerLog GenerateTmpFile error: %v", err)
	}

	// 下载容器日志到本地 （单线程pipeline）
	//logDir := pipelineTask.ConfigPayload.NFS.GetLogPath()
	//if err = os.MkdirAll(logDir, os.ModePerm); err != nil {
	//	return fmt.Errorf("failed to create log dir: %v", err)
	//}
	//
	//localFile := path.Join(logDir, fileName)
	//err = saveFile(buf, localFile)
	//if err != nil {
	//	return fmt.Errorf("save build log file error: %v", err)
	//}

	return nil
}

type JobCtxBuilder struct {
	JobName        string
	ArchiveFile    string
	TestReportFile string
	PipelineCtx    *task.PipelineCtx
	JobCtx         task.JobCtx
	Installs       []*task.Install
}

func replaceWrapLine(script string) string {
	return strings.Replace(strings.Replace(
		script,
		"\r\n",
		"\n",
		-1,
	), "\r", "\n", -1)
}

// BuildReaperContext builds a yaml
func (b *JobCtxBuilder) BuildReaperContext(pipelineTask *task.Task, serviceName string) *types.Context {
	ctx := &types.Context{
		APIToken:       pipelineTask.ConfigPayload.APIToken,
		Workspace:      b.PipelineCtx.Workspace,
		CleanWorkspace: b.JobCtx.CleanWorkspace,
		IgnoreCache:    pipelineTask.ConfigPayload.IgnoreCache,
		// ResetCache:     pipelineTask.ConfigPayload.ResetCache,
		Proxy: &types.Proxy{
			Type:                   pipelineTask.ConfigPayload.Proxy.Type,
			Address:                pipelineTask.ConfigPayload.Proxy.Address,
			Port:                   pipelineTask.ConfigPayload.Proxy.Port,
			NeedPassword:           pipelineTask.ConfigPayload.Proxy.NeedPassword,
			Username:               pipelineTask.ConfigPayload.Proxy.Username,
			Password:               pipelineTask.ConfigPayload.Proxy.Password,
			EnableRepoProxy:        pipelineTask.ConfigPayload.Proxy.EnableRepoProxy,
			EnableApplicationProxy: pipelineTask.ConfigPayload.Proxy.EnableApplicationProxy,
		},
		Installs:   make([]*types.Install, 0),
		Repos:      make([]*types.Repo, 0),
		Envs:       []string{},
		SecretEnvs: types.EnvVar{},
		Git: &types.Git{
			GithubSSHKey: pipelineTask.ConfigPayload.Github.SSHKey,
			GitlabSSHKey: pipelineTask.ConfigPayload.Gitlab.SSHKey,
			GitKnownHost: pipelineTask.ConfigPayload.GetGitKnownHost(),
		},
		Scripts:         make([]string, 0),
		PostScripts:     make([]string, 0),
		PMDeployScripts: make([]string, 0),
		SSHs:            b.JobCtx.SSHs,
		TestType:        b.JobCtx.TestType,
		ClassicBuild:    pipelineTask.ConfigPayload.ClassicBuild,
		StorageURI:      pipelineTask.StorageURI,
		PipelineName:    pipelineTask.PipelineName,
		TaskID:          pipelineTask.TaskID,
		ServiceName:     serviceName,
		StorageEndpoint: pipelineTask.StorageEndpoint,
		AesKey:          pipelineTask.ConfigPayload.AesKey,
	}

	if b.PipelineCtx.CacheEnable && !pipelineTask.ConfigPayload.ResetCache {
		ctx.CacheEnable = true
		ctx.Cache = b.PipelineCtx.Cache
		ctx.CacheDirType = b.PipelineCtx.CacheDirType
		ctx.CacheUserDir = b.PipelineCtx.CacheUserDir
	}

	for _, install := range b.Installs {
		inst := &types.Install{
			// TODO: 之后可以适配 install.Scripts 为[]string
			// adapt windows style new line
			Name:     install.Name,
			Version:  install.Version,
			Download: install.DownloadPath,
			Scripts: strings.Split(
				replaceWrapLine(install.Scripts), "\n",
			),
			BinPath: install.BinPath,
			Envs:    install.Envs,
		}
		ctx.Installs = append(ctx.Installs, inst)
	}

	for _, build := range b.JobCtx.Builds {
		repo := &types.Repo{
			Source:       build.Source,
			Owner:        build.RepoOwner,
			Name:         build.RepoName,
			RemoteName:   build.RemoteName,
			Branch:       build.Branch,
			PR:           build.PR,
			Tag:          build.Tag,
			CheckoutPath: build.CheckoutPath,
			SubModules:   build.SubModules,
			OauthToken:   build.OauthToken,
			Address:      build.Address,
			CheckoutRef:  build.CheckoutRef,
			User:         build.Username,
			Password:     build.Password,
			EnableProxy:  build.EnableProxy,
		}
		ctx.Repos = append(ctx.Repos, repo)
	}

	for _, ev := range b.JobCtx.EnvVars {
		val := fmt.Sprintf("%s=%s", ev.Key, ev.Value)
		if ev.IsCredential {
			ctx.SecretEnvs = append(ctx.SecretEnvs, val)
		} else {
			ctx.Envs = append(ctx.Envs, val)
		}
	}

	//Support multi build steps
	for _, buildStep := range b.JobCtx.BuildSteps {
		ctx.Scripts = append(ctx.Scripts, strings.Split(replaceWrapLine(buildStep.Scripts), "\n")...)
	}

	if b.JobCtx.PostScripts != "" {
		ctx.PostScripts = append(ctx.PostScripts, strings.Split(replaceWrapLine(b.JobCtx.PostScripts), "\n")...)
	}

	if b.JobCtx.PMDeployScripts != "" {
		ctx.PMDeployScripts = append(ctx.PMDeployScripts, strings.Split(replaceWrapLine(b.JobCtx.PMDeployScripts), "\n")...)
	}

	ctx.Archive = &types.Archive{
		Dir:            b.PipelineCtx.DistDir,
		File:           b.ArchiveFile,
		TestReportFile: b.TestReportFile,
		//StorageUri: b.JobCtx.StorageUri,
	}

	ctx.DockerRegistry = &types.DockerRegistry{
		Host:     pipelineTask.ConfigPayload.Registry.Addr,
		UserName: pipelineTask.ConfigPayload.Registry.AccessKey,
		Password: pipelineTask.ConfigPayload.Registry.SecretKey,
	}

	if b.JobCtx.DockerBuildCtx != nil {
		ctx.DockerBuildCtx = &task.DockerBuildCtx{
			Source:                b.JobCtx.DockerBuildCtx.Source,
			WorkDir:               b.JobCtx.DockerBuildCtx.WorkDir,
			DockerFile:            b.JobCtx.DockerBuildCtx.DockerFile,
			ImageName:             b.JobCtx.DockerBuildCtx.ImageName,
			BuildArgs:             b.JobCtx.DockerBuildCtx.BuildArgs,
			DockerTemplateContent: b.JobCtx.DockerBuildCtx.DockerTemplateContent,
		}
	}

	if b.JobCtx.FileArchiveCtx != nil {
		ctx.FileArchiveCtx = b.JobCtx.FileArchiveCtx
	}

	ctx.Caches = b.JobCtx.Caches

	if b.JobCtx.TestResultPath != "" {
		ctx.GinkgoTest = &types.GinkgoTest{
			ResultPath:     b.JobCtx.TestResultPath,
			TestReportPath: b.JobCtx.TestReportPath,
			ArtifactPaths:  b.JobCtx.ArtifactPaths,
		}
	}

	ctx.StorageEndpoint = pipelineTask.ConfigPayload.S3Storage.Endpoint
	ctx.StorageAK = pipelineTask.ConfigPayload.S3Storage.Ak
	ctx.StorageSK = pipelineTask.ConfigPayload.S3Storage.Sk
	ctx.StorageBucket = pipelineTask.ConfigPayload.S3Storage.Bucket
	ctx.StorageProvider = pipelineTask.ConfigPayload.S3Storage.Provider
	if pipelineTask.ArtifactInfo != nil {
		ctx.ArtifactInfo = &types.ArtifactInfo{
			URL:          pipelineTask.ArtifactInfo.URL,
			WorkflowName: pipelineTask.ArtifactInfo.WorkflowName,
			TaskID:       pipelineTask.ArtifactInfo.TaskID,
			FileName:     pipelineTask.ArtifactInfo.FileName,
		}
	}
	if b.JobCtx.ArtifactPath != "" {
		ctx.ArtifactPath = b.JobCtx.ArtifactPath
	}

	return ctx
}

func ensureDeleteConfigMap(namespace string, jobLabel *JobLabel, kubeClient client.Client) error {
	ls := getJobLabels(jobLabel)
	return updater.DeleteConfigMapsAndWait(namespace, labels.Set(ls).AsSelector(), kubeClient)
}

func ensureDeleteJob(namespace string, jobLabel *JobLabel, kubeClient client.Client) error {
	ls := getJobLabels(jobLabel)
	return updater.DeleteJobsAndWait(namespace, labels.Set(ls).AsSelector(), kubeClient)
}

// JobLabel is to describe labels that specify job identity
type JobLabel struct {
	PipelineName string
	TaskID       int64
	TaskType     string
	ServiceName  string
	PipelineType string
}

const (
	jobLabelTaskKey    = "s-task"
	jobLabelServiceKey = "s-service"
	jobLabelSTypeKey   = "s-type"
	jobLabelPTypeKey   = "p-type"
)

// getJobLabels get labels k-v map from JobLabel struct
func getJobLabels(jobLabel *JobLabel) map[string]string {
	retMap := map[string]string{
		jobLabelTaskKey:    fmt.Sprintf("%s-%d", strings.ToLower(jobLabel.PipelineName), jobLabel.TaskID),
		jobLabelServiceKey: strings.ToLower(jobLabel.ServiceName),
		jobLabelSTypeKey:   strings.Replace(jobLabel.TaskType, "_", "-", -1),
		jobLabelPTypeKey:   jobLabel.PipelineType,
	}
	// no need to add labels with empty value to a job
	for k, v := range retMap {
		if len(v) == 0 {
			delete(retMap, k)
		}
	}
	return retMap
}

func createJobConfigMap(namespace, jobName string, jobLabel *JobLabel, jobCtx string, kubeClient client.Client) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels:    getJobLabels(jobLabel),
		},
		Data: map[string]string{
			"job-config.xml": jobCtx,
		},
	}

	return updater.CreateConfigMap(cm, kubeClient)
}

// JobName is pipelinename-taskid-tasktype-servicename
// e.g. build task JOBNAME = pipelinename-taskid-buildv2-servicename
// e.g. release image JOBNAME = pipelinename-taskid-release-image-servicename
// JOB Label
//"s-job":  pipelinename-taskid-tasktype-servicename,
//"s-task": pipelinename-taskid,
//"s-type": tasktype,
func buildJob(taskType config.TaskType, jobImage, jobName, serviceName, clusterID, currentNamespace string, resReq setting.Request, resReqSpec setting.RequestSpec, ctx *task.PipelineCtx, pipelineTask *task.Task, registries []*task.RegistryNamespace) (*batchv1.Job, error) {
	return buildJobWithLinkedNs(
		taskType,
		jobImage,
		jobName,
		serviceName,
		clusterID,
		currentNamespace,
		resReq,
		resReqSpec,
		ctx,
		pipelineTask,
		registries,
		"",
		"",
	)
}

func buildJobWithLinkedNs(taskType config.TaskType, jobImage, jobName, serviceName, clusterID, currentNamespace string, resReq setting.Request, resReqSpec setting.RequestSpec, ctx *task.PipelineCtx, pipelineTask *task.Task, registries []*task.RegistryNamespace, execNs, linkedNs string) (*batchv1.Job, error) {
	var (
		reaperBootingScript string
		reaperBinaryFile    = pipelineTask.ConfigPayload.Release.ReaperBinaryFile
	)
	// not local cluster
	if clusterID != "" && clusterID != setting.LocalClusterID {
		reaperBinaryFile = strings.Replace(reaperBinaryFile, ResourceServer, ResourceServer+".koderover-agent", -1)
	} else {
		reaperBinaryFile = strings.Replace(reaperBinaryFile, ResourceServer, ResourceServer+"."+currentNamespace, -1)
	}

	if !strings.Contains(jobImage, PredatorPlugin) && !strings.Contains(jobImage, JenkinsPlugin) && !strings.Contains(jobImage, PackagerPlugin) {
		reaperBootingScript = fmt.Sprintf("curl -m 60 --retry-delay 5 --retry 3 -sL %s -o reaper && chmod +x reaper && mv reaper /usr/local/bin && /usr/local/bin/reaper", reaperBinaryFile)
		if pipelineTask.ConfigPayload.Proxy.EnableApplicationProxy && pipelineTask.ConfigPayload.Proxy.Type == "http" {
			reaperBootingScript = fmt.Sprintf("curl -m 60 --retry-delay 5 --retry 3 -sL --proxy %s %s -o reaper && chmod +x reaper && mv reaper /usr/local/bin && /usr/local/bin/reaper",
				pipelineTask.ConfigPayload.Proxy.GetProxyURL(),
				reaperBinaryFile,
			)
		}
	}

	labels := getJobLabels(&JobLabel{
		PipelineName: pipelineTask.PipelineName,
		ServiceName:  serviceName,
		TaskID:       pipelineTask.TaskID,
		TaskType:     string(taskType),
		PipelineType: string(pipelineTask.Type),
	})

	// 引用集成到系统中的私有镜像仓库的访问权限
	ImagePullSecrets := []corev1.LocalObjectReference{
		{
			Name: setting.DefaultImagePullSecret,
		},
	}
	for _, reg := range registries {
		secretName, err := genRegistrySecretName(reg)
		if err != nil {
			return nil, fmt.Errorf("failed to generate registry secret name: %s", err)
		}

		secret := corev1.LocalObjectReference{
			Name: secretName,
		}
		ImagePullSecrets = append(ImagePullSecrets, secret)
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:   jobName,
			Labels: labels,
		},
		Spec: batchv1.JobSpec{
			Completions:  int32Ptr(1),
			Parallelism:  int32Ptr(1),
			BackoffLimit: int32Ptr(0),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy:    corev1.RestartPolicyNever,
					ImagePullSecrets: ImagePullSecrets,
					Containers: []corev1.Container{
						{
							ImagePullPolicy: corev1.PullAlways,
							Name:            labels["s-type"],
							Image:           jobImage,
							Env: []corev1.EnvVar{
								{
									Name:  "JOB_CONFIG_FILE",
									Value: path.Join(ctx.ConfigMapMountDir, "job-config.xml"),
								},
								// 连接对应wd上的dockerdeamon
								{
									Name:  "DOCKER_HOST",
									Value: ctx.DockerHost,
								},
							},
							VolumeMounts: getVolumeMounts(ctx),
							Resources:    getResourceRequirements(resReq, resReqSpec),

							TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
						},
					},
					Volumes: getVolumes(jobName),
				},
			},
		},
	}

	if ctx.CacheEnable && ctx.Cache.MediumType == commontypes.NFSMedium {
		volumeName := "build-cache"
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: ctx.Cache.NFSProperties.PVC,
				},
			},
		})

		mountPath := ctx.CacheUserDir
		if ctx.CacheDirType == commontypes.WorkspaceCacheDir {
			mountPath = "/workspace"
		}

		job.Spec.Template.Spec.Containers[0].VolumeMounts = append(job.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: mountPath,
		})
	}

	if !strings.Contains(jobImage, PredatorPlugin) && !strings.Contains(jobImage, JenkinsPlugin) && !strings.Contains(jobImage, PackagerPlugin) {
		job.Spec.Template.Spec.Containers[0].Command = []string{"/bin/sh", "-c"}
		job.Spec.Template.Spec.Containers[0].Args = []string{reaperBootingScript}
	}

	if affinity := addNodeAffinity(clusterID, pipelineTask.ConfigPayload.K8SClusters); affinity != nil {
		job.Spec.Template.Spec.Affinity = affinity
	}

	if linkedNs != "" && execNs != "" && pipelineTask.ConfigPayload.CustomDNSSupported {
		job.Spec.Template.Spec.DNSConfig = &corev1.PodDNSConfig{
			Searches: []string{
				linkedNs + ".svc.cluster.local",
				execNs + ".svc.cluster.local",
				"svc.cluster.local",
				"cluster.local",
			},
		}

		if addresses, lookupErr := lookupKubeDNSServerHost(); lookupErr == nil {
			job.Spec.Template.Spec.DNSPolicy = corev1.DNSNone
			// https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-s-dns-config
			// There can be at most 3 IP addresses specified
			job.Spec.Template.Spec.DNSConfig.Nameservers = addresses[:Min(3, len(addresses))]
			value := "5"
			job.Spec.Template.Spec.DNSConfig.Options = []corev1.PodDNSConfigOption{
				{Name: "ndots", Value: &value},
			}
		} else {
			log.SugaredLogger().Errorf("failed to find ip of kube dns %v", lookupErr)
		}
	}

	return job, nil
}

// Note: The name of a Secret object must be a valid DNS subdomain name:
//   https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names
func formatRegistryName(namespaceInRegistry string) (string, error) {
	reg, err := regexp.Compile("[^a-zA-Z0-9\\.-]+")
	if err != nil {
		return "", err
	}
	processedName := reg.ReplaceAllString(namespaceInRegistry, "")
	processedName = strings.ToLower(processedName)
	if len(processedName) > 237 {
		processedName = processedName[:237]
	}
	return processedName, nil
}

func createOrUpdateRegistrySecrets(namespace, registryID string, registries []*task.RegistryNamespace, kubeClient client.Client) error {
	for _, reg := range registries {
		if reg.AccessKey == "" {
			continue
		}

		secretName, err := genRegistrySecretName(reg)
		if err != nil {
			return fmt.Errorf("failed to generate registry secret name: %s", err)
		}

		data := make(map[string][]byte)
		dockerConfig := fmt.Sprintf(
			`{"%s":{"username":"%s","password":"%s","email":"%s"}}`,
			reg.RegAddr,
			reg.AccessKey,
			reg.SecretKey,
			defaultSecretEmail,
		)
		data[".dockercfg"] = []byte(dockerConfig)

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      secretName,
			},
			Data: data,
			Type: corev1.SecretTypeDockercfg,
		}
		if err := updater.UpdateOrCreateSecret(secret, kubeClient); err != nil {
			return err
		}
	}

	return nil
}

func Min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func lookupKubeDNSServerHost() (addresses []string, err error) {
	addresses, err = net.LookupHost("kube-dns.kube-system.svc.cluster.local")
	if err != nil {
		return
	}

	if len(addresses) == 0 {
		err = errors.New("no host found")
		return
	}

	return
}

func getVolumeMounts(ctx *task.PipelineCtx) []corev1.VolumeMount {
	resp := make([]corev1.VolumeMount, 0)

	resp = append(resp, corev1.VolumeMount{
		Name:      "job-config",
		MountPath: ctx.ConfigMapMountDir,
	})

	return resp
}

func getVolumes(jobName string) []corev1.Volume {
	resp := make([]corev1.Volume, 0)
	resp = append(resp, corev1.Volume{
		Name: "job-config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: jobName,
				},
			},
		},
	})
	return resp
}

// getResourceRequirements
// ResReqHigh 16 CPU 32 G
// ResReqMedium 8 CPU 16 G
// ResReqLow 4 CPU 8 G used by testing module
// ResReqMin 2 CPU 2 G used by docker build, release image module
// Fallback ResReq 1 CPU 1 G
func getResourceRequirements(resReq setting.Request, resReqSpec setting.RequestSpec) corev1.ResourceRequirements {

	switch resReq {
	case setting.HighRequest:
		return generateResourceRequirements(setting.HighRequest, setting.HighRequestSpec)

	case setting.MediumRequest:
		return generateResourceRequirements(setting.MediumRequest, setting.MediumRequestSpec)

	case setting.LowRequest:
		return generateResourceRequirements(setting.LowRequest, setting.LowRequestSpec)

	case setting.MinRequest:
		return generateResourceRequirements(setting.MinRequest, setting.MinRequestSpec)

	case setting.DefineRequest:
		return generateResourceRequirements(resReq, resReqSpec)

	default:
		return generateResourceRequirements(setting.DefaultRequest, setting.DefaultRequestSpec)
	}
}

//generateResourceRequirements
//cpu Request:Limit=1:4
//memory default Request:Limit=1:4 ; if memoryLimit>= 8Gi,Request:Limit=1:8
func generateResourceRequirements(req setting.Request, reqSpec setting.RequestSpec) corev1.ResourceRequirements {

	if req != setting.DefineRequest {
		return corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(strconv.Itoa(reqSpec.CpuLimit) + setting.CpuUintM),
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(reqSpec.MemoryLimit) + setting.MemoryUintMi),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(strconv.Itoa(reqSpec.CpuReq) + setting.CpuUintM),
				corev1.ResourceMemory: resource.MustParse(strconv.Itoa(reqSpec.MemoryReq) + setting.MemoryUintMi),
			},
		}
	}

	cpuReqInt := reqSpec.CpuLimit / 4
	if cpuReqInt < 1 {
		cpuReqInt = 1
	}
	memoryReqInt := reqSpec.MemoryLimit / 4
	if memoryReqInt >= 2*1024 {
		memoryReqInt = memoryReqInt / 2
	}
	if memoryReqInt < 1 {
		memoryReqInt = 1
	}

	return corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(strconv.Itoa(reqSpec.CpuLimit) + setting.CpuUintM),
			corev1.ResourceMemory: resource.MustParse(strconv.Itoa(reqSpec.MemoryLimit) + setting.MemoryUintMi),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(strconv.Itoa(cpuReqInt) + setting.CpuUintM),
			corev1.ResourceMemory: resource.MustParse(strconv.Itoa(memoryReqInt) + setting.MemoryUintMi),
		},
	}
}

//waitJobEnd
//Returns job status
func waitJobEnd(ctx context.Context, taskTimeout int, namspace, jobName string, kubeClient client.Client, xl *zap.SugaredLogger) (status config.Status) {
	return waitJobEndWithFile(ctx, taskTimeout, namspace, jobName, false, kubeClient, xl)
}

func waitJobReady(ctx context.Context, namespace, jobName string, kubeClient client.Client, xl *zap.SugaredLogger) (status config.Status) {
	xl.Infof("Wait job to start: %s/%s", namespace, jobName)
	podTimeout := time.After(120 * time.Second)

	var started bool
	for {
		select {
		case <-podTimeout:
			return config.StatusTimeout
		default:
			time.Sleep(time.Second)

			job, _, err := getter.GetJob(namespace, jobName, kubeClient)
			if err != nil {
				xl.Errorf("Failed to get job `%s` in namespace `%s`: %s", jobName, namespace, err)
				continue
			}

			podLabels := labels.Set{
				jobLabelPTypeKey:   job.Labels[jobLabelPTypeKey],
				jobLabelServiceKey: job.Labels[jobLabelServiceKey],
				jobLabelTaskKey:    job.Labels[jobLabelTaskKey],
				jobLabelSTypeKey:   job.Labels[jobLabelSTypeKey],
			}
			pods, err := getter.ListPods(namespace, podLabels.AsSelector(), kubeClient)
			if err != nil {
				xl.Errorf("Failed to get pods in namespace `%s` for Job `%s`: %s", namespace, jobName, err)
				continue
			}

			for _, pod := range pods {
				if pod.Status.Phase == corev1.PodRunning {
					started = true
					break
				}
			}
		}

		if started {
			break
		}
	}

	return config.StatusRunning
}

func waitJobEndWithFile(ctx context.Context, taskTimeout int, namespace, jobName string, checkFile bool, kubeClient client.Client, xl *zap.SugaredLogger) (status config.Status) {
	xl.Infof("wait job to start: %s/%s", namespace, jobName)
	timeout := time.After(time.Duration(taskTimeout) * time.Second)
	podTimeout := time.After(120 * time.Second)
	// 等待job运行

	var started bool
	for {
		select {
		case <-podTimeout:
			return config.StatusTimeout
		default:
			job, _, err := getter.GetJob(namespace, jobName, kubeClient)
			if err != nil {
				xl.Errorf("get job failed, namespace:%s, jobName:%s, err:%v", namespace, jobName, err)
			}
			if job != nil {
				started = job.Status.Active > 0
			}
		}
		if started {
			break
		}

		time.Sleep(time.Second)
	}

	// 等待job 运行结束
	xl.Infof("wait job to end: %s %s", namespace, jobName)
	for {
		select {
		case <-ctx.Done():
			return config.StatusCancelled

		case <-timeout:
			return config.StatusTimeout

		default:
			job, found, err := getter.GetJob(namespace, jobName, kubeClient)
			if err != nil || !found {
				xl.Errorf("failed to get pod with label job-name=%s %v", jobName, err)
				return config.StatusFailed
			}
			// pod is still running
			if job.Status.Active != 0 {
				if !checkFile {
					// break only break the select{}, not the outside for{}
					break
				}

				pods, err := getter.ListPods(namespace, labels.Set{"job-name": jobName}.AsSelector(), kubeClient)
				if err != nil {
					xl.Errorf("failed to find pod with label job-name=%s %v", jobName, err)
					return config.StatusFailed
				}

				var done, exists bool
				var jobStatus commontypes.JobStatus
				for _, pod := range pods {
					ipod := wrapper.Pod(pod)
					if ipod.Pending() {
						continue
					}
					if ipod.Failed() {
						return config.StatusFailed
					}

					if !ipod.Finished() {
						jobStatus, exists, err = checkDogFoodExistsInContainer(namespace, ipod.Name, ipod.ContainerNames()[0])
						if err != nil {
							xl.Errorf("Failed to check dog food file %s: %s.", pods[0].Name, err)
							break
						}
						if !exists {
							break
						}
					}
					done = true
				}

				if done {
					xl.Infof("Dog food is found, stop to wait %s. Job status: %s.", job.Name, jobStatus)

					switch jobStatus {
					case commontypes.JobFail:
						return config.StatusFailed
					default:
						return config.StatusPassed
					}
				}
			} else if job.Status.Succeeded != 0 {
				return config.StatusPassed
			} else {
				return config.StatusFailed
			}
		}

		time.Sleep(time.Second * 1)
	}

}

func checkDogFoodExistsInContainer(namespace string, pod string, container string) (commontypes.JobStatus, bool, error) {
	stdout, _, success, err := podexec.ExecWithOptions(podexec.ExecOptions{
		Command:       []string{"/bin/sh", "-c", fmt.Sprintf("test -f %[1]s && cat %[1]s", setting.DogFood)},
		Namespace:     namespace,
		PodName:       pod,
		ContainerName: container,
	})

	return commontypes.JobStatus(stdout), success, err
}

func addNodeAffinity(clusterID string, K8SClusters []*task.K8SCluster) *corev1.Affinity {
	clusterConfig := findClusterConfig(clusterID, K8SClusters)
	if clusterConfig == nil {
		return nil
	}

	if len(clusterConfig.NodeLabels) == 0 {
		return nil
	}

	switch clusterConfig.Strategy {
	case RequiredSchedule:
		nodeSelectorTerms := make([]corev1.NodeSelectorTerm, 0)
		for _, nodeLabel := range clusterConfig.NodeLabels {
			var matchExpressions []corev1.NodeSelectorRequirement
			matchExpressions = append(matchExpressions, corev1.NodeSelectorRequirement{
				Key:      nodeLabel.Key,
				Operator: nodeLabel.Operator,
				Values:   nodeLabel.Value,
			})
			nodeSelectorTerms = append(nodeSelectorTerms, corev1.NodeSelectorTerm{
				MatchExpressions: matchExpressions,
			})
		}

		affinity := &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: nodeSelectorTerms,
				},
			},
		}
		return affinity
	case PreferredSchedule:
		preferredScheduleTerms := make([]corev1.PreferredSchedulingTerm, 0)
		for _, nodeLabel := range clusterConfig.NodeLabels {
			var matchExpressions []corev1.NodeSelectorRequirement
			matchExpressions = append(matchExpressions, corev1.NodeSelectorRequirement{
				Key:      nodeLabel.Key,
				Operator: nodeLabel.Operator,
				Values:   nodeLabel.Value,
			})
			nodeSelectorTerm := corev1.NodeSelectorTerm{
				MatchExpressions: matchExpressions,
			}
			preferredScheduleTerms = append(preferredScheduleTerms, corev1.PreferredSchedulingTerm{
				Weight:     10,
				Preference: nodeSelectorTerm,
			})
		}
		affinity := &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: preferredScheduleTerms,
			},
		}
		return affinity
	default:
		return nil
	}
}

func findClusterConfig(clusterID string, K8SClusters []*task.K8SCluster) *task.AdvancedConfig {
	for _, K8SCluster := range K8SClusters {
		if K8SCluster.ID == clusterID {
			return K8SCluster.AdvancedConfig
		}
	}
	return nil
}

func genRegistrySecretName(reg *task.RegistryNamespace) (string, error) {
	if reg.IsDefault {
		return setting.DefaultImagePullSecret, nil
	}

	arr := strings.Split(reg.Namespace, "/")
	namespaceInRegistry := arr[len(arr)-1]

	// for AWS ECR, there are no namespace, thus we need to find the NS from the URI
	if namespaceInRegistry == "" {
		uriDecipher := strings.Split(reg.RegAddr, ".")
		namespaceInRegistry = uriDecipher[0]
	}

	filteredName, err := formatRegistryName(namespaceInRegistry)
	if err != nil {
		return "", err
	}

	secretName := filteredName + registrySecretSuffix
	if reg.RegType != "" {
		secretName = filteredName + "-" + reg.RegType + registrySecretSuffix
	}

	return secretName, nil
}
