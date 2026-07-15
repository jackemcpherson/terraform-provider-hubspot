// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package hubspot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	maxAttempts       = 5
	maxSuccessBody    = 32 << 20
	maxErrorBody      = 64 << 10
	defaultBurstLimit = 10
)

// ReplaySafety describes whether a request can be replayed after a transient
// response. Ambiguous creates are ReplayNever by design.
type ReplaySafety uint8

const (
	ReplayNever ReplaySafety = iota
	ReplayExplicit
	ReplaySafe
)

// Operation is a typed request contract. Paths are route paths, never complete
// URLs, so the transport owns the configured origin and secret boundary.
type Operation struct {
	Name   string
	Method string
	Path   string
	Replay ReplaySafety
}

// Event contains only safe transport telemetry. It deliberately has no URL,
// token, body, or CRM identity fields.
type Event struct {
	Operation string
	Attempt   int
	Status    int
	Retry     bool
}

type TransportConfig struct {
	BaseURL     *url.URL
	AccessToken string
	UserAgent   string
	HTTPClient  *http.Client
	Clock       func() time.Time
	Sleep       func(context.Context, time.Duration) error
	Jitter      func(time.Duration) time.Duration
	EventSink   func(Event)
}

type Transport struct {
	baseURL     *url.URL
	accessToken string
	userAgent   string
	client      *http.Client
	clock       func() time.Time
	sleep       func(context.Context, time.Duration) error
	jitter      func(time.Duration) time.Duration
	eventSink   func(Event)
	limiter     *rate.Limiter
	limitMu     sync.Mutex
}

type Error struct {
	Operation   string
	Status      int
	Category    string
	SubCategory string
	Message     string
	Correlation string
	RetryAfter  time.Duration
	Cause       error
	Ambiguous   bool
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("hubspot %s: %v", e.Operation, e.Cause)
	}
	if e.Message == "" {
		return fmt.Sprintf("hubspot %s: HTTP %d", e.Operation, e.Status)
	}
	return fmt.Sprintf("hubspot %s: HTTP %d: %s", e.Operation, e.Status, e.Message)
}

func (e *Error) Unwrap() error { return e.Cause }

type errorEnvelope struct {
	Status        string `json:"status"`
	Message       string `json:"message"`
	Category      string `json:"category"`
	SubCategory   string `json:"subCategory"`
	CorrelationID string `json:"correlationId"`
}

