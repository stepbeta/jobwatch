package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/stepbeta/jobwatch/internal/model"
)

const (
	// UserAgent identifies JobWatch honestly; RobotsToken is the product
	// token matched against robots.txt user-agent lines.
	UserAgent   = "Mozilla/5.0 (compatible; JobWatch/0.1; personal job-search tracker; +https://github.com/stepbeta/jobwatch)"
	RobotsToken = "JobWatch"

	DefaultInterval = 2 * time.Second
	DefaultTimeout  = 30 * time.Second
	robotsTTL       = 24 * time.Hour
	maxBodyBytes    = 20 << 20
)

// HTTPError is returned for non-2xx responses. 401/403/429 also wrap
// ErrBlocked.
type HTTPError struct {
	URL    string
	Status int
}

func (e *HTTPError) Error() string { return fmt.Sprintf("GET %s: HTTP %d", e.URL, e.Status) }

func (e *HTTPError) Unwrap() error {
	switch e.Status {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests:
		return ErrBlocked
	}
	return nil
}

type robotsEntry struct {
	rules   *robotsRules
	fetched time.Time
}

// Client is the shared polite HTTP client used by all adapters. It
// honors robots.txt (cached per host), spaces requests to the same host
// by the source's interval (or the robots Crawl-delay, whichever is
// larger), and sends a descriptive User-Agent.
type Client struct {
	HTTP            *http.Client
	UserAgent       string
	DefaultInterval time.Duration

	now   func() time.Time
	sleep func(context.Context, time.Duration) error

	mu      sync.Mutex
	robots  map[string]robotsEntry
	nextReq map[string]time.Time // per host: earliest time of next request
}

// NewClient returns a Client with default timeout, interval and UA.
func NewClient() *Client {
	return &Client{
		HTTP:            &http.Client{Timeout: DefaultTimeout},
		UserAgent:       UserAgent,
		DefaultInterval: DefaultInterval,
		now:             time.Now,
		sleep:           sleepCtx,
		robots:          map[string]robotsEntry{},
		nextReq:         map[string]time.Time{},
	}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// wait blocks until a request to host is allowed, reserving the slot.
func (c *Client) wait(ctx context.Context, host string, interval time.Duration) error {
	c.mu.Lock()
	now := c.now()
	at := c.nextReq[host]
	if at.Before(now) {
		at = now
	}
	c.nextReq[host] = at.Add(interval)
	c.mu.Unlock()
	return c.sleep(ctx, at.Sub(now))
}

// GetJSON fetches u on behalf of src and decodes the JSON body into v.
func (c *Client) GetJSON(ctx context.Context, src model.Source, u string, v any) error {
	body, err := c.Get(ctx, src, u, "application/json")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("decode %s: %w", u, err)
	}
	return nil
}

// Get fetches u politely and returns the body of a 2xx response.
func (c *Client) Get(ctx context.Context, src model.Source, u, accept string) ([]byte, error) {
	cfg, err := ParseConfig(src)
	if err != nil {
		return nil, err
	}
	parsed, err := url.Parse(u)
	if err != nil {
		return nil, fmt.Errorf("parse url %q: %w", u, err)
	}
	interval := cfg.IntervalFor(src.Type, c.DefaultInterval)

	rules, err := c.robotsFor(ctx, parsed, interval)
	if err != nil {
		return nil, err
	}
	path := parsed.EscapedPath()
	if parsed.RawQuery != "" {
		path += "?" + parsed.RawQuery
	}
	if !rules.allowed(path) {
		return nil, fmt.Errorf("robots.txt disallows %s: %w", u, ErrBlocked)
	}
	if rules.crawlDelay > interval {
		interval = rules.crawlDelay
	}

	if err := c.wait(ctx, parsed.Host, interval); err != nil {
		return nil, err
	}
	resp, err := c.do(ctx, u, accept)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
		return nil, &HTTPError{URL: u, Status: resp.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", u, err)
	}
	return body, nil
}

func (c *Client) do(ctx context.Context, u, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build request %s: %w", u, err)
	}
	req.Header.Set("User-Agent", c.UserAgent)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", u, err)
	}
	return resp, nil
}

// robotsFor returns the cached or freshly fetched robots rules for the
// host of u. Per RFC 9309: 2xx is parsed, 4xx means no restrictions, 5xx
// or network failure means the host cannot be crawled for now (error,
// not cached). 429 is treated as blocked.
func (c *Client) robotsFor(ctx context.Context, u *url.URL, interval time.Duration) (*robotsRules, error) {
	key := u.Scheme + "://" + u.Host
	c.mu.Lock()
	e, ok := c.robots[key]
	c.mu.Unlock()
	if ok && c.now().Sub(e.fetched) < robotsTTL {
		return e.rules, nil
	}

	robotsURL := key + "/robots.txt"
	if err := c.wait(ctx, u.Host, interval); err != nil {
		return nil, err
	}
	resp, err := c.do(ctx, robotsURL, "text/plain")
	if err != nil {
		return nil, fmt.Errorf("robots.txt unreachable: %w", err)
	}
	defer resp.Body.Close()

	var rules *robotsRules
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		rules = parseRobots(resp.Body, RobotsToken)
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, fmt.Errorf("robots.txt: %w", &HTTPError{URL: robotsURL, Status: resp.StatusCode})
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		rules = allowAll
	default:
		return nil, fmt.Errorf("robots.txt unavailable: %w", &HTTPError{URL: robotsURL, Status: resp.StatusCode})
	}
	c.mu.Lock()
	c.robots[key] = robotsEntry{rules: rules, fetched: c.now()}
	c.mu.Unlock()
	return rules, nil
}
