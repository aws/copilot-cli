module github.com/aws/copilot-cli

go 1.14

require (
	github.com/AlecAivazis/survey/v2 v2.2.7
	github.com/Netflix/go-expect v0.0.0-20190729225929-0e00d9168667 // indirect
	github.com/aws/aws-sdk-go v1.37.6
	github.com/briandowns/spinner v1.12.0
	github.com/dustin/go-humanize v1.0.0
	github.com/fatih/color v1.10.0
	github.com/fatih/structs v1.1.0
	github.com/gobuffalo/packd v1.0.0
	github.com/gobuffalo/packr/v2 v2.8.1
	github.com/golang/mock v1.4.4
	github.com/google/go-cmp v0.5.1 // indirect
	github.com/google/shlex v0.0.0-20150127133951-6f45313302b9
	github.com/google/uuid v1.2.0
	github.com/hinshun/vt10x v0.0.0-20180809195222-d55458df857c // indirect
	github.com/imdario/mergo v0.3.9
	github.com/lnquy/cron v1.1.1
	github.com/moby/buildkit v0.7.2
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.4
	github.com/robfig/cron/v3 v3.0.1
	github.com/rogpeppe/go-internal v1.6.1 // indirect
	github.com/sirupsen/logrus v1.6.0 // indirect
	github.com/spf13/afero v1.5.1
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/xlab/treeprint v1.0.0
	golang.org/x/mod v0.4.1
	golang.org/x/sync v0.0.0-20200625203802-6e8e738ad208
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/ini.v1 v1.62.0
	gopkg.in/yaml.v3 v3.0.0-20200605160147-a5ece683394c
)

replace github.com/containerd/containerd => github.com/containerd/containerd v1.3.1-0.20200227195959-4d242818bf55

replace github.com/docker/docker => github.com/docker/docker v1.4.2-0.20200227233006-38f52c9fec82
