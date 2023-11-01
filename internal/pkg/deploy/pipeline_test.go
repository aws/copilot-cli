// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"errors"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/aws/copilot-cli/internal/pkg/config"

	"github.com/aws/copilot-cli/internal/pkg/manifest"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"
)

func TestPipelineSourceFromManifest(t *testing.T) {
	testCases := map[string]struct {
		mfSource             *manifest.Source
		expectedDeploySource interface{}
		expectedShouldPrompt bool
		expectedErr          error
	}{
		"transforms GitHubV1 source": {
			mfSource: &manifest.Source{
				ProviderName: manifest.GithubV1ProviderName,
				Properties: map[string]interface{}{
					"branch":              "test",
					"repository":          "some/repository/URL",
					"access_token_secret": "secretiveSecret",
				},
			},
			expectedDeploySource: &GitHubV1Source{
				ProviderName:                manifest.GithubV1ProviderName,
				Branch:                      "test",
				RepositoryURL:               "some/repository/URL",
				PersonalAccessTokenSecretID: "secretiveSecret",
			},
			expectedShouldPrompt: false,
			expectedErr:          nil,
		},
		"error out if using GitHubV1 while not having access token secret": {
			mfSource: &manifest.Source{
				ProviderName: manifest.GithubV1ProviderName,
				Properties: map[string]interface{}{
					"branch":     "test",
					"repository": "some/repository/URL",
				},
			},
			expectedShouldPrompt: false,
			expectedErr:          errors.New("missing `access_token_secret` in properties"),
		},
		"error out if a string property is not a string": {
			mfSource: &manifest.Source{
				ProviderName: manifest.GithubV1ProviderName,
				Properties: map[string]interface{}{
					"branch":     "test",
					"repository": []int{1, 2, 3},
				},
			},
			expectedShouldPrompt: false,
			expectedErr:          errors.New("property `repository` is not a string"),
		},
		"transforms GitHub (v2) source without existing connection": {
			mfSource: &manifest.Source{
				ProviderName: manifest.GithubProviderName,
				Properties: map[string]interface{}{
					"branch":     "test",
					"repository": "some/repository/URL",
				},
			},
			expectedDeploySource: &GitHubSource{
				ProviderName:  manifest.GithubProviderName,
				Branch:        "test",
				RepositoryURL: "some/repository/URL",
			},
			expectedShouldPrompt: true,
			expectedErr:          nil,
		},
		"transforms GitHub (v2) source with existing connection": {
			mfSource: &manifest.Source{
				ProviderName: manifest.GithubProviderName,
				Properties: map[string]interface{}{
					"branch":         "test",
					"repository":     "some/repository/URL",
					"connection_arn": "barnARN",
				},
			},
			expectedDeploySource: &GitHubSource{
				ProviderName:  manifest.GithubProviderName,
				Branch:        "test",
				RepositoryURL: "some/repository/URL",
				ConnectionARN: "barnARN",
			},
			expectedShouldPrompt: false,
			expectedErr:          nil,
		},
		"transforms Bitbucket source without existing connection": {
			mfSource: &manifest.Source{
				ProviderName: manifest.BitbucketProviderName,
				Properties: map[string]interface{}{
					"branch":     "test",
					"repository": "some/repository/URL",
				},
			},
			expectedDeploySource: &BitbucketSource{
				ProviderName:  manifest.BitbucketProviderName,
				Branch:        "test",
				RepositoryURL: "some/repository/URL",
			},
			expectedShouldPrompt: true,
			expectedErr:          nil,
		},
		"transforms Bitbucket source with existing connection": {
			mfSource: &manifest.Source{
				ProviderName: manifest.BitbucketProviderName,
				Properties: map[string]interface{}{
					"branch":         "test",
					"repository":     "some/repository/URL",
					"connection_arn": "yarnARN",
				},
			},
			expectedDeploySource: &BitbucketSource{
				ProviderName:  manifest.BitbucketProviderName,
				Branch:        "test",
				RepositoryURL: "some/repository/URL",
				ConnectionARN: "yarnARN",
			},
			expectedShouldPrompt: false,
			expectedErr:          nil,
		},
		"transforms CodeCommit source": {
			mfSource: &manifest.Source{
				ProviderName: manifest.CodeCommitProviderName,
				Properties: map[string]interface{}{
					"branch":                 "test",
					"repository":             "some/repository/URL",
					"output_artifact_format": "testFormat",
				},
			},
			expectedDeploySource: &CodeCommitSource{
				ProviderName:         manifest.CodeCommitProviderName,
				Branch:               "test",
				RepositoryURL:        "some/repository/URL",
				OutputArtifactFormat: "testFormat",
			},
			expectedShouldPrompt: false,
			expectedErr:          nil,
		},
		"use default branch `main` if branch is not configured": {
			mfSource: &manifest.Source{
				ProviderName: manifest.CodeCommitProviderName,
				Properties: map[string]interface{}{
					"repository": "some/repository/URL",
				},
			},
			expectedDeploySource: &CodeCommitSource{
				ProviderName:  manifest.CodeCommitProviderName,
				Branch:        "main",
				RepositoryURL: "some/repository/URL",
			},
			expectedShouldPrompt: false,
			expectedErr:          nil,
		},
		"error out if repository is not configured": {
			mfSource: &manifest.Source{
				ProviderName: manifest.CodeCommitProviderName,
				Properties:   map[string]interface{}{},
			},
			expectedShouldPrompt: false,
			expectedErr:          errors.New("missing `repository` in properties"),
		},
		"errors if user changed provider name in manifest to unsupported source": {
			mfSource: &manifest.Source{
				ProviderName: "BitCommitHubBucket",
				Properties: map[string]interface{}{
					"branch":     "test",
					"repository": "some/repository/URL",
				},
			},
			expectedDeploySource: nil,
			expectedShouldPrompt: false,
			expectedErr:          errors.New("invalid repo source provider: BitCommitHubBucket"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			source, shouldPrompt, err := PipelineSourceFromManifest(tc.mfSource)
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedDeploySource, source, "mismatched source")
				require.Equal(t, tc.expectedShouldPrompt, shouldPrompt, "mismatched bool for prompting")
			}
		})
	}
}

