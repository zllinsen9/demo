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

package kube

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"
	"text/template"

	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/tool/crypto"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/multicluster"
)

func GetKubeAPIReader(clusterID string) (client.Reader, error) {
	return kubeclient.GetKubeAPIReader(config.HubServerAddress(), clusterID)
}

func GetRESTConfig(clusterID string) (*rest.Config, error) {
	return kubeclient.GetRESTConfig(config.HubServerAddress(), clusterID)
}

// GetClientset returns a client to interact with APIServer which implements kubernetes.Interface
func GetClientset(clusterID string) (kubernetes.Interface, error) {
	return kubeclient.GetClientset(config.HubServerAddress(), clusterID)
}

type Service struct {
	*multicluster.Agent

	coll *mongodb.K8SClusterColl
}

func NewService(hubServerAddr string) (*Service, error) {
	if hubServerAddr == "" {
		return &Service{coll: mongodb.NewK8SClusterColl()}, nil
	}

	agent, err := multicluster.NewAgent(hubServerAddr)
	if err != nil {
		return nil, err
	}

	return &Service{
		coll:  mongodb.NewK8SClusterColl(),
		Agent: agent,
	}, nil
}

func (s *Service) ListClusters(clusterType string, logger *zap.SugaredLogger) ([]*models.K8SCluster, error) {
	clusters, err := s.coll.Find(clusterType)
	if err != nil {
		logger.Errorf("failed to list clusters %v", err)
		return nil, e.ErrListK8SCluster.AddErr(err)
	}

	if len(clusters) == 0 {
		return make([]*models.K8SCluster, 0), nil
	}
	for _, cluster := range clusters {
		token, err := crypto.AesEncrypt(cluster.ID.Hex())
		if err != nil {
			return nil, err
		}
		cluster.Token = token
	}

	return clusters, nil
}

func (s *Service) CreateCluster(cluster *models.K8SCluster, id string, logger *zap.SugaredLogger) (*models.K8SCluster, error) {
	_, err := s.coll.FindByName(cluster.Name)
	if err == nil {
		logger.Errorf("failed to create cluster %s %v", cluster.Name, err)
		return nil, e.ErrCreateCluster.AddDesc(e.DuplicateClusterNameFound)
	}
	cluster.Status = setting.Pending
	if id == setting.LocalClusterID {
		cluster.Status = setting.Normal
		cluster.Local = true
		cluster.AdvancedConfig = &commonmodels.AdvancedConfig{
			Strategy: "normal",
		}
	}
	err = s.coll.Create(cluster, id)
	if err != nil {
		return nil, e.ErrCreateCluster.AddErr(err)
	}

	if cluster.AdvancedConfig != nil {
		for _, projectName := range cluster.AdvancedConfig.ProjectNames {
			err = commonrepo.NewProjectClusterRelationColl().Create(&commonmodels.ProjectClusterRelation{
				ProjectName: projectName,
				ClusterID:   cluster.ID.Hex(),
				CreatedBy:   cluster.CreatedBy,
			})
			if err != nil {
				logger.Errorf("Failed to create projectClusterRelation err:%s", err)
			}
		}
	}

	token, err := crypto.AesEncrypt(cluster.ID.Hex())
	if err != nil {
		return nil, err
	}
	cluster.Token = token
	return cluster, nil
}

func (s *Service) UpdateCluster(id string, cluster *models.K8SCluster, logger *zap.SugaredLogger) (*models.K8SCluster, error) {
	_, err := s.coll.Get(id)

	if err != nil {
		return nil, e.ErrUpdateCluster.AddErr(e.ErrClusterNotFound.AddDesc(cluster.Name))
	}

	if existed, err := s.coll.HasDuplicateName(id, cluster.Name); existed || err != nil {
		if err != nil {
			logger.Warnf("failed to find duplicated name %v", err)
		}

		return nil, e.ErrUpdateCluster.AddDesc(e.DuplicateClusterNameFound)
	}

	err = s.coll.UpdateMutableFields(cluster, id)
	if err != nil {
		logger.Errorf("failed to update mutable fields %v", err)
		return nil, e.ErrUpdateCluster.AddErr(err)
	}

	token, err := crypto.AesEncrypt(cluster.ID.Hex())
	if err != nil {
		return nil, err
	}
	cluster.Token = token
	return cluster, nil
}

