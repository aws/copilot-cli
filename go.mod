module github.com/aws/copilot-cli

go 1.14

require (
	github.com/AlecAivazis/survey/v2 v2.1.1
	github.com/aws/aws-sdk-go v1.35.7
	github.com/awslabs/goformation/v4 v4.15.2
	github.com/briandowns/spinner v1.11.1
	github.com/dustin/go-humanize v1.0.0
	github.com/fatih/color v1.9.0
	github.com/fatih/structs v1.1.0
	github.com/gobuffalo/packd v1.0.0
	github.com/gobuffalo/packr/v2 v2.8.0
	github.com/golang/mock v1.4.4
	github.com/google/shlex v0.0.0-20150127133951-6f45313302b9
	github.com/google/uuid v1.1.2
	github.com/imdario/mergo v0.3.9
	github.com/lnquy/cron v1.0.1
	github.com/moby/buildkit v0.7.2
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/robfig/cron/v3 v3.0.1
	github.com/spf13/afero v1.4.1
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	github.com/xlab/treeprint v1.0.0
	golang.org/x/mod v0.3.0
	gopkg.in/ini.v1 v1.62.0
	gopkg.in/yaml.v3 v3.0.0-20200605160147-a5ece683394c
)

replace github.com/containerd/containerd => github.com/containerd/containerd v1.3.1-0.20200227195959-4d242818bf55

replace github.com/docker/docker => github.com/docker/docker v1.4.2-0.20200227233006-38f52c9fec82
