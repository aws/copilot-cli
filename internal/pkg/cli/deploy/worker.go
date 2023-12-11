// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"fmt"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/session"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/partitions"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/deploy/upload/customresource"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var (
	fmtErrTopicSubscriptionNotAllowed = "SNS topic %s does not exist in environment %s"
	resourceNameFormat                = "%s-%s-%s-%s" // Format for copilot resource names of form app-env-svc-name
)

type snsTopicsLister interface {
	ListSNSTopics(appName string, envName string) ([]deploy.Topic, error)
}

type workerSvcDeployer struct {
	*svcDeployer
	wsMft *manifest.WorkerService

	// Overriden in tests.
	topicLister snsTopicsLister
	newStack    func() cloudformation.StackConfiguration
}

// IsServiceAvailableInRegion checks if service type exist in the given region.
func (workerSvcDeployer) IsServiceAvailableInRegion(region string) (bool, error) {
	return partitions.IsAvailableInRegion(awsecs.EndpointsID, region)
}

// NewWorkerSvcDeployer is the constructor for workerSvcDeployer.
func NewWorkerSvcDeployer(in *WorkloadDeployerInput) (*workerSvcDeployer, error) {
	in.customResources = workerCustomResources
	svcDeployer, err := newSvcDeployer(in)
	if err != nil {
		return nil, err
	}
	deployStore, err := deploy.NewStore(in.SessionProvider, svcDeployer.store)
	if err != nil {
		return nil, fmt.Errorf("new deploy store: %w", err)
	}
	wsMft, ok := in.Mft.(*manifest.WorkerService)
	if !ok {
		return nil, fmt.Errorf("manifest is not of type %s", manifestinfo.WorkerServiceType)
	}
	return &workerSvcDeployer{
		svcDeployer: svcDeployer,
		topicLister: deployStore,
		wsMft:       wsMft,
	}, nil
}

func workerCustomResources(fs template.Reader) ([]*customresource.CustomResource, error) {
	crs, err := customresource.Worker(fs)
	if err != nil {
		return nil, fmt.Errorf("read custom resources for a %q: %w", manifestinfo.WorkerServiceType, err)
	}
	return crs, nil
}

// UploadArtifacts uploads the deployment artifacts such as the container image, custom resources, addons and env files.
func (d *workerSvcDeployer) UploadArtifacts() (*UploadArtifactsOutput, error) {
	return d.uploadArtifacts(d.buildAndPushContainerImages, d.uploadArtifactsToS3, d.uploadCustomResources)
}

type workerSvcDeployOutput struct {
	subs []manifest.TopicSubscription
}

// RecommendedActions returns the recommended actions after deployment.
func (d *workerSvcDeployOutput) RecommendedActions() []string {
	if d.subs == nil {
		return nil
	}
	retrieveEnvVarCode := "const eventsQueueURI = process.env.COPILOT_QUEUE_URI"
	actionRetrieveEnvVar := fmt.Sprintf(
		`Update worker service code to leverage the injected environment variable "COPILOT_QUEUE_URI".
    In JavaScript you can write %s.`,
		color.HighlightCode(retrieveEnvVarCode),
	)
	recs := []string{actionRetrieveEnvVar}
	topicQueueNames := d.buildWorkerQueueNames()
	if topicQueueNames == "" {
		return recs
	}
	retrieveTopicQueueEnvVarCode := fmt.Sprintf("const {%s} = JSON.parse(process.env.COPILOT_TOPIC_QUEUE_URIS)", topicQueueNames)
	actionRetrieveTopicQueues := fmt.Sprintf(
		`You can retrieve topic-specific queues by writing
    %s.`,
		color.HighlightCode(retrieveTopicQueueEnvVarCode),
	)
	recs = append(recs, actionRetrieveTopicQueues)
	return recs
}

// GenerateCloudFormationTemplate generates a CloudFormation template and parameters for a workload.
func (d *workerSvcDeployer) GenerateCloudFormationTemplate(in *GenerateCloudFormationTemplateInput) (
	*GenerateCloudFormationTemplateOutput, error) {
	output, err := d.stackConfiguration(&in.StackRuntimeConfiguration)
	if err != nil {
		return nil, err
	}
	return d.generateCloudFormationTemplate(output.conf)
}