func TestPipelineBuild_Init(t *testing.T) {
	const (
		defaultImage   = "aws/codebuild/amazonlinux2-x86_64-standard:4.0"
		defaultEnvType = "LINUX_CONTAINER"
	)
	yamlNode := yaml.Node{}
	policyDocument := []byte(`
  Statement:
    Action: '*'
    Effect: Allow
    Resource: '*'
  Version: 2012-10-17`)

	require.NoError(t, yaml.Unmarshal(policyDocument, &yamlNode))

	testCases := map[string]struct {
		mfBuild       *manifest.Build
		mfDirPath     string
		expectedBuild Build
	}{
		"set default image and env type if not specified in manifest; override default if buildspec path in manifest": {
			mfBuild: &manifest.Build{
				Buildspec: "some/path",
			},
			expectedBuild: Build{
				Image:           defaultImage,
				EnvironmentType: defaultEnvType,
				BuildspecPath:   "some/path",
			},
		},
		"set image according to manifest": {
			mfBuild: &manifest.Build{
				Image:     "aws/codebuild/standard:3.0",
				Buildspec: "some/path",
			},
			expectedBuild: Build{
				Image:           "aws/codebuild/standard:3.0",
				EnvironmentType: defaultEnvType,
				BuildspecPath:   "some/path",
			},
		},
		"set image according to manifest (ARM based)": {
			mfBuild: &manifest.Build{
				Image:     "aws/codebuild/amazonlinux2-aarch64-standard:2.0",
				Buildspec: "some/path",
			},
			expectedBuild: Build{
				Image:           "aws/codebuild/amazonlinux2-aarch64-standard:2.0",
				EnvironmentType: "ARM_CONTAINER",
				BuildspecPath:   "some/path",
			},
		},
		"additional policy is not empty": {
			mfBuild: &manifest.Build{
				Image:     "aws/codebuild/amazonlinux2-aarch64-standard:2.0",
				Buildspec: "some/path",
				AdditionalPolicy: struct {
					Document yaml.Node `yaml:"PolicyDocument,omitempty"`
				}{
					Document: yamlNode,
				},
			},
			expectedBuild: Build{
				Image:                    "aws/codebuild/amazonlinux2-aarch64-standard:2.0",
				EnvironmentType:          "ARM_CONTAINER",
				BuildspecPath:            "some/path",
				AdditionalPolicyDocument: "Statement:\n    Action: '*'\n    Effect: Allow\n    Resource: '*'\nVersion: 2012-10-17",
			},
		},
		"additional policy is empty": {
			mfBuild: &manifest.Build{
				Image:     "aws/codebuild/amazonlinux2-aarch64-standard:2.0",
				Buildspec: "some/path",
			},
			expectedBuild: Build{
				Image:                    "aws/codebuild/amazonlinux2-aarch64-standard:2.0",
				EnvironmentType:          "ARM_CONTAINER",
				BuildspecPath:            "some/path",
				AdditionalPolicyDocument: "",
			},
		},
		"by default convert legacy manifest path to buildspec path": {
			mfDirPath: "copilot/",
			expectedBuild: Build{
				Image:           defaultImage,
				EnvironmentType: defaultEnvType,
				BuildspecPath:   "copilot/buildspec.yml",
			},
		},
		"by default convert non-legacy manifest path to buildspec path": {
			mfDirPath: "copilot/pipelines/my-pipeline/",
			expectedBuild: Build{
				Image:           defaultImage,
				EnvironmentType: defaultEnvType,
				BuildspecPath:   "copilot/pipelines/my-pipeline/buildspec.yml",
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var build Build
			build.Init(tc.mfBuild, tc.mfDirPath)
			require.Equal(t, tc.expectedBuild, build, "mismatched build")
		})
	}
}

