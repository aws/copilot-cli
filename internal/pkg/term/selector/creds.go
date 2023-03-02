// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package selector

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
)

const (
	tempCredsOption       = "Enter temporary credentials"
	accessKeyIDPrompt     = "What's your AWS Access Key ID?"
	secretAccessKeyPrompt = "What's your AWS Secret Access Key?"
	sessionTokenPrompt    = "What's your AWS Session Token?"
)

// Names wraps the method that returns a list of names.
type Names interface {
	Names() []string
}

// SessionProvider wraps the methods to create AWS sessions.
type SessionProvider interface {
	Default() (*session.Session, error)
	FromProfile(name string) (*session.Session, error)
	FromStaticCreds(accessKeyID, secretAccessKey, sessionToken string) (*session.Session, error)
}

// CredsSelect prompts users for credentials.
type CredsSelect struct {
	Prompt  Prompter
	Profile Names
	Session SessionProvider
}

// Creds prompts users to choose either use temporary credentials or choose from one of their existing AWS named profiles.
func (s *CredsSelect) Creds(msg, help string) (*session.Session, error) {
	profileFrom := make(map[string]string)
	options := []string{tempCredsOption}
	for _, name := range s.Profile.Names() {
		pretty := fmt.Sprintf("[profile %s]", name)
		options = append(options, pretty)
		profileFrom[pretty] = name
	}

	selected, err := s.Prompt.SelectOne(
		msg,
		help,
		options,
		prompt.WithFinalMessage("Credential source:"))
	if err != nil {
		return nil, fmt.Errorf("select credential source: %w", err)
	}

	if selected == tempCredsOption {
		return s.askTempCreds()
	}
	sess, err := s.Session.FromProfile(profileFrom[selected])
	if err != nil {
		return nil, fmt.Errorf("create session from profile %s: %w", profileFrom[selected], err)
	}
	return sess, nil
}

func (s *CredsSelect) askTempCreds() (*session.Session, error) {
	defaultAccessKey, defaultSecretAccessKey, defaultSessToken := defaultCreds(s.Session)

	accessKeyID, err := s.askWithMaskedDefault(accessKeyIDPrompt, defaultAccessKey, prompt.RequireNonEmpty, prompt.WithFinalMessage("AWS Access Key ID:"))
	if err != nil {
		return nil, fmt.Errorf("get access key id: %w", err)
	}
	secretAccessKey, err := s.askWithMaskedDefault(secretAccessKeyPrompt, defaultSecretAccessKey, prompt.RequireNonEmpty, prompt.WithFinalMessage("AWS Secret Access Key:"))
	if err != nil {
		return nil, fmt.Errorf("get secret access key: %w", err)
	}
	sessionToken, err := s.askWithMaskedDefault(sessionTokenPrompt, defaultSessToken, nil, prompt.WithFinalMessage("AWS Session Token:"))
	if err != nil {
		return nil, fmt.Errorf("get session token: %w", err)
	}

	sess, err := s.Session.FromStaticCreds(accessKeyID, secretAccessKey, sessionToken)
	if err != nil {
		return nil, fmt.Errorf("create session from temporary credentials: %w", err)
	}
	return sess, nil
}

func (s *CredsSelect) askWithMaskedDefault(msg, defaultValue string, f prompt.ValidatorFunc, opts ...prompt.PromptConfig) (string, error) {
	accessKeyId, err := s.Prompt.Get(msg, "", f, append([]prompt.PromptConfig{prompt.WithDefaultInput(mask(defaultValue))}, opts...)...)
	if err != nil {
		return "", err
	}
	if accessKeyId == mask(defaultValue) {
		// Return the original default instead of the masked value.
		return defaultValue, nil
	}
	return accessKeyId, nil
}

// defaultCreds returns the credential values from the default session.
// If an error occurs, returns empty strings.
func defaultCreds(session SessionProvider) (accessKeyID, secretAccessKey, sessionToken string) {
	// If we cannot retrieve default creds, return empty credentials as default instead of an error.
	defaultSess, err := session.Default()
	if err != nil {
		return
	}
	v, err := sessions.Creds(defaultSess)
	if err != nil {
		return
	}
	return v.AccessKeyID, v.SecretAccessKey, v.SessionToken
}

// mask hides the value of s with "*"s except the last 4 characters.
// Taken from the AWS CLI, see:
// https://github.com/aws/aws-cli/blob/4ff0cbacbac69a21d4dd701921fe0759cf7852ed/awscli/customizations/configure/__init__.py#L38-L42
// TODO(efekarakus): Move the masking logic to be part of the prompt package.
func mask(s string) string {
	if s == "" {
		return ""
	}

	hint := s
	if len(hint) >= 4 {
		hint = hint[len(hint)-4:]
	}
	return fmt.Sprintf("%s%s", strings.Repeat("*", 16), hint)
}
