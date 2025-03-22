# GoSearch: Advanced Search Engine Scraper

<p align="center">
  <img src="https://raw.githubusercontent.com/advanced-search-engine/gosearch/main/assets/logo.png" alt="GoSearch Logo" width="200"/>
  <br>
  <em>High-performance search engine results scraper written in Go</em>
</p>

<p align="center">
  <a href="#features">Features</a> •
  <a href="#installation">Installation</a> •
  <a href="#usage">Usage</a> •
  <a href="#examples">Examples</a> •
  <a href="#advanced-usage">Advanced Usage</a> •
  <a href="#troubleshooting">Troubleshooting</a> •
  <a href="#contributing">Contributing</a>
</p>

<p align="center">
  <img src="https://raw.githubusercontent.com/advanced-search-engine/gosearch/main/assets/demo.gif" alt="GoSearch Demo" width="600"/>
</p>

## Features

- **Multi-Engine Support**: Search Google, Bing, and DuckDuckGo simultaneously
- **Anti-Detection Technology**: Sophisticated browser fingerprinting avoidance
- **Headless Browser Integration**: Chrome-based scraping for JavaScript-heavy sites
- **Advanced Filtering**: Filter by domain, keyword, word count, and result type
- **Smart Result Processing**: Automatic keyword extraction and ad detection
- **Proxy Support**: Route requests through proxies to avoid rate limiting
- **Comprehensive Output Formats**: JSON, CSV, and terminal-friendly tables
- **Debug Mode**: Save HTML responses for troubleshooting
- **Performance Optimization**: Connection pooling and concurrent processing

## Installation

### From Binary Releases

Download the latest binary for your platform from the [releases page](https://github.com/advanced-search-engine/gosearch/releases).

```bash
# Linux/macOS
chmod +x gosearch
./gosearch --query "test search" --engine google

# Windows
gosearch.exe --query "test search" --engine google
```

### From Source

```bash
# Clone repository
git clone https://github.com/advanced-search-engine/gosearch.git
cd gosearch

# Build
go build -ldflags="-s -w" -trimpath -o gosearch

# Run
./gosearch --query "test search" --engine google
```

### Using Docker

```bash
docker pull advancedsearchengine/gosearch:latest
docker run advancedsearchengine/gosearch --query "test search" --engine google
```

## Usage

```
Usage: gosearch [OPTIONS] [QUERY]

Options:
  --query string         Search query
  --engine string        Search engine (google, bing, duckduckgo, all) (default "google")
  --max int              Maximum results to fetch (default 10)
  --ads                  Include advertisements in results
  --timeout duration     Search timeout (default 30s)
  --proxy string         Proxy URL (e.g., http://user:pass@host:port)
  --headless             Use headless browser (recommended for avoiding detection)
  --lang string          Language code (default "en")
  --region string        Region code (default "us")
  --format string        Output format (json, csv, table) (default "json")
  --output string        Output file (default: stdout)
  --page int             Result page number (default 1)
  --min-words int        Minimum word count in description
  --max-words int        Maximum word count in description
  --domain string        Filter results by domain (include)
  --exclude-domain string Filter results by domain (exclude)
  --keyword string       Filter results by keyword
  --type string          Filter by result type (organic, special, etc.)
  --site string          Limit results to specific site
  --filetype string      Limit results to specific file type
  --verbose              Enable verbose logging
  --debug                Enable debug mode (saves HTML responses)
  --log string           Log file path
  --stats string         Statistics output file
  --help                 Show help
```

## Examples

### Basic Search with Google

```bash
./gosearch --query "golang programming"
```

### Search with Bing and Filter by Domain

```bash
./gosearch --query "machine learning" --engine bing --domain edu
```

### Search Multiple Engines and Format as Table

```bash
./gosearch --query "climate science" --engine all --format table
```

### Use Headless Browser with Proxy

```bash
./gosearch --query "sensitive topic" --headless --proxy http://user:pass@host:port
```

### Advanced Query with File Type and Domain Restrictions

```bash
./gosearch --query "research paper" --filetype pdf --site edu --exclude-domain commercial.com
```

### Debug Mode with Custom Output

```bash
./gosearch --query "troubleshooting issue" --debug --output results.json
```

## Advanced Usage

### Using as a Library

GoSearch can be used as a Go package in your own projects:

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/advanced-search-engine/gosearch/pkg/engines"
    "github.com/advanced-search-engine/gosearch/pkg/models"
)

func main() {
    // Create a new Google search engine
    engine := engines.NewGoogleSearchEngine()
    
    // Configure search request
    request := models.SearchRequest{
        Query:       "golang programming",
        MaxResults:  5,
        Timeout:     30 * time.Second,
        UseHeadless: true,
    }
    
    // Perform search
    results, err := engine.Search(context.Background(), request)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    // Process results
    for _, result := range results {
        fmt.Printf("%s\n%s\n\n", result.Title, result.URL)
    }
}
```

### Custom Rate Limiting

You can set custom rate limits for each search engine:

```bash
# Rate limit in the configuration file
# ~/.config/gosearch/config.yaml
rate_limits:
  google: 10   # requests per minute
  bing: 15
  duckduckgo: 20
```

## Troubleshooting

### No Results Found

If you're not getting any results, try these solutions:

1. **Use Headless Mode**: Enable `--headless` to avoid detection
   ```bash
   ./gosearch --query "your search" --headless
   ```

2. **Use a Proxy**: Route through a clean IP address
   ```bash
   ./gosearch --query "your search" --proxy http://your-proxy-server:port
   ```

3. **Enable Debug Mode**: Examine the HTML response
   ```bash
   ./gosearch --query "your search" --debug
   ```
   This will create files like `google_debug.html` that you can inspect.

4. **Check for Captcha**: If you see "No results found," it might be due to a CAPTCHA. Look in the debug output for signs of CAPTCHAs.

### Selector Issues

If the selectors are not matching the current search engine layout:

1. Run with debug mode to capture the HTML
2. Examine the HTML structure
3. Update the selectors in the source code

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgements

- [PuerkitoBio/goquery](https://github.com/PuerkitoBio/goquery) for HTML parsing
- [chromedp/chromedp](https://github.com/chromedp/chromedp) for headless browser automation
- [fatih/color](https://github.com/fatih/color) for terminal output formatting

---

<p align="center">
Made with ❤️ by <a href="https://github.com/advanced-search-engine">Advanced Search Engine Team</a>
</p>