func TestParseOwnerAndRepo(t *testing.T) {
	testCases := map[string]struct {
		src            *GitHubSource
		expectedErrMsg *string
		expectedOwner  string
		expectedRepo   string
	}{
		"missing repository property": {
			src: &GitHubSource{
				RepositoryURL: "",
				Branch:        "main",
			},
			expectedErrMsg: aws.String("unable to locate the repository"),
		},
		"valid GH repository property": {
			src: &GitHubSource{
				RepositoryURL: "chicken/wings",
			},
			expectedErrMsg: nil,
			expectedOwner:  "chicken",
			expectedRepo:   "wings",
		},
		"valid full GH repository name": {
			src: &GitHubSource{
				RepositoryURL: "https://github.com/badgoose/chaOS",
			},
			expectedErrMsg: nil,
			expectedOwner:  "badgoose",
			expectedRepo:   "chaOS",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			owner, repo, err := tc.src.RepositoryURL.parse()
			if tc.expectedErrMsg != nil {
				require.Contains(t, err.Error(), *tc.expectedErrMsg)
			} else {
				require.NoError(t, err, "expected error")
				require.Equal(t, tc.expectedOwner, owner, "mismatched owner")
				require.Equal(t, tc.expectedRepo, repo, "mismatched repo")
			}
		})
	}
}

func TestPipelineStage_Init(t *testing.T) {
	var stg PipelineStage
	stg.Init(&config.Environment{
		Name:             "test",
		App:              "badgoose",
		Region:           "us-west-2",
		AccountID:        "123456789012",
		ManagerRoleARN:   "arn:aws:iam::123456789012:role/badgoose-test-EnvManagerRole",
		ExecutionRoleARN: "arn:aws:iam::123456789012:role/badgoose-test-CFNExecutionRole",
	}, &manifest.PipelineStage{
		Name:             "test",
		RequiresApproval: true,
		TestCommands:     []string{"make test", "echo \"made test\""},
	}, []string{"frontend", "backend"})

	t.Run("stage name matches the environment's name", func(t *testing.T) {
		require.Equal(t, "test", stg.Name())
	})
	t.Run("stage full name matches the pipeline stage's name", func(t *testing.T) {
		require.Equal(t, "DeployTo-test", stg.FullName())
	})
	t.Run("stage region matches the environment's region", func(t *testing.T) {
		require.Equal(t, "us-west-2", stg.Region())
	})
	t.Run("stage env manager role ARN matches the environment's config", func(t *testing.T) {
		require.Equal(t, "arn:aws:iam::123456789012:role/badgoose-test-EnvManagerRole", stg.EnvManagerRoleARN())
	})
	t.Run("stage exec role ARN matches the environment's config", func(t *testing.T) {
		require.Equal(t, "arn:aws:iam::123456789012:role/badgoose-test-CFNExecutionRole", stg.ExecRoleARN())
	})
	t.Run("number of expected deployments match", func(t *testing.T) {
		deployments, err := stg.Deployments()
		require.NoError(t, err)
		require.Equal(t, 2, len(deployments))
	})
	t.Run("manual approval button", func(t *testing.T) {
		require.NotNil(t, stg.Approval(), "should require approval action for stages when the manifest requires it")

		stg := PipelineStage{}
		require.Nil(t, stg.Approval(), "should return nil by default")
	})
}

