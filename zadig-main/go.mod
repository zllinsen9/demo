module github.com/koderover/zadig

go 1.16

require (
	github.com/27149chen/afero v1.6.2
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 // indirect
	github.com/RyanCarrier/dijkstra v1.1.0
	github.com/andygrunwald/go-gerrit v0.0.0-20171029143327-95b11af228a1
	github.com/aws/aws-sdk-go v1.34.28
	github.com/blang/semver/v4 v4.0.0
	github.com/bndr/gojenkins v1.1.0
	github.com/bradleyfalzon/ghinstallation v1.1.1
	github.com/bugsnag/bugsnag-go v2.1.0+incompatible // indirect
	github.com/bugsnag/panicwrap v1.3.1 // indirect
	github.com/cenkalti/backoff/v4 v4.1.1
	github.com/chartmuseum/helm-push v0.10.1
	github.com/coocood/freecache v1.1.0
	github.com/coreos/go-oidc/v3 v3.0.0
	github.com/dexidp/dex v0.0.0-20210802203454-3fac2ab6bc3b
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v20.10.11+incompatible
	github.com/docker/go-connections v0.4.0
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/dsnet/compress v0.0.1 // indirect
	github.com/garyburd/redigo v1.6.2 // indirect
	github.com/gin-contrib/sse v0.1.0
	github.com/gin-gonic/gin v1.7.2
	github.com/go-ldap/ldap/v3 v3.3.0
	github.com/go-resty/resty/v2 v2.6.0
	github.com/go-sql-driver/mysql v1.6.0
	github.com/gofrs/uuid v4.0.0+incompatible // indirect
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/google/go-github/v35 v35.3.0
	github.com/google/uuid v1.2.0
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.4.2
	github.com/gregjones/httpcache v0.0.0-20181110185634-c63ab54fda8f
	github.com/hashicorp/go-multierror v1.1.1
	github.com/huaweicloud/huaweicloud-sdk-go-v3 v0.0.50
	github.com/jasonlvhit/gocron v0.0.0-20171226191223-3c914c8681c3
	github.com/jinzhu/now v1.1.2
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/mittwald/go-helm-client v0.8.4
	github.com/moby/buildkit v0.9.1
	github.com/nsqio/go-nsq v1.0.7
	github.com/nwaples/rardecode v1.0.0 // indirect
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.15.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/otiai10/copy v1.6.0
	github.com/pkg/errors v0.9.1
	github.com/rfyiamcool/cronlib v1.0.0
	github.com/satori/go.uuid v1.2.0
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.8.1
	github.com/stevvooe/resumable v0.0.0-20180830230917-22b14a53ba50 // indirect
	github.com/stretchr/testify v1.7.0
	github.com/swaggo/files v0.0.0-20190704085106-630677cd5c14
	github.com/swaggo/gin-swagger v1.3.0
	github.com/swaggo/swag v1.5.1
	github.com/xanzy/go-gitlab v0.50.0
	github.com/yvasiyarov/go-metrics v0.0.0-20150112132944-c25f46c4b940 // indirect
	github.com/yvasiyarov/gorelic v0.0.7 // indirect
	github.com/yvasiyarov/newrelic_platform_go v0.0.0-20160601141957-9c099fbc30e9 // indirect
	go.mongodb.org/mongo-driver v1.5.0
	go.uber.org/zap v1.19.0
	golang.org/x/crypto v0.0.0-20211117183948-ae814b36b871
	golang.org/x/net v0.0.0-20211112202133-69e39bad7dc2
	golang.org/x/oauth2 v0.0.0-20210628180205-a41e5a781914
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df
	gopkg.in/mholt/archiver.v3 v3.1.1
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	gorm.io/driver/mysql v1.1.2
	gorm.io/gorm v1.21.16
	helm.sh/helm/v3 v3.7.2
	k8s.io/api v0.22.4
	k8s.io/apiextensions-apiserver v0.22.4
	k8s.io/apimachinery v0.22.4
	k8s.io/client-go v0.22.4
	k8s.io/kubectl v0.22.4
	k8s.io/utils v0.0.0-20210819203725-bdf08cb9a70a
	sigs.k8s.io/controller-runtime v0.10.0
	sigs.k8s.io/yaml v1.3.0
)

replace (
	github.com/docker/distribution => github.com/docker/distribution v2.6.0-rc.1.0.20170726174610-edc3ab29cdff+incompatible
	github.com/docker/docker => github.com/docker/docker v0.0.0-20180502112750-51a9119f6b81
	github.com/docker/go-connections => github.com/docker/go-connections v0.3.1-0.20180212134524-7beb39f0b969
	github.com/go-logr/logr => github.com/go-logr/logr v0.4.0

	k8s.io/api => k8s.io/api v0.21.0
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.21.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.21.0
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.21.0
	k8s.io/client-go => k8s.io/client-go v0.21.0
	k8s.io/klog/v2 => k8s.io/klog/v2 v2.9.0
	k8s.io/kubectl => k8s.io/kubectl v0.21.0
	oras.land/oras-go => oras.land/oras-go v0.4.0
)
