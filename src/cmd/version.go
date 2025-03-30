package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/tcnksm/go-latest"
)

var (
	// Tag is the git tag of the current build
	Tag = "develop"
	// Version is the version of the current build
	Version = "develop"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Shows the version of the application",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%s - Comic downloading tool\n", color.YellowString("Comic Downloader"))
		fmt.Printf("All Rights Reserved © 2025-2026 %s\n", color.HiBlackString("Richard Lora"))
		fmt.Printf("Version: %s - ", color.MagentaString("%s (%s)", Version, Tag))

		vcheck := &latest.GithubTag{
			Owner:             "NorkzYT",
			Repository:        "comic-downloader",
			FixVersionStrFunc: latest.DeleteFrontV(),
		}

		res, err := latest.Check(vcheck, Tag)
		if err != nil {
			fmt.Printf("Error checking for updates: %s\n", err)
			return
		}
		if res.Outdated {
			fmt.Printf(
				"%s Download latest (%s) from:\n%s\n",
				color.HiRedString("App is outdated."),
				color.RedString(res.Current),
				"https://github.com/NorkzYT/comic-downloader/releases/tag/v"+res.Current,
			)
		} else {
			fmt.Printf("%s\n", color.GreenString("App is up to date."))
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