func TestPipelineStage_PreDeployments(t *testing.T) {
	testCases := map[string]struct {
		stg *PipelineStage

		wantedRunOrder      map[string]int
		wantedTemplateOrder []string
		wantedErr           error
	}{
		"should come after manual approval": {
			stg: func() *PipelineStage {
				// Create a pre-deployment with parallel actions after approval.
				var stg PipelineStage
				stg.Init(&config.Environment{Name: "test"}, &manifest.PipelineStage{
					Name: "test",
					PreDeployments: map[string]*manifest.PrePostDeployment{
						"ipa": {},
						"api": {},
					},
					RequiresApproval: true,
				}, nil)
				return &stg
			}(),
			wantedRunOrder: map[string]int{
				"api": 2,
				"ipa": 2,
			},
		},
		"actions should be ordered as indicated by depends_on": {
			stg: func() *PipelineStage {
				// Create a pre-deployment with ordered actions.
				var stg PipelineStage
				stg.Init(&config.Environment{Name: "test"}, &manifest.PipelineStage{
					Name: "test",
					PreDeployments: map[string]*manifest.PrePostDeployment{
						"a": {},
						"b": {
							DependsOn: []string{"a"},
						},
						"c": {
							DependsOn: []string{"a"},
						},
						"d": {
							DependsOn: []string{"a", "b"},
						},
					},
				}, nil)
				return &stg
			}(),
			wantedRunOrder: map[string]int{
				"a": 1,
				"b": 2,
				"c": 2,
				"d": 3,
			},
			wantedTemplateOrder: []string{"a", "b", "c", "d"},
		},
		"actions should be alphabetized for integ tests": {
			stg: func() *PipelineStage {
				// Create a pre-deployment with all actions deployed in parallel.
				var stg PipelineStage
				stg.Init(&config.Environment{Name: "test"}, &manifest.PipelineStage{
					Name: "test",
					PreDeployments: map[string]*manifest.PrePostDeployment{
						"ipa": {},
						"api": {},
					},
				}, nil)
				return &stg
			}(),
			wantedRunOrder: map[string]int{
				"api": 1,
				"ipa": 1,
			},
			wantedTemplateOrder: []string{"api", "ipa"},
		},
		"should error if cyclical depends_on actions": {
			stg: func() *PipelineStage {
				// Create a pre-deployment with mutually-dependent actions.
				var stg PipelineStage
				stg.Init(&config.Environment{Name: "test"}, &manifest.PipelineStage{
					Name: "test",
					PreDeployments: map[string]*manifest.PrePostDeployment{
						"api": {
							DependsOn: []string{"api"},
						},
					},
				}, nil)
				return &stg
			}(),
			wantedErr: errors.New("find an ordering for deployments: graph contains a cycle: api"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			preDeployments, err := tc.stg.PreDeployments()

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				for _, preDeployment := range preDeployments {
					wanted, ok := tc.wantedRunOrder[preDeployment.Name()]
					require.True(t, ok, "expected pre-deployment action named %s to be created", preDeployment.Name())
					require.Equal(t, wanted, preDeployment.RunOrder(), "order for predeployment action %s does not match", preDeployment.Name())
				}
				for i, wanted := range tc.wantedTemplateOrder {
					require.Equal(t, wanted, preDeployments[i].Name(), "predeployment name at index %d does not match", i)
				}
			}
		})
	}
}