func (s *Service) DeleteCluster(user string, id string, logger *zap.SugaredLogger) error {
	_, err := s.coll.Get(id)
	if err != nil {
		return e.ErrDeleteCluster.AddErr(e.ErrClusterNotFound.AddDesc(id))
	}

	err = s.coll.Delete(id)
	if err != nil {
		logger.Errorf("failed to delete cluster by id %s %v", id, err)
		return e.ErrDeleteCluster.AddErr(err)
	}

	return nil
}

func (s *Service) GetCluster(id string, logger *zap.SugaredLogger) (*models.K8SCluster, error) {
	cluster, err := s.coll.Get(id)
	if err != nil {
		return nil, e.ErrClusterNotFound.AddErr(err)
	}

	token, err := crypto.AesEncrypt(cluster.ID.Hex())
	if err != nil {
		return nil, err
	}
	cluster.Token = token
	return cluster, nil
}

func (s *Service) GetClusterByToken(token string, logger *zap.SugaredLogger) (*models.K8SCluster, error) {
	id, err := crypto.AesDecrypt(token)
	if err != nil {
		return nil, err
	}

	return s.GetCluster(id, logger)
}

func (s *Service) ListConnectedClusters(logger *zap.SugaredLogger) ([]*models.K8SCluster, error) {
	clusters, err := s.coll.FindConnectedClusters()
	if err != nil {
		logger.Errorf("failed to list connected clusters %v", err)
		return nil, e.ErrListK8SCluster.AddErr(err)
	}

	if len(clusters) == 0 {
		return make([]*models.K8SCluster, 0), nil
	}
	for _, cluster := range clusters {
		token, err := crypto.AesEncrypt(cluster.ID.Hex())
		if err != nil {
			return nil, err
		}
		cluster.Token = token
	}

	return clusters, nil
}

