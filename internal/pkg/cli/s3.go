package cli

import (
	"github.com/aws/amazon-ecs-cli-v2/cmd/ecs-preview/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/group"
	"github.com/spf13/cobra"
)

// BuildS3Cmd is the top level command for the s3 options.
func BuildS3Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "s3",
		Short: "S3 storage commands.",
		Long:  `Command for working with S3 storage.`,
	}

	cmd.AddCommand(BuildS3AddCmd())
	cmd.AddCommand(BuildS3DeleteCmd())

	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.Storage,
	}

	return cmd
}
