package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/fatih/color"
	"golang.org/x/sync/errgroup"
	"golang.org/x/text/language"
	"golang.org/x/text/search"
)

type SearchResult struct {
	Title       string   `json:"title"`
	URL         string   `json:"url"`
	Description string   `json:"description"`
	IsAd        bool     `json:"is_ad"`
	Rank        int      `json:"rank"`
	Keywords    []string `json:"keywords,omitempty"`
	Metadata    struct {
		Domain        string    `json:"domain"`
		FetchedAt     time.Time `json:"fetched_at"`
		ContentType   string    `json:"content_type,omitempty"`
		ResultType    string    `json:"result_type,omitempty"`
		SchemaType    string    `json:"schema_type,omitempty"`
		PositionData  string    `json:"position_data,omitempty"`
		SearchFeature string    `json:"search_feature,omitempty"`
	} `json:"metadata"`
}

type SearchRequest struct {
	Query          string
	Engine         string
	MaxResults     int
	IncludeAds     bool
	Timeout        time.Duration
	ProxyURL       string
	UseHeadless    bool
	Language       string
	Region         string
	Page           int
	AdvancedQuery  map[string]string
	ExcludeDomains []string
	MinWordCount   int
	MaxWordCount   int
	DateRange      struct {
		Start time.Time
		End   time.Time
	}
	Debug bool
}

type SearchEngine interface {
	Search(ctx context.Context, request SearchRequest) ([]SearchResult, error)
	Name() string
	Capabilities() []string
	SetRateLimit(requestsPerMinute int)
}

type BaseSearchEngine struct {
	adPatterns      []*regexp.Regexp
	resultSelectors map[string]string
	userAgents      []string
	currentUA       int
	uaMutex         sync.Mutex
	rateLimit       int
	lastRequest     time.Time
	rateLimitMutex  sync.Mutex
	debugMode       bool
}

