package grabber

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/NorkzYT/comic-downloader/src/http"
	"github.com/NorkzYT/comic-downloader/src/logger"
	"github.com/PuerkitoBio/goquery"
)

// PlainHTML is a grabber for any plain HTML page (with no AJAX pagination).
type PlainHTML struct {
	*Grabber
	doc  *goquery.Document
	rows *goquery.Selection
	site SiteSelector
}

type SiteSelector struct {
	Title        string
	Rows         string
	Link         string
	Chapter      string
	ChapterTitle string
	Image        string
}

// PlainHTMLChapter represents a PlainHTML Chapter.
type PlainHTMLChapter struct {
	Chapter
	URL string
}

// Test returns true if the URL is a valid grabber URL.
func (m *PlainHTML) Test() (bool, error) {
	// Skip plain HTML for asuracomic.net.
	if strings.Contains(m.URL, "asuracomic.net") {
		logger.Debug("PlainHTML.Test: URL contains asuracomic.net; skipping plain HTML grabber.")
		return false, nil
	}
	body, err := http.Get(http.RequestParams{
		URL: m.URL,
	})
	if err != nil {
		logger.Error("PlainHTML.Test: Error fetching URL: %v", err)
		return false, err
	}
	m.doc, err = goquery.NewDocumentFromReader(body)
	if err != nil {
		logger.Error("PlainHTML.Test: Error parsing document: %v", err)
		return false, err
	}

	selectors := []SiteSelector{
		// tcbscans.com
		{
			Title:        "h1",
			Rows:         "main .mx-auto .grid .col-span-2 a",
			Chapter:      ".font-bold",
			ChapterTitle: ".text-gray-500",
			Image:        "picture img",
		},
		// manganelo/manganato
		{
			Title:        "h1",
			Rows:         "div.panel-story-chapter-list .row-content-chapter li",
			Chapter:      "a",
			ChapterTitle: "a",
			Link:         "a",
			Image:        "div.container-chapter-reader img",
		},
		// manganelos/mangapanda
		{
			Title:        "h1",
			Rows:         "#examples div.chapter-list .row",
			Chapter:      "a",
			ChapterTitle: "a",
			Link:         "a",
			Image:        "div.container-chapter-reader img",
		},
		// mangakakalot
		{
			Title:        "h1",
			Rows:         "div.chapter-list .row",
			Chapter:      "a",
			ChapterTitle: "a",
			Link:         "a",
			Image:        "div.container-chapter-reader img,#vungdoc img",
		},
		// mangamonks
		{
			Title:        "h3.info-title",
			Rows:         "#chapter .chapter-list li",
			Chapter:      ".chapter-number",
			ChapterTitle: ".chapter-number",
			Link:         "a",
			Image:        "#imageContainer img",
		},
	}

	for _, selector := range selectors {
		logger.Debug("PlainHTML.Test: Testing selector for Title: '%s'", selector.Title)
		rows := m.doc.Find(selector.Rows)
		logger.Debug("PlainHTML.Test: Selector '%s' found %d elements", selector.Rows, rows.Length())

		if rows.Length() > 0 {
			m.rows = rows
			m.site = selector
			logger.Debug("PlainHTML.Test: Selector matched: %+v", selector)
			break
		}
	}

	if m.rows == nil || m.rows.Length() == 0 {
		logger.Error("PlainHTML.Test: No matching elements found with any selector")
		return false, nil
	}
	return m.rows.Length() > 0, nil
}

// FetchTitle returns the comic title.
func (m PlainHTML) FetchTitle() (string, error) {
	title := m.doc.Find(m.site.Title)
	logger.Debug("PlainHTML.FetchTitle: Fetched title: %s", title.Text())
	return sanitizeTitle(title.Text()), nil
}

// FetchChapters returns a slice of chapters.
func (m PlainHTML) FetchChapters() (chapters Filterables, errs []error) {
	logger.Debug("PlainHTML.FetchChapters: Starting to fetch chapters.")
	m.rows.Each(func(i int, s *goquery.Selection) {
		re := regexp.MustCompile(`Chapter\s*(\d+\.?\d*)`)
		chap := re.FindStringSubmatch(s.Find(m.site.Chapter).Text())
		if len(chap) == 0 {
			logger.Debug("PlainHTML.FetchChapters: Skipping non-chapter row at index %d", i)
			return
		}
		num := chap[1]
		number, err := strconv.ParseFloat(num, 64)
		if err != nil {
			logger.Error("PlainHTML.FetchChapters: Error parsing chapter number: %v", err)
			errs = append(errs, err)
			return
		}
		u := s.AttrOr("href", "")
		if m.site.Link != "" {
			u = s.Find(m.site.Link).AttrOr("href", "")
		}
		if !strings.HasPrefix(u, "http") {
			u = m.BaseUrl() + u
		}
		chapter := &PlainHTMLChapter{
			Chapter: Chapter{
				Number: number,
				Title:  s.Find(m.site.ChapterTitle).Text(),
			},
			URL: u,
		}
		logger.Debug("PlainHTML.FetchChapters: Found chapter: %s", chapter.GetTitle())
		chapters = append(chapters, chapter)
	})
	return
}

// FetchChapter fetches a chapter and its pages.
func (m PlainHTML) FetchChapter(f Filterable) (*Chapter, error) {
	mchap := f.(*PlainHTMLChapter)
	logger.Debug("PlainHTML.FetchChapter: Fetching chapter from URL: %s", mchap.URL)
	body, err := http.Get(http.RequestParams{
		URL: mchap.URL,
	})
	if err != nil {
		logger.Error("PlainHTML.FetchChapter: Error fetching chapter URL: %v", err)
		return nil, err
	}
	defer body.Close()
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		logger.Error("PlainHTML.FetchChapter: Error parsing chapter document: %v", err)
		return nil, err
	}

	pimages := getPlainHTMLImageURL(m.site.Image, doc)
	pcount := len(pimages)

	chapter := &Chapter{
		Title:      f.GetTitle(),
		Number:     f.GetNumber(),
		PagesCount: int64(pcount),
		Language:   "en",
	}

	for i, img := range pimages {
		if img == "" {
			logger.Info("PlainHTML.FetchChapter: page %d of %s has no URL (will be ignored)", i, chapter.GetTitle())
			continue
		}
		if !strings.HasPrefix(img, "http") {
			img = m.BaseUrl() + img
		}
		logger.Debug("PlainHTML.FetchChapter: Adding page %d with URL: %s", i+1, img)
		page := Page{
			Number: int64(i),
			URL:    img,
		}
		chapter.Pages = append(chapter.Pages, page)
	}

	return chapter, nil
}

func getPlainHTMLImageURL(selector string, doc *goquery.Document) []string {
	pimages := doc.Find("#arraydata")
	if pimages.Length() == 1 {
		logger.Debug("getPlainHTMLImageURL: Found hidden arraydata element.")
		return strings.Split(pimages.Text(), ",")
	}

	pimages = doc.Find(selector)
	imgs := []string{}
	pimages.Each(func(i int, s *goquery.Selection) {
		src := s.AttrOr("src", "")
		if src == "" || strings.HasPrefix(src, "data:image") {
			src = s.AttrOr("data-src", "")
		}
		imgs = append(imgs, src)
	})
	return imgs
}

func sanitizeTitle(title string) string {
	spaces := regexp.MustCompile(`\s+`)
	title = spaces.ReplaceAllString(title, " ")
	title = strings.TrimSpace(title)
	logger.Debug("sanitizeTitle: Sanitized title: %s", title)
	return title
}
