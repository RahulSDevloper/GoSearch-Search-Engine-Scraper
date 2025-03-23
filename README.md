# 🔍 Search Engine Scraper - GoSearch

<div align="center">

```
   _____                     _     _____             _            
  / ____|                   | |   |  ___|           (_)           
 | (___   ___  __ _ _ __ ___| |__ | |__ _ __   __ _  _ _ __   ___ 
  \___ \ / _ \/ _` | '__/ __| '_ \|  __| '_ \ / _` || | '_ \ / _ \
  ____) |  __/ (_| | | | (__| | | | |__| | | | (_| || | | | |  __/
 |_____/ \___|\__,_|_|  \___|_| |_\____/_| |_|\__, ||_|_| |_|\___|
   _____                                        __/ |              
  / ____|                                      |___/               
 | (___   ___ _ __ __ _ _ __   ___ _ __                            
  \___ \ / __| '__/ _` | '_ \ / _ \ '__|                           
  ____) | (__| | | (_| | |_) |  __/ |                              
 |_____/ \___|_|  \__,_| .__/ \___|_|                              
                       | |                                         
                       |_|                                         
```

<img src="https://img.shields.io/badge/Go-1.19%2B-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go Version"/>
<img src="https://img.shields.io/badge/Powered%20by-Chromedp-4285F4?style=for-the-badge&logo=google-chrome&logoColor=white" alt="Powered by Chromedp"/>
<img src="https://img.shields.io/badge/Version-1.0.0-success?style=for-the-badge" alt="Version"/>

<img src="https://raw.githubusercontent.com/RahulSDevloper/Search-Engine-Scraper---Golang/main/assets/demo.gif" alt="Search Engine Scraper Demo" width="700"/>

<p>
<b>High-performance, anti-detection search engine scraper - Built with advanced Go concurrency patterns</b>
</p>

</div>

<h2 align="center">
  <a href="#%EF%B8%8F-key-features">✨ Features</a> •
  <a href="#-installation">🚀 Install</a> •
  <a href="#-usage">🔧 Usage</a> •
  <a href="#%EF%B8%8F-examples">🌟 Examples</a> •
  <a href="#-advanced-techniques">🧠 Advanced</a> •
  <a href="#-debugging">🐞 Debug</a>
</h2>

---

## ✨ Key Features

<div align="center">
<table>
<tr>
<td align="center" width="33%">
<img width="120" src="https://img.shields.io/badge/Multi--Engine-Support-blue?style=for-the-badge&logo=google&logoColor=white" />
<br><sub>Google, Bing & DuckDuckGo</sub>
</td>
<td align="center" width="33%">
<img width="120" src="https://img.shields.io/badge/Anti--Detection-Technology-red?style=for-the-badge&logo=shield&logoColor=white" />
<br><sub>Bypass CAPTCHAs & Blocks</sub>
</td>
<td align="center" width="33%">
<img width="120" src="https://img.shields.io/badge/Headless-Browser-green?style=for-the-badge&logo=googlechrome&logoColor=white" />
<br><sub>Chrome-Based Scraping</sub>
</td>
</tr>
<tr>
<td align="center" width="33%">
<img width="120" src="https://img.shields.io/badge/Advanced-Filtering-orange?style=for-the-badge&logo=filter&logoColor=white" />
<br><sub>Domain, Keyword & More</sub>
</td>
<td align="center" width="33%">
<img width="120" src="https://img.shields.io/badge/Smart-Processing-purple?style=for-the-badge&logo=brain&logoColor=white" />
<br><sub>Keyword Extraction & Ad Detection</sub>
</td>
<td align="center" width="33%">
<img width="120" src="https://img.shields.io/badge/Proxy-Support-teal?style=for-the-badge&logo=privatenetwork&logoColor=white" />
<br><sub>Avoid Rate Limiting</sub>
</td>
</tr>
</table>
</div>

## 🚀 Installation

<div align="center">
<table>
<tr><th>Method</th><th>Commands</th></tr>
<tr>
<td><img src="https://img.shields.io/badge/From-Binary-brightgreen?style=flat-square" alt="From Binary"/></td>
<td>

```bash
# Download the latest release
curl -sSL https://github.com/RahulSDevloper/Search-Engine-Scraper---Golang/releases/download/v1.0.0/gosearch-linux-amd64 -o gosearch
chmod +x gosearch
./gosearch --query "golang programming"
```

</td>
</tr>
<tr>
<td><img src="https://img.shields.io/badge/From-Source-blue?style=flat-square" alt="From Source"/></td>
<td>

```bash
git clone https://github.com/RahulSDevloper/Search-Engine-Scraper---Golang.git
cd Search-Engine-Scraper---Golang
go build -ldflags="-s -w" -o gosearch
./gosearch --query "golang programming"
```

</td>
</tr>
<tr>
<td><img src="https://img.shields.io/badge/Using-Docker-blue?style=flat-square&logo=docker" alt="Using Docker"/></td>
<td>

```bash
docker pull rahulsdevloper/gosearch:latest
docker run rahulsdevloper/gosearch --query "golang programming"
```

</td>
</tr>
</table>
</div>

## 🔧 Usage

<div align="center">
<img src="https://raw.githubusercontent.com/RahulSDevloper/GoSearch-Search-Engine-Scraper/main/assets/GoSearch.png" alt="Usage Animation" width="700"/>
</div>

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

## 🌟 Examples

<details open>
<summary><b>Basic Search with Google</b> 🔍</summary>

```bash
./gosearch --query "golang programming"
```

<img src="https://raw.githubusercontent.com/RahulSDevloper/Search-Engine-Scraper---Golang/main/assets/example1.gif" alt="Basic Search Example" width="600"/>
</details>

<details>
<summary><b>Search with Advanced Filters</b> 🧰</summary>

```bash
./gosearch --query "machine learning" --engine bing --domain edu --format table
```

<img src="https://raw.githubusercontent.com/RahulSDevloper/Search-Engine-Scraper---Golang/main/assets/example2.gif" alt="Advanced Search Example" width="600"/>
</details>

<details>
<summary><b>Multi-Engine Search with Headless Browser</b> 🌐</summary>

```bash
./gosearch --query "climate science" --engine all --headless --output results.json
```

<img src="https://raw.githubusercontent.com/RahulSDevloper/Search-Engine-Scraper---Golang/main/assets/example3.gif" alt="Multi-Engine Example" width="600"/>
</details>

<details>
<summary><b>Filetype Specific Search</b> 📄</summary>

```bash
./gosearch --query "research papers" --filetype pdf --site edu --max 20
```
</details>

## 🧠 Advanced Techniques

<div align="center">
<img src="https://raw.githubusercontent.com/RahulSDevloper/Search-Engine-Scraper---Golang/main/assets/advanced.svg" alt="Advanced Features" width="700"/>
</div>

### Using as a Library

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/RahulSDevloper/Search-Engine-Scraper---Golang/pkg/engines"
    "github.com/RahulSDevloper/Search-Engine-Scraper---Golang/pkg/models"
)

func main() {
    // Create a new Google search engine
    engine := engines.NewGoogleSearchEngine()
    
    // Configure search request with optimization strategy
    request := models.SearchRequest{
        Query:       "golang concurrency patterns",
        MaxResults:  10,
        Timeout:     30 * time.Second,
        UseHeadless: true,
        Debug:       true,
    }
    
    // Execute search with context for cancellation
    ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
    defer cancel()
    
    results, err := engine.Search(ctx, request)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    // Process and analyze results
    for i, result := range results {
        fmt.Printf("%d. %s\n%s\n\n", i+1, result.Title, result.URL)
    }
}
```

### Custom Rate Limiting

```yaml
# ~/.config/gosearch/config.yaml
rate_limits:
  google: 10   # requests per minute
  bing: 15
  duckduckgo: 20

proxy_rotation:
  enabled: true
  proxies:
    - http://proxy1:8080
    - http://proxy2:8080
  rotation_strategy: round-robin  # or random
```

## 🐞 Debugging

<div align="center">
<img src="https://raw.githubusercontent.com/RahulSDevloper/Search-Engine-Scraper---Golang/main/assets/debugging.svg" alt="Debugging Techniques" width="700"/>
</div>

### No Results Found?

If you're not getting any results, try these solutions:

1. **Use Headless Mode** to avoid detection
   ```bash
   ./gosearch --query "your search" --headless
   ```

2. **Use a Proxy** to route through a clean IP address
   ```bash
   ./gosearch --query "your search" --proxy http://your-proxy-server:port
   ```

3. **Enable Debug Mode** to examine the HTML response
   ```bash
   ./gosearch --query "your search" --debug
   ```

### Debugging Process Flow

```mermaid
graph TD
    A[Run Search] --> B{Results Found?}
    B -->|Yes| C[Process Results]
    B -->|No| D[Enable Debug Mode]
    D --> E[Check HTML Responses]
    E --> F{Captcha Present?}
    F -->|Yes| G[Use Headless + Proxy]
    F -->|No| H[Check Selectors]
    H --> I[Update Selectors]
    I --> A
    G --> A
```

## 📊 Performance Benchmarks

<div align="center">
<table>
<tr><th>Engine</th><th>Results/Second</th><th>Memory Usage</th><th>Detection Avoidance</th></tr>
<tr><td>Google</td><td>6.5</td><td>Low</td><td>High</td></tr>
<tr><td>Bing</td><td>8.2</td><td>Low</td><td>Medium</td></tr>
<tr><td>DuckDuckGo</td><td>7.3</td><td>Low</td><td>Very High</td></tr>
<tr><td>All (Concurrent)</td><td>4.8</td><td>Medium</td><td>Medium</td></tr>
</table>
</div>

## 📚 Design Philosophy

The Search Engine Scraper follows these core principles:

1. **Resilience First**: Designed to handle the constantly changing DOM structures of search engines
2. **Performance Focused**: Optimized for speed while maintaining low resource usage
3. **Privacy Conscious**: Minimal footprint to avoid detection
4. **Developer Friendly**: Clean API for integration into other Go applications

## 📝 License

<div align="center">

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

</div>

---

<div align="center">

<img src="https://img.shields.io/badge/Made%20with%20%E2%9D%A4%EF%B8%8F%20by-RahulSDevloper-red.svg?style=for-the-badge" alt="Made with love by RahulSDevloper"/>

⭐ Star this project if you find it useful! ⭐

</div>