func NewTransport(config TransportConfig) (*Transport, error) {
	if config.BaseURL == nil || config.BaseURL.Scheme == "" || config.BaseURL.Host == "" || config.BaseURL.Hostname() == "" {
		return nil, errors.New("HubSpot API base URL must be absolute")
	}
	if config.BaseURL.User != nil || config.BaseURL.RawQuery != "" || config.BaseURL.Fragment != "" {
		return nil, errors.New("HubSpot API base URL must not contain userinfo, query, or fragment")
	}
	if config.BaseURL.Scheme != "https" && !(config.BaseURL.Scheme == "http" && isLoopbackHost(config.BaseURL.Hostname())) {
		return nil, errors.New("HubSpot API base URL must use HTTPS except for loopback tests")
	}

	client := config.HTTPClient
	if client == nil {
		base, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return nil, errors.New("default HTTP transport is not configurable")
		}
		transport := base.Clone()
		transport.DialContext = (&net.Dialer{Timeout: 10 * time.Second}).DialContext
		transport.TLSHandshakeTimeout = 10 * time.Second
		transport.ResponseHeaderTimeout = 30 * time.Second
		client = &http.Client{
			Transport: transport,
			Timeout:   60 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	if config.UserAgent == "" {
		config.UserAgent = "terraform-provider-hubspot/dev protocol/6"
	}
	if config.Sleep == nil {
		config.Sleep = sleepContext
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	if config.Jitter == nil {
		config.Jitter = func(duration time.Duration) time.Duration {
			if duration <= 0 {
				return 0
			}
			return time.Duration(rand.Float64() * float64(duration))
		}
	}

	return &Transport{
		baseURL:     cloneURL(config.BaseURL),
		accessToken: config.AccessToken,
		userAgent:   config.UserAgent,
		client:      client,
		clock:       config.Clock,
		sleep:       config.Sleep,
		jitter:      config.Jitter,
		eventSink:   config.EventSink,
		limiter:     rate.NewLimiter(rate.Limit(defaultBurstLimit), defaultBurstLimit),
	}, nil
}

func (t *Transport) Do(ctx context.Context, operation Operation, requestBody io.Reader, responseBody any) error {
	if operation.Method == "" || operation.Path == "" || !strings.HasPrefix(operation.Path, "/") {
		return &Error{Operation: operation.Name, Cause: errors.New("operation requires an HTTP method and absolute route path")}
	}
	body, err := readRequestBody(requestBody)
	if err != nil {
		return &Error{Operation: operation.Name, Cause: err}
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := t.limiter.Wait(ctx); err != nil {
			return &Error{Operation: operation.Name, Cause: err}
		}
		request, err := t.newRequest(ctx, operation, body)
		if err != nil {
			return &Error{Operation: operation.Name, Cause: err}
		}
		response, requestErr := t.client.Do(request)
		if requestErr != nil {
			if operation.Replay == ReplaySafe && attempt < maxAttempts {
				t.emit(Event{Operation: operation.Name, Attempt: attempt, Retry: true})
				if err := t.sleep(ctx, t.jitter(backoff(attempt))); err != nil {
					return &Error{Operation: operation.Name, Cause: err}
				}
				continue
			}
			return &Error{Operation: operation.Name, Cause: requestErr, Ambiguous: true}
		}

		if response.StatusCode >= 200 && response.StatusCode < 300 {
			err := decodeSuccess(response.Body, responseBody)
			response.Body.Close()
			if err != nil {
				return &Error{Operation: operation.Name, Status: response.StatusCode, Cause: err}
			}
			t.adaptRateLimit(response.Header)
			t.emit(Event{Operation: operation.Name, Attempt: attempt, Status: response.StatusCode})
			return nil
		}

		errorBody, readErr := io.ReadAll(io.LimitReader(response.Body, maxErrorBody+1))
		response.Body.Close()
		apiError := parseError(operation.Name, response.StatusCode, response.Header, errorBody, readErr, t.clock())
		retryable := isRetryableStatus(response.StatusCode)
		canRetry := retryable && attempt < maxAttempts && operation.Replay != ReplayNever
		if !canRetry {
			t.emit(Event{Operation: operation.Name, Attempt: attempt, Status: response.StatusCode})
			return apiError
		}

		delay := retryDelay(response.Header, response.StatusCode, attempt, t.clock())
		t.emit(Event{Operation: operation.Name, Attempt: attempt, Status: response.StatusCode, Retry: true})
		if err := t.sleep(ctx, t.jitter(delay)); err != nil {
			return &Error{Operation: operation.Name, Status: response.StatusCode, Cause: err}
		}
	}

	return &Error{Operation: operation.Name, Cause: errors.New("retry budget exhausted")}
}

func (t *Transport) newRequest(ctx context.Context, operation Operation, body []byte) (*http.Request, error) {
	requestURL := cloneURL(t.baseURL)
	route, err := url.Parse(operation.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid operation route: %w", err)
	}
	if route.Path == "" || !strings.HasPrefix(route.Path, "/") || route.Fragment != "" {
		return nil, errors.New("invalid operation route")
	}
	routePath, err := url.PathUnescape(route.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid operation route: %w", err)
	}
	requestURL.Path = strings.TrimSuffix(requestURL.Path, "/") + routePath
	requestURL.RawQuery = route.RawQuery
	requestURL.Fragment = ""
	request, err := http.NewRequestWithContext(ctx, operation.Method, requestURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", t.userAgent)
	if t.accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+t.accessToken)
	}
	if len(body) != 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	return request, nil
}

func (t *Transport) adaptRateLimit(header http.Header) {
	interval, intervalErr := strconv.ParseInt(header.Get("X-HubSpot-RateLimit-Interval-Milliseconds"), 10, 64)
	limit, limitErr := strconv.Atoi(header.Get("X-HubSpot-RateLimit-Max"))
	if intervalErr != nil || limitErr != nil || interval <= 0 || limit <= 0 {
		return
	}
	t.limitMu.Lock()
	defer t.limitMu.Unlock()
	t.limiter.SetLimit(rate.Limit(float64(limit) / (float64(interval) / float64(time.Second))))
	t.limiter.SetBurst(limit)
}

func (t *Transport) emit(event Event) {
	if t.eventSink != nil {
		t.eventSink(event)
	}
}

func parseError(operation string, status int, header http.Header, body []byte, readErr error, now time.Time) *Error {
	apiError := &Error{Operation: operation, Status: status, RetryAfter: parseRetryAfterAt(header.Get("Retry-After"), now)}
	if len(body) > maxErrorBody {
		apiError.Cause = errors.New("error response body exceeds limit")
		return apiError
	}
	if readErr != nil {
		apiError.Cause = readErr
		return apiError
	}
	var envelope errorEnvelope
	if json.Unmarshal(body, &envelope) == nil {
		apiError.Category = safeCategory(envelope.Category)
		apiError.SubCategory = safeCategory(envelope.SubCategory)
		apiError.Message = safeMessage(envelope.Message)
		apiError.Correlation = envelope.CorrelationID
		var nested errorEnvelope
		if strings.HasPrefix(strings.TrimSpace(envelope.Message), "{") && json.Unmarshal([]byte(envelope.Message), &nested) == nil {
			if nested.Message != "" {
				apiError.Message = safeMessage(nested.Message)
			}
			if nested.Category != "" {
				apiError.Category = safeCategory(nested.Category)
			}
			if nested.SubCategory != "" {
				apiError.SubCategory = safeCategory(nested.SubCategory)
			}
			if nested.CorrelationID != "" {
				apiError.Correlation = nested.CorrelationID
			}
		}
	}
	return apiError
}

func decodeSuccess(body io.Reader, destination any) error {
	data, err := io.ReadAll(io.LimitReader(body, maxSuccessBody+1))
	if err != nil {
		return err
	}
	if len(data) > maxSuccessBody {
		return errors.New("response body exceeds limit")
	}
	if destination == nil {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	return nil
}

func readRequestBody(body io.Reader) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	data, err := io.ReadAll(io.LimitReader(body, maxSuccessBody+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxSuccessBody {
		return nil, errors.New("request body exceeds limit")
	}
	return data, nil
}

func isRetryableStatus(status int) bool {
	switch status {
	case http.StatusRequestTimeout, 423, http.StatusTooManyRequests, 477, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout, 521, 523, 524:
		return true
	default:
		return false
	}
}

func retryDelay(header http.Header, status, attempt int, now time.Time) time.Duration {
	if retryAfter := parseRetryAfterAt(header.Get("Retry-After"), now); retryAfter > 0 {
		if status == 423 && retryAfter < 2*time.Second {
			return 2 * time.Second
		}
		return retryAfter
	}
	if status == 423 {
		return 2 * time.Second
	}
	return backoff(attempt)
}

func parseRetryAfterAt(value string, now time.Time) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if timestamp, err := http.ParseTime(value); err == nil {
		if duration := timestamp.Sub(now); duration > 0 {
			return duration
		}
	}
	return 0
}

func backoff(attempt int) time.Duration {
	return time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
}

func safeMessage(message string) string {
	message = strings.TrimSpace(strings.ReplaceAll(message, "\n", " "))
	if len(message) > 512 {
		return message[:512]
	}
	return message
}

func safeCategory(category string) string {
	category = strings.TrimSpace(category)
	if category == "" || len(category) > 128 {
		return ""
	}
	lower := strings.ToLower(category)
	for _, forbidden := range []string{"pat-", "token", "secret", "bearer"} {
		if strings.Contains(lower, forbidden) {
			return ""
		}
	}
	if looksLikeUUID(category) || looksLikeHexCredential(category) {
		return ""
	}
	for _, character := range category {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '_' && character != '-' && character != '.' {
			return ""
		}
	}
	return category
}

func looksLikeUUID(value string) bool {
	parts := strings.Split(value, "-")
	if len(parts) != 5 {
		return false
	}
	lengths := []int{8, 4, 4, 4, 12}
	for index, part := range parts {
		if len(part) != lengths[index] || !isHex(part) {
			return false
		}
	}
	return true
}

func looksLikeHexCredential(value string) bool {
	return len(value) >= 24 && isHex(value)
}

func isHex(value string) bool {
	for _, character := range value {
		if (character < 'a' || character > 'f') && (character < 'A' || character > 'F') && (character < '0' || character > '9') {
			return false
		}
	}
	return value != ""
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func cloneURL(value *url.URL) *url.URL {
	clone := *value
	return &clone
}
