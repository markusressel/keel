package poll

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/keel-hq/keel/types"
	"github.com/keel-hq/keel/util/image"

	log "github.com/sirupsen/logrus"
)

// Limits of an on-demand poll.
const (
	// DefaultPollImageTimeout bounds the check of a single image.
	DefaultPollImageTimeout = 30 * time.Second
	// DefaultPollTotalTimeout bounds a whole on-demand poll.
	DefaultPollTotalTimeout = 60 * time.Second
	// DefaultPollConcurrency is how many images are checked at once.
	DefaultPollConcurrency = 4
)

var (
	// ErrNotWatched is returned when the requested resource or image is not
	// watched by the poll trigger.
	ErrNotWatched = errors.New("not watched by the poll trigger")
	// ErrPollAllRunning is returned when a poll of every image is requested
	// while a previous one is still running.
	ErrPollAllRunning = errors.New("a poll of all images is already running")
	// ErrInvalidRequest is returned when both an identifier and an image are set.
	ErrInvalidRequest = errors.New("set either identifier or image, not both")
)

// pollOutcome is the result of one run of a poll job.
type pollOutcome struct {
	// submitted is true when an event was submitted to the providers
	submitted bool
	// tag is the newest tag (tag watcher) or digest (digest watcher) seen
	tag string
	err error
}

// pollJob is a job the cron runs on schedule and an on-demand poll runs now.
type pollJob interface {
	Run()
	check() pollOutcome
}

// PollRequest selects the images of an on-demand poll. At most one field may
// be set; an empty request polls every watched image.
type PollRequest struct {
	// Identifier is a resource identifier (<kind>/<namespace>/<name>)
	Identifier string
	// Image is an image repository, with or without tag
	Image string
}

// PollResult is the outcome of the on-demand poll of one watched image.
type PollResult struct {
	// Image is the watcher key: registry/name, plus :tag for watchers that
	// keep the tag (force policy with keel.sh/matchTag)
	Image     string
	Submitted bool
	Tag       string
	Error     string
}

type pollTarget struct {
	key     string
	details *watchDetails
}

// PollNow runs the poll jobs of the selected images right away and waits for
// them. It runs the same check as the scheduled job, so policies, approvals and
// notifications behave like on a scheduled poll.
func (w *RepositoryWatcher) PollNow(ctx context.Context, req PollRequest) ([]PollResult, error) {
	if req.Identifier != "" && req.Image != "" {
		return nil, ErrInvalidRequest
	}

	var (
		targets []pollTarget
		err     error
	)
	switch {
	case req.Identifier != "":
		targets, err = w.identifierTargets(req.Identifier)
	case req.Image != "":
		targets, err = w.imageTargets(req.Image)
	default:
		if !w.pollAllRunning.CompareAndSwap(false, true) {
			return nil, ErrPollAllRunning
		}
		defer w.pollAllRunning.Store(false)
		targets = w.allTargets()
	}
	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		"identifier": req.Identifier,
		"image":      req.Image,
		"images":     len(targets),
	}).Info("trigger.poll.RepositoryWatcher: on-demand poll requested")

	return w.runTargets(ctx, targets), nil
}

func (w *RepositoryWatcher) lookup(key string) (*watchDetails, bool) {
	w.watchedMu.RLock()
	defer w.watchedMu.RUnlock()
	details, ok := w.watched[key]
	return details, ok
}

func (w *RepositoryWatcher) allTargets() []pollTarget {
	w.watchedMu.RLock()
	defer w.watchedMu.RUnlock()
	targets := make([]pollTarget, 0, len(w.watched))
	for key, details := range w.watched {
		targets = append(targets, pollTarget{key: key, details: details})
	}
	sortTargets(targets)
	return targets
}

// identifierTargets returns the watchers of the images of a resource. Images
// the poll trigger tracks but has no watcher for (for example because the
// registry could not be reached when it was added) are returned without
// details, so they are reported as errors.
func (w *RepositoryWatcher) identifierTargets(identifier string) ([]pollTarget, error) {
	trackedImages, err := w.providers.TrackedImages()
	if err != nil {
		return nil, fmt.Errorf("failed to list tracked images: %w", err)
	}

	seen := map[string]bool{}
	var targets []pollTarget
	found := false
	for _, ti := range trackedImages {
		if ti.Identifier != identifier || ti.Trigger != types.TriggerTypePoll {
			continue
		}
		key := getImageIdentifier(ti.Image, ti.Policy.KeepTag())
		if seen[key] {
			continue
		}
		seen[key] = true
		details, ok := w.lookup(key)
		if ok {
			found = true
		} else {
			details = nil
		}
		targets = append(targets, pollTarget{key: key, details: details})
	}
	if !found {
		return nil, fmt.Errorf("resource %q: %w", identifier, ErrNotWatched)
	}
	sortTargets(targets)
	return targets, nil
}

