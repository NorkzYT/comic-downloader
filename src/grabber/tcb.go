package grabber

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/NorkzYT/comic-downloader/src/http"
	"github.com/NorkzYT/comic-downloader/src/logger"
	"github.com/PuerkitoBio/goquery"
)

// Tcb is a grabber for tcbscans.com (and possibly other WordPress sites).
type Tcb struct {
	*Grabber
	chaps *goquery.Selection
	title string
}

// TcbChapter is a chapter for TCBScans.
type TcbChapter struct {
	Chapter
	URL string
}

// Test returns true if the URL is a compatible TCBScans URL.
func (t *Tcb) Test() (bool, error) {
	logger.Debug("Tcb.Test: Checking URL: %s", t.URL)
	re := regexp.MustCompile(`manga\/(.*)\/$`)
	if !re.MatchString(t.URL) {
		logger.Debug("Tcb.Test: URL does not match expected pattern.")
		return false, nil
	}

	mid := re.FindStringSubmatch(t.URL)[1]
	uri, _ := url.JoinPath(t.BaseUrl(), "manga", mid, "ajax", "chapters")

	rbody, err := http.Post(http.RequestParams{
		URL:     uri,
		Referer: t.BaseUrl(),
	})
	if err != nil {
		logger.Error("Tcb.Test: Error posting to %s: %v", uri, err)
		return false, err
	}

	body, err := goquery.NewDocumentFromReader(rbody)
	if err != nil {
		logger.Error("Tcb.Test: Error parsing document from %s: %v", uri, err)
		return false, err
	}

	t.chaps = body.Find("li")
	if t.chaps.Length() > 0 {
		logger.Debug("Tcb.Test: Found %d chapters", t.chaps.Length())
	} else {
		logger.Debug("Tcb.Test: No chapters found.")
	}
	return t.chaps.Length() > 0, nil
}

// FetchTitle fetches and returns the comic title.
func (t *Tcb) FetchTitle() (string, error) {
	logger.Debug("Tcb.FetchTitle: Fetching title from URL: %s", t.URL)
	if t.title != "" {
		logger.Debug("Tcb.FetchTitle: Returning cached title: %s", t.title)
		return t.title, nil
	}
	rbody, err := http.Get(http.RequestParams{
		URL: t.URL,
	})
	if err != nil {
		logger.Error("Tcb.FetchTitle: Error fetching URL: %v", err)
		return "", err
	}
	defer rbody.Close()
	body, err := goquery.NewDocumentFromReader(rbody)
	if err != nil {
		logger.Error("Tcb.FetchTitle: Error parsing document: %v", err)
		return "", err
	}

	t.title = strings.TrimSpace(body.Find("h1").Text())
	logger.Debug("Tcb.FetchTitle: Fetched title: %s", t.title)
	return t.title, nil
}

// FetchChapters returns a slice of chapters.
func (t Tcb) FetchChapters() (chapters Filterables, errs []error) {
	logger.Debug("Tcb.FetchChapters: Starting to fetch chapters.")
	t.chaps.Each(func(i int, s *goquery.Selection) {
		link := s.Find("a")
		if len(link.Children().Nodes) > 0 {
			link.Children().Remove()
		}
		title := strings.TrimSpace(link.Text())
		re := regexp.MustCompile(`(\d+\.?\d*)`)
		ns := re.FindString(title)
		num, err := strconv.ParseFloat(ns, 64)
		if err != nil {
			logger.Error("Tcb.FetchChapters: Error parsing chapter number: %v", err)
			errs = append(errs, err)
		}
		chapter := &TcbChapter{
			Chapter: Chapter{
				Title:  title,
				Number: num,
			},
			URL: s.Find("a").AttrOr("href", ""),
		}
		logger.Debug("Tcb.FetchChapters: Found chapter: %s", chapter.GetTitle())
		chapters = append(chapters, chapter)
	})

	return
}

// FetchChapter fetches a chapter and its pages.
func (t Tcb) FetchChapter(f Filterable) (*Chapter, error) {
	tchap := f.(*TcbChapter)
	logger.Debug("Tcb.FetchChapter: Fetching chapter from URL: %s", tchap.URL)
	rbody, err := http.Get(http.RequestParams{
		URL:     tchap.URL,
		Referer: t.BaseUrl(),
	})
	if err != nil {
		logger.Error("Tcb.FetchChapter: Error fetching chapter page: %v", err)
		return nil, err
	}
	defer rbody.Close()
	body, err := goquery.NewDocumentFromReader(rbody)
	if err != nil {
		logger.Error("Tcb.FetchChapter: Error parsing chapter page: %v", err)
		return nil, err
	}

	pageURLs := []string{}
	body.Find("#single-pager option").Each(func(i int, s *goquery.Selection) {
		if url := s.AttrOr("data-redirect", ""); url != "" {
			pageURLs = append(pageURLs, url)
		}
	})

	if len(pageURLs) == 0 {
		pageURLs = append(pageURLs, tchap.URL)
	}

	pages := []Page{}
	for pageNum, pageURL := range pageURLs {
		rbody, err := http.Get(http.RequestParams{
			URL:     pageURL,
			Referer: t.BaseUrl(),
		})
		if err != nil {
			logger.Info("Tcb.FetchChapter: Error fetching page %d: %v", pageNum+1, err)
			continue
		}

		pageDoc, err := goquery.NewDocumentFromReader(rbody)
		rbody.Close()
		if err != nil {
			logger.Info("Tcb.FetchChapter: Error parsing page %d: %v", pageNum+1, err)
			continue
		}

		found := false
		pageDoc.Find("div.reading-content img").Each(func(i int, s *goquery.Selection) {
			if found {
				return
			}
			u := strings.TrimSpace(s.AttrOr("data-src", s.AttrOr("src", "")))
			if u == "" {
				logger.Info("Tcb.FetchChapter: Page %d of %s has no URL", pageNum+1, f.GetTitle())
				return
			}
			if !strings.HasPrefix(u, "http") {
				u = t.BaseUrl() + u
			}
			pages = append(pages, Page{
				Number: int64(pageNum + 1),
				URL:    u,
			})
			found = true
		})
	}

	chapter := &Chapter{
		Title:      f.GetTitle(),
		Number:     f.GetNumber(),
		PagesCount: int64(len(pages)),
		Language:   "en",
		Pages:      pages,
	}
	logger.Debug("Tcb.FetchChapter: Fetched chapter %s with %d pages", chapter.GetTitle(), len(pages))
	return chapter, nil
}
