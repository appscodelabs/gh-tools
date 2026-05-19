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
	"strings"
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

		kind := classifyResponse(resp)
		if kind == kindNone {
			return resp, nil
		}
		if attempt >= maxRateLimitRetryAttempts {
			log.Printf("GitHub API still failing after %d retries (status=%d); giving up", attempt, resp.StatusCode)
			return resp, nil
		}
		if !canRetryBody {
			return resp, nil
		}

		delay, source := retryDelay(resp, kind, attempt)
		logRetry(resp, kind, delay, source)

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

type responseKind int

const (
	kindNone responseKind = iota
	kindPrimaryRateLimit
	kindSecondaryRateLimit
	kindServerError
)

// classifyResponse decides whether a response is retryable per
// https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api.
//
// A 403/429 is only treated as a rate limit when GitHub provides a Retry-After
// header or X-RateLimit-Remaining=0. Bare 403s are permission errors and
// retrying them is wasteful.
func classifyResponse(resp *http.Response) responseKind {
	switch resp.StatusCode {
	case http.StatusForbidden, http.StatusTooManyRequests:
		hasRetryAfter := resp.Header.Get("Retry-After") != ""
		exhausted := resp.Header.Get("X-RateLimit-Remaining") == "0"
		switch {
		case exhausted:
			return kindPrimaryRateLimit
		case hasRetryAfter:
			return kindSecondaryRateLimit
		default:
			return kindNone
		}
	case http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return kindServerError
	default:
		return kindNone
	}
}

func retryDelay(resp *http.Response, kind responseKind, attempt int) (time.Duration, string) {
	if d := parseRetryAfter(resp.Header.Get("Retry-After")); d > 0 {
		return d, "Retry-After header"
	}

	switch kind {
	case kindPrimaryRateLimit:
		if reset := parseUnixTime(resp.Header.Get("X-RateLimit-Reset")); !reset.IsZero() {
			return max(time.Until(reset)+time.Second, time.Second), "X-RateLimit-Reset header"
		}
		return secondaryBackoff(attempt), "rate-limit backoff (no reset header)"
	case kindSecondaryRateLimit:
		return secondaryBackoff(attempt), "secondary rate-limit backoff"
	case kindServerError:
		return serverErrorBackoff(attempt), "exponential backoff after server error"
	}
	return time.Second, ""
}

// secondaryBackoff implements the GitHub-recommended "at least 1 minute,
// increase exponentially" backoff for secondary rate limits.
func secondaryBackoff(attempt int) time.Duration {
	d := time.Duration(float64(defaultSecondaryRetryDelay) * math.Pow(2, float64(attempt)))
	return max(min(d, maxSecondaryRetryDelay), defaultSecondaryRetryDelay)
}

func serverErrorBackoff(attempt int) time.Duration {
	d := time.Duration(float64(defaultServerErrorDelay) * math.Pow(2, float64(attempt)))
	return max(min(d, maxSecondaryRetryDelay), time.Second)
}

func logRetry(resp *http.Response, kind responseKind, delay time.Duration, source string) {
	rounded := delay.Round(time.Second)
	resource := resp.Header.Get("X-RateLimit-Resource")
	suffix := ""
	if resource != "" {
		suffix = " resource=" + resource
	}

	switch kind {
	case kindPrimaryRateLimit:
		log.Printf("GitHub API primary rate limit hit (%d%s); waiting %s before retry (%s)", resp.StatusCode, suffix, rounded, source)
	case kindSecondaryRateLimit:
		log.Printf("GitHub API secondary rate limit hit (%d%s); waiting %s before retry (%s)", resp.StatusCode, suffix, rounded, source)
	case kindServerError:
		log.Printf("GitHub API server error (%d); waiting %s before retry (%s)", resp.StatusCode, rounded, source)
	}
}

func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds < 1 {
			seconds = 1
		}
		return time.Duration(seconds) * time.Second
	}
	if ts, err := http.ParseTime(value); err == nil {
		return max(time.Until(ts), time.Second)
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
