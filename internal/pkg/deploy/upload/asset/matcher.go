// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package asset

import (
	"fmt"
	"path/filepath"
)

type filepathMatcher interface {
	match(path string) (bool, error)
}

type reincludeMatcher string

func buildReincludeMatchers(reincludes []string) []filepathMatcher {
	var matchers []filepathMatcher
	for _, reinclude := range reincludes {
		matchers = append(matchers, reincludeMatcher(reinclude))
	}
	return matchers
}

func (m reincludeMatcher) match(path string) (bool, error) {
	return match(string(m), path)
}

type excludeMatcher string

func buildExcludeMatchers(excludes []string) []filepathMatcher {
	var matchers []filepathMatcher
	for _, exclude := range excludes {
		matchers = append(matchers, excludeMatcher(exclude))
	}
	return matchers
}

func (m excludeMatcher) match(path string) (bool, error) {
	return match(string(m), path)
}

// compositeMatcher is a composite matcher consisting of reinclude matchers and exclude matchers.
// Note that exclude matchers will be applied before reinclude matchers.
type compositeMatcher struct {
	reincludeMatchers []filepathMatcher
	excludeMatchers   []filepathMatcher
}

func buildCompositeMatchers(reincludeMatchers, excludeMatchers []filepathMatcher) compositeMatcher {
	return compositeMatcher{
		reincludeMatchers: reincludeMatchers,
		excludeMatchers:   excludeMatchers,
	}
}

func (m compositeMatcher) match(path string) (bool, error) {
	shouldInclude := true
	for _, matcher := range m.excludeMatchers {
		isMatch, err := matcher.match(path)
		if err != nil {
			return false, err
		}
		if isMatch {
			shouldInclude = false
		}
	}
	for _, matcher := range m.reincludeMatchers {
		isMatch, err := matcher.match(path)
		if err != nil {
			return false, err
		}
		if isMatch {
			shouldInclude = true
		}
	}
	return shouldInclude, nil
}

func match(pattern, path string) (bool, error) {
	isMatch, err := filepath.Match(pattern, path)
	if err != nil {
		return false, fmt.Errorf("match file path %s against pattern %s: %w", path, pattern, err)
	}
	return isMatch, nil
}
