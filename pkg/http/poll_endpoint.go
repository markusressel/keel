package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/keel-hq/keel/trigger/poll"
)

// maxPollRequestBytes bounds the body of a poll request.
const maxPollRequestBytes = 64 * 1024

// Poller runs the poll trigger's checks on demand.
type Poller interface {
	PollNow(ctx context.Context, req poll.PollRequest) ([]poll.PollResult, error)
}

// PollRequest selects the images to poll. Set at most one field; an empty
// request polls every image the poll trigger watches.
type PollRequest struct {
	// Identifier is a resource identifier as returned by GET /v1/resources
	Identifier string `json:"identifier,omitempty" example:"deployment/default/wd"`
	// Image is an image repository, with or without tag
	Image string `json:"image,omitempty" example:"ghcr.io/music-assistant/server"`
}

// PollResult is the outcome of polling one watched image.
type PollResult struct {
	// Image is the watched repository (registry/name), plus :tag for
	// watchers that keep the tag (force policy with keel.sh/matchTag)
	Image string `json:"image" example:"ghcr.io/music-assistant/server"`
	// Submitted is true when the poll found a newer image and submitted an
	// event to the providers
	Submitted bool `json:"submitted"`
	// Tag is the newest tag, or digest for keel.sh/matchTag watchers, the poll
	// saw; the current tag when nothing is newer, empty if unknown
	Tag string `json:"tag" example:"2.5.0"`
	// Error is empty when the poll succeeded
	Error string `json:"error"`
}

// PollResponse lists the outcome per polled image.
type PollResponse struct {
	Results []PollResult `json:"results"`
}

// PollErrorResponse describes why a poll request was refused.
type PollErrorResponse struct {
	Error string `json:"error"`
}

// pollHandler checks registries for newer images right away.
// @Summary Poll registries now
// @Description Runs the poll trigger's check for the selected images right away, the same check as a scheduled poll, so policies, approvals and notifications behave the same. Select images with either identifier (a resource from GET /v1/resources) or image (a repository, with or without tag); an empty body polls every watched image. Images are checked with limited concurrency, each bounded by a per-image timeout and the whole request by a total timeout. submitted is true when the poll found a newer image and submitted an event to the providers. tag is the newest tag (or digest, for keel.sh/matchTag watchers) the poll saw; when nothing is newer it is the current tag. A poll of every image is refused with 429 while a previous one is still running. This route exists only when the authenticator is enabled.
// @Tags Admin
// @ID pollNow
// @Accept json
// @Produce json
// @Security BasicAuth
// @Security BearerAuth
// @Param body body PollRequest false "Images to poll"
// @Success 200 {object} PollResponse
// @Failure 400 {object} PollErrorResponse "Malformed request or both fields set"
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {object} PollErrorResponse "Resource or image not watched by the poll trigger"
// @Failure 429 {object} PollErrorResponse "A poll of every image is already running"
// @Failure 500 {object} PollErrorResponse "Poll failed"
// @Failure 503 {object} PollErrorResponse "Poll trigger disabled"
// @Router /v1/poll [post]
func (s *TriggerServer) pollHandler(resp http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	body, err := io.ReadAll(io.LimitReader(req.Body, maxPollRequestBytes+1))
	if err != nil {
		pollError(resp, http.StatusBadRequest, fmt.Sprintf("failed to read request body: %s", err))
		return
	}
	if len(body) > maxPollRequestBytes {
		pollError(resp, http.StatusBadRequest, "request body too large")
		return
	}

	var pollReq PollRequest
	if len(bytes.TrimSpace(body)) > 0 {
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&pollReq); err != nil {
			pollError(resp, http.StatusBadRequest, fmt.Sprintf("invalid request body: %s", err))
			return
		}
	}

	if pollReq.Identifier != "" && pollReq.Image != "" {
		pollError(resp, http.StatusBadRequest, poll.ErrInvalidRequest.Error())
		return
	}

	if s.poller == nil {
		pollError(resp, http.StatusServiceUnavailable, "poll trigger is disabled")
		return
	}

	results, err := s.poller.PollNow(req.Context(), poll.PollRequest{
		Identifier: pollReq.Identifier,
		Image:      pollReq.Image,
	})
	switch {
	case err == nil:
	case errors.Is(err, poll.ErrNotWatched):
		pollError(resp, http.StatusNotFound, err.Error())
		return
	case errors.Is(err, poll.ErrPollAllRunning):
		pollError(resp, http.StatusTooManyRequests, err.Error())
		return
	case errors.Is(err, poll.ErrInvalidRequest):
		pollError(resp, http.StatusBadRequest, err.Error())
		return
	default:
		pollError(resp, http.StatusInternalServerError, err.Error())
		return
	}

	out := PollResponse{Results: make([]PollResult, 0, len(results))}
	for _, r := range results {
		out.Results = append(out.Results, PollResult{
			Image:     r.Image,
			Submitted: r.Submitted,
			Tag:       r.Tag,
			Error:     r.Error,
		})
	}
	writeJSON(resp, http.StatusOK, out)
}

func pollError(resp http.ResponseWriter, status int, msg string) {
	writeJSON(resp, status, PollErrorResponse{Error: msg})
}

func writeJSON(resp http.ResponseWriter, status int, obj interface{}) {
	resp.Header().Set("Content-Type", "application/json")
	resp.WriteHeader(status)
	json.NewEncoder(resp).Encode(obj)
}
