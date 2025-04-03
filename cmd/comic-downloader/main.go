package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/NorkzYT/comic-downloader/internal/downloader"
	"github.com/NorkzYT/comic-downloader/internal/grabber"
	"github.com/NorkzYT/comic-downloader/internal/logger"
	"github.com/NorkzYT/comic-downloader/internal/packer"
	"github.com/NorkzYT/comic-downloader/internal/ranges"
	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/progress"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	cc "github.com/ivanpirog/coloredcobra"
)

var settings grabber.Settings

type BrowserlessUser interface {
	UsesBrowser() bool
}

var rootCmd = &cobra.Command{
	Use:   "comic-downloader [flags] [url] [ranges]",
	Short: "Helps you download comics from supported popular websites",
	Long: `With comic-downloader you can easily convert/download web based comic files.

You only need to specify the URL of the comic and the chapters you want to download as a range.

Note the URL must be of the index of the comic, not a single chapter.`,
	Example: colorizeHelp(`  comic-downloader https://inmanga.com/ver/manga/Fire-Punch/17748683-8986-4628-934a-e94a47fe5d59

Would ask you if you want to download all chapters of Fire Punch (1-83).

  comic-downloader https://inmanga.com/ver/manga/Dr-Stone/d9e47ba6-7dfc-401d-a21c-19326c2ea45f 1-10

Would download chapters 1 to 10 of Dr. Stone from inmanga.com.

  comic-downloader https://inmanga.com/ver/manga/Dr-Stone/d9e47ba6-7dfc-401d-a21c-19326c2ea45f 1-10,12,15-20

Would download chapters 1 to 10, 12 and 15 to 20 of Dr. Stone from inmanga.com.

  comic-downloader --language es https://mangadex.org/title/e7eabe96-aa17-476f-b431-2497d5e9d060/black-clover 10-20

Would download chapters 10 to 20 of Black Clover from mangadex.org in Spanish.

  comic-downloader --language es --bundle https://mangadex.org/title/e7eabe96-aa17-476f-b431-2497d5e9d060/black-clover 10-20

It would also download chapters 10 to 20 of Black Clover from mangadex.org in Spanish, although in this case would bundle them into a single file.

Note arguments aren't positional, thus you can specify them in any order:

comic-downloader --language es 10-20 https://mangadex.org/title/e7eabe96-aa17-476f-b431-2497d5e9d060/black-clover --bundle

Would download and bundle chapters 10 to 20 of Black Clover from mangadex.org in Spanish.`),
	Args: cobra.MinimumNArgs(1),
	Run:  run,
}

