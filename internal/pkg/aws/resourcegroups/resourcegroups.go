// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package resourcegroups provides a client to make API requests to AWS Resource Groups
package resourcegroups

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/resourcegroups"
)

const (
	resourceQueryType = "TAG_FILTERS_1_0"
)

type api interface {
	SearchResources(input *resourcegroups.SearchResourcesInput) (*resourcegroups.SearchResourcesOutput, error)
}

// ResourceGroups wraps an AWS ResourceGroups client.
type ResourceGroups struct {
	client api
}

type tagFilter struct {
	Key    string
	Values []string
}

type query struct {
	ResourceTypeFilters []string
	TagFilters          []tagFilter
}

// New returns a ResourceGroup struct configured against the input session.
func New(s *session.Session) *ResourceGroups {
	return &ResourceGroups{
		client: resourcegroups.New(s),
	}
}

// GetResourcesByTags internally sets the type to TAG_FILTERS_1_0 and generates the query with input resource type and tags.
func (rg *ResourceGroups) GetResourcesByTags(resourceType string, tags map[string]string) ([]string, error) {
	var resourceArns []string
	resourceResp := &resourcegroups.SearchResourcesOutput{}

	query, err := rg.searchResourcesQuery(resourceType, tags)
	if err != nil {
		return nil, fmt.Errorf("construct search resource query: %w", err)
	}
	for {
		resourceResp, err = rg.client.SearchResources(&resourcegroups.SearchResourcesInput{
			NextToken: resourceResp.NextToken,
			ResourceQuery: &resourcegroups.ResourceQuery{
				Type:  aws.String(resourceQueryType),
				Query: aws.String(string(query)),
			},
		})
		if err != nil {
			return nil, fmt.Errorf("search resource group with resource type %s: %w", resourceType, err)
		}
		for _, identifier := range resourceResp.ResourceIdentifiers {
			arn := aws.StringValue(identifier.ResourceArn)
			resourceArns = append(resourceArns, arn)
		}
		if resourceResp.NextToken == nil {
			break
		}
	}

	return resourceArns, nil
}

// searchResourcesQuery returns a query string with the tag filters used to filter Resources
func (rg *ResourceGroups) searchResourcesQuery(resourceType string, tags map[string]string) (string, error) {
	var tagFilters []tagFilter
	for k, v := range tags {
		tagFilters = append(tagFilters, tagFilter{
			Key:    k,
			Values: []string{v},
		})
	}

	q := query{
		ResourceTypeFilters: []string{resourceType},
		TagFilters:          tagFilters,
	}
	bytes, err := json.Marshal(q)

	if err != nil {
		return "", err
	}

	return string(bytes), nil
}