func TestPipelineStage_Deployments(t *testing.T) {
	testCases := map[string]struct {
		stg *PipelineStage

		wantedRunOrder      map[string]int
		wantedTemplateOrder []string
		wantedErr           error
	}{
		"should return an error when the deployments contain a cycle": {
			stg: func() *PipelineStage {
				// Create a pipeline with a self-depending deployment.
				var stg PipelineStage
				stg.Init(&config.Environment{Name: "test"}, &manifest.PipelineStage{
					Name: "test",
					Deployments: map[string]*manifest.Deployment{
						"api": {
							DependsOn: []string{"api"},
						},
					},
				}, nil)

				return &stg
			}(),
			wantedErr: errors.New("find an ordering for deployments: graph contains a cycle: api"),
		},
		"should return the expected run orders": {
			stg: func() *PipelineStage {
				// Create a pipeline with a manual approval and 4 deployments.
				var stg PipelineStage
				stg.Init(&config.Environment{Name: "test"}, &manifest.PipelineStage{
					Name:             "test",
					RequiresApproval: true,
					Deployments: map[string]*manifest.Deployment{
						"frontend": {
							DependsOn: []string{"orders", "payments"},
						},
						"orders": {
							DependsOn: []string{"warehouse"},
						},
						"payments":  nil,
						"warehouse": nil,
					},
				}, nil)
				return &stg
			}(),
			wantedRunOrder: map[string]int{
				"CreateOrUpdate-frontend-test":  4,
				"CreateOrUpdate-orders-test":    3,
				"CreateOrUpdate-payments-test":  2,
				"CreateOrUpdate-warehouse-test": 2,
			},
		},
		"deployments should be alphabetically sorted so that integration tests are deterministic": {
			stg: func() *PipelineStage {
				// Create a pipeline with all local workloads deployed in parallel.
				var stg PipelineStage
				stg.Init(&config.Environment{Name: "test"}, &manifest.PipelineStage{
					Name: "test",
				}, []string{"b", "a", "d", "c"})

				return &stg
			}(),
			wantedRunOrder: map[string]int{
				"CreateOrUpdate-a-test": 1,
				"CreateOrUpdate-b-test": 1,
				"CreateOrUpdate-c-test": 1,
				"CreateOrUpdate-d-test": 1,
			},
			wantedTemplateOrder: []string{"CreateOrUpdate-a-test", "CreateOrUpdate-b-test", "CreateOrUpdate-c-test", "CreateOrUpdate-d-test"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			deployments, err := tc.stg.Deployments()

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				for _, deployment := range deployments {
					wanted, ok := tc.wantedRunOrder[deployment.Name()]
					require.True(t, ok, "expected deployment named %s to be created", deployment.Name())
					require.Equal(t, wanted, deployment.RunOrder(), "order for deployment %s does not match", deployment.Name())
				}
				for i, wanted := range tc.wantedTemplateOrder {
					require.Equal(t, wanted, deployments[i].Name(), "deployment name at index %d do not match", i)
				}
			}
		})
	}
}

func TestPipelineStage_PostDeployments(t *testing.T) {
	testCases := map[string]struct {
		stg *PipelineStage

		wantedRunOrder      map[string]int
		wantedTemplateOrder []string
		wantedErr           error
	}{
		"should come after manual approval, pre-deployments, deployments; alpha in template": {
			stg: func() *PipelineStage {
				var stg PipelineStage
				stg.Init(&config.Environment{Name: "test"}, &manifest.PipelineStage{
					Name:             "test",
					RequiresApproval: true,
					PreDeployments: map[string]*manifest.PrePostDeployment{
						"ipa": {},
						"api": {},
					},
					Deployments: map[string]*manifest.Deployment{
						"frontend":  nil,
						"orders":    nil,
						"payments":  nil,
						"warehouse": nil,
					},
					PostDeployments: map[string]*manifest.PrePostDeployment{
						"post": {},
						"it":   {},
					},
				}, nil)
				return &stg
			}(),
			wantedRunOrder: map[string]int{
				"post": 4,
				"it":   4,
			},
			wantedTemplateOrder: []string{"it", "post"},
		},
		"should error if cyclical depends_on actions": {
			stg: func() *PipelineStage {
				// Create a post-deployment with mutually-dependent actions.
				var stg PipelineStage
				stg.Init(&config.Environment{Name: "test"}, &manifest.PipelineStage{
					Name: "test",
					PostDeployments: map[string]*manifest.PrePostDeployment{
						"post": {
							DependsOn: []string{"post"},
						},
					},
				}, nil)
				return &stg
			}(),
			wantedErr: errors.New("find an ordering for deployments: graph contains a cycle: post"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			postDeployments, err := tc.stg.PostDeployments()

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				for _, postDeployment := range postDeployments {
					wanted, ok := tc.wantedRunOrder[postDeployment.Name()]
					require.True(t, ok, "expected deployment named %s to be created", postDeployment.Name())
					require.Equal(t, wanted, postDeployment.RunOrder(), "order for deployment %s does not match", postDeployment.Name())
				}
				for i, wanted := range tc.wantedTemplateOrder {
					require.Equal(t, wanted, postDeployments[i].Name(), "deployment name at index %d do not match", i)
				}
			}
		})
	}
}

type mockAction struct {
	order int
}

func (ma mockAction) RunOrder() int {
	return ma.order
}

