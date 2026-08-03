package util

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"
)

const maxHTTPRetryAttempts = 3

func shouldRetryHTTPStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= 500
}

func isStaleConnError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if errors.Is(opErr.Err, syscall.ECONNRESET) || errors.Is(opErr.Err, syscall.EPIPE) {
			return true
		}
	}
	msg := err.Error()
	return strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "use of closed network connection")
}

func sleepWithCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// DoWithRetry retries transient HTTP failures (429/5xx, stale keep-alive).
func DoWithRetry(client *http.Client, req *http.Request) (*http.Response, error) {
	var bodyBytes []byte
	hasBody := req.Body != nil
	if hasBody {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		_ = req.Body.Close()
	}

	newAttempt := func() *http.Request {
		r := req.Clone(req.Context())
		if hasBody {
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			r.ContentLength = int64(len(bodyBytes))
		}
		return r
	}

	var (
		resp *http.Response
		err  error
	)
	for attempt := range maxHTTPRetryAttempts {
		if resp != nil {
			resp.Body.Close()
			resp = nil
		}
		resp, err = client.Do(newAttempt())
		if err != nil {
			if attempt == 0 && isStaleConnError(err) {
				continue
			}
			return nil, err
		}
		if !shouldRetryHTTPStatus(resp.StatusCode) {
			return resp, nil
		}
		if attempt < maxHTTPRetryAttempts-1 {
			backoff := time.Duration(attempt+1) * time.Second
			if attempt == 0 {
				backoff = 0
			}
			if backoff > 0 {
				if err = sleepWithCtx(req.Context(), backoff); err != nil {
					resp.Body.Close()
					return nil, err
				}
			}
		}
	}
	return resp, nil
}
