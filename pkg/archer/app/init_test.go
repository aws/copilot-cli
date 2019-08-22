package app

import (
	"bytes"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/Netflix/go-expect"
	"github.com/hinshun/vt10x"
	"github.com/stretchr/testify/require"
)

type mockManifestWriteCloser struct {
	buf *bytes.Buffer
}

func (m *mockManifestWriteCloser) Write(p []byte) (int, error) {
	return m.buf.Write(p)
}

func (m *mockManifestWriteCloser) Close() error {
	return nil
}

func TestManifestTypeFileNamePairs(t *testing.T) {
	require.Equal(t, len(manifestFileNames), len(manifestTypes), "manifest file names and types must match")
}

func TestInitOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		input string
		want  error
	}{
		"empty manifest type": {
			input: "",
			want:  nil,
		},
		"invalid manifest type": {
			input: "Horrible Service",
			want:  ErrInvalidManifestType,
		},
		"valid manifest type": {
			input: "Load Balanced Web App",
			want:  nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			opts := InitOpts{ManifestType: tc.input}

			if tc.want == nil {
				require.NoError(t, opts.Validate())
			} else {
				require.EqualError(t, opts.Validate(), tc.want.Error())
			}
		})
	}
}

func TestApp_Init(t *testing.T) {
	testCases := map[string]struct {
		projectName string
		appName     string
		input       func(c *expect.Console) // Interactions with the terminal

		wantedManifestType string
		wantedErr          error
		wantedManifest     string
	}{
		"select Load Balanced Web App": {
			projectName: "heartbeat",
			appName:     "api",
			input: func(c *expect.Console) {
				c.SendLine("") // Select the first option
			},
			wantedManifestType: manifestTypes[0],
			wantedErr:          nil,
			wantedManifest: `# First is the Project name. The Project is the grouping of the
# environments related to each other.
project: heartbeat

application:
  # Your application name will be used in naming your resources
  # like log groups, services, etc.
  api:
    # The "Type" of the application you're running. For a list of all types that we support see
    # https://github.com/aws/PRIVATE-amazon-ecs-archer/app/template/manifest/
    type: LoadBalancedFargateService

    # The port exposed through your container. We need to know
    # this so that we can route traffic to it.
    containerPort: 80

    # Size of CPU
    cpu: '256'

    # Size of memory
    memory: '512'

    # Logging is enabled by default. We'll create a loggroup that is
    # the heartbeat/api/Stage
    logging: true

    # Determines whether the application will have a public IP or not.
    public: true

    # You can also pass in environment variables as key/value pairs
    #environment-variables:
    #  dog: 'Clyde'
    #  cute: 'hekya'
    #
    # Additional Sidecar apps that can run along side your main application
    #sidecars:
    #  fluentbit:
    #    containerPort: 80
    #    image: 'amazon/aws-for-fluent-bit:1.2.0'
    #    memory: '512'

# This section defines each of the release stages
# and their specific configuration for your app.
stages:
  -
    # The "environment" (cluster/vpc/lb) to contain this service.
    env: test
    # The number of tasks that we want, at minimum.
    desiredCount: 1
    # Any secrets via ARNs
    #secrets:
      #lemonaidpassword: arn:aws:secretsmanager:us-west-2:902697171733:secret:DavidsLemons/DavidsFrontEnd
`,
		},
		"select Empty": {
			projectName: "heartbeat",
			appName:     "replicator",
			input: func(c *expect.Console) {
				c.SendLine(string(terminal.KeyArrowDown))
			},
			wantedManifestType: manifestTypes[1],
			wantedErr:          nil,
			wantedManifest: `# First is the Project name. The Project is the grouping of the
# environments related to each other.
project: heartbeat

application:
  # Your application name will be used in naming your resources
  # like log groups, services, etc.
  replicator:
    # The "Type" of the application you're running. For a list of all types that we support see
    # https://github.com/aws/PRIVATE-amazon-ecs-archer/app/template/manifest/
    type: empty

    # You can define the rest of your infrastructure with CloudFormation templates under ../replicator/infra/

# This section defines each of the release stages
# and their specific configuration for your app.
stages:
  -
    # The "environment" (cluster/vpc/lb) to contain this service.
    env: test
`,
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

			var buf bytes.Buffer
			opts := &InitOpts{
				wc: &mockManifestWriteCloser{
					&buf,
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
			require.Equal(t, tc.wantedManifestType, opts.ManifestType, "incorrect manifest type selected")
			require.Equal(t, tc.wantedManifest, buf.String(), "rendered manifest templates aren't same")
		})
	}
}