func TestAction_RunOrder(t *testing.T) {
	testCases := map[string]struct {
		previous []orderedRunner
		wanted   int
	}{
		"should return 1 when there are no previous actions": {
			wanted: 1,
		},
		"should return the max of previous actions + 1": {
			previous: []orderedRunner{
				mockAction{order: 8},
				mockAction{order: 7},
				mockAction{order: 9},
				mockAction{order: 8},
			},
			wanted: 10,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			action := action{
				prevActions: tc.previous,
			}

			// THEN
			require.Equal(t, tc.wanted, action.RunOrder())
		})
	}
}

func TestManualApprovalAction_Name(t *testing.T) {
	action := ManualApprovalAction{
		name: "test",
	}

	require.Equal(t, "ApprovePromotionTo-test", action.Name())
}

func TestDeployAction_Name(t *testing.T) {
	action := DeployAction{
		name:    "frontend",
		envName: "test",
	}

	require.Equal(t, "CreateOrUpdate-frontend-test", action.Name())
}

func TestDeployAction_StackName(t *testing.T) {
	testCases := map[string]struct {
		in     DeployAction
		wanted string
	}{
		"should default to app-env-name when user does not override the stack name": {
			in: DeployAction{
				name:    "frontend",
				envName: "test",
				appName: "phonetool",
			},
			wanted: "phonetool-test-frontend",
		},
		"should use custom user override stack name when present": {
			in: DeployAction{
				override: &manifest.Deployment{
					StackName: "other-stack",
				},
			},
			wanted: "other-stack",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, tc.in.StackName())
		})
	}
}

func TestDeployAction_TemplatePath(t *testing.T) {
	testCases := map[string]struct {
		in     DeployAction
		wanted string
	}{
		"default location for workload templates": {
			in: DeployAction{
				name:    "frontend",
				envName: "test",
			},
			wanted: "infrastructure/frontend-test.stack.yml",
		},
		"should use custom override template path when present": {
			in: DeployAction{
				override: &manifest.Deployment{
					TemplatePath: "infrastructure/custom.yml",
				},
			},
			wanted: "infrastructure/custom.yml",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, tc.in.TemplatePath())
		})
	}
}

func TestDeployAction_TemplateConfigPath(t *testing.T) {
	testCases := map[string]struct {
		in     DeployAction
		wanted string
	}{
		"default location for workload template configs": {
			in: DeployAction{
				name:    "frontend",
				envName: "test",
			},
			wanted: "infrastructure/frontend-test.params.json",
		},
		"should use custom override template config path when present": {
			in: DeployAction{
				override: &manifest.Deployment{
					TemplateConfig: "infrastructure/custom.params.json",
				},
			},
			wanted: "infrastructure/custom.params.json",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, tc.in.TemplateConfigPath())
		})
	}
}

type mockRanker struct {
	rank int
	ok   bool
}

func (m mockRanker) Rank(name string) (int, bool) {
	return m.rank, m.ok
}

func TestDeployAction_RunOrder(t *testing.T) {
	// GIVEN
	ranker := mockRanker{rank: 2}
	past := []orderedRunner{
		mockAction{
			order: 3,
		},
	}
	in := DeployAction{
		action: action{prevActions: past},
		ranker: ranker,
	}

	// THEN
	require.Equal(t, 6, in.RunOrder(), "should be past actions + 1 + rank")
}

func TestTestCommandsAction_Name(t *testing.T) {
	require.Equal(t, "TestCommands", (&TestCommandsAction{}).Name())
}

func TestParseRepo(t *testing.T) {
	testCases := map[string]struct {
		src           *CodeCommitSource
		expectedErr   error
		expectedOwner string
		expectedRepo  string
	}{
		"missing repository property": {
			src: &CodeCommitSource{
				RepositoryURL: "",
			},
			expectedErr: errors.New("unable to locate the repository"),
		},
		"unable to parse repository name from URL": {
			src: &CodeCommitSource{
				RepositoryURL: "https://hahahaha.wrong.URL/repositories/wings/browse",
			},
			expectedErr:   errors.New("unable to parse the repository from the URL https://hahahaha.wrong.URL/repositories/wings/browse"),
			expectedOwner: "",
			expectedRepo:  "wings",
		},
		"valid full CC repository name": {
			src: &CodeCommitSource{
				RepositoryURL: "https://us-west-2.console.aws.amazon.com/codesuite/codecommit/repositories/wings/browse",
			},
			expectedOwner: "",
			expectedRepo:  "wings",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			repo, err := tc.src.parseRepo()
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err, "expected error")
				require.Equal(t, tc.expectedRepo, repo, "mismatched repo")
			}
		})
	}
}
