package endpoints

import (
	"io"
	"net/http"
	"strconv"
	"time"
)

// Read-only source GETs retry transient failures, with a bounded Retry-After.
func feedbackGET(client http.Client, req *http.Request) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		res, err := client.Do(req)
		if attempt >= 2 || (err == nil && res.StatusCode != 429 && res.StatusCode < 500) {
			return res, err
		}
		delay := time.Duration(attempt+1) * time.Second
		if res != nil {
			if seconds, e := strconv.Atoi(res.Header.Get("Retry-After")); e == nil && seconds >= 0 {
				delay = time.Duration(seconds) * time.Second
			}
			io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
			res.Body.Close()
		}
		if delay > 10*time.Second {
			delay = 10 * time.Second
		}
		timer := time.NewTimer(delay)
		select {
		case <-req.Context().Done():
			timer.Stop()
			return nil, req.Context().Err()
		case <-timer.C:
		}
	}
}
