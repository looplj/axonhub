package fetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	nethtml "golang.org/x/net/html"
	htmlstd "html"
)

const (
	defaultTimeout     = 30 * time.Second
	defaultMaxBodySize = 5 * 1024 * 1024
	defaultUserAgent   = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36"
	acceptLanguage     = "en-US,en;q=0.9"
)

type Client struct {
	httpClient  *http.Client
	userAgent   string
	maxBodySize int
	converter   *Converter
}

type Option func(*Client)

func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

func WithUserAgent(ua string) Option {
	return func(c *Client) {
		c.userAgent = ua
	}
}

func WithMaxBodySize(size int) Option {
	return func(c *Client) {
		c.maxBodySize = size
	}
}

func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

type Result struct {
	URL         string
	Title       string
	Description string
	Content     string
	ContentType string
	StatusCode  int
	Truncated   bool
}

func NewClient(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		userAgent:   defaultUserAgent,
		maxBodySize: defaultMaxBodySize,
		converter:   NewConverter(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *Client) Fetch(ctx context.Context, rawURL string) (*Result, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme: %s", parsedURL.Scheme)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "text/markdown,text/html,application/xhtml+xml,application/xml;q=0.9,text/plain;q=0.8,*/*;q=0.7")
	req.Header.Set("Accept-Language", acceptLanguage)
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	result := &Result{
		URL:         rawURL,
		StatusCode:  resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
	}

	limitedReader := io.LimitReader(resp.Body, int64(c.maxBodySize)+1)

	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	truncated := len(body) > c.maxBodySize
	if truncated {
		body = body[:c.maxBodySize]
	}

	result.Truncated = truncated

	contentType := resp.Header.Get("Content-Type")
	if isMarkdownContent(contentType) {
		title, description, content := parseMarkdownFrontMatter(string(body))
		result.Title = title
		result.Description = description
		result.Content = content
	} else if isHTMLContent(contentType) {
		title, description := extractHTMLTitleAndDescription(string(body))
		result.Title = title
		result.Description = description
		result.Content = c.converter.Convert(string(body))
	} else {
		result.Content = string(body)
	}

	return result, nil
}

func (c *Client) FetchRaw(ctx context.Context, rawURL string) ([]byte, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme: %s", parsedURL.Scheme)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "*/*")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	limitedReader := io.LimitReader(resp.Body, int64(c.maxBodySize))

	return io.ReadAll(limitedReader)
}

func isHTMLContent(contentType string) bool {
	contentType = strings.ToLower(contentType)

	return strings.Contains(contentType, "text/html") ||
		strings.Contains(contentType, "application/xhtml+xml")
}

func isMarkdownContent(contentType string) bool {
	contentType = strings.ToLower(contentType)
	return strings.Contains(contentType, "text/markdown")
}

func extractTitle(html string) string {
	title, _ := extractHTMLTitleAndDescription(html)
	return title
}

func extractDescription(html string) string {
	_, description := extractHTMLTitleAndDescription(html)
	return description
}

func extractHTMLTitleAndDescription(html string) (string, string) {
	z := nethtml.NewTokenizer(strings.NewReader(html))

	var (
		title         string
		description   string
		ogDescription string
	)

	for {
		tt := z.Next()
		if tt == nethtml.ErrorToken {
			if title != "" {
				title = cleanText(title)
			}

			if ogDescription != "" {
				return title, cleanText(ogDescription)
			}

			if description != "" {
				return title, cleanText(description)
			}

			return title, ""
		}

		if tt != nethtml.StartTagToken && tt != nethtml.SelfClosingTagToken {
			continue
		}

		tagName, hasAttr := z.TagName()
		if title == "" && string(tagName) == "title" && tt == nethtml.StartTagToken {
			var b strings.Builder

			for {
				tt2 := z.Next()
				if tt2 == nethtml.ErrorToken {
					break
				}

				if tt2 == nethtml.TextToken {
					b.Write(z.Text())
				}

				if tt2 == nethtml.EndTagToken {
					endTagName, _ := z.TagName()
					if string(endTagName) == "title" {
						break
					}
				}
			}

			title = htmlstd.UnescapeString(b.String())
			title = cleanText(title)
		}

		if string(tagName) == "meta" {
			var (
				property string
				name     string
				content  string
			)

			for hasAttr {
				k, v, moreAttr := z.TagAttr()
				switch strings.ToLower(string(k)) {
				case "property":
					property = string(v)
				case "name":
					name = string(v)
				case "content":
					content = string(v)
				}

				hasAttr = moreAttr
			}

			if content == "" {
				continue
			}

			content = htmlstd.UnescapeString(content)

			switch strings.ToLower(strings.TrimSpace(property)) {
			case "og:description":
				if ogDescription == "" {
					ogDescription = content
				}
			case "description":
				if description == "" {
					description = content
				}
			default:
				if strings.EqualFold(strings.TrimSpace(name), "description") && description == "" {
					description = content
				}
			}
		}

		if title != "" && ogDescription != "" {
			return title, cleanText(ogDescription)
		}
	}
}

func cleanText(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Join(strings.Fields(s), " ")

	return s
}
