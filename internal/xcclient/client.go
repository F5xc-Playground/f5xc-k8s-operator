package xcclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"software.sslmate.com/src/go-pkcs12"
)

// Client is an HTTP client for the F5 XC REST API.
type Client struct {
	httpClient  *http.Client
	tenantURL   string
	apiToken    string
	maxRetries  int
	baseDelay   time.Duration
	rateLimiter *EndpointRateLimiter
	metrics     *Metrics
	log         logr.Logger
}

// NewClient validates cfg, configures the HTTP transport (with mutual TLS if a
// P12 certificate is provided), and returns a ready-to-use *Client.
func NewClient(cfg Config, log logr.Logger, reg prometheus.Registerer) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	transport := &http.Transport{}

	if cfg.CertP12Path != "" {
		cert, err := loadP12(cfg.CertP12Path, cfg.CertPassword)
		if err != nil {
			return nil, fmt.Errorf("loading P12 certificate: %w", err)
		}
		transport.TLSClientConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
	}

	c := &Client{
		httpClient: &http.Client{
			Timeout:   cfg.HTTPTimeout,
			Transport: transport,
		},
		tenantURL:   cfg.TenantURL,
		apiToken:    cfg.APIToken,
		maxRetries:  cfg.MaxRetries,
		baseDelay:   1 * time.Second,
		rateLimiter: NewEndpointRateLimiter(cfg.RateLimits),
		metrics:     NewMetrics(reg),
		log:         log,
	}

	return c, nil
}

// SetBaseDelay overrides the base delay used for exponential backoff. Intended
// for use in tests to keep retry loops fast.
func (c *Client) SetBaseDelay(d time.Duration) {
	c.baseDelay = d
}

// Do is the exported wrapper around the private do() method. It exists so that
// tests in the xcclient_test package can exercise request dispatch without
// needing resource-specific wrappers.
func (c *Client) Do(ctx context.Context, method, resource, namespace, name string, body, result interface{}) error {
	return c.do(ctx, method, resource, namespace, name, body, result)
}

// do is the core request dispatcher. It builds the URL, waits on the rate
// limiter, and executes the request with retry logic for 429 responses.
func (c *Client) do(ctx context.Context, method, resource, namespace, name string, body, result interface{}) error {
	// Build the endpoint path used for rate limiting and metrics.
	endpoint := fmt.Sprintf("/api/config/namespaces/%s/%s", namespace, resource)

	// Build the full URL.
	url := c.tenantURL + endpoint
	if name != "" {
		url += "/" + name
	}

	// Wait for a rate limiter token before issuing any request.
	if err := c.rateLimiter.Wait(ctx, resource); err != nil {
		return fmt.Errorf("rate limiter wait: %w", err)
	}

	// Retry loop — only 429 responses are retried.
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential back-off with jitter.
			delay := c.backoffDelay(attempt)
			c.log.V(1).Info("retrying after 429", "attempt", attempt, "delay", delay, "url", url)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
			// Count the retry in metrics.
			c.metrics.RetriesTotal.WithLabelValues(resource, "rate_limited").Inc()
		}

		lastErr = c.doOnce(ctx, method, url, resource, body, result)
		if lastErr == nil {
			return nil
		}

		// Only retry on 429 (ErrRateLimited).
		if !errors.Is(lastErr, ErrRateLimited) {
			return lastErr
		}

		// Record the rate limit hit.
		c.metrics.RateLimitHits.WithLabelValues(resource).Inc()
	}

	return lastErr
}

// doOnce performs a single HTTP round-trip: marshal body, set headers, execute
// request, read response, record metrics, and map errors.
func (c *Client) doOnce(ctx context.Context, method, url, endpoint string, body, result interface{}) error {
	// Marshal the body if one was provided.
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshalling request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}

	// Set auth and content-negotiation headers.
	req.Header.Set("Authorization", "APIToken "+c.apiToken)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(start)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body so we can include it in errors and metrics.
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	statusStr := strconv.Itoa(resp.StatusCode)
	c.metrics.RequestsTotal.WithLabelValues(endpoint, method, statusStr).Inc()
	c.metrics.RequestDuration.WithLabelValues(endpoint, method).Observe(duration.Seconds())

	// Map non-2xx status codes to typed errors.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return StatusToError(resp.StatusCode, url, string(respBytes))
	}

	// Unmarshal the success response into result if a destination was provided.
	if result != nil && len(respBytes) > 0 {
		if err := json.Unmarshal(respBytes, result); err != nil {
			return fmt.Errorf("unmarshalling response: %w", err)
		}
	}

	return nil
}

// backoffDelay computes an exponential delay for the given retry attempt
// (1-based). Jitter up to baseDelay is added to spread retries. Using
// baseDelay as the jitter cap ensures the jitter scales down proportionally
// when baseDelay is reduced for tests.
func (c *Client) backoffDelay(attempt int) time.Duration {
	exp := c.baseDelay
	for i := 1; i < attempt; i++ {
		exp *= 2
	}
	jitterCap := c.baseDelay
	if jitterCap < 1 {
		jitterCap = 1
	}
	jitter := time.Duration(rand.Int63n(int64(jitterCap)))
	return exp + jitter
}

// loadP12 reads a PKCS#12 file from path and decodes it using the supplied
// password, returning the first certificate and its private key.
func loadP12(path, password string) (tls.Certificate, error) {
	// Read the P12 file.
	data, err := os.ReadFile(path)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("reading P12 file: %w", err)
	}

	privateKey, certificate, _, err := pkcs12.DecodeChain(data, password)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("decoding P12 data: %w", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{certificate.Raw},
		PrivateKey:  privateKey,
		Leaf:        certificate,
	}, nil
}
