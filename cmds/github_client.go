/*
Copyright AppsCode Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmds

import (
	"context"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/google/go-github/v84/github"
	"golang.org/x/oauth2"
)

const (
	defaultSecondaryRetryDelay = time.Minute
	defaultServerErrorDelay    = 5 * time.Second
	maxSecondaryRetryDelay     = 15 * time.Minute
	maxRateLimitRetryAttempts  = 8
)

func newGitHubClient(ctx context.Context) *github.Client {
	token, found := os.LookupEnv("GH_TOOLS_TOKEN")
	if !found {
		log.Fatalln("GH_TOOLS_TOKEN env var is not set")
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(ctx, ts)

	baseTransport := httpClient.Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}
	httpClient.Transport = &rateLimitTransport{base: baseTransport}

	return github.NewClient(httpClient)
}

type rateLimitTransport struct {
	base http.RoundTripper
}

func (t *rateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.GetBody == nil {
		if f, ok := req.Body.(*os.File); ok {
			name := f.Name()
			req.GetBody = func() (io.ReadCloser, error) {
				return os.Open(name)
			}
		}
	}

	canRetryBody := req.Body == nil || req.Body == http.NoBody || req.GetBody != nil

	for attempt := 0; ; attempt++ {
		currReq := req
		if attempt > 0 {
			currReq = req.Clone(req.Context())
			if req.GetBody != nil {
				body, err := req.GetBody()
				if err != nil {
					return nil, err
				}
				currReq.Body = body
			}
		}

		resp, err := t.base.RoundTrip(currReq)
		if err != nil {
			return nil, err
		}
		if resp == nil {
			return resp, nil
		}
		retryable := resp.StatusCode == http.StatusForbidden ||
			resp.StatusCode == http.StatusTooManyRequests ||
			resp.StatusCode == http.StatusInternalServerError
		if !retryable {
			return resp, nil
		}

		delay := retryDelay(resp, attempt)
		retryHint := retryDelayHint(resp, attempt)
		if attempt >= maxRateLimitRetryAttempts {
			return resp, nil
		}
		if !canRetryBody {
			return resp, nil
		}

		if resp.StatusCode == http.StatusInternalServerError {
			log.Printf("GitHub API server error (500), waiting %s before retry", delay.Round(time.Second))
		} else {
			if requestedWait, requestedFrom, ok := requestedRateLimitWait(resp); ok {
				log.Printf("GitHub API rate limited (%d), waiting %s before retry (%s). GitHub asked to wait %s (%s)", resp.StatusCode, delay.Round(time.Second), retryHint, requestedWait.Round(time.Second), requestedFrom)
			} else {
				log.Printf("GitHub API rate limited (%d), waiting %s before retry (%s)", resp.StatusCode, delay.Round(time.Second), retryHint)
			}
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()

		timer := time.NewTimer(delay)
		select {
		case <-req.Context().Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil, req.Context().Err()
		case <-timer.C:
		}
	}
}

func retryDelay(resp *http.Response, attempt int) time.Duration {
	// For server errors, use a short exponential backoff (5s, 10s, 20s, …)
	if resp.StatusCode == http.StatusInternalServerError {
		backoff := min(time.Duration(float64(defaultServerErrorDelay)*math.Pow(2, float64(attempt))), maxSecondaryRetryDelay)
		return max(backoff, time.Second)
	}
	delay, _ := rateLimitRetryDelay(resp, attempt)
	return delay
}

func retryDelayHint(resp *http.Response, secondaryAttempt int) string {
	if resp.StatusCode == http.StatusInternalServerError {
		return "exponential backoff after 500 response"
	}
	_, hint := rateLimitRetryDelay(resp, secondaryAttempt)
	return hint
}

func rateLimitRetryDelay(resp *http.Response, secondaryAttempt int) (time.Duration, string) {
	retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
	if retryAfter > 0 {
		return retryAfter, "from Retry-After header"
	}

	remaining := resp.Header.Get("X-RateLimit-Remaining")
	if remaining == "0" {
		if reset := parseUnixTime(resp.Header.Get("X-RateLimit-Reset")); !reset.IsZero() {
			return max(time.Until(reset)+time.Second, time.Second), "from X-RateLimit-Reset header"
		}
	}

	// Secondary limit guidance from GitHub docs: wait at least 1 minute and increase with backoff.
	backoff := max(min(time.Duration(float64(defaultSecondaryRetryDelay)*math.Pow(2, float64(secondaryAttempt))), maxSecondaryRetryDelay), time.Second)
	return backoff, "using secondary rate-limit backoff"
}

func requestedRateLimitWait(resp *http.Response) (time.Duration, string, bool) {
	retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
	if retryAfter > 0 {
		return retryAfter, "Retry-After header", true
	}

	remaining := resp.Header.Get("X-RateLimit-Remaining")
	if remaining == "0" {
		if reset := parseUnixTime(resp.Header.Get("X-RateLimit-Reset")); !reset.IsZero() {
			return max(time.Until(reset), time.Second), "X-RateLimit-Reset header", true
		}
	}

	return 0, "", false
}

func parseRetryAfter(value string) time.Duration {
	if value == "" {
		return 0
	}
	seconds, err := strconv.Atoi(value)
	if err == nil {
		if seconds < 1 {
			seconds = 1
		}
		return time.Duration(seconds) * time.Second
	}
	if ts, err := http.ParseTime(value); err == nil {
		d := max(time.Until(ts), time.Second)
		return d
	}
	return 0
}

func parseUnixTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	sec, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(sec, 0).UTC()
}