func NewBaseSearchEngine() *BaseSearchEngine {
	return &BaseSearchEngine{
		adPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)sponsored`),
			regexp.MustCompile(`(?i)advertisement`),
			regexp.MustCompile(`(?i)^ad\s`),
			regexp.MustCompile(`(?i)promoted`),
		},
		resultSelectors: map[string]string{
			"title":       "",
			"url":         "",
			"description": "",
		},
		userAgents: []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Safari/605.1.15",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:123.0) Gecko/20100101 Firefox/123.0",
			"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
			"Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/122.0.6261.89 Mobile/15E148 Safari/604.1",
		},
		rateLimit: 10,
	}
}

func (b *BaseSearchEngine) GetNextUserAgent() string {
	b.uaMutex.Lock()
	defer b.uaMutex.Unlock()

	ua := b.userAgents[b.currentUA]
	b.currentUA = (b.currentUA + 1) % len(b.userAgents)
	return ua
}

func (b *BaseSearchEngine) IsAd(text string) bool {
	for _, pattern := range b.adPatterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

func (b *BaseSearchEngine) ExtractKeywords(text string, query string) []string {
	matcher := search.New(language.English, search.IgnoreCase)
	keywords := make(map[string]bool)

	queryWords := strings.Fields(strings.ToLower(query))
	for _, word := range queryWords {
		if len(word) > 3 {
			start, _ := matcher.IndexString(text, word)
			for start >= 0 {
				keywords[word] = true
				start, _ = matcher.IndexString(text[start+len(word):], word)
				if start >= 0 {
					start = start + len(word)
				}
			}
		}
	}

	result := make([]string, 0, len(keywords))
	for k := range keywords {
		result = append(result, k)
	}
	return result
}

func (b *BaseSearchEngine) SetRateLimit(requestsPerMinute int) {
	b.rateLimitMutex.Lock()
	defer b.rateLimitMutex.Unlock()
	b.rateLimit = requestsPerMinute
}

func (b *BaseSearchEngine) RespectRateLimit() {
	b.rateLimitMutex.Lock()
	defer b.rateLimitMutex.Unlock()

	if b.rateLimit <= 0 {
		return
	}

	minInterval := time.Minute / time.Duration(b.rateLimit)
	elapsed := time.Since(b.lastRequest)

	if elapsed < minInterval {
		time.Sleep(minInterval - elapsed)
	}

	b.lastRequest = time.Now()
}

func (b *BaseSearchEngine) Capabilities() []string {
	return []string{"basic", "text"}
}

func (b *BaseSearchEngine) SetDebugMode(debug bool) {
	b.debugMode = debug
}

func (b *BaseSearchEngine) DebugLog(format string, args ...interface{}) {
	if b.debugMode {
		log.Printf("[DEBUG] "+format, args...)
	}
}

type GoogleSearchEngine struct {
	*BaseSearchEngine
	apiKey string
}

func NewGoogleSearchEngine() *GoogleSearchEngine {
	base := NewBaseSearchEngine()

	// Updated selectors for Google's current DOM structure
	base.resultSelectors = map[string]string{
		"container":   "#search .g, #rso .g, #search .MjjYud, #rso .MjjYud",
		"title":       "h3, h3.LC20lb",
		"url":         "a, .yuRUbf a",
		"description": ".VwiC3b, .IsZvec",
		"knowledge":   ".kp-wholepage, .ULSxyf",
		"video":       ".X7NTVe",
		"ad":          ".uEierd, .commercial-unit-desktop-top",
	}

	return &GoogleSearchEngine{
		BaseSearchEngine: base,
	}
}

func (g *GoogleSearchEngine) Name() string {
	return "Google"
}

func (g *GoogleSearchEngine) Capabilities() []string {
	capabilities := g.BaseSearchEngine.Capabilities()
	return append(capabilities, "knowledge_graph", "featured_snippets", "videos")
}

func (g *GoogleSearchEngine) Search(ctx context.Context, request SearchRequest) ([]SearchResult, error) {
	g.RespectRateLimit()
	g.SetDebugMode(request.Debug)

	var results []SearchResult

	searchURL := fmt.Sprintf("https://www.google.com/search?q=%s&num=%d&hl=%s&safe=off&pws=0",
		url.QueryEscape(request.Query),
		request.MaxResults+5, // Request more results than needed to account for filtering
		request.Language,
	)

	if request.Region != "" {
		searchURL += "&gl=" + request.Region
	}

	if request.Page > 1 {
		searchURL += fmt.Sprintf("&start=%d", (request.Page-1)*10)
	}

	// Add date range parameters if specified
	if !request.DateRange.Start.IsZero() && !request.DateRange.End.IsZero() {
		searchURL += fmt.Sprintf("&tbs=cdr:1,cd_min:%s,cd_max:%s",
			request.DateRange.Start.Format("01/02/2006"),
			request.DateRange.End.Format("01/02/2006"),
		)
	}

	// Add advanced query parameters
	for domain, value := range request.AdvancedQuery {
		if domain == "site" {
			searchURL += fmt.Sprintf("+site:%s", value)
		} else if domain == "filetype" {
			searchURL += fmt.Sprintf("+filetype:%s", value)
		}
	}

	// Add domain exclusions
	for _, domain := range request.ExcludeDomains {
		searchURL += fmt.Sprintf("+-site:%s", domain)
	}

	g.DebugLog("Search URL: %s", searchURL)

	if request.UseHeadless {
		return g.searchHeadless(ctx, searchURL, request)
	}

	// Set up custom HTTP client with improved configuration
	client := &http.Client{
		Timeout: request.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression:  false,
			ForceAttemptHTTP2:   true,
		},
	}

	// Configure proxy if specified
	if request.ProxyURL != "" {
		proxyURL, err := url.Parse(request.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}
		client.Transport = &http.Transport{
			Proxy:               http.ProxyURL(proxyURL),
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
		}
	}

	// Prepare HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	// Set browser-like headers
	userAgent := g.GetNextUserAgent()
	g.DebugLog("Using User-Agent: %s", userAgent)

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Cache-Control", "max-age=0")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("DNT", "1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Ch-Ua", "\"Chromium\";v=\"122\", \"Not(A:Brand\";v=\"24\", \"Google Chrome\";v=\"122\"")
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", "\"Windows\"")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")

	// Randomized wait time to seem more human-like
	time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		g.DebugLog("Non-200 response: %d", resp.StatusCode)
		return nil, fmt.Errorf("received non-200 response: %d", resp.StatusCode)
	}

	// Parse HTML response
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	if request.Debug {
		// Save HTML for debugging
		html, _ := doc.Html()
		err = os.WriteFile("google_debug.html", []byte(html), 0644)
		if err != nil {
			g.DebugLog("Failed to save debug HTML: %v", err)
		} else {
			g.DebugLog("Saved debug HTML to google_debug.html")
		}
	}

	// Check for captcha
	if g.checkForCaptcha(doc) {
		g.DebugLog("Captcha detected!")
		return nil, fmt.Errorf("captcha detected - try using the --headless option or a proxy")
	}

	// Check if we're getting search results
	if doc.Find(g.resultSelectors["container"]).Length() == 0 {
		g.DebugLog("No search result containers found using selector: %s", g.resultSelectors["container"])
		g.DebugLog("Trying alternate selectors...")

		// Try alternate selectors
		alternateSelectors := []string{
			"div.g",
			".MjjYud",
			"#search div[data-hveid]",
			"#rso > div",
			"#main div[data-header-feature]",
		}

		for _, selector := range alternateSelectors {
			g.DebugLog("Trying selector: %s", selector)
			count := doc.Find(selector).Length()
			g.DebugLog("Found %d elements with selector %s", count, selector)

			if count > 0 {
				g.DebugLog("Using alternate selector: %s", selector)
				g.resultSelectors["container"] = selector
				break
			}
		}
	}

	// Extract search results
	rank := 1
	g.DebugLog("Starting to extract results using container selector: %s", g.resultSelectors["container"])

	doc.Find(g.resultSelectors["container"]).Each(func(i int, s *goquery.Selection) {
		// Try to find title and skip if not found (likely not a result)
		titleEl := s.Find(g.resultSelectors["title"])
		if titleEl.Length() == 0 {
			g.DebugLog("No title found for result #%d, trying alternative selector", i+1)
			titleEl = s.Find("h3")
			if titleEl.Length() == 0 {
				g.DebugLog("Still no title found for result #%d, skipping", i+1)
				return
			}
		}

		title := titleEl.Text()
		if title == "" {
			g.DebugLog("Empty title for result #%d, skipping", i+1)
			return
		}

		// Try to find URL
		urlElement := s.Find(g.resultSelectors["url"])
		if urlElement.Length() == 0 {
			g.DebugLog("No URL element found for result #%d, trying alternative selector", i+1)
			urlElement = s.Find("a")
		}

		urlHref, exists := urlElement.Attr("href")
		if !exists || urlHref == "" {
			g.DebugLog("No href attribute found for result #%d, skipping", i+1)
			return
		}

		// Clean URL if it's a Google redirect
		if strings.HasPrefix(urlHref, "/url?") || strings.HasPrefix(urlHref, "/search?") {
			urlHref = extractGoogleRedirectURL(urlHref)
		} else if !strings.HasPrefix(urlHref, "http") {
			g.DebugLog("URL doesn't start with http: %s", urlHref)
			return
		}

		// Try to find description
		descElement := s.Find(g.resultSelectors["description"])
		if descElement.Length() == 0 {
			g.DebugLog("No description found for result #%d, trying alternative selector", i+1)
			descElement = s.Find("div[role='doc-subtitle'], .VwiC3b, span.st, .IsZvec, [data-content-feature='1']")
		}

		desc := descElement.Text()
		if desc == "" {
			desc = "No description available"
		}

		// Check if this is an ad
		isAd := g.IsAd(title) || g.IsAd(desc) || s.Find(g.resultSelectors["ad"]).Length() > 0

		// Skip ads if not requested
		if isAd && !request.IncludeAds {
			g.DebugLog("Skipping ad result: %s", title)
			return
		}

		// Apply word count filters
		wordCount := len(strings.Fields(desc))
		if request.MinWordCount > 0 && wordCount < request.MinWordCount {
			g.DebugLog("Skipping result with too few words (%d): %s", wordCount, title)
			return
		}
		if request.MaxWordCount > 0 && wordCount > request.MaxWordCount {
			g.DebugLog("Skipping result with too many words (%d): %s", wordCount, title)
			return
		}

		// Create search result
		result := SearchResult{
			Title:       title,
			URL:         urlHref,
			Description: desc,
			IsAd:        isAd,
			Rank:        rank,
			Keywords:    g.ExtractKeywords(desc, request.Query),
		}

		// Extract domain
		if parsedURL, err := url.Parse(urlHref); err == nil {
			result.Metadata.Domain = parsedURL.Hostname()
		}

		result.Metadata.FetchedAt = time.Now()

		// Identify result type
		if g.detectResultType(s, &result) {
			result.Metadata.ResultType = "special"
		} else {
			result.Metadata.ResultType = "organic"
		}

		g.DebugLog("Extracted result #%d: %s", rank, title)
		results = append(results, result)
		rank++

		// Stop if we have enough results
		if len(results) >= request.MaxResults {
			return
		}
	})

	g.DebugLog("Found %d results", len(results))

	// If still no results, dump a more detailed analysis
	if len(results) == 0 && request.Debug {
		g.DebugLog("No results found, performing selector analysis:")
		doc.Find("body *").Each(func(i int, s *goquery.Selection) {
			if i < 50 { // Just check the first 50 elements to avoid huge output
				tagName := goquery.NodeName(s)
				classes, _ := s.Attr("class")
				id, _ := s.Attr("id")
				if tagName == "div" && (classes != "" || id != "") {
					g.DebugLog("Element #%d: <%s class='%s' id='%s'>", i, tagName, classes, id)
				}
			}
		})

		// Save HTML for debugging
		html, _ := doc.Html()
		err = os.WriteFile("google_debug_empty.html", []byte(html), 0644)
		if err != nil {
			g.DebugLog("Failed to save debug HTML: %v", err)
		}
	}

	return results, nil
}

// Helper function to extract URL from Google's redirect
func extractGoogleRedirectURL(redirectURL string) string {
	if strings.Contains(redirectURL, "url?") {
		params, err := url.ParseQuery(strings.SplitN(redirectURL, "?", 2)[1])
		if err == nil && params.Get("q") != "" {
			return params.Get("q")
		}
	}

	// Fallback - try to extract with regex
	re := regexp.MustCompile(`[?&](?:q|url)=([^&]+)`)
	matches := re.FindStringSubmatch(redirectURL)
	if len(matches) > 1 {
		extractedURL, err := url.QueryUnescape(matches[1])
		if err == nil {
			return extractedURL
		}
	}

	return redirectURL
}

func (g *GoogleSearchEngine) detectResultType(s *goquery.Selection, result *SearchResult) bool {
	isSpecial := false

	if s.HasClass("kp-wholepage") || s.Find(".kp-wholepage").Length() > 0 {
		result.Metadata.SearchFeature = "knowledge_panel"
		isSpecial = true
	} else if s.HasClass("g-blk") || s.Find(".g-blk").Length() > 0 {
		result.Metadata.SearchFeature = "featured_snippet"
		isSpecial = true
	} else if s.HasClass("video-voyager") || s.Find(".video-voyager").Length() > 0 {
		result.Metadata.SearchFeature = "video"
		isSpecial = true
	} else if s.Find("g-review-stars, .PZPZlf").Length() > 0 {
		result.Metadata.SearchFeature = "review"
		isSpecial = true
	}

	return isSpecial
}

func (g *GoogleSearchEngine) checkForCaptcha(doc *goquery.Document) bool {
	// Check for captcha form
	if doc.Find("form#captcha-form, div.g-recaptcha, #recaptcha, body.captcha").Length() > 0 {
		return true
	}

	// Check for typical captcha text
	captchaText := doc.Find("body").Text()
	captchaPhrases := []string{
		"unusual traffic",
		"confirm you are a human",
		"verify you are a human",
		"automated system",
		"suspicious activity",
		"verify it's you",
	}

	for _, phrase := range captchaPhrases {
		if strings.Contains(strings.ToLower(captchaText), phrase) {
			return true
		}
	}

	return false
}

func (g *GoogleSearchEngine) searchHeadless(ctx context.Context, searchURL string, request SearchRequest) ([]SearchResult, error) {
	var results []SearchResult
	g.DebugLog("Starting headless search: %s", searchURL)

	// Configure ChromeDP options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-popup-blocking", true),
		chromedp.WindowSize(1920, 1080),
		chromedp.UserAgent(g.GetNextUserAgent()),
	)

	// Configure proxy if specified
	if request.ProxyURL != "" {
		opts = append(opts, chromedp.Flag("proxy-server", request.ProxyURL))
	}

	// Create new Chrome instance
	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	chromeCtx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	defer cancel()

	// Set timeout
	chromeCtx, cancel = context.WithTimeout(chromeCtx, request.Timeout)
	defer cancel()

	// Random wait to seem more human-like
	time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)

	g.DebugLog("Navigating to URL with Chrome")

	// Container for page HTML
	var res string

	// Execute Chrome actions
	err := chromedp.Run(chromeCtx,
		emulation.SetDeviceMetricsOverride(1920, 1080, 1.0, false),
		network.Enable(),
		network.SetExtraHTTPHeaders(network.Headers(map[string]interface{}{
			"Accept-Language": "en-US,en;q=0.9",
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8",
			"Cache-Control":   "max-age=0",
			"Connection":      "keep-alive",
			"DNT":             "1",
		})),

		// Randomized delays to mimic human behavior
		chromedp.Navigate(searchURL),
		chromedp.Sleep(time.Duration(rand.Intn(2000)+500)*time.Millisecond),

		// Scroll down slightly
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, exp, err := runtime.Evaluate(`window.scrollBy(0, ${Math.floor(Math.random() * 400) + 100});`).Do(ctx)
			if err != nil {
				return err
			}
			if exp != nil {
				return exp
			}
			return nil
		}),
		chromedp.Sleep(time.Duration(rand.Intn(1000)+500)*time.Millisecond),

		// Wait for search results to appear using various possible selectors
		chromedp.ActionFunc(func(ctx context.Context) error {
			selectors := []string{
				g.resultSelectors["container"],
				"#search .g",
				".MjjYud",
				"#rso > div",
				"h3",
				"#main",
			}

			for _, selector := range selectors {
				var nodes []*cdp.Node
				if err := chromedp.Nodes(selector, &nodes, chromedp.ByQueryAll).Do(ctx); err == nil && len(nodes) > 0 {
					g.DebugLog("Found %d nodes with selector: %s", len(nodes), selector)
					return nil
				}
			}

			return fmt.Errorf("no search result elements found")
		}),

		// Get the page HTML
		chromedp.OuterHTML("html", &res),
	)

	if err != nil {
		g.DebugLog("Chrome error: %v", err)
		return nil, fmt.Errorf("headless browser error: %w", err)
	}

	// Save debug HTML if requested
	if request.Debug {
		err = os.WriteFile("google_headless_debug.html", []byte(res), 0644)
		if err != nil {
			g.DebugLog("Failed to save debug HTML: %v", err)
		} else {
			g.DebugLog("Saved headless debug HTML to google_headless_debug.html")
		}
	}

	// Parse the HTML
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(res))
	if err != nil {
		return nil, err
	}

	// Extract results similar to non-headless version
	rank := 1

	// Try different selectors if needed
	containerSelectors := []string{
		g.resultSelectors["container"],
		"#search .g",
		".MjjYud",
		"#rso > div",
		"div[data-hveid]",
	}

	foundResults := false
	for _, containerSelector := range containerSelectors {
		g.DebugLog("Trying container selector: %s", containerSelector)
		resultCount := 0

		doc.Find(containerSelector).Each(func(i int, s *goquery.Selection) {
			// Extract title
			title := s.Find(g.resultSelectors["title"]).Text()
			if title == "" {
				title = s.Find("h3").Text()
			}
			if title == "" {
				return
			}

			// Extract URL
			urlElement := s.Find(g.resultSelectors["url"])
			if urlElement.Length() == 0 {
				urlElement = s.Find("a")
			}

			urlHref, exists := urlElement.Attr("href")
			if !exists || urlHref == "" {
				return
			}

			// Clean URL
			if strings.HasPrefix(urlHref, "/url?") || strings.HasPrefix(urlHref, "/search?") {
				urlHref = extractGoogleRedirectURL(urlHref)
			} else if !strings.HasPrefix(urlHref, "http") {
				return
			}

			// Extract description
			descElement := s.Find(g.resultSelectors["description"])
			if descElement.Length() == 0 {
				descElement = s.Find("div[role='doc-subtitle'], .VwiC3b, span.st, .IsZvec")
			}

			desc := descElement.Text()
			if desc == "" {
				desc = "No description available"
			}

			// Check if this is an ad
			isAd := g.IsAd(title) || g.IsAd(desc)

			// Skip ads if not requested
			if isAd && !request.IncludeAds {
				return
			}

			// Apply word count filters
			wordCount := len(strings.Fields(desc))
			if request.MinWordCount > 0 && wordCount < request.MinWordCount {
				return
			}
			if request.MaxWordCount > 0 && wordCount > request.MaxWordCount {
				return
			}

			// Create result
			result := SearchResult{
				Title:       title,
				URL:         urlHref,
				Description: desc,
				IsAd:        isAd,
				Rank:        rank,
				Keywords:    g.ExtractKeywords(desc, request.Query),
			}

			// Extract domain
			if parsedURL, err := url.Parse(urlHref); err == nil {
				result.Metadata.Domain = parsedURL.Hostname()
			}

			result.Metadata.FetchedAt = time.Now()

			// Set result type
			if g.detectResultType(s, &result) {
				result.Metadata.ResultType = "special"
			} else {
				result.Metadata.ResultType = "organic"
			}

			results = append(results, result)
			rank++
			resultCount++

			// Stop if we have enough results
			if len(results) >= request.MaxResults {
				return
			}
		})

		// If we found results with this selector, break the loop
		if resultCount > 0 {
			g.DebugLog("Found %d results with selector: %s", resultCount, containerSelector)
			foundResults = true
			break
		}
	}

	if !foundResults && request.Debug {
		// Try to find any links that might be results
		g.DebugLog("No results found with standard selectors, trying fallback approach")

		doc.Find("a").Each(func(i int, s *goquery.Selection) {
			// Only process if we don't have enough results yet
			if len(results) >= request.MaxResults {
				return
			}

			href, exists := s.Attr("href")
			if !exists || href == "" || !strings.HasPrefix(href, "http") {
				return
			}

			// Skip obvious non-results
			if strings.Contains(href, "google.com") {
				return
			}

			title := s.Text()
			if title == "" {
				title = href
			}

			parentText := s.Parent().Text()
			desc := strings.TrimSpace(strings.Replace(parentText, title, "", 1))
			if len(desc) > 300 {
				desc = desc[:300] + "..."
			}

			result := SearchResult{
				Title:       title,
				URL:         href,
				Description: desc,
				IsAd:        false,
				Rank:        rank,
				Keywords:    g.ExtractKeywords(desc, request.Query),
			}

			if parsedURL, err := url.Parse(href); err == nil {
				result.Metadata.Domain = parsedURL.Hostname()
			}

			result.Metadata.FetchedAt = time.Now()
			result.Metadata.ResultType = "fallback"

			results = append(results, result)
			rank++
		})

		g.DebugLog("Found %d results with fallback approach", len(results))
	}

	return results, nil
}

type BingSearchEngine struct {
	*BaseSearchEngine
}

func NewBingSearchEngine() *BingSearchEngine {
	base := NewBaseSearchEngine()

	// Updated selectors for Bing's current DOM structure
	base.resultSelectors = map[string]string{
		"container":   "li.b_algo",
		"title":       "h2",
		"url":         "cite",
		"description": "div.b_caption p",
		"deeplink":    "ul.b_deeplinks_expand",
	}

	return &BingSearchEngine{
		BaseSearchEngine: base,
	}
}

func (b *BingSearchEngine) Name() string {
	return "Bing"
}

func (b *BingSearchEngine) Capabilities() []string {
	capabilities := b.BaseSearchEngine.Capabilities()
	return append(capabilities, "deeplinks", "entity_info")
}

func (b *BingSearchEngine) Search(ctx context.Context, request SearchRequest) ([]SearchResult, error) {
	b.RespectRateLimit()
	b.SetDebugMode(request.Debug)

	var results []SearchResult

	searchURL := fmt.Sprintf("https://www.bing.com/search?q=%s&count=%d&setlang=%s",
		url.QueryEscape(request.Query),
		request.MaxResults,
		request.Language,
	)

	if request.Region != "" {
		searchURL += "&cc=" + request.Region
	}

	if request.Page > 1 {
		searchURL += fmt.Sprintf("&first=%d", (request.Page-1)*10+1)
	}

	b.DebugLog("Search URL: %s", searchURL)

	if request.UseHeadless {
		return b.searchHeadless(ctx, searchURL, request)
	}

	client := &http.Client{
		Timeout: request.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression:  false,
			ForceAttemptHTTP2:   true,
		},
	}

	if request.ProxyURL != "" {
		proxyURL, err := url.Parse(request.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}
		client.Transport = &http.Transport{
			Proxy:               http.ProxyURL(proxyURL),
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	userAgent := b.GetNextUserAgent()
	b.DebugLog("Using User-Agent: %s", userAgent)

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Cache-Control", "max-age=0")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Ch-Ua", "\"Chromium\";v=\"122\", \"Not(A:Brand\";v=\"24\", \"Google Chrome\";v=\"122\"")
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", "\"Windows\"")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")

	time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b.DebugLog("Non-200 response: %d", resp.StatusCode)
		return nil, fmt.Errorf("received non-200 response: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	if request.Debug {
		html, _ := doc.Html()
		err = os.WriteFile("bing_debug.html", []byte(html), 0644)
		if err != nil {
			b.DebugLog("Failed to save debug HTML: %v", err)
		} else {
			b.DebugLog("Saved debug HTML to bing_debug.html")
		}
	}

	rank := 1

	doc.Find(b.resultSelectors["container"]).Each(func(i int, s *goquery.Selection) {
		titleEl := s.Find(b.resultSelectors["title"])
		if titleEl.Length() == 0 {
			return
		}

		title := titleEl.Text()

		urlEl := s.Find(b.resultSelectors["url"])
		if urlEl.Length() == 0 {
			urlEl = s.Find("a").First()
		}

		var urlText string
		urlHref, exists := urlEl.Attr("href")
		if exists && urlHref != "" {
			urlText = urlHref
		} else {
			urlText = urlEl.Text()
			if !strings.HasPrefix(urlText, "http") {
				urlText = "https://" + urlText
			}
		}

		descEl := s.Find(b.resultSelectors["description"])
		if descEl.Length() == 0 {
			descEl = s.Find("p")
		}

		desc := descEl.Text()
		if desc == "" {
			desc = "No description available"
		}

		isAd := b.IsAd(title) || b.IsAd(desc)

		if isAd && !request.IncludeAds {
			return
		}

		wordCount := len(strings.Fields(desc))
		if request.MinWordCount > 0 && wordCount < request.MinWordCount {
			return
		}
		if request.MaxWordCount > 0 && wordCount > request.MaxWordCount {
			return
		}

		result := SearchResult{
			Title:       title,
			URL:         urlText,
			Description: desc,
			IsAd:        isAd,
			Rank:        rank,
			Keywords:    b.ExtractKeywords(desc, request.Query),
		}

		if parsedURL, err := url.Parse(urlText); err == nil {
			result.Metadata.Domain = parsedURL.Hostname()
		}

		result.Metadata.FetchedAt = time.Now()

		deeplinks := b.extractDeeplinks(s)
		if len(deeplinks) > 0 {
			result.Metadata.ResultType = "with_deeplinks"
		} else {
			result.Metadata.ResultType = "organic"
		}

		results = append(results, result)
		rank++

		if len(results) >= request.MaxResults {
			return
		}
	})

	b.DebugLog("Found %d results", len(results))

	return results, nil
}

func (b *BingSearchEngine) extractDeeplinks(s *goquery.Selection) []string {
	var links []string
	s.Find(b.resultSelectors["deeplink"]).Find("li a").Each(func(i int, s *goquery.Selection) {
		link, exists := s.Attr("href")
		if exists {
			links = append(links, link)
		}
	})
	return links
}

func (b *BingSearchEngine) searchHeadless(ctx context.Context, searchURL string, request SearchRequest) ([]SearchResult, error) {
	var results []SearchResult
	b.DebugLog("Starting headless search: %s", searchURL)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.WindowSize(1920, 1080),
		chromedp.UserAgent(b.GetNextUserAgent()),
	)

	if request.ProxyURL != "" {
		opts = append(opts, chromedp.Flag("proxy-server", request.ProxyURL))
	}

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	chromeCtx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	defer cancel()

	chromeCtx, cancel = context.WithTimeout(chromeCtx, request.Timeout)
	defer cancel()

	time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)

	var res string
	err := chromedp.Run(chromeCtx,
		emulation.SetDeviceMetricsOverride(1920, 1080, 1.0, false),
		network.Enable(),
		network.SetExtraHTTPHeaders(network.Headers(map[string]interface{}{
			"Accept-Language": "en-US,en;q=0.9",
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8",
			"Cache-Control":   "max-age=0",
			"Connection":      "keep-alive",
		})),
		chromedp.Navigate(searchURL),
		chromedp.Sleep(time.Duration(rand.Intn(2000)+500)*time.Millisecond),
		chromedp.WaitVisible(b.resultSelectors["container"], chromedp.ByQuery),
		chromedp.OuterHTML("html", &res),
	)

	if err != nil {
		b.DebugLog("Chrome error: %v", err)
		return nil, err
	}

	if request.Debug {
		err = os.WriteFile("bing_headless_debug.html", []byte(res), 0644)
		if err != nil {
			b.DebugLog("Failed to save debug HTML: %v", err)
		} else {
			b.DebugLog("Saved headless debug HTML to bing_headless_debug.html")
		}
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(res))
	if err != nil {
		return nil, err
	}

	rank := 1

	doc.Find(b.resultSelectors["container"]).Each(func(i int, s *goquery.Selection) {
		title := s.Find(b.resultSelectors["title"]).Text()

		urlEl := s.Find(b.resultSelectors["url"])
		if urlEl.Length() == 0 {
			urlEl = s.Find("a").First()
		}

		var urlText string
		urlHref, exists := urlEl.Attr("href")
		if exists && urlHref != "" {
			urlText = urlHref
		} else {
			urlText = urlEl.Text()
			if !strings.HasPrefix(urlText, "http") {
				urlText = "https://" + urlText
			}
		}

		desc := s.Find(b.resultSelectors["description"]).Text()
		if desc == "" {
			desc = "No description available"
		}

		isAd := b.IsAd(title) || b.IsAd(desc)

		if isAd && !request.IncludeAds {
			return
		}

		wordCount := len(strings.Fields(desc))
		if request.MinWordCount > 0 && wordCount < request.MinWordCount {
			return
		}
		if request.MaxWordCount > 0 && wordCount > request.MaxWordCount {
			return
		}

		result := SearchResult{
			Title:       title,
			URL:         urlText,
			Description: desc,
			IsAd:        isAd,
			Rank:        rank,
			Keywords:    b.ExtractKeywords(desc, request.Query),
		}

		if parsedURL, err := url.Parse(urlText); err == nil {
			result.Metadata.Domain = parsedURL.Hostname()
		}

		result.Metadata.FetchedAt = time.Now()

		deeplinks := b.extractDeeplinks(s)
		if len(deeplinks) > 0 {
			result.Metadata.ResultType = "with_deeplinks"
		} else {
			result.Metadata.ResultType = "organic"
		}

		results = append(results, result)
		rank++

		if len(results) >= request.MaxResults {
			return
		}
	})

	b.DebugLog("Found %d results", len(results))

	return results, nil
}

type DuckDuckGoSearchEngine struct {
	*BaseSearchEngine
}

func NewDuckDuckGoSearchEngine() *DuckDuckGoSearchEngine {
	base := NewBaseSearchEngine()

	// Updated selectors for DDG
	base.resultSelectors = map[string]string{
		"container":   ".result, article.result, .web-result",
		"title":       "h2, .result__title, .result__a",
		"url":         ".result__url, a.result__url, a.result__a",
		"description": ".result__snippet, .result__snippet-truncate",
	}

	return &DuckDuckGoSearchEngine{
		BaseSearchEngine: base,
	}
}

func (d *DuckDuckGoSearchEngine) Name() string {
	return "DuckDuckGo"
}

func (d *DuckDuckGoSearchEngine) Capabilities() []string {
	capabilities := d.BaseSearchEngine.Capabilities()
	return append(capabilities, "privacy_focused", "instant_answers")
}

func (d *DuckDuckGoSearchEngine) Search(ctx context.Context, request SearchRequest) ([]SearchResult, error) {
	d.RespectRateLimit()
	d.SetDebugMode(request.Debug)

	// DDG's HTML SERP for scraping
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(request.Query))

	d.DebugLog("Search URL: %s", searchURL)

	if request.UseHeadless {
		return d.searchHeadless(ctx, searchURL, request)
	}

	client := &http.Client{
		Timeout: request.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression:  false,
			ForceAttemptHTTP2:   true,
		},
	}

	if request.ProxyURL != "" {
		proxyURL, err := url.Parse(request.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}
		client.Transport = &http.Transport{
			Proxy:               http.ProxyURL(proxyURL),
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	userAgent := d.GetNextUserAgent()
	d.DebugLog("Using User-Agent: %s", userAgent)

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")

	time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		d.DebugLog("Non-200 response: %d", resp.StatusCode)
		return nil, fmt.Errorf("received non-200 response: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	if request.Debug {
		html, _ := doc.Html()
		err = os.WriteFile("ddg_debug.html", []byte(html), 0644)
		if err != nil {
			d.DebugLog("Failed to save debug HTML: %v", err)
		} else {
			d.DebugLog("Saved debug HTML to ddg_debug.html")
		}
	}

	var results []SearchResult
	rank := 1

	doc.Find(d.resultSelectors["container"]).Each(func(i int, s *goquery.Selection) {
		titleEl := s.Find(d.resultSelectors["title"])
		if titleEl.Length() == 0 {
			titleEl = s.Find("a.result__a")
		}

		title := titleEl.Text()
		if title == "" {
			return
		}

		urlEl := s.Find(d.resultSelectors["url"])
		if urlEl.Length() == 0 {
			urlEl = titleEl
		}

		urlHref, exists := urlEl.Attr("href")
		if !exists {
			return
		}

		// DDG uses redirects
		if strings.Contains(urlHref, "duckduckgo.com/l/?") {
			parsed, err := url.Parse(urlHref)
			if err == nil {
				if redirectURL := parsed.Query().Get("uddg"); redirectURL != "" {
					urlHref = redirectURL
				}
			}
		}

		descEl := s.Find(d.resultSelectors["description"])
		desc := descEl.Text()
		if desc == "" {
			desc = "No description available"
		}

		isAd := d.IsAd(title) || d.IsAd(desc)

		if isAd && !request.IncludeAds {
			return
		}

		wordCount := len(strings.Fields(desc))
		if request.MinWordCount > 0 && wordCount < request.MinWordCount {
			return
		}
		if request.MaxWordCount > 0 && wordCount > request.MaxWordCount {
			return
		}

		result := SearchResult{
			Title:       title,
			URL:         urlHref,
			Description: desc,
			IsAd:        isAd,
			Rank:        rank,
			Keywords:    d.ExtractKeywords(desc, request.Query),
		}

		if parsedURL, err := url.Parse(urlHref); err == nil {
			result.Metadata.Domain = parsedURL.Hostname()
		}

		result.Metadata.FetchedAt = time.Now()
		result.Metadata.ResultType = "organic"

		results = append(results, result)
		rank++

		if len(results) >= request.MaxResults {
			return
		}
	})

	d.DebugLog("Found %d results", len(results))

	return results, nil
}

func (d *DuckDuckGoSearchEngine) searchHeadless(ctx context.Context, searchURL string, request SearchRequest) ([]SearchResult, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-web-security", true),
		chromedp.WindowSize(1920, 1080),
		chromedp.UserAgent(d.GetNextUserAgent()),
	)

	if request.ProxyURL != "" {
		opts = append(opts, chromedp.Flag("proxy-server", request.ProxyURL))
	}

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	chromeCtx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	defer cancel()

	chromeCtx, cancel = context.WithTimeout(chromeCtx, request.Timeout)
	defer cancel()

	time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)

	var res string
	err := chromedp.Run(chromeCtx,
		emulation.SetDeviceMetricsOverride(1920, 1080, 1.0, false),
		network.Enable(),
		network.SetExtraHTTPHeaders(network.Headers(map[string]interface{}{
			"Accept-Language": "en-US,en;q=0.9",
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
			"Cache-Control":   "max-age=0",
			"Connection":      "keep-alive",
		})),
		chromedp.Navigate(searchURL),
		chromedp.Sleep(time.Duration(rand.Intn(2000)+500)*time.Millisecond),
		chromedp.WaitVisible(d.resultSelectors["container"], chromedp.ByQuery),
		chromedp.OuterHTML("html", &res),
	)

	if err != nil {
		d.DebugLog("Chrome error: %v", err)
		return nil, err
	}

	if request.Debug {
		err = os.WriteFile("ddg_headless_debug.html", []byte(res), 0644)
		if err != nil {
			d.DebugLog("Failed to save debug HTML: %v", err)
		} else {
			d.DebugLog("Saved headless debug HTML to ddg_headless_debug.html")
		}
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(res))
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	rank := 1

	doc.Find(d.resultSelectors["container"]).Each(func(i int, s *goquery.Selection) {
		titleEl := s.Find(d.resultSelectors["title"])
		if titleEl.Length() == 0 {
			titleEl = s.Find("a.result__a")
		}

		title := titleEl.Text()
		if title == "" {
			return
		}

		urlEl := s.Find(d.resultSelectors["url"])
		if urlEl.Length() == 0 {
			urlEl = titleEl
		}

		urlHref, exists := urlEl.Attr("href")
		if !exists {
			return
		}

		// DDG uses redirects
		if strings.Contains(urlHref, "duckduckgo.com/l/?") {
			parsed, err := url.Parse(urlHref)
			if err == nil {
				if redirectURL := parsed.Query().Get("uddg"); redirectURL != "" {
					urlHref = redirectURL
				}
			}
		}

		descEl := s.Find(d.resultSelectors["description"])
		desc := descEl.Text()
		if desc == "" {
			desc = "No description available"
		}

		isAd := d.IsAd(title) || d.IsAd(desc)

		if isAd && !request.IncludeAds {
			return
		}

		wordCount := len(strings.Fields(desc))
		if request.MinWordCount > 0 && wordCount < request.MinWordCount {
			return
		}
		if request.MaxWordCount > 0 && wordCount > request.MaxWordCount {
			return
		}

		result := SearchResult{
			Title:       title,
			URL:         urlHref,
			Description: desc,
			IsAd:        isAd,
			Rank:        rank,
			Keywords:    d.ExtractKeywords(desc, request.Query),
		}

		if parsedURL, err := url.Parse(urlHref); err == nil {
			result.Metadata.Domain = parsedURL.Hostname()
		}

		result.Metadata.FetchedAt = time.Now()
		result.Metadata.ResultType = "organic"

		results = append(results, result)
		rank++

		if len(results) >= request.MaxResults {
			return
		}
	})

	d.DebugLog("Found %d results", len(results))

	return results, nil
}

type SearchManager struct {
	engines map[string]SearchEngine
	mutex   sync.RWMutex
	metrics struct {
		TotalSearches      int64
		SuccessfulSearches int64
		FailedSearches     int64
	}
}

func NewSearchManager() *SearchManager {
	return &SearchManager{
		engines: make(map[string]SearchEngine),
	}
}

func (m *SearchManager) RegisterEngine(engine SearchEngine) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.engines[strings.ToLower(engine.Name())] = engine
}

func (m *SearchManager) GetEngine(name string) (SearchEngine, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	engine, ok := m.engines[strings.ToLower(name)]
	return engine, ok
}

func (m *SearchManager) GetAvailableEngines() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	engines := make([]string, 0, len(m.engines))
	for name := range m.engines {
		engines = append(engines, name)
	}
	return engines
}

func (m *SearchManager) SearchAll(ctx context.Context, request SearchRequest) (map[string][]SearchResult, error) {
	results := make(map[string][]SearchResult)
	errorMap := make(map[string]error)
	var mu sync.Mutex

	g, ctx := errgroup.WithContext(ctx)

	m.mutex.RLock()
	engines := make([]SearchEngine, 0, len(m.engines))
	for _, engine := range m.engines {
		engines = append(engines, engine)
	}
	m.mutex.RUnlock()

	for _, engine := range engines {
		engine := engine
		g.Go(func() error {
			engineName := engine.Name()
			engineResults, err := engine.Search(ctx, request)

			mu.Lock()
			m.metrics.TotalSearches++
			if err != nil {
				m.metrics.FailedSearches++
				errorMap[engineName] = err
			} else {
				m.metrics.SuccessfulSearches++
				results[engineName] = engineResults
			}
			mu.Unlock()

			return nil
		})
	}

	_ = g.Wait()

	if len(results) == 0 && len(errorMap) > 0 {
		var errMsgs []string
		for engine, err := range errorMap {
			errMsgs = append(errMsgs, fmt.Sprintf("%s: %v", engine, err))
		}
		return nil, fmt.Errorf("all searches failed: %s", strings.Join(errMsgs, "; "))
	}

	return results, nil
}

func (m *SearchManager) Deduplicate(results map[string][]SearchResult) []SearchResult {
	seen := make(map[string]bool)
	deduplicated := make([]SearchResult, 0)

	type engineResult struct {
		engine string
		result SearchResult
	}

	allResults := make([]engineResult, 0)
	for engine, engineResults := range results {
		for _, result := range engineResults {
			allResults = append(allResults, engineResult{engine, result})
		}
	}

	for _, er := range allResults {
		normalizedURL := strings.ToLower(er.result.URL)
		normalizedURL = strings.TrimPrefix(normalizedURL, "http://")
		normalizedURL = strings.TrimPrefix(normalizedURL, "https://")
		normalizedURL = strings.TrimPrefix(normalizedURL, "www.")
		normalizedURL = strings.TrimSuffix(normalizedURL, "/")

		if !seen[normalizedURL] {
			seen[normalizedURL] = true
			er.result.Metadata.ContentType = er.engine
			deduplicated = append(deduplicated, er.result)
		}
	}

	return deduplicated
}

func (m *SearchManager) GetMetrics() map[string]int64 {
	return map[string]int64{
		"total_searches":      m.metrics.TotalSearches,
		"successful_searches": m.metrics.SuccessfulSearches,
		"failed_searches":     m.metrics.FailedSearches,
	}
}

type ResultFilter interface {
	Apply(results []SearchResult) []SearchResult
}

type DomainFilter struct {
	Domain    string
	Inclusive bool
}

func (f *DomainFilter) Apply(results []SearchResult) []SearchResult {
	filtered := make([]SearchResult, 0)

	for _, result := range results {
		include := !f.Inclusive

		if strings.Contains(result.Metadata.Domain, f.Domain) {
			include = f.Inclusive
		}

		if include {
			filtered = append(filtered, result)
		}
	}

	return filtered
}

type KeywordFilter struct {
	Keyword string
}

func (f *KeywordFilter) Apply(results []SearchResult) []SearchResult {
	filtered := make([]SearchResult, 0)
	keyword := strings.ToLower(f.Keyword)

	for _, result := range results {
		found := false
		for _, k := range result.Keywords {
			if strings.Contains(strings.ToLower(k), keyword) {
				found = true
				break
			}
		}

		if found {
			filtered = append(filtered, result)
		}
	}

	return filtered
}

type ResultTypeFilter struct {
	ResultType string
}

func (f *ResultTypeFilter) Apply(results []SearchResult) []SearchResult {
	filtered := make([]SearchResult, 0)

	for _, result := range results {
		if result.Metadata.ResultType == f.ResultType {
			filtered = append(filtered, result)
		}
	}

	return filtered
}

type WordCountFilter struct {
	Min int
	Max int
}

func (f *WordCountFilter) Apply(results []SearchResult) []SearchResult {
	filtered := make([]SearchResult, 0)

	for _, result := range results {
		wordCount := len(strings.Fields(result.Description))

		if (f.Min <= 0 || wordCount >= f.Min) && (f.Max <= 0 || wordCount <= f.Max) {
			filtered = append(filtered, result)
		}
	}

	return filtered
}

type CompositeFilter struct {
	Filters []ResultFilter
}

func (f *CompositeFilter) Apply(results []SearchResult) []SearchResult {
	for _, filter := range f.Filters {
		results = filter.Apply(results)
	}
	return results
}

type CLIConfig struct {
	Query          string
	Engine         string
	MaxResults     int
	IncludeAds     bool
	Timeout        time.Duration
	ProxyURL       string
	UseHeadless    bool
	Language       string
	Region         string
	Format         string
	OutputFile     string
	Page           int
	MinWordCount   int
	MaxWordCount   int
	Filters        []ResultFilter
	AdvancedQuery  map[string]string
	ExcludeDomains []string
	Verbose        bool
	LogFile        string
	StatsFile      string
	Debug          bool
}

func parseFlags() CLIConfig {
	query := flag.String("query", "", "Search query")
	engine := flag.String("engine", "google", "Search engine (google, bing, duckduckgo, all)")
	maxResults := flag.Int("max", 10, "Maximum results to fetch")
	includeAds := flag.Bool("ads", false, "Include advertisements in results")
	timeout := flag.Duration("timeout", 30*time.Second, "Search timeout")
	proxyURL := flag.String("proxy", "", "Proxy URL")
	useHeadless := flag.Bool("headless", false, "Use headless browser")
	language := flag.String("lang", "en", "Language code")
	region := flag.String("region", "us", "Region code")
	format := flag.String("format", "json", "Output format (json, csv, table)")
	outputFile := flag.String("output", "", "Output file (default: stdout)")
	page := flag.Int("page", 1, "Result page number")
	minWords := flag.Int("min-words", 0, "Minimum word count in description")
	maxWords := flag.Int("max-words", 0, "Maximum word count in description")
	domain := flag.String("domain", "", "Filter results by domain (include)")
	excludeDomain := flag.String("exclude-domain", "", "Filter results by domain (exclude)")
	keyword := flag.String("keyword", "", "Filter results by keyword")
	resultType := flag.String("type", "", "Filter by result type (organic, special, etc.)")
	site := flag.String("site", "", "Limit results to specific site")
	filetype := flag.String("filetype", "", "Limit results to specific file type")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	debug := flag.Bool("debug", false, "Enable debug mode (saves HTML responses)")
	logFile := flag.String("log", "", "Log file path")
	statsFile := flag.String("stats", "", "Statistics output file")

	flag.Parse()

	if *query == "" && flag.NArg() > 0 {
		*query = flag.Arg(0)
	}

	advancedQuery := make(map[string]string)
	if *site != "" {
		advancedQuery["site"] = *site
	}
	if *filetype != "" {
		advancedQuery["filetype"] = *filetype
	}

	excludeDomains := []string{}
	if *excludeDomain != "" {
		for _, domain := range strings.Split(*excludeDomain, ",") {
			excludeDomains = append(excludeDomains, strings.TrimSpace(domain))
		}
	}

	var filters []ResultFilter
	if *domain != "" {
		filters = append(filters, &DomainFilter{Domain: *domain, Inclusive: true})
	}
	if *keyword != "" {
		filters = append(filters, &KeywordFilter{Keyword: *keyword})
	}
	if *resultType != "" {
		filters = append(filters, &ResultTypeFilter{ResultType: *resultType})
	}
	if *minWords > 0 || *maxWords > 0 {
		filters = append(filters, &WordCountFilter{Min: *minWords, Max: *maxWords})
	}

	return CLIConfig{
		Query:          *query,
		Engine:         *engine,
		MaxResults:     *maxResults,
		IncludeAds:     *includeAds,
		Timeout:        *timeout,
		ProxyURL:       *proxyURL,
		UseHeadless:    *useHeadless,
		Language:       *language,
		Region:         *region,
		Format:         *format,
		OutputFile:     *outputFile,
		Page:           *page,
		MinWordCount:   *minWords,
		MaxWordCount:   *maxWords,
		Filters:        filters,
		AdvancedQuery:  advancedQuery,
		ExcludeDomains: excludeDomains,
		Verbose:        *verbose,
		LogFile:        *logFile,
		StatsFile:      *statsFile,
		Debug:          *debug,
	}
}

func setupLogging(config CLIConfig) *log.Logger {
	var logWriter *os.File
	var err error

	if config.LogFile != "" {
		logWriter, err = os.OpenFile(config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not open log file: %v. Using stderr.\n", err)
			logWriter = os.Stderr
		}
	} else {
		logWriter = os.Stderr
	}

	logFlags := log.Ldate | log.Ltime
	if config.Verbose {
		logFlags |= log.Lshortfile
	}

	return log.New(logWriter, "", logFlags)
}

func printResults(results []SearchResult, format string, outputFile string) error {
	var output string

	switch format {
	case "json":
		jsonData, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return err
		}
		output = string(jsonData)

	case "csv":
		output = "Rank,Title,URL,Description,IsAd,Domain,FetchedAt,ResultType\n"
		for _, result := range results {
			output += fmt.Sprintf("%d,\"%s\",\"%s\",\"%s\",%t,%s,%s,%s\n",
				result.Rank,
				strings.ReplaceAll(result.Title, "\"", "\"\""),
				result.URL,
				strings.ReplaceAll(result.Description, "\"", "\"\""),
				result.IsAd,
				result.Metadata.Domain,
				result.Metadata.FetchedAt.Format(time.RFC3339),
				result.Metadata.ResultType,
			)
		}

	case "table":
		titleColor := color.New(color.FgCyan, color.Bold).SprintFunc()
		urlColor := color.New(color.FgGreen).SprintFunc()
		adColor := color.New(color.FgRed, color.Bold).SprintFunc()
		typeColor := color.New(color.FgYellow).SprintFunc()

		for _, result := range results {
			output += fmt.Sprintf("%d. %s\n", result.Rank, titleColor(result.Title))
			output += fmt.Sprintf("   %s\n", urlColor(result.URL))
			output += fmt.Sprintf("   %s\n", result.Description)

			typeInfo := ""
			if result.Metadata.ResultType != "" && result.Metadata.ResultType != "organic" {
				typeInfo = fmt.Sprintf(" [%s]", typeColor(result.Metadata.ResultType))
			}

			adInfo := ""
			if result.IsAd {
				adInfo = fmt.Sprintf(" %s", adColor("Advertisement"))
			}

			if typeInfo != "" || adInfo != "" {
				output += fmt.Sprintf("   %s%s\n", typeInfo, adInfo)
			}

			output += "\n"
		}

	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}

	if outputFile != "" {
		return os.WriteFile(outputFile, []byte(output), 0644)
	}

	fmt.Println(output)
	return nil
}

func printStatistics(manager *SearchManager, statsFile string) error {
	metrics := manager.GetMetrics()

	stats := map[string]interface{}{
		"timestamp":           time.Now().Format(time.RFC3339),
		"total_searches":      metrics["total_searches"],
		"successful_searches": metrics["successful_searches"],
		"failed_searches":     metrics["failed_searches"],
		"success_rate":        float64(metrics["successful_searches"]) / float64(metrics["total_searches"]),
	}

	statsJson, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return err
	}

	if statsFile != "" {
		return os.WriteFile(statsFile, statsJson, 0644)
	}

	return nil
}

func applyFilters(results []SearchResult, filters []ResultFilter) []SearchResult {
	for _, filter := range filters {
		results = filter.Apply(results)
	}

	return results
}

func welcome() {
	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│ Search Engine Scraper - Golang v1.0 │")
	fmt.Println("└─────────────────────────────────────┘")
	fmt.Println("For help and options, use --help flag")
	fmt.Println()
}

func main() {
	welcome()

	// Set random seed
	rand.Seed(time.Now().UnixNano())

	config := parseFlags()

	logger := setupLogging(config)

	if config.Query == "" {
		fmt.Println("Error: Search query is required")
		flag.Usage()
		os.Exit(1)
	}

	manager := NewSearchManager()

	manager.RegisterEngine(NewGoogleSearchEngine())
	manager.RegisterEngine(NewBingSearchEngine())
	manager.RegisterEngine(NewDuckDuckGoSearchEngine())

	request := SearchRequest{
		Query:          config.Query,
		MaxResults:     config.MaxResults,
		IncludeAds:     config.IncludeAds,
		Timeout:        config.Timeout,
		ProxyURL:       config.ProxyURL,
		UseHeadless:    config.UseHeadless,
		Language:       config.Language,
		Region:         config.Region,
		Page:           config.Page,
		AdvancedQuery:  config.AdvancedQuery,
		ExcludeDomains: config.ExcludeDomains,
		MinWordCount:   config.MinWordCount,
		MaxWordCount:   config.MaxWordCount,
		Debug:          config.Debug,
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	var results []SearchResult
	var err error

	startTime := time.Now()

	if strings.ToLower(config.Engine) == "all" {
		logger.Println("Searching with all engines...")
		allResults, err := manager.SearchAll(ctx, request)
		if err != nil {
			logger.Printf("Error during search: %v\n", err)
		}

		results = manager.Deduplicate(allResults)
		logger.Printf("Found %d unique results across all engines\n", len(results))
	} else {
		engine, ok := manager.GetEngine(config.Engine)
		if !ok {
			logger.Printf("Unknown search engine '%s'\n", config.Engine)
			fmt.Fprintf(os.Stderr, "Error: Unknown search engine '%s'\n", config.Engine)
			fmt.Fprintf(os.Stderr, "Available engines: %s\n", strings.Join(manager.GetAvailableEngines(), ", "))
			os.Exit(1)
		}

		logger.Printf("Searching with %s engine...\n", engine.Name())
		request.Engine = engine.Name()
		results, err = engine.Search(ctx, request)
		if err != nil {
			logger.Printf("Error during search: %v\n", err)
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		logger.Printf("Found %d results\n", len(results))
	}

	if len(config.Filters) > 0 {
		logger.Println("Applying filters...")
		beforeCount := len(results)
		results = applyFilters(results, config.Filters)
		logger.Printf("Filtered from %d to %d results\n", beforeCount, len(results))
	}

	elapsedTime := time.Since(startTime)
	logger.Printf("Search completed in %v\n", elapsedTime)

	if config.Verbose {
		fmt.Printf("Search completed in %v\n", elapsedTime)
		fmt.Printf("Found %d results\n", len(results))
	}

	if config.StatsFile != "" {
		if err := printStatistics(manager, config.StatsFile); err != nil {
			logger.Printf("Error writing statistics: %v\n", err)
		}
	}

	if err := printResults(results, config.Format, config.OutputFile); err != nil {
		logger.Printf("Error printing results: %v\n", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	logger.Println("Search operation completed successfully")
}
