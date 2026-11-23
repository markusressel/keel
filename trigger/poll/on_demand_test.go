package poll

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/keel-hq/keel/approvals"
	"github.com/keel-hq/keel/internal/policy"
	"github.com/keel-hq/keel/provider"
	"github.com/keel-hq/keel/types"
	"github.com/keel-hq/keel/util/image"
)

func newOnDemandWatcher(t *testing.T, fp *fakeProvider, frc *fakeRegistryClient) *RepositoryWatcher {
	t.Helper()
	store, teardown := newTestingUtils()
	t.Cleanup(teardown)
	am := approvals.New(&approvals.Opts{Store: store})
	providers := provider.New([]provider.Provider{fp}, am)
	watcher := NewRepositoryWatcher(providers, frc)
	// the fake registry client is not safe for concurrent use
	watcher.pollConcurrency = 1
	return watcher
}

func trackedPollImage(t *testing.T, img, identifier string, plc policy.Policy) *types.TrackedImage {
	t.Helper()
	ref, err := image.Parse(img)
	if err != nil {
		t.Fatalf("failed to parse %s: %s", img, err)
	}
	return &types.TrackedImage{
		Image:        ref,
		Identifier:   identifier,
		Trigger:      types.TriggerTypePoll,
		PollSchedule: types.KeelPollDefaultSchedule,
		Policy:       plc,
	}
}

func cronJob(t *testing.T, w *RepositoryWatcher, key string) interface{ Run() } {
	t.Helper()
	for _, entry := range w.cron.Entries() {
		if entry.Name == key {
			return entry.Job
		}
	}
	t.Fatalf("no cron entry %q", key)
	return nil
}

// TestPollNowRunsScheduledJob checks that an on-demand poll runs the very job
// the cron schedules, and that both produce the same event.
func TestPollNowRunsScheduledJob(t *testing.T) {
	fp := &fakeProvider{images: []*types.TrackedImage{
		trackedPollImage(t, "foo/bar:1.1.0", "deployment/default/bar", policy.NewSemverPolicy(policy.SemverPolicyTypeAll, true)),
	}}
	frc := &fakeRegistryClient{tagsToReturn: []string{"1.1.0"}}
	watcher := newOnDemandWatcher(t, fp, frc)

	if err := watcher.Watch(fp.images...); err != nil {
		t.Fatalf("watch failed: %s", err)
	}
	if len(fp.submitted) != 0 {
		t.Fatalf("unexpected events on watch: %v", fp.submitted)
	}

	key := "index.docker.io/foo/bar"
	details, ok := watcher.lookup(key)
	if !ok {
		t.Fatalf("image not watched under %q", key)
	}
	scheduled := cronJob(t, watcher, key)
	if scheduled != details.job.(interface{ Run() }) {
		t.Fatalf("on-demand job %p differs from the scheduled job %p", details.job, scheduled)
	}

	// nothing newer yet
	results, err := watcher.PollNow(context.Background(), PollRequest{Image: "foo/bar"})
	if err != nil {
		t.Fatalf("poll failed: %s", err)
	}
	want := []PollResult{{Image: key, Tag: "1.1.0"}}
	if len(results) != 1 || results[0] != want[0] {
		t.Fatalf("results = %+v, want %+v", results, want)
	}

	// a newer tag is pushed
	frc.tagsToReturn = []string{"1.1.0", "1.2.0"}
	results, err = watcher.PollNow(context.Background(), PollRequest{Identifier: "deployment/default/bar"})
	if err != nil {
		t.Fatalf("poll failed: %s", err)
	}
	want = []PollResult{{Image: key, Submitted: true, Tag: "1.2.0"}}
	if len(results) != 1 || results[0] != want[0] {
		t.Fatalf("results = %+v, want %+v", results, want)
	}
	if len(fp.submitted) != 1 {
		t.Fatalf("expected one event, got %v", fp.submitted)
	}
	onDemand := fp.submitted[0]
	if onDemand.Repository.Name != key || onDemand.Repository.Tag != "1.2.0" || onDemand.TriggerName != types.TriggerTypePoll.String() {
		t.Fatalf("unexpected event: %+v", onDemand)
	}

	// the scheduled run submits the same event
	scheduled.Run()
	if len(fp.submitted) != 2 {
		t.Fatalf("expected the scheduled run to submit an event, got %v", fp.submitted)
	}
	if fp.submitted[1].Repository.Name != onDemand.Repository.Name || fp.submitted[1].Repository.Tag != onDemand.Repository.Tag || fp.submitted[1].TriggerName != onDemand.TriggerName {
		t.Fatalf("scheduled event %+v differs from on-demand event %+v", fp.submitted[1], onDemand)
	}
}