// DeployWorkload deploys a worker service using CloudFormation.
func (d *workerSvcDeployer) DeployWorkload(in *DeployWorkloadInput) (ActionRecommender, error) {
	stackConfigOutput, err := d.stackConfiguration(&in.StackRuntimeConfiguration)
	if err != nil {
		return nil, err
	}
	if err := d.deploy(in.Options, stackConfigOutput.svcStackConfigurationOutput); err != nil {
		return nil, err
	}
	return &workerSvcDeployOutput{
		subs: stackConfigOutput.subscriptions,
	}, nil
}

func (d *workerSvcDeployOutput) buildWorkerQueueNames() string {
	var queueNames []string
	for _, subscription := range d.subs {
		if subscription.Queue.IsEmpty() {
			continue
		}
		svc := template.StripNonAlphaNumFunc(aws.StringValue(subscription.Service))
		topic := template.StripNonAlphaNumFunc(aws.StringValue(subscription.Name))
		queueNames = append(queueNames, fmt.Sprintf("%s%sEventsQueue", svc, cases.Title(language.English).String(topic)))
	}
	return strings.Join(queueNames, ", ")
}

type workerSvcStackConfigurationOutput struct {
	svcStackConfigurationOutput
	subscriptions []manifest.TopicSubscription
}

func (d *workerSvcDeployer) stackConfiguration(in *StackRuntimeConfiguration) (*workerSvcStackConfigurationOutput, error) {
	rc, err := d.runtimeConfig(in)
	if err != nil {
		return nil, err
	}
	var topics []deploy.Topic
	topics, err = d.topicLister.ListSNSTopics(d.app.Name, d.env.Name)
	if err != nil {
		return nil, fmt.Errorf("get SNS topics for app %s and environment %s: %w", d.app.Name, d.env.Name, err)
	}
	var topicARNs []string
	for _, topic := range topics {
		topicARNs = append(topicARNs, topic.ARN())
	}
	subs := d.wsMft.Subscriptions()
	if err = validateTopicsExist(subs, topicARNs, d.app.Name, d.env.Name); err != nil {
		return nil, err
	}

	var conf cloudformation.StackConfiguration
	switch {
	case d.newStack != nil:
		conf = d.newStack()
	default:
		conf, err = stack.NewWorkerService(stack.WorkerServiceConfig{
			App:                d.app,
			Env:                d.env.Name,
			Manifest:           d.wsMft,
			RawManifest:        d.rawMft,
			ArtifactBucketName: d.resources.S3Bucket,
			ArtifactKey:        d.resources.KMSKeyARN,
			RuntimeConfig:      *rc,
			Addons:             d.addons,
		})
		if err != nil {
			return nil, fmt.Errorf("create stack configuration: %w", err)
		}
	}

	return &workerSvcStackConfigurationOutput{
		svcStackConfigurationOutput: svcStackConfigurationOutput{
			conf: cloudformation.WrapWithTemplateOverrider(conf, d.overrider),
			svcUpdater: d.newSvcUpdater(func(s *session.Session) serviceForceUpdater {
				return ecs.New(s)
			}),
		},
		subscriptions: subs,
	}, nil
}

func validateTopicsExist(subscriptions []manifest.TopicSubscription, topicARNs []string, app, env string) error {
	validTopicResources := make([]string, 0, len(topicARNs))
	for _, topic := range topicARNs {
		parsedTopic, err := arn.Parse(topic)
		if err != nil {
			continue
		}
		validTopicResources = append(validTopicResources, parsedTopic.Resource)
	}

	for _, ts := range subscriptions {
		topicName := fmt.Sprintf(resourceNameFormat, app, env, aws.StringValue(ts.Service), aws.StringValue(ts.Name))
		if !slices.Contains(validTopicResources, topicName) {
			return fmt.Errorf(fmtErrTopicSubscriptionNotAllowed, topicName, env)
		}
	}
	return nil
}
