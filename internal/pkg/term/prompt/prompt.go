// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package prompt provides functionality to retrieve free-form text, selection,
// and confirmation input from the user via a terminal.
package prompt

import (
	"errors"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/core"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

func init() {
	survey.ConfirmQuestionTemplate = `{{if not .Answer}}
{{end}}
{{- if .ShowHelp }}{{- color .Config.Icons.Help.Format }}{{ .Config.Icons.Help.Text }}{{$lines := split .Help "\n"}}{{range $i, $line := $lines}}
{{- if eq $i 0}}  {{ $line }}
{{ else }}  {{ $line }}
{{ end }}{{- end }}{{color "reset"}}{{end}}
{{- color .Config.Icons.Question.Format }}{{if not .Answer}}  {{ .Config.Icons.Question.Text }}{{else}}{{ .Config.Icons.Question.Text }}{{end}}{{color "reset"}}
{{- color "default"}}{{ .Message }} {{color "reset"}}
{{- if .Answer}}
  {{- color "default"}}{{.Answer}}{{color "reset"}}{{"\n"}}
{{- else }}
  {{- if and .Help (not .ShowHelp)}}{{color "white"}}[{{ .Config.HelpInput }} for help]{{color "reset"}} {{end}}
  {{- color "default"}}{{if .Default}}(Y/n) {{else}}(y/N) {{end}}{{color "reset"}}
{{- end}}`

	survey.SelectQuestionTemplate = `{{if not .Answer}}
{{end}}
{{- if .ShowHelp }}{{- color .Config.Icons.Help.Format }}{{ .Config.Icons.Help.Text }}{{$lines := split .Help "\n"}}{{range $i, $line := $lines}}
{{- if eq $i 0}}  {{ $line }}
{{ else }}  {{ $line }}
{{ end }}{{- end }}{{color "reset"}}{{end}}
{{- color .Config.Icons.Question.Format }}{{if not .ShowAnswer}}  {{ .Config.Icons.Question.Text }}{{else}}{{ .Config.Icons.Question.Text }}{{end}}{{color "reset"}}
{{- color "default"}}{{ .Message }}{{ .FilterMessage }}{{color "reset"}}
{{- if .ShowAnswer}}{{color "default"}} {{parseAnswer .Answer}}{{color "reset"}}{{"\n"}}
{{- else}}
  {{- "  "}}{{- color "white"}}[Use arrows to move, type to filter{{- if and .Help (not .ShowHelp)}}, {{ .Config.HelpInput }} for more help{{end}}]{{color "reset"}}
  {{- "\n"}}
  {{- range $ix, $choice := .PageEntries}}
    {{- if eq $ix $.SelectedIndex }}{{color "default+b" }}  {{ $.Config.Icons.SelectFocus.Text }} {{else}}{{color "default"}}    {{end}}
    {{- $choice.Value}}
    {{- color "reset"}}{{"\n"}}
  {{- end}}
{{- end}}`

	survey.InputQuestionTemplate = `{{if not .ShowAnswer}}
{{end}}
{{- if .ShowHelp }}{{- color .Config.Icons.Help.Format }}{{ .Config.Icons.Help.Text }}{{$lines := split .Help "\n"}}{{range $i, $line := $lines}}
{{- if eq $i 0}}  {{ $line }}
{{ else }}  {{ $line }}
{{ end }}{{- end }}{{color "reset"}}{{end}}
{{- color .Config.Icons.Question.Format }}{{if not .ShowAnswer}}  {{ .Config.Icons.Question.Text }}{{else}}{{ .Config.Icons.Question.Text }}{{end}}{{color "reset"}}
{{- color "default"}}{{ .Message }} {{color "reset"}}
{{- if .ShowAnswer}}
  {{- color "default"}}{{.Answer}}{{color "reset"}}{{"\n"}}
{{- else }}
  {{- if and .Help (not .ShowHelp)}}{{color "white"}}[{{ print .Config.HelpInput }} for help]{{color "reset"}} {{end}}
  {{- if .Default}}{{color "default"}}({{.Default}}) {{color "reset"}}{{end}}
  {{- .Answer -}}
{{- end}}`

	survey.PasswordQuestionTemplate = `
{{- if .ShowHelp }}{{- color .Config.Icons.Help.Format }}{{ .Config.Icons.Help.Text }}{{$lines := split .Help "\n"}}{{range $i, $line := $lines}}
{{- if eq $i 0}}  {{ $line }}
{{ else }}  {{ $line }}
{{ end }}{{- end }}{{color "reset"}}{{end}}
{{- color .Config.Icons.Question.Format }}  {{ .Config.Icons.Question.Text }}{{color "reset"}}
{{- color "default"}}{{ .Message }} {{color "reset"}}
{{- if and .Help (not .ShowHelp)}}{{color "white"}}[{{ .Config.HelpInput }} for help]{{color "reset"}} {{end}}`

	survey.MultiSelectQuestionTemplate = `{{if not .Answer}}
{{end}}
{{- if .ShowHelp }}{{- color .Config.Icons.Help.Format }}{{ .Config.Icons.Help.Text }}{{$lines := split .Help "\n"}}{{range $i, $line := $lines}}
{{- if eq $i 0}}  {{ $line }}
{{ else }}  {{ $line }}
{{ end }}{{- end }}{{color "reset"}}{{end}}
{{- color .Config.Icons.Question.Format }}{{if not .ShowAnswer}}  {{ .Config.Icons.Question.Text }}{{else}}{{ .Config.Icons.Question.Text }}{{end}}{{color "reset"}}
{{- color "default"}}{{ .Message }}{{ .FilterMessage }}{{color "reset"}}
{{- if .ShowAnswer}}{{color "default"}} {{parseAnswers .Answer}}{{color "reset"}}{{"\n"}}
{{- else }}
	{{- "  "}}{{- color "white"}}[Use arrows to move, space to select, type to filter{{- if and .Help (not .ShowHelp)}}, {{ .Config.HelpInput }} for more help{{end}}]{{color "reset"}}
  {{- "\n"}}
  {{- range $ix, $option := .PageEntries}}
    {{- if eq $ix $.SelectedIndex }}{{color "default+b" }}  {{ $.Config.Icons.SelectFocus.Text }}{{color "reset"}}{{else}} {{end}}
    {{- if index $.Checked $option.Index }}{{color "default+b" }} {{ $.Config.Icons.MarkedOption.Text }} {{else}}{{color "default" }} {{ $.Config.Icons.UnmarkedOption.Text }} {{end}}
    {{- color "reset"}}
    {{- " "}}{{$option.Value}}{{"\n"}}
  {{- end}}
{{- end}}`

	split := func(s string, sep string) []string {
		return strings.Split(s, sep)
	}
	core.TemplateFuncsWithColor["split"] = split
	core.TemplateFuncsWithColor["parseAnswer"] = parseValueFromOptionFmt
	core.TemplateFuncsWithColor["parseAnswers"] = parseValuesFromOptions
	core.TemplateFuncsNoColor["split"] = split
	core.TemplateFuncsNoColor["parseAnswer"] = parseValueFromOptionFmt
	core.TemplateFuncsNoColor["parseAnswers"] = parseValuesFromOptions
}

// ErrEmptyOptions indicates the input options list was empty.
var ErrEmptyOptions = errors.New("list of provided options is empty")

// Prompt abstracts the survey.Askone function.
type Prompt func(survey.Prompt, interface{}, ...survey.AskOpt) error

// ValidatorFunc defines the function signature for validating inputs.
type ValidatorFunc func(interface{}) error

// New returns a Prompt with default configuration.
func New() Prompt {
	return survey.AskOne
}

type prompter interface {
	Prompt(config *survey.PromptConfig) (interface{}, error)
	Cleanup(*survey.PromptConfig, interface{}) error
	Error(*survey.PromptConfig, error) error
	WithStdio(terminal.Stdio)
}

type prompt struct {
	prompter
	FinalMessage string // Text to display after the user selects an answer.
}

// Cleanup does a final render with the user's chosen value.
// This method overrides survey.Select's Cleanup method by assigning the prompt's message to be the final message.
func (p *prompt) Cleanup(config *survey.PromptConfig, val interface{}) error {
	if p.FinalMessage == "" {
		return p.prompter.Cleanup(config, val) // Delegate to the parent Cleanup.
	}
	// Update the message of the underlying struct.
	switch typedPrompt := p.prompter.(type) {
	case *survey.Select:
		typedPrompt.Message = p.FinalMessage
	case *survey.Input:
		typedPrompt.Message = p.FinalMessage
	case *passwordPrompt:
		typedPrompt.Message = p.FinalMessage
	case *survey.Confirm:
		typedPrompt.Message = p.FinalMessage
	case *survey.MultiSelect:
		typedPrompt.Message = p.FinalMessage
	}
	return p.prompter.Cleanup(config, val)
}

// Get prompts the user for free-form text input.
func (p Prompt) Get(message, help string, validator ValidatorFunc, promptOpts ...PromptConfig) (string, error) {
	input := &survey.Input{
		Message: message,
	}
	if help != "" {
		input.Help = color.Help(help)
	}

	prompt := &prompt{
		prompter: input,
	}
	for _, opt := range promptOpts {
		opt(prompt)
	}

	var result string
	var err error
	if validator == nil {
		err = p(prompt, &result, stdio(), icons())
	} else {
		err = p(prompt, &result, stdio(), validators(validator), icons())
	}
	return result, err
}

type passwordPrompt struct {
	*survey.Password
}

// Cleanup renders a new template that's left-shifted when the user answers the prompt.
func (pp *passwordPrompt) Cleanup(config *survey.PromptConfig, val interface{}) error {
	// The user already entered their password, move the cursor one level up to override the prompt.
	pp.Password.NewCursor().PreviousLine(1)

	// survey.Password unlike other survey structs doesn't have an "Answer" field. Therefore, we can't use a single
	// template like other prompts. Instead, when Cleanup is called, we render a new template
	// that behaves as if the question is answered.
	return pp.Password.Render(`
{{- color .Config.Icons.Question.Format }}{{ .Config.Icons.Question.Text }}{{color "reset"}}
{{- color "default"}}{{ .Message }} {{color "reset"}}
`,
		survey.PasswordTemplateData{
			Password: *pp.Password,
			Config:   config,
			ShowHelp: false,
		})
}

// GetSecret prompts the user for sensitive input. Wraps survey.Password
func (p Prompt) GetSecret(message, help string, promptOpts ...PromptConfig) (string, error) {
	passwd := &passwordPrompt{
		Password: &survey.Password{
			Message: message,
		},
	}
	if help != "" {
		passwd.Help = color.Help(help)
	}

	prompt := &prompt{
		prompter: passwd,
	}
	for _, opt := range promptOpts {
		opt(prompt)
	}

	var result string
	err := p(prompt, &result, stdio(), icons())
	return result, err
}

// Confirm prompts the user with a yes/no option.
func (p Prompt) Confirm(message, help string, promptCfgs ...PromptConfig) (bool, error) {
	confirm := &survey.Confirm{
		Message: message,
	}
	if help != "" {
		confirm.Help = color.Help(help)
	}

	prompt := &prompt{
		prompter: confirm,
	}
	for _, cfg := range promptCfgs {
		cfg(prompt)
	}

	var result bool
	err := p(prompt, &result, stdio(), icons())
	return result, err
}

// PromptConfig is a functional option to configure the prompt.
type PromptConfig func(*prompt)

// WithDefaultInput sets a default message for an input prompt.
func WithDefaultInput(s string) PromptConfig {
	return func(p *prompt) {
		if get, ok := p.prompter.(*survey.Input); ok {
			get.Default = s
		}
	}
}

// WithFinalMessage sets a final message that replaces the question prompt once the user enters an answer.
func WithFinalMessage(msg string) PromptConfig {
	return func(p *prompt) {
		p.FinalMessage = color.Emphasize(msg)
	}
}

// WithConfirmFinalMessage sets a short final message for to confirm the user's input.
func WithConfirmFinalMessage() PromptConfig {
	return func(p *prompt) {
		p.FinalMessage = color.Emphasize("Sure?")
	}
}

// WithDefaultSelections selects the options to be checked by default for a multiselect prompt.
func WithDefaultSelections(options []string) PromptConfig {
	return func(p *prompt) {
		if confirm, ok := p.prompter.(*survey.MultiSelect); ok {
			confirm.Default = options
		}
	}
}

// WithTrueDefault sets the default for a confirm prompt to true.
func WithTrueDefault() PromptConfig {
	return func(p *prompt) {
		if confirm, ok := p.prompter.(*survey.Confirm); ok {
			confirm.Default = true
		}
	}
}

func stdio() survey.AskOpt {
	return survey.WithStdio(os.Stdin, os.Stderr, os.Stderr)
}

func icons() survey.AskOpt {
	return survey.WithIcons(func(icons *survey.IconSet) {
		// The question mark "?" icon to denote a prompt will be colored in bold.
		icons.Question.Text = ""
		icons.Question.Format = "default+b"

		// Survey uses https://github.com/mgutz/ansi to set colors which unfortunately doesn't support the "Faint" style.
		// We are setting the help text to be fainted in the individual prompt methods instead.
		icons.Help.Text = ""
		icons.Help.Format = "default"
	})
}

// RequireNonEmpty returns an error if v is a zero-value.
func RequireNonEmpty(v interface{}) error {
	return survey.Required(v)
}

// RequireMinItems enforces at least min elements to be selected from MultiSelect.
func RequireMinItems(min int) ValidatorFunc {
	return (ValidatorFunc)(survey.MinItems(min))
}

func validators(validatorFunc ValidatorFunc) survey.AskOpt {
	return survey.WithValidator(survey.ComposeValidators(survey.Required, survey.Validator(validatorFunc)))
}
