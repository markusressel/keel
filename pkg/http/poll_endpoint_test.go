package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/keel-hq/keel/pkg/auth"
	"github.com/keel-hq/keel/trigger/poll"
)

type fakePoller struct {
	requests []poll.PollRequest
	results  []poll.PollResult
	err      error
}

func (p *fakePoller) PollNow(_ context.Context, req poll.PollRequest) ([]poll.PollResult, error) {
	p.requests = append(p.requests, req)
	return p.results, p.err
}

func newPollTestServer(poller Poller) *TriggerServer {
	opts := &Opts{
		Authenticator: auth.New(&auth.Opts{Username: "user-1", Password: "secret"}),
	}
	if poller != nil {
		opts.Poller = poller
	}
	srv := NewTriggerServer(opts)
	srv.registerRoutes(srv.router)
	return srv
}

func doPoll(t *testing.T, srv *TriggerServer, body string, authenticate bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/poll", strings.NewReader(body))
	if authenticate {
		req.SetBasicAuth("user-1", "secret")
	}
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)
	return rec
}

func decodePollError(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body PollErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode error body %q: %s", rec.Body.String(), err)
	}
	if body.Error == "" {
		t.Fatalf("expected an error message, got body %q", rec.Body.String())
	}
	return body.Error
}

func TestPollRequiresAuthentication(t *testing.T) {
	poller := &fakePoller{}
	srv := newPollTestServer(poller)

	rec := doPoll(t, srv, `{}`, false)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body: %s", rec.Code, rec.Body.String())
	}
	if len(poller.requests) != 0 {
		t.Fatalf("unauthenticated request reached the poller: %v", poller.requests)
	}
}

func TestPollSelectsImages(t *testing.T) {
	tests := []struct {
		name string
		body string
		want poll.PollRequest
	}{
		{"empty body polls all", ``, poll.PollRequest{}},
		{"empty object polls all", `{}`, poll.PollRequest{}},
		{"identifier", `{"identifier":"deployment/homeassistant/music-assistant"}`, poll.PollRequest{Identifier: "deployment/homeassistant/music-assistant"}},
		{"image", `{"image":"ghcr.io/music-assistant/server"}`, poll.PollRequest{Image: "ghcr.io/music-assistant/server"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			poller := &fakePoller{results: []poll.PollResult{
				{Image: "ghcr.io/music-assistant/server", Submitted: true, Tag: "2.5.0"},
				{Image: "index.docker.io/library/nginx", Error: "boom"},
			}}
			srv := newPollTestServer(poller)

			rec := doPoll(t, srv, tt.body, true)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
			}
			if len(poller.requests) != 1 || poller.requests[0] != tt.want {
				t.Fatalf("poller requests = %+v, want [%+v]", poller.requests, tt.want)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("content type = %q", ct)
			}

			// check the exact wire format the bot relies on
			var raw map[string][]map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
				t.Fatalf("failed to decode body %q: %s", rec.Body.String(), err)
			}
			results := raw["results"]
			if len(results) != 2 {
				t.Fatalf("results = %v", results)
			}
			want := map[string]interface{}{"image": "ghcr.io/music-assistant/server", "submitted": true, "tag": "2.5.0", "error": ""}
			if fmt.Sprint(results[0]) != fmt.Sprint(want) {
				t.Errorf("result[0] = %v, want %v", results[0], want)
			}
			if results[1]["error"] != "boom" || results[1]["submitted"] != false {
				t.Errorf("result[1] = %v", results[1])
			}
		})
	}
}

func TestPollEmptyResultsIsArray(t *testing.T) {
	srv := newPollTestServer(&fakePoller{})
	rec := doPoll(t, srv, `{}`, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"results":[]}` {
		t.Fatalf("body = %s", got)
	}
}

func TestPollNotWatched(t *testing.T) {
	poller := &fakePoller{err: fmt.Errorf("image %q: %w", "nginx", poll.ErrNotWatched)}
	srv := newPollTestServer(poller)

	rec := doPoll(t, srv, `{"image":"nginx"}`, true)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
	}
	decodePollError(t, rec)
}

func TestPollBadRequest(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"malformed json", `{`},
		{"wrong type", `{"image": 1}`},
		{"unknown field", `{"identifer":"deployment/default/wd"}`},
		{"both set", `{"identifier":"deployment/default/wd","image":"nginx"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			poller := &fakePoller{}
			srv := newPollTestServer(poller)

			rec := doPoll(t, srv, tt.body, true)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
			}
			decodePollError(t, rec)
			if len(poller.requests) != 0 {
				t.Fatalf("bad request reached the poller: %v", poller.requests)
			}
		})
	}
}

func TestPollAllAlreadyRunning(t *testing.T) {
	srv := newPollTestServer(&fakePoller{err: poll.ErrPollAllRunning})
	rec := doPoll(t, srv, `{}`, true)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429; body: %s", rec.Code, rec.Body.String())
	}
	decodePollError(t, rec)
}

func TestPollTriggerDisabled(t *testing.T) {
	srv := newPollTestServer(nil)
	rec := doPoll(t, srv, `{}`, true)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body: %s", rec.Code, rec.Body.String())
	}
	decodePollError(t, rec)
}