func TestPollNowDigestWatcher(t *testing.T) {
	fp := &fakeProvider{images: []*types.TrackedImage{
		trackedPollImage(t, "ghcr.io/music-assistant/server:latest", "deployment/homeassistant/music-assistant", policy.NewForcePolicy(true)),
	}}
	frc := &fakeRegistryClient{digestToReturn: "sha256:aaa"}
	watcher := newOnDemandWatcher(t, fp, frc)
	if err := watcher.Watch(fp.images...); err != nil {
		t.Fatalf("watch failed: %s", err)
	}

	key := "ghcr.io/music-assistant/server:latest"
	results, err := watcher.PollNow(context.Background(), PollRequest{Identifier: "deployment/homeassistant/music-assistant"})
	if err != nil {
		t.Fatalf("poll failed: %s", err)
	}
	if want := (PollResult{Image: key, Tag: "sha256:aaa"}); len(results) != 1 || results[0] != want {
		t.Fatalf("results = %+v, want %+v", results, want)
	}

	frc.digestToReturn = "sha256:bbb"
	results, err = watcher.PollNow(context.Background(), PollRequest{Image: "ghcr.io/music-assistant/server:latest"})
	if err != nil {
		t.Fatalf("poll failed: %s", err)
	}
	if want := (PollResult{Image: key, Submitted: true, Tag: "sha256:bbb"}); len(results) != 1 || results[0] != want {
		t.Fatalf("results = %+v, want %+v", results, want)
	}
	if len(fp.submitted) != 1 || fp.submitted[0].Repository.Digest != "sha256:bbb" {
		t.Fatalf("unexpected events: %v", fp.submitted)
	}
}

func TestPollNowRegistryError(t *testing.T) {
	fp := &fakeProvider{images: []*types.TrackedImage{
		trackedPollImage(t, "ghcr.io/foo/bar:latest", "deployment/default/bar", policy.NewForcePolicy(true)),
	}}
	frc := &fakeRegistryClient{digestToReturn: "sha256:aaa"}
	watcher := newOnDemandWatcher(t, fp, frc)
	if err := watcher.Watch(fp.images...); err != nil {
		t.Fatalf("watch failed: %s", err)
	}

	frc.digestErrToReturn = errors.New("registry down")
	results, err := watcher.PollNow(context.Background(), PollRequest{})
	if err != nil {
		t.Fatalf("poll failed: %s", err)
	}
	if len(results) != 1 || results[0].Error != "registry down" || results[0].Submitted {
		t.Fatalf("results = %+v", results)
	}
}

func TestPollNowImageMatching(t *testing.T) {
	semver := policy.NewSemverPolicy(policy.SemverPolicyTypeAll, true)
	fp := &fakeProvider{images: []*types.TrackedImage{
		trackedPollImage(t, "nginx:1.25.0", "deployment/default/nginx", semver),
		trackedPollImage(t, "ghcr.io/foo/app:main", "deployment/default/app-main", policy.NewForcePolicy(true)),
		trackedPollImage(t, "ghcr.io/foo/app:dev", "deployment/default/app-dev", policy.NewForcePolicy(true)),
		trackedPollImage(t, "ghcr.io/foo/application:1.0.0", "deployment/default/application", semver),
	}}
	frc := &fakeRegistryClient{digestToReturn: "sha256:aaa", tagsToReturn: []string{"1.0.0", "1.25.0"}}
	watcher := newOnDemandWatcher(t, fp, frc)
	if err := watcher.Watch(fp.images...); err != nil {
		t.Fatalf("watch failed: %s", err)
	}

	tests := []struct {
		image string
		want  []string
	}{
		{"nginx", []string{"index.docker.io/library/nginx"}},
		{"library/nginx", []string{"index.docker.io/library/nginx"}},
		{"docker.io/library/nginx:1.25.0", []string{"index.docker.io/library/nginx"}},
		{"index.docker.io/library/nginx", []string{"index.docker.io/library/nginx"}},
		{"ghcr.io/foo/app", []string{"ghcr.io/foo/app:dev", "ghcr.io/foo/app:main"}},
		{"ghcr.io/foo/app:dev", []string{"ghcr.io/foo/app:dev"}},
		{"ghcr.io/foo/application", []string{"ghcr.io/foo/application"}},
	}
	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			results, err := watcher.PollNow(context.Background(), PollRequest{Image: tt.image})
			if err != nil {
				t.Fatalf("poll failed: %s", err)
			}
			var got []string
			for _, r := range results {
				got = append(got, r.Image)
			}
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("matched %v, want %v", got, tt.want)
			}
		})
	}

	for _, img := range []string{"ghcr.io/foo/ap", "ghcr.io/foo/app:other", "redis", "::"} {
		if _, err := watcher.PollNow(context.Background(), PollRequest{Image: img}); !errors.Is(err, ErrNotWatched) {
			t.Errorf("image %q: err = %v, want ErrNotWatched", img, err)
		}
	}
	if _, err := watcher.PollNow(context.Background(), PollRequest{Identifier: "deployment/default/unknown"}); !errors.Is(err, ErrNotWatched) {
		t.Errorf("unknown identifier: err = %v, want ErrNotWatched", err)
	}
	if _, err := watcher.PollNow(context.Background(), PollRequest{Identifier: "x", Image: "nginx"}); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("both set: err = %v, want ErrInvalidRequest", err)
	}

	results, err := watcher.PollNow(context.Background(), PollRequest{})
	if err != nil {
		t.Fatalf("poll all failed: %s", err)
	}
	if len(results) != 4 {
		t.Fatalf("poll all returned %d results: %+v", len(results), results)
	}
}