func run(cmd *cobra.Command, args []string) {
	logger.Debug("rootCmd.Run: Starting execution with args: %v", args)
	s, errs := grabber.NewSite(getUrlArg(args), &settings)
	if len(errs) > 0 {
		logger.Error("rootCmd.Run: Errors testing site:")
		for _, err := range errs {
			logger.Error("rootCmd.Run: %v", err)
		}
	}
	if s == nil {
		logger.Info("rootCmd.Run: Site not recognised")
		fmt.Println(color.YellowString("Site not recognised"))
		os.Exit(1)
	}
	s.InitFlags(cmd)

	if bl, ok := s.(BrowserlessUser); ok && bl.UsesBrowser() {
		fmt.Println("Initializing remote browser; please wait...")
	}

	title, err := s.FetchTitle()
	cerr(err, "Error fetching title: ")

	chapters, errs := s.FetchChapters()
	if len(errs) > 0 {
		logger.Error("rootCmd.Run: Errors fetching chapters:")
		for _, err := range errs {
			logger.Error("rootCmd.Run: %v", err)
		}
		os.Exit(1)
	}
	chapters = chapters.SortByNumber()

	var rngs []ranges.Range
	if len(args) == 1 {
		lastChapter := chapters[len(chapters)-1].GetNumber()
		prompt := promptui.Prompt{
			Label:     fmt.Sprintf("Do you want to download all %g chapters", lastChapter),
			IsConfirm: true,
		}
		_, err := prompt.Run()
		if err != nil {
			logger.Info("rootCmd.Run: Download canceled by user")
			fmt.Println(color.YellowString("Canceled by user"))
			os.Exit(0)
		}
		rngs = []ranges.Range{{Begin: 1.0, End: lastChapter}}
	} else {
		settings.Range = getRangesArg(args)
		rngs, err = ranges.Parse(settings.Range)
		cerr(err, "Error parsing ranges: ")
	}
	chapters = chapters.FilterRanges(rngs)
	if err := os.MkdirAll(settings.OutputDir, 0755); err != nil {
		logger.Error("rootCmd.Run: Error creating output directory: %v", err)
		fmt.Println(color.RedString("Error creating output directory: " + err.Error()))
		os.Exit(1)
	}
	if len(chapters) == 0 {
		logger.Info("rootCmd.Run: No chapters found for the specified ranges")
		fmt.Println(color.YellowString("No chapters found for the specified ranges"))
		os.Exit(1)
	}

	pw := progress.NewWriter()
	pw.SetAutoStop(false)
	pw.SetUpdateFrequency(100 * time.Millisecond)
	pw.SetStyle(progress.StyleBlocks)
	pw.Style().Colors = progress.StyleColorsExample
	pw.Style().Visibility.ETA = true
	pw.Style().Visibility.ETAOverall = true
	pw.Style().Visibility.Percentage = true
	pw.Style().Visibility.Speed = true
	pw.Style().Visibility.SpeedOverall = true
	pw.Style().Visibility.Time = true
	pw.Style().Visibility.Tracker = true
	pw.Style().Visibility.TrackerOverall = true
	pw.Style().Visibility.Value = true

	pw.SetSortBy(progress.SortByMessage)

	go pw.Render()

	wg := sync.WaitGroup{}
	guard := make(chan struct{}, s.GetMaxConcurrency().Chapters)
	termWidth := getTerminalWidth()
	comicLen, chapterLen := calculateTitleLengths(termWidth)

	trackers := make([]*progress.Tracker, len(chapters))
	for i, chap := range chapters {
		barTitle := fmt.Sprintf("%s - %s", truncateString(title, comicLen), truncateString(chap.GetTitle(), chapterLen))
		tracker := &progress.Tracker{
			Message:            barTitle + " [Fetching]",
			Total:              80,
			RemoveOnCompletion: false,
		}
		trackers[i] = tracker
		pw.AppendTracker(tracker)
	}

	var mu sync.Mutex
	var bundledChapters []*packer.DownloadedChapter

	for i, chap := range chapters {
		guard <- struct{}{}
		wg.Add(1)
		go func(chap grabber.Filterable, tracker *progress.Tracker, barTitle string) {
			defer wg.Done()
			var chapter *grabber.Chapter
			var err error
			if fetcher, ok := s.(interface {
				FetchChapterWithProgress(grabber.Filterable, func()) (*grabber.Chapter, error)
			}); ok {
				chapter, err = fetcher.FetchChapterWithProgress(chap, func() {
					tracker.Increment(1)
				})
			} else {
				chapter, err = s.FetchChapter(chap)
			}
			if err != nil {
				logger.Error("rootCmd.Run: Error fetching chapter %s: %v", chap.GetTitle(), err)
				tracker.UpdateMessage(barTitle + " [Download Failed]")
				<-guard
				return
			}

			downloadingTicks := chapter.PagesCount
			archivingTicks := chapter.PagesCount
			newTotal := int64(80) + downloadingTicks
			if !settings.Bundle {
				newTotal += archivingTicks
			}
			tracker.Total = newTotal

			tracker.UpdateMessage(barTitle + " [Downloading]")
			files, err := downloader.FetchChapter(s, chapter, func(page int, progressValue int, err error) {
				if err != nil {
					tracker.UpdateMessage(barTitle + " [Downloading: Error " + err.Error() + "]")
				} else {
					tracker.Increment(1)
				}
			})
			if err != nil {
				logger.Error("rootCmd.Run: Error downloading chapter %s: %v", chapter.GetTitle(), err)
				tracker.UpdateMessage(barTitle + " [Download Failed]")
				<-guard
				return
			}

			if settings.Bundle {
				mu.Lock()
				bundledChapters = append(bundledChapters, &packer.DownloadedChapter{
					Chapter: chapter,
					Files:   files,
				})
				mu.Unlock()
			} else {
				tracker.UpdateMessage(barTitle + " [Archiving]")
				d := &packer.DownloadedChapter{
					Chapter: chapter,
					Files:   files,
				}
				_, err := packer.PackSingle(settings.OutputDir, s, d, func(page, _ int) {
					tracker.Increment(1)
				})
				if err != nil {
					logger.Error("rootCmd.Run: Error archiving chapter: %v", err)
				}
			}
			tracker.MarkAsDone()
			<-guard
		}(chap, trackers[i], fmt.Sprintf("%s - %s", truncateString(title, comicLen), truncateString(chap.GetTitle(), chapterLen)))
	}
	wg.Wait()

	if !settings.Bundle {
		pw.Stop()
		os.Exit(0)
	}

	sort.SliceStable(bundledChapters, func(i, j int) bool {
		return bundledChapters[i].Chapter.Number < bundledChapters[j].Chapter.Number
	})
	totalPages := 0
	for _, d := range bundledChapters {
		totalPages += int(d.Chapter.PagesCount)
	}
	bundleTracker := progress.Tracker{
		Message: "Bundle [Archiving All Chapters]",
		Total:   int64(totalPages),
	}
	pw.AppendTracker(&bundleTracker)

	filename, err := packer.PackBundle(settings.OutputDir, s, bundledChapters, settings.Range, func(page, _ int) {
		bundleTracker.Increment(1)
	})
	if err != nil {
		logger.Error("rootCmd.Run: Error bundling chapters: %v", err)
		fmt.Println(color.RedString(err.Error()))
		os.Exit(1)
	}
	bundleTracker.MarkAsDone()
	fmt.Printf("- %s %s\n", color.GreenString("saved file"), color.HiBlackString(filename))
	pw.Stop()
}

