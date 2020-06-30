module github.com/aws/copilot-cli

go 1.13

require (
	github.com/AlecAivazis/survey/v2 v2.0.7
	github.com/Netflix/go-expect v0.0.0-20190729225929-0e00d9168667 // indirect
	github.com/aws/aws-sdk-go v1.32.11
	github.com/awslabs/goformation/v4 v4.11.0
	github.com/briandowns/spinner v1.11.1
	github.com/dustin/go-humanize v1.0.0
	github.com/fatih/color v1.9.0
	github.com/fatih/structs v1.1.0
	github.com/gobuffalo/packd v1.0.0
	github.com/gobuffalo/packr/v2 v2.8.0
	github.com/golang/mock v1.4.3
	github.com/google/uuid v1.1.1
	github.com/hinshun/vt10x v0.0.0-20180809195222-d55458df857c // indirect
	github.com/imdario/mergo v0.3.9
	github.com/karrick/godirwalk v1.15.6 // indirect
	github.com/mattn/go-colorable v0.1.6 // indirect
	github.com/mitchellh/mapstructure v1.3.0 // indirect
	github.com/moby/buildkit v0.7.1
	github.com/onsi/ginkgo v1.13.0
	github.com/onsi/gomega v1.10.1
	github.com/pelletier/go-toml v1.8.0 // indirect
	github.com/sirupsen/logrus v1.6.0 // indirect
	github.com/spf13/afero v1.3.1
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/cobra v1.0.0
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.0
	github.com/stretchr/testify v1.6.1
	github.com/xlab/treeprint v1.0.0
	golang.org/x/crypto v0.0.0-20200510223506-06a226fb4e37 // indirect
	gopkg.in/ini.v1 v1.57.0
	gopkg.in/yaml.v3 v3.0.0-20200605160147-a5ece683394c
)

replace github.com/containerd/containerd => github.com/containerd/containerd v1.3.1-0.20200227195959-4d242818bf55

replace github.com/docker/docker => github.com/docker/docker v1.4.2-0.20200227233006-38f52c9fec82
