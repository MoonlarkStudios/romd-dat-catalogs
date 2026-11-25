package distribution

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"syscall"
	"time"
)

// NewMirrorClient retries transient downloads from the live mirror. TUF and
// candidate integrity checks still run once on the bytes returned by HTTP.
func NewMirrorClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: mirrorTransport{
			base:   http.DefaultTransport,
			delays: []time.Duration{3 * time.Second, 6 * time.Second, 12 * time.Second},
		},
	}
}

type mirrorTransport struct {
	base   http.RoundTripper
	delays []time.Duration
}

func (t mirrorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// All mirror requests are GETs without a body, including redirect requests.
	if req.Method != http.MethodGet || req.Body != nil {
		return t.base.RoundTrip(req)
	}
	for attempt := 0; ; attempt++ {
		response, err := t.base.RoundTrip(req)
		if attempt == len(t.delays) || !isTransientDownload(response, err) {
			return response, err
		}
		delay := t.delays[attempt]
		if response != nil {
			// Do not retry earlier than the server asks, or wait beyond the
			// bounded backoff when the server asks for a longer pause.
			if retryAfter := response.Header.Get("Retry-After"); retryAfter != "" && !canRetryAfter(retryAfter, delay) {
				return response, nil
			}
			response.Body.Close()
		}
		timer := time.NewTimer(delay)
		select {
		case <-req.Context().Done():
			timer.Stop()
			return nil, req.Context().Err()
		case <-timer.C:
			if err := req.Context().Err(); err != nil {
				return nil, err
			}
		}
	}
}

func isTransientDownload(response *http.Response, err error) bool {
	if err != nil {
		return errors.Is(err, syscall.ECONNRESET) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
	}
	switch response.StatusCode {
	case http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusInternalServerError,
		http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func canRetryAfter(value string, delay time.Duration) bool {
	if seconds, err := strconv.ParseUint(value, 10, 64); err == nil {
		return seconds <= uint64(delay/time.Second)
	}
	date, err := http.ParseTime(value)
	return err == nil && time.Until(date) <= delay
}