// blockingJob blocks every check until released.
type blockingJob struct {
	started chan struct{}
	release chan struct{}
}

func (j *blockingJob) Run() { j.check() }

func (j *blockingJob) check() pollOutcome {
	j.started <- struct{}{}
	<-j.release
	return pollOutcome{tag: "1.0.0"}
}

func TestPollNowAllRateLimit(t *testing.T) {
	watcher := NewRepositoryWatcher(nil, nil)
	job := &blockingJob{started: make(chan struct{}, 10), release: make(chan struct{})}
	watcher.addWatched("ghcr.io/foo/bar", &watchDetails{}, job)

	done := make(chan []PollResult)
	go func() {
		results, err := watcher.PollNow(context.Background(), PollRequest{})
		if err != nil {
			t.Errorf("first poll failed: %s", err)
		}
		done <- results
	}()
	<-job.started

	if _, err := watcher.PollNow(context.Background(), PollRequest{}); !errors.Is(err, ErrPollAllRunning) {
		t.Fatalf("second poll all: err = %v, want ErrPollAllRunning", err)
	}

	// a targeted poll is not limited
	targeted := make(chan error)
	go func() {
		_, err := watcher.PollNow(context.Background(), PollRequest{Image: "ghcr.io/foo/bar"})
		targeted <- err
	}()

	close(job.release)
	if results := <-done; len(results) != 1 || results[0].Tag != "1.0.0" {
		t.Fatalf("first poll results = %+v", results)
	}
	if err := <-targeted; err != nil {
		t.Fatalf("targeted poll failed: %s", err)
	}

	// the limit is lifted once the poll finished
	if _, err := watcher.PollNow(context.Background(), PollRequest{}); err != nil {
		t.Fatalf("poll all after the first finished: %s", err)
	}
}

func TestPollNowTimeouts(t *testing.T) {
	watcher := NewRepositoryWatcher(nil, nil)
	watcher.pollImageTimeout = 20 * time.Millisecond
	watcher.pollConcurrency = 1
	job := &blockingJob{started: make(chan struct{}, 10), release: make(chan struct{})}
	defer close(job.release)
	watcher.addWatched("ghcr.io/foo/a", &watchDetails{}, job)
	watcher.addWatched("ghcr.io/foo/b", &watchDetails{}, job)

	results, err := watcher.PollNow(context.Background(), PollRequest{})
	if err != nil {
		t.Fatalf("poll failed: %s", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %+v", results)
	}
	for _, r := range results {
		if !strings.HasPrefix(r.Error, "timed out") {
			t.Errorf("result %+v, want a timeout", r)
		}
	}

	// the total timeout stops images that have not started
	watcher.pollImageTimeout = time.Minute
	watcher.pollTotalTimeout = 20 * time.Millisecond
	results, err = watcher.PollNow(context.Background(), PollRequest{})
	if err != nil {
		t.Fatalf("poll failed: %s", err)
	}
	if !strings.HasPrefix(results[0].Error, "aborted") || !strings.HasPrefix(results[1].Error, "skipped") {
		t.Fatalf("results = %+v, want first aborted and second skipped", results)
	}
}
