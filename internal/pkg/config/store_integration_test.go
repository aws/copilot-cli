//go:build integration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/stretchr/testify/require"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func Test_SSM_Application_Integration(t *testing.T) {
	defaultSess, err := sessions.ImmutableProvider().Default()
	require.NoError(t, err)

	store := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))
	applicationToCreate := config.Application{Name: randStringBytes(10), Version: "1.0"}
	defer store.DeleteApplication(applicationToCreate.Name)

	t.Run("Create, Get and List Applications", func(t *testing.T) {
		// Create our first application
		err := store.CreateApplication(&applicationToCreate)
		require.NoError(t, err)

		// Can't overwrite an existing application
		err = store.CreateApplication(&applicationToCreate)
		require.NoError(t, err)

		// Fetch the application back from SSM
		application, err := store.GetApplication(applicationToCreate.Name)
		require.NoError(t, err)
		require.Equal(t, applicationToCreate, *application)

		// List returns a non-empty list of applications
		applications, err := store.ListApplications()
		require.NoError(t, err)
		require.NotEmpty(t, applications)
	})
}

func Test_SSM_Environment_Integration(t *testing.T) {
	defaultSess, err := sessions.ImmutableProvider().Default()
	require.NoError(t, err)

	store := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))
	applicationToCreate := config.Application{Name: randStringBytes(10), Version: "1.0"}
	testEnvironment := config.Environment{Name: "test", App: applicationToCreate.Name, Region: "us-west-2", AccountID: " 1234"}
	prodEnvironment := config.Environment{Name: "prod", App: applicationToCreate.Name, Region: "us-west-2", AccountID: " 1234"}

	defer func() {
		store.DeleteEnvironment(applicationToCreate.Name, testEnvironment.Name)
		store.DeleteEnvironment(applicationToCreate.Name, prodEnvironment.Name)
		store.DeleteApplication(applicationToCreate.Name)
	}()
	t.Run("Create, Get and List Environments", func(t *testing.T) {
		// Create our first application
		err := store.CreateApplication(&applicationToCreate)
		require.NoError(t, err)

		// Make sure there are no envs with our new application
		envs, err := store.ListEnvironments(applicationToCreate.Name)
		require.NoError(t, err)
		require.Empty(t, envs)

		// Add our environments
		err = store.CreateEnvironment(&testEnvironment)
		require.NoError(t, err)

		err = store.CreateEnvironment(&prodEnvironment)
		require.NoError(t, err)

		// Skip and do not return error if environment already exists
		err = store.CreateEnvironment(&prodEnvironment)
		require.NoError(t, err)

		// Wait for consistency to kick in (ssm path commands are eventually consistent)
		time.Sleep(5 * time.Second)

		// Make sure all the environments are under our application
		envs, err = store.ListEnvironments(applicationToCreate.Name)
		require.NoError(t, err)
		var environments []config.Environment
		for _, e := range envs {
			environments = append(environments, *e)
		}
		require.ElementsMatch(t, environments, []config.Environment{testEnvironment, prodEnvironment})

		// Fetch our saved environments, one by one
		env, err := store.GetEnvironment(applicationToCreate.Name, testEnvironment.Name)
		require.NoError(t, err)
		require.Equal(t, testEnvironment, *env)

		env, err = store.GetEnvironment(applicationToCreate.Name, prodEnvironment.Name)
		require.NoError(t, err)
		require.Equal(t, prodEnvironment, *env)
	})
}

func Test_SSM_Service_Integration(t *testing.T) {
	defaultSess, err := sessions.ImmutableProvider().Default()
	require.NoError(t, err)

	store := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))
	applicationToCreate := config.Application{Name: randStringBytes(10), Version: "1.0"}
	apiService := config.Workload{Name: "api", App: applicationToCreate.Name, Type: "Load Balanced Web Service"}
	feService := config.Workload{Name: "front-end", App: applicationToCreate.Name, Type: "Load Balanced Web Service"}

	defer func() {
		store.DeleteService(applicationToCreate.Name, apiService.Name)
		store.DeleteService(applicationToCreate.Name, feService.Name)
		store.DeleteApplication(applicationToCreate.Name)
	}()

	t.Run("Create, Get and List Applications", func(t *testing.T) {
		// Create our first application
		err := store.CreateApplication(&applicationToCreate)
		require.NoError(t, err)

		// Make sure there are no svcs with our new application
		svcs, err := store.ListServices(applicationToCreate.Name)
		require.NoError(t, err)
		require.Empty(t, svcs)

		// Add our services
		err = store.CreateService(&apiService)
		require.NoError(t, err)

		err = store.CreateService(&feService)
		require.NoError(t, err)

		// Skip and do not return error if services already exists
		err = store.CreateService(&feService)
		require.NoError(t, err)

		// Wait for consistency to kick in (ssm path commands are eventually consistent)
		time.Sleep(5 * time.Second)

		// Make sure all the svcs are under our application
		svcs, err = store.ListServices(applicationToCreate.Name)
		require.NoError(t, err)
		var services []config.Workload
		for _, s := range svcs {
			services = append(services, *s)
		}
		require.ElementsMatch(t, services, []config.Workload{apiService, feService})

		// Fetch our saved svcs, one by one
		svc, err := store.GetService(applicationToCreate.Name, apiService.Name)
		require.NoError(t, err)
		require.Equal(t, apiService, *svc)

		svc, err = store.GetService(applicationToCreate.Name, feService.Name)
		require.NoError(t, err)
		require.Equal(t, feService, *svc)
	})
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func randStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
