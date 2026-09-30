// Package sender POSTs a JSON document to an HTTP endpoint with retries.
package sender

import (
	"bytes"
	"context"
	"crypto/sha256"
	"e-commerce_order_analytics_system/pkg/logger"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

const MaxBodySize = 10 << 20

type Request struct {
	URL      string
	Token    string
	Body     []byte
	Attempts int
	Timeout  time.Duration
}

type Response struct {
	StatusCode int
	Body       string
	Attempts   int
}

var Client = &http.Client{}

var baseBackoff = time.Second

func ValidateURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf(
			"invalid url %q: expected something like https://api.example.com/v1/reports",
			raw,
		)
	}
	switch u.Scheme {
	case "https":
		return u, nil
	case "http":
		host := u.Hostname()
		if host == "localhost" {
			return u, nil
		}
		if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
			return u, nil
		}
		return nil, fmt.Errorf(
			"refusing to send over plain http to %s: use https (http is only allowed for localhost)",
			host,
		)
	}
	return nil, fmt.Errorf("invalid url scheme %q: expected https", u.Scheme)
}

func Send(ctx context.Context, req Request) (Response, error) {
	res := Response{}
	if _, err := ValidateURL(req.URL); err != nil {
		return res, err
	}
	if len(req.Body) > MaxBodySize {
		return res, fmt.Errorf("body is %d bytes, maximum is %d", len(req.Body), MaxBodySize)
	}

	attempts := max(req.Attempts, 1)
	sum := sha256.Sum256(req.Body)
	idempotencyKey := hex.EncodeToString(sum[:])

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		res.Attempts = attempt

		start := time.Now()
		status, body, retryAfter, err := post(ctx, req, idempotencyKey)
		res.StatusCode, res.Body = status, body
		logAttempt(ctx, req.URL, attempt, attempts, status, time.Since(start), err)

		switch {
		case err != nil:
			if ctx.Err() != nil {
				return res, ctx.Err()
			}
			lastErr = err
		case status >= 200 && status < 300:
			return res, nil
		case !retryable(status):
			return res, fmt.Errorf(
				"server rejected the request: %s%s",
				http.StatusText(status)+statusSuffix(status),
				bodySuffix(body),
			)
		default:
			lastErr = fmt.Errorf(
				"server returned %d %s%s",
				status,
				http.StatusText(status),
				bodySuffix(body),
			)
		}

		if attempt == attempts {
			break
		}
		wait := baseBackoff << (attempt - 1)
		if retryAfter > 0 {
			wait = min(retryAfter, 30*time.Second)
		}
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return res, ctx.Err()
		}
	}

	return res, fmt.Errorf("giving up after %d attempt(s): %w", attempts, lastErr)
}

func post(
	ctx context.Context,
	req Request,
	idempotencyKey string,
) (status int, body string, retryAfter time.Duration, err error) {
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		req.URL,
		bytes.NewReader(req.Body),
	)
	if err != nil {
		return 0, "", 0, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "e-commerce-report-cli/1.0")
	httpReq.Header.Set("Idempotency-Key", idempotencyKey)
	if req.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+req.Token)
	}

	resp, err := Client.Do(httpReq)
	if err != nil {
		return 0, "", 0, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
	_, _ = io.Copy(io.Discard, resp.Body)

	if secs, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && secs > 0 {
		retryAfter = time.Duration(secs) * time.Second
	}
	return resp.StatusCode, strings.TrimSpace(string(raw)), retryAfter, nil
}

func retryable(status int) bool {
	return status == http.StatusRequestTimeout ||
		status == http.StatusTooManyRequests ||
		status >= 500
}

func statusSuffix(status int) string {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Sprintf(" (%d, check --token)", status)
	case http.StatusNotFound:
		return fmt.Sprintf(" (%d, check --url)", status)
	}
	return fmt.Sprintf(" (%d)", status)
}

func bodySuffix(body string) string {
	if body == "" {
		return ""
	}
	return ": " + body
}

func logAttempt(
	ctx context.Context,
	url string,
	attempt, attempts, status int,
	d time.Duration,
	err error,
) {
	fields := []zap.Field{
		zap.String("url", url),
		zap.Int("attempt", attempt),
		zap.Int("max_attempts", attempts),
		zap.Duration("duration", d.Round(time.Millisecond)),
	}
	log := logger.WithContext(ctx)
	switch {
	case err != nil:
		log.Warn("request failed", append(fields, zap.Error(err))...)
	case status >= 200 && status < 300:
		log.Info("request succeeded", append(fields, zap.Int("status", status))...)
	default:
		log.Warn("request rejected", append(fields, zap.Int("status", status))...)
	}
}