func (s *Service) GetYaml(id, agentImage, rsImage, aslanURL, hubURI string, useDeployment bool, logger *zap.SugaredLogger) ([]byte, error) {
	var (
		cluster *models.K8SCluster
		err     error
	)
	if cluster, err = s.GetCluster(id, logger); err != nil {
		return nil, err
	}

	var hubBase *url.URL
	hubBase, err = url.Parse(fmt.Sprintf("%s%s", aslanURL, hubURI))
	if err != nil {
		return nil, err
	}

	if strings.ToLower(hubBase.Scheme) == "https" {
		hubBase.Scheme = "wss"
	} else {
		hubBase.Scheme = "ws"
	}

	buffer := bytes.NewBufferString("")
	token, err := crypto.AesEncrypt(cluster.ID.Hex())
	if err != nil {
		return nil, err
	}

	if cluster.Namespace == "" {
		err = YamlTemplate.Execute(buffer, TemplateSchema{
			HubAgentImage:       agentImage,
			ResourceServerImage: rsImage,
			ClientToken:         token,
			HubServerBaseAddr:   hubBase.String(),
			UseDeployment:       useDeployment,
		})
	} else {
		err = YamlTemplateForNamespace.Execute(buffer, TemplateSchema{
			HubAgentImage:       agentImage,
			ResourceServerImage: rsImage,
			ClientToken:         token,
			HubServerBaseAddr:   hubBase.String(),
			UseDeployment:       useDeployment,
			Namespace:           cluster.Namespace,
		})
	}

	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

type TemplateSchema struct {
	HubAgentImage       string
	ResourceServerImage string
	ClientToken         string
	HubServerBaseAddr   string
	Namespace           string
	UseDeployment       bool
}

var YamlTemplate = template.Must(template.New("agentYaml").Parse(`
---

apiVersion: v1
kind: Namespace
metadata:
  name: koderover-agent

---

apiVersion: v1
kind: ServiceAccount
metadata:
  name: koderover-agent
  namespace: koderover-agent

---

apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: koderover-agent-admin-binding
  namespace: koderover-agent
subjects:
- kind: ServiceAccount
  name: koderover-agent
  namespace: koderover-agent
roleRef:
  kind: ClusterRole
  name: koderover-agent-admin
  apiGroup: rbac.authorization.k8s.io

---

apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: koderover-agent-admin
rules:
- apiGroups:
  - '*'
  resources:
  - '*'
  verbs:
  - '*'
- nonResourceURLs:
  - '*'
  verbs:
  - '*'

---
apiVersion: apps/v1
{{- if .UseDeployment }}
kind: Deployment
{{- else }}
kind: DaemonSet
{{- end }}
metadata:
    name: koderover-agent-node-agent
    namespace: koderover-agent
spec:
  selector:
    matchLabels:
      app: koderover-agent-agent
  template:
    metadata:
      labels:
        app: koderover-agent-agent
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                - key: beta.kubernetes.io/os
                  operator: NotIn
                  values:
                    - windows
{{- if .UseDeployment }}
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              topologyKey: kubernetes.io/hostname
{{- end }}
      hostNetwork: true
      serviceAccountName: koderover-agent
      containers:
      - name: agent
        image: {{.HubAgentImage}}
        imagePullPolicy: Always
        env:
        - name: AGENT_NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: HUB_AGENT_TOKEN
          value: "{{.ClientToken}}"
        - name: HUB_SERVER_BASE_ADDR
          value: "{{.HubServerBaseAddr}}"
        resources:
          limits:
            cpu: 1000m
            memory: 1Gi
          requests:
            cpu: 100m
            memory: 256Mi
{{- if .UseDeployment }}
  replicas: 1
{{- else }}
  updateStrategy:
    type: RollingUpdate
{{- end }}

---

apiVersion: apps/v1
kind: Deployment
metadata:
  name: resource-server
  namespace: koderover-agent
  labels:
    app.kubernetes.io/component: resource-server
    app.kubernetes.io/name: zadig
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/component: resource-server
      app.kubernetes.io/name: zadig
  template:
    metadata:
      labels:
        app.kubernetes.io/component: resource-server
        app.kubernetes.io/name: zadig
    spec:
      containers:
        - image: {{.ResourceServerImage}}
          imagePullPolicy: Always
          name: resource-server
          resources:
            limits:
              cpu: 500m
              memory: 500Mi
            requests:
              cpu: 100m
              memory: 100Mi

---

apiVersion: v1
kind: Service
metadata:
  name: resource-server
  namespace: koderover-agent
  labels:
    app.kubernetes.io/component: resource-server
    app.kubernetes.io/name: zadig
spec:
  type: ClusterIP
  ports:
    - protocol: TCP
      port: 80
      targetPort: 80
  selector:
    app.kubernetes.io/component: resource-server
    app.kubernetes.io/name: zadig

---

apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: dind
  namespace: koderover-agent
  labels:
    app.kubernetes.io/component: dind
    app.kubernetes.io/name: zadig
spec:
  serviceName: dind
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/component: dind
      app.kubernetes.io/name: zadig
  template:
    metadata:
      labels:
        app.kubernetes.io/component: dind
        app.kubernetes.io/name: zadig
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                topologyKey: kubernetes.io/hostname
      containers:
        - name: dind
          image: ccr.ccs.tencentyun.com/koderover-public/library-docker:stable-dind
          args:
            - --mtu=1376
          env:
            - name: DOCKER_TLS_CERTDIR
              value: ""
          securityContext:
            privileged: true
          ports:
            - protocol: TCP
              containerPort: 2375
          resources:
            limits:
              cpu: "4"
              memory: 8Gi
            requests:
              cpu: 100m
              memory: 128Mi

---

apiVersion: v1
kind: Service
metadata:
  name: dind
  namespace: koderover-agent
  labels:
    app.kubernetes.io/component: dind
    app.kubernetes.io/name: zadig
spec:
  ports:
    - name: dind
      protocol: TCP
      port: 2375
      targetPort: 2375
  clusterIP: None
  selector:
    app.kubernetes.io/component: dind
    app.kubernetes.io/name: zadig
`))

var YamlTemplateForNamespace = template.Must(template.New("agentYaml").Parse(`
---

apiVersion: v1
kind: ServiceAccount
metadata:
  name: koderover-agent-sa
  namespace: {{.Namespace}}

---

apiVersion: rbac.authorization.k8s.io/v1beta1
kind: RoleBinding
metadata:
  name: koderover-agent-admin-binding
  namespace: {{.Namespace}}
subjects:
- kind: ServiceAccount
  name: koderover-agent-sa
  namespace: {{.Namespace}}
roleRef:
  kind: Role
  name: koderover-agent-admin-role
  apiGroup: rbac.authorization.k8s.io

---

apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: koderover-agent-admin-role
  namespace: {{.Namespace}}
rules:
- apiGroups:
  - '*'
  resources:
  - '*'
  verbs:
  - '*'

---
apiVersion: apps/v1
{{- if .UseDeployment }}
kind: Deployment
{{- else }}
kind: DaemonSet
{{- end }}
metadata:
    name: koderover-agent-node-agent
    namespace: {{.Namespace}}
spec:
  selector:
    matchLabels:
      app: koderover-agent-agent
  template:
    metadata:
      labels:
        app: koderover-agent-agent
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                - key: beta.kubernetes.io/os
                  operator: NotIn
                  values:
                    - windows
{{- if .UseDeployment }}
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              topologyKey: kubernetes.io/hostname
{{- end }}
      hostNetwork: true
      serviceAccountName: koderover-agent-sa
      containers:
      - name: agent
        image: {{.HubAgentImage}}
        imagePullPolicy: Always
        env:
        - name: AGENT_NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: HUB_AGENT_TOKEN
          value: "{{.ClientToken}}"
        - name: HUB_SERVER_BASE_ADDR
          value: "{{.HubServerBaseAddr}}"
        resources:
          limits:
            cpu: 1000m
            memory: 1Gi
          requests:
            cpu: 100m
            memory: 256Mi
{{- if .UseDeployment }}
  replicas: 1
{{- else }}
  updateStrategy:
    type: RollingUpdate
{{- end }}

---

apiVersion: apps/v1
kind: Deployment
metadata:
  name: resource-server
  namespace: {{.Namespace}}
  labels:
    app.kubernetes.io/component: resource-server
    app.kubernetes.io/name: zadig
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/component: resource-server
      app.kubernetes.io/name: zadig
  template:
    metadata:
      labels:
        app.kubernetes.io/component: resource-server
        app.kubernetes.io/name: zadig
    spec:
      containers:
        - image: {{.ResourceServerImage}}
          imagePullPolicy: Always
          name: resource-server
          resources:
            limits:
              cpu: 500m
              memory: 500Mi
            requests:
              cpu: 100m
              memory: 100Mi

---

apiVersion: v1
kind: Service
metadata:
  name: resource-server
  namespace: {{.Namespace}}
  labels:
    app.kubernetes.io/component: resource-server
    app.kubernetes.io/instance: zadig-zadig
    app.kubernetes.io/name: zadig
spec:
  type: ClusterIP
  ports:
    - protocol: TCP
      port: 80
      targetPort: 80
  selector:
    app.kubernetes.io/component: resource-server
    app.kubernetes.io/name: zadig

---

apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: dind
  namespace: {{.Namespace}}
  labels:
    app.kubernetes.io/component: dind
    app.kubernetes.io/name: zadig
spec:
  serviceName: dind
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/component: dind
      app.kubernetes.io/name: zadig
  template:
    metadata:
      labels:
        app.kubernetes.io/component: dind
        app.kubernetes.io/name: zadig
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                topologyKey: kubernetes.io/hostname
      containers:
        - name: dind
          image: ccr.ccs.tencentyun.com/koderover-public/library-docker:stable-dind
          args:
            - --mtu=1376
          env:
            - name: DOCKER_TLS_CERTDIR
              value: ""
          securityContext:
            privileged: true
          ports:
            - protocol: TCP
              containerPort: 2375
          resources:
            limits:
              cpu: "4"
              memory: 8Gi
            requests:
              cpu: 100m
              memory: 128Mi

---

apiVersion: v1
kind: Service
metadata:
  name: dind
  namespace: {{.Namespace}}
  labels:
    app.kubernetes.io/component: dind
    app.kubernetes.io/name: zadig
spec:
  ports:
    - name: dind
      protocol: TCP
      port: 2375
      targetPort: 2375
  clusterIP: None
  selector:
    app.kubernetes.io/component: dind
    app.kubernetes.io/name: zadig
`))