// imageTargets returns the watchers of an image repository. Without a tag it
// matches every watcher of the repository; with a tag, watchers that keep the
// tag only match that tag.
func (w *RepositoryWatcher) imageTargets(img string) ([]pollTarget, error) {
	ref, err := image.Parse(img)
	if err != nil {
		return nil, fmt.Errorf("image %q: %w", img, ErrNotWatched)
	}
	repo := ref.Registry() + "/" + ref.ShortName()

	explicitTag := false
	cleaned := strings.TrimPrefix(strings.TrimPrefix(img, "https://"), "http://")
	if named, err := image.ParseNamed(cleaned); err == nil {
		explicitTag = !image.IsNameOnly(named)
	}

	w.watchedMu.RLock()
	var targets []pollTarget
	for key, details := range w.watched {
		match := key == repo
		if !match && strings.HasPrefix(key, repo+":") {
			match = !explicitTag || key == repo+":"+ref.Tag()
		}
		if match {
			targets = append(targets, pollTarget{key: key, details: details})
		}
	}
	w.watchedMu.RUnlock()

	if len(targets) == 0 {
		return nil, fmt.Errorf("image %q: %w", img, ErrNotWatched)
	}
	sortTargets(targets)
	return targets, nil
}

func sortTargets(targets []pollTarget) {
	sort.Slice(targets, func(i, j int) bool { return targets[i].key < targets[j].key })
}

// runTargets polls the targets with limited concurrency. Images that cannot
// start before the total timeout are skipped and reported as errors.
func (w *RepositoryWatcher) runTargets(ctx context.Context, targets []pollTarget) []PollResult {
	ctx, cancel := context.WithTimeout(ctx, w.pollTotalTimeout)
	defer cancel()

	results := make([]PollResult, len(targets))
	sem := make(chan struct{}, w.pollConcurrency)
	done := make(chan struct{}, len(targets))
	pending := 0

	for i, target := range targets {
		results[i].Image = target.key
		if target.details == nil || target.details.job == nil {
			results[i].Error = ErrNotWatched.Error()
			continue
		}

		acquired := false
		select {
		case sem <- struct{}{}:
			acquired = true
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			// the deadline may pass while waiting for a free slot
			if acquired {
				<-sem
			}
			results[i].Error = fmt.Sprintf("skipped: %s", ctx.Err())
			continue
		}

		pending++
		go func(i int, target pollTarget) {
			defer func() {
				<-sem
				done <- struct{}{}
			}()
			results[i] = w.pollOne(ctx, target)
		}(i, target)
	}

	for ; pending > 0; pending-- {
		<-done
	}
	return results
}

// pollOne runs the job of one image. When the job does not finish in time it
// keeps running in the background and the result reports the timeout.
func (w *RepositoryWatcher) pollOne(ctx context.Context, target pollTarget) PollResult {
	result := PollResult{Image: target.key}

	outcomes := make(chan pollOutcome, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				outcomes <- pollOutcome{err: fmt.Errorf("poll panicked: %v", r)}
			}
		}()
		outcomes <- target.details.job.check()
	}()

	timer := time.NewTimer(w.pollImageTimeout)
	defer timer.Stop()

	select {
	case outcome := <-outcomes:
		result.Submitted = outcome.submitted
		result.Tag = outcome.tag
		if outcome.err != nil {
			result.Error = outcome.err.Error()
		}
	case <-timer.C:
		result.Error = fmt.Sprintf("timed out after %s", w.pollImageTimeout)
	case <-ctx.Done():
		result.Error = fmt.Sprintf("aborted: %s", ctx.Err())
	}

	log.WithFields(log.Fields{
		"image":     result.Image,
		"submitted": result.Submitted,
		"tag":       result.Tag,
		"error":     result.Error,
	}).Info("trigger.poll.RepositoryWatcher: on-demand poll finished")

	return result
}
