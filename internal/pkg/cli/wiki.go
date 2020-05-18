package cli

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/aws/amazon-ecs-cli-v2/cmd/copilot/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/group"
	"github.com/spf13/cobra"
)

const (
	wikiURL = "https://github.com/aws/amazon-ecs-cli-v2/wiki"
)

// BuildWikiCmd builds the command for opening the wiki.
func BuildWikiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wiki",
		Short: "Open the project wiki.",
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			var err error

			switch runtime.GOOS {
			case "linux":
				err = exec.Command("xdg-open", wikiURL).Start()
			case "windows":
				err = exec.Command("rundll32", "url.dll,FileProtocolHandler", wikiURL).Start()
			case "darwin":
				err = exec.Command("open", wikiURL).Start()
			default:
				err = fmt.Errorf("unsupported platform")
			}
			if err != nil {
				return fmt.Errorf("open wiki: %w", err)
			}

			return nil
		}),
		Annotations: map[string]string{
			"group": group.GettingStarted,
		},
	}

	cmd.SetUsageTemplate(template.Usage)

	return cmd
}