func Execute() {
	cc.Init(&cc.Config{
		RootCmd:       rootCmd,
		Headings:      cc.HiCyan + cc.Bold,
		Commands:      cc.HiYellow + cc.Bold,
		Aliases:       cc.Bold + cc.Italic,
		CmdShortDescr: cc.HiRed,
		ExecName:      cc.Bold,
		Flags:         cc.Bold,
		FlagsDescr:    cc.HiMagenta,
		FlagsDataType: cc.Italic,
	})
	if err := rootCmd.Execute(); err != nil {
		logger.Error("rootCmd.Execute: %v", err)
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolVarP(&settings.Bundle, "bundle", "b", false, "bundle all specified chapters into a single file")
	rootCmd.Flags().Uint8VarP(&settings.MaxConcurrency.Chapters, "concurrency", "c", 5, "number of concurrent chapter downloads, hard-limited to 5")
	rootCmd.Flags().Uint8VarP(&settings.MaxConcurrency.Pages, "concurrency-pages", "C", 10, "number of concurrent page downloads, hard-limited to 10")
	rootCmd.Flags().StringVarP(&settings.Language, "language", "l", "", "only download the specified language")
	rootCmd.Flags().StringVarP(&settings.FilenameTemplate, "filename-template", "t", packer.FilenameTemplateDefault, "template for the resulting filename")
	rootCmd.PersistentFlags().StringVarP(&settings.OutputDir, "output-dir", "o", "./", "output directory for the downloaded files")
	rootCmd.Flags().StringVarP(&settings.Format, "format", "f", "cbz", "archive format: cbz, zip, raw")
}

func cerr(err error, prefix string) {
	if err != nil {
		logger.Error("rootCmd.cerr: %s %v", prefix, err)
		fmt.Println(color.RedString(prefix + err.Error()))
		os.Exit(1)
	}
}

func colorizeHelp(help string) string {
	yre := regexp.MustCompile(`comic-downloader|nada`)
	help = yre.ReplaceAllStringFunc(help, func(s string) string {
		return color.YellowString(s)
	})
	gre := regexp.MustCompile(`http[^ ]*|[\d]+-[\d,-]+`)
	help = gre.ReplaceAllStringFunc(help, func(s string) string {
		return color.HiBlackString(s)
	})
	bre := regexp.MustCompile(`((--language|--bundle)( es)?)`)
	help = bre.ReplaceAllStringFunc(help, func(s string) string {
		return color.HiBlueString(s)
	})
	return help
}

func getRangesArg(args []string) string {
	if len(args) == 1 {
		return ""
	}
	if strings.HasPrefix(args[0], "http") {
		return args[1]
	}
	return args[0]
}

func getUrlArg(args []string) string {
	if len(args) == 1 {
		return args[0]
	}
	if strings.HasPrefix(args[0], "http") {
		return args[0]
	}
	return args[1]
}

func getTerminalWidth() int {
	width, _, err := term.GetSize(int(syscall.Stdin))
	if err != nil {
		return 80
	}
	return width
}

func calculateTitleLengths(termWidth int) (comicLen, chapterLen int) {
	reservedSpace := 35
	availableSpace := termWidth - reservedSpace
	if availableSpace > 20 {
		comicLen = (availableSpace * 60) / 100
		chapterLen = (availableSpace * 40) / 100
	} else {
		comicLen = 10
		chapterLen = 10
	}
	return
}

func truncateString(input string, maxLength int) string {
	if maxLength <= 0 {
		return ""
	}
	if len(input) <= maxLength {
		return input
	}
	truncationPoint := strings.LastIndex(input[:maxLength], " ")
	if truncationPoint == -1 {
		return input[:maxLength] + "..."
	}
	return input[:truncationPoint] + "..."
}

func main() {
	Execute()
}
