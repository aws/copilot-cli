package app

import (
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/Netflix/go-expect"
	"github.com/hinshun/vt10x"
	"github.com/stretchr/testify/require"
)

type mockRender struct {
	t                *testing.T
	wantedFilePrefix string
	wantedData       interface{}
}

func (r *mockRender) Render(filePrefix string, data interface{}) error {
	require.Equal(r.t, r.wantedFilePrefix, filePrefix)
	require.Equal(r.t, r.wantedData, data)
	return nil
}

func TestApp_Init(t *testing.T) {
	testCases := map[string]struct {
		// inputs to App and InitOpts
		projectName      string
		appName          string
		manifestTemplate string

		input func(c *expect.Console) // Interactions with the terminal

		wantedErr error
	}{
		"select Load Balanced Web App": {
			projectName:      "heartbeat",
			appName:          "api",
			manifestTemplate: "", // nothing selected
			input: func(c *expect.Console) {
				c.SendLine("") // Select the first option
			},
			wantedErr: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			mockTerminal, _, _ := vt10x.NewVT10XConsole()
			defer mockTerminal.Close()
			app := &App{
				Project: tc.projectName,
				Name:    tc.appName,
				prompt: terminal.Stdio{
					In:  mockTerminal.Tty(),
					Out: mockTerminal.Tty(),
					Err: mockTerminal.Tty(),
				},
			}
			opts := &InitOpts{
				ManifestTemplate: tc.manifestTemplate,
				m: &mockRender{
					t:                t,
					wantedData:       app,
					wantedFilePrefix: app.Name,
				},
			}

			// Write inputs to the terminal
			done := make(chan struct{})
			go func() {
				defer close(done)
				tc.input(mockTerminal)
			}()

			// WHEN
			err := app.Init(opts)

			// Wait until the terminal receives the input
			mockTerminal.Tty().Close()
			<-done

			// THEN
			if tc.wantedErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.wantedErr.Error())
			}
		})
	}
}
