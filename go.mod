module github.com/buildpack/forge

require (
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78
	github.com/Microsoft/go-winio v0.4.8
	github.com/Nvveen/Gotty v0.0.0-20170406111628-a8b993ba6abd
	github.com/PuerkitoBio/purell v1.0.0
	github.com/PuerkitoBio/urlesc v0.0.0-20160726150825-5bd2802263f2
	github.com/agl/ed25519 v0.0.0-20140907235247-d2b94fd789ea
	github.com/beorn7/perks v0.0.0-20160804104726-4c0e84591b9a
	github.com/containerd/console v0.0.0-20180703212128-5d1b48d6114b
	github.com/containerd/containerd v0.0.0-20180706181834-b41633746ed4
	github.com/containerd/continuity v0.0.0-20180524210603-d3c23511c1bf
	github.com/coreos/etcd v3.2.1+incompatible
	github.com/cpuguy83/go-md2man v1.0.8
	github.com/davecgh/go-spew v1.1.0
	github.com/docker/distribution v0.0.0-20180327202408-83389a148052
	github.com/docker/docker v0.0.0-20180712004716-371b590ace0d
	github.com/docker/docker-credential-helpers v0.6.1
	github.com/docker/go v0.0.0-20160303222718-d30aec9fd63c
	github.com/docker/go-connections v0.0.0-20180212134524-7beb39f0b969
	github.com/docker/go-events v0.0.0-20170721190031-9461782956ad
	github.com/docker/go-metrics v0.0.0-20170502235133-d466d4f6fd96
	github.com/docker/go-units v0.0.0-20170127094116-9e638d38cf69
	github.com/docker/swarmkit v0.0.0-20180705210007-199cf49cd996
	github.com/emicklei/go-restful v0.0.0-20170410110728-ff4f55a20633
	github.com/emicklei/go-restful-swagger12 v0.0.0-20170208215640-dcef7f557305
	github.com/flynn-archive/go-shlex v0.0.0-20150515145356-3f9db97f8568
	github.com/ghodss/yaml v1.0.0
	github.com/go-openapi/jsonpointer v0.0.0-20160704185906-46af16f9f7b1
	github.com/go-openapi/jsonreference v0.0.0-20160704190145-13c6e3589ad9
	github.com/go-openapi/spec v0.0.0-20160808142527-6aced65f8501
	github.com/go-openapi/swag v0.0.0-20160704191624-1d0bd113de87
	github.com/gogo/protobuf v1.0.0
	github.com/golang/glog v0.0.0-20141105023935-44145f04b68c
	github.com/golang/mock v1.1.1
	github.com/golang/protobuf v1.1.0
	github.com/google/btree v0.0.0-20161217183710-316fb6d3f031
	github.com/google/go-cmp v0.2.0
	github.com/google/gofuzz v0.0.0-20161122191042-44d81051d367
	github.com/google/shlex v0.0.0-20150127133951-6f45313302b9
	github.com/googleapis/gnostic v0.0.0-20170717235551-e4f56557df62
	github.com/gorilla/context v0.0.0-20160226214623-1ea25387ff6f
	github.com/gorilla/mux v0.0.0-20160317213430-0eeaf8392f5b
	github.com/gregjones/httpcache v0.0.0-20170926212834-c1f8028e62ad
	github.com/grpc-ecosystem/grpc-gateway v0.0.0-20170714172803-1a03ca3bad1e
	github.com/grpc-ecosystem/grpc-opentracing v0.0.0-20180507213350-8e809c8a8645
	github.com/hashicorp/golang-lru v0.0.0-20160207214719-a0d98a5f2880
	github.com/howeyc/gopass v0.0.0-20160826175423-3ca23474a7c7
	github.com/hpcloud/tail v1.0.0 // indirect
	github.com/imdario/mergo v0.3.5
	github.com/inconshreveable/mousetrap v1.0.0
	github.com/json-iterator/go v0.0.0-20171010005702-6240e1e7983a
	github.com/juju/ratelimit v0.0.0-20170523012141-5b9ff8664717
	github.com/mailru/easyjson v0.0.0-20160728113105-d5b7844b561a
	github.com/mattn/go-shellwords v1.0.3
	github.com/matttproud/golang_protobuf_extensions v1.0.0
	github.com/miekg/pkcs11 v0.0.0-20180208123018-5f6e0d0dad6f
	github.com/mitchellh/mapstructure v0.0.0-20161020161836-f3009df150da
	github.com/moby/buildkit v0.0.0-20180713055055-98f1604134f9
	github.com/morikuni/aec v0.0.0-20170113033406-39771216ff4c
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/onsi/ginkgo v1.6.0
	github.com/onsi/gomega v1.4.1
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/opencontainers/image-spec v1.0.1
	github.com/opencontainers/runc v0.0.0-20180615140650-ad0f5255060d
	github.com/opentracing/opentracing-go v0.0.0-20171003133519-1361b9cd60be
	github.com/peterbourgon/diskv v2.0.1+incompatible
	github.com/pkg/errors v0.0.0-20161002052512-839d9e913e06
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_golang v0.0.0-20160802072246-52437c81da6b
	github.com/prometheus/client_model v0.0.0-20150212101744-fa8ad6fec335
	github.com/prometheus/common v0.0.0-20160801171955-ebdfc6da4652
	github.com/prometheus/procfs v0.0.0-20160411190841-abf152e5f3e9
	github.com/russross/blackfriday v0.0.0-20160531111224-1d6b8e9301e7
	github.com/shurcooL/sanitized_anchor_name v0.0.0-20151028001915-10ef21a441db
	github.com/sirupsen/logrus v1.0.3
	github.com/spf13/cobra v0.0.3
	github.com/spf13/pflag v1.0.1
	github.com/stretchr/testify v1.2.2 // indirect
	github.com/theupdateframework/notary v0.6.1
	github.com/tonistiigi/fsutil v0.0.0-20180610154556-8abad97ee396
	github.com/tonistiigi/units v0.0.0-20180711220420-6950e57a87ea
	github.com/xeipuuv/gojsonpointer v0.0.0-20151027082146-e0fe6f683076
	github.com/xeipuuv/gojsonreference v0.0.0-20150808065054-e02fc20de94c
	github.com/xeipuuv/gojsonschema v0.0.0-20160323030313-93e72a773fad
	golang.org/x/crypto v0.0.0-20180515001509-1a580b3eff78
	golang.org/x/net v0.0.0-20180124060956-0ed95abb35c4
	golang.org/x/sync v0.0.0-20171101214715-fd80eb99c8f6
	golang.org/x/sys v0.0.0-20180202135801-37707fdb30a5
	golang.org/x/text v0.0.0-20170113092929-f72d8390a633
	golang.org/x/time v0.0.0-20160202183820-a4bde1265759
	google.golang.org/genproto v0.0.0-20180523212516-694d95ba50e6
	google.golang.org/grpc v1.12.0
	gopkg.in/fsnotify.v1 v1.4.7 // indirect
	gopkg.in/inf.v0 v0.9.0
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v2 v2.0.0-20170125143719-4c78c975fe7c
	gotest.tools v2.1.0+incompatible
	k8s.io/api v0.0.0-20180510182548-a315a049e7a9
	k8s.io/apimachinery v0.0.0-20180510182146-40eaf68ee188
	k8s.io/client-go v0.0.0-20180510183819-c7d1a4cb7528
	k8s.io/kube-openapi v0.0.0-20180509233829-0c329704159e
	k8s.io/kubernetes v1.8.14
	vbom.ml/util v0.0.0-20151108152656-928aaa586d77
)

replace (
	github.com/Nvveen/Gotty v0.0.0-20170406111628-a8b993ba6abd => github.com/ijc25/Gotty v0.0.0-20170406111628-a8b993ba6abd
	github.com/docker/docker v0.0.0-20180712004716-371b590ace0d => github.com/docker/engine v0.0.0-20180712004716-371b590ace0d
)
