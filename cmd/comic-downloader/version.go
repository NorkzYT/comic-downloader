package main

import (
	"fmt"

	"github.com/NorkzYT/comic-downloader/internal/logger"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/tcnksm/go-latest"
)

var (
	// Tag is the git tag of the current build.
	Tag = "develop"
	// Version is the version of the current build.
	Version = "develop"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Shows the version of the application",
	Run: func(cmd *cobra.Command, args []string) {
		logger.Info("Comic Downloader - Comic downloading tool")
		logger.Info("All Rights Reserved © 2025-2026 %s", color.HiBlackString("Richard Lora"))
		logger.Info("Version: %s - %s", Version, Tag)

		vcheck := &latest.GithubTag{
			Owner:             "NorkzYT",
			Repository:        "comic-downloader",
			FixVersionStrFunc: latest.DeleteFrontV(),
		}

		res, err := latest.Check(vcheck, Tag)
		if err != nil {
			logger.Error("versionCmd: Error checking for updates: %v", err)
			fmt.Printf("Error checking for updates: %s\n", err)
			return
		}
		if res.Outdated {
			logger.Info("versionCmd: App is outdated. Latest version: %s", res.Current)
			fmt.Printf(
				"%s Download latest (%s) from:\n%s\n",
				color.HiRedString("App is outdated."),
				color.RedString(res.Current),
				"https://github.com/NorkzYT/comic-downloader/releases/tag/v"+res.Current,
			)
		} else {
			logger.Info("versionCmd: App is up to date.")
			fmt.Printf("%s\n", color.GreenString("App is up to date."))
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
