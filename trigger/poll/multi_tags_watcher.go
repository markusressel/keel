package poll

import (
	"fmt"
	"strings"

	"github.com/keel-hq/keel/extension/credentialshelper"
	"github.com/keel-hq/keel/provider"
	"github.com/keel-hq/keel/registry"
	"github.com/keel-hq/keel/types"

	"github.com/prometheus/client_golang/prometheus"

	log "github.com/sirupsen/logrus"
)

// WatchRepositoryTagsJob - watch all tags
type WatchRepositoryTagsJob struct {
	providers      provider.Providers
	registryClient registry.Client
	details        *watchDetails

	// latests map[string]string // a map of prerelease tags and their corresponding latest versions
}

// NewWatchRepositoryTagsJob - new tags watcher job
func NewWatchRepositoryTagsJob(providers provider.Providers, registryClient registry.Client, details *watchDetails) *WatchRepositoryTagsJob {
	return &WatchRepositoryTagsJob{
		providers:      providers,
		registryClient: registryClient,
		details:        details,
		// latests:        details.trackedImage.SemverPreReleaseTags,
	}
}

// Run - main function to check schedule
func (j *WatchRepositoryTagsJob) Run() {
	j.check()
}

// check lists the repository's tags and submits an event for every related
// workload that has a newer tag. Scheduled and on-demand polls both run it.
func (j *WatchRepositoryTagsJob) check() pollOutcome {
	j.details.runMu.Lock()
	defer j.details.runMu.Unlock()

	j.details.mu.RLock()
	defer j.details.mu.RUnlock()

	reg := j.details.trackedImage.Image.Scheme() + "://" + j.details.trackedImage.Image.Registry()
	if j.details.latest == "" {
		j.details.latest = j.details.trackedImage.Image.Tag()
	}

	registryOpts := registry.Opts{
		Registry: reg,
		Name:     j.details.trackedImage.Image.ShortName(),
		Tag:      j.details.latest,
	}

	creds, err := credentialshelper.GetCredentials(j.details.trackedImage)
	if err == nil {
		registryOpts.Username = creds.Username
		registryOpts.Password = creds.Password
	}

	repository, err := j.getRepository(registryOpts)

	if err != nil {
		log.WithFields(log.Fields{
			"error":        err,
			"registry_url": reg,
			"image":        j.details.trackedImage.Image.String(),
		}).Error("trigger.poll.WatchRepositoryTagsJob: failed to get repository")
		return pollOutcome{err: err}
	}

	registriesScannedCounter.With(prometheus.Labels{"registry": j.details.trackedImage.Image.Registry(), "image": j.details.trackedImage.Image.Repository()}).Inc()

	log.WithFields(log.Fields{
		"current_tag":     j.details.trackedImage.Image.Tag(),
		"repository_tags": repository.Tags,
		"image_name":      j.details.trackedImage.Image.Remote(),
	}).Debug("trigger.poll.WatchRepositoryTagsJob: checking tags")

	outcome := j.processTags(repository.Tags)
	if outcome.err != nil && !outcome.submitted {
		log.WithFields(log.Fields{
			"error":           outcome.err,
			"repository_tags": repository.Tags,
			"image":           j.details.trackedImage.Image.String(),
		}).Error("trigger.poll.WatchRepositoryTagsJob: failed to process tags")
	}
	return outcome
}

// getRepository lists the repository's tags. When every workload sharing this
// watcher opted in to keel.sh/pollTagsAfterCurrent, it lists only the tags
// pushed after each workload's current tag; otherwise it lists every tag.
func (j *WatchRepositoryTagsJob) getRepository(opts registry.Opts) (*registry.Repository, error) {
	afterTags := j.pollTagsAfter()
	if len(afterTags) == 0 {
		return j.registryClient.Get(opts)
	}

	repository := &registry.Repository{Name: opts.Name}
	seen := make(map[string]bool)
	for _, after := range afterTags {
		opts.After = after
		repo, err := j.registryClient.Get(opts)
		if err != nil {
			return nil, err
		}
		for _, tag := range repo.Tags {
			if !seen[tag] {
				seen[tag] = true
				repository.Tags = append(repository.Tags, tag)
			}
		}
	}
	return repository, nil
}

// pollTagsAfter returns the distinct current tags of the related workloads,
// or nil unless every one of them opted in to keel.sh/pollTagsAfterCurrent.
func (j *WatchRepositoryTagsJob) pollTagsAfter() []string {
	trackedImages, err := j.providers.TrackedImages()
	if err != nil {
		return nil
	}

	var afterTags []string
	seen := make(map[string]bool)
	for _, trackedImage := range getRelatedTrackedImages(j.details.trackedImage, trackedImages) {
		if !trackedImage.PollTagsAfterCurrent {
			return nil
		}
		tag := trackedImage.Image.Tag()
		if !seen[tag] {
			seen[tag] = true
			afterTags = append(afterTags, tag)
		}
	}
	return afterTags
}

func (j *WatchRepositoryTagsJob) computeEvents(tags []string) ([]types.Event, error) {
	trackedImages, err := j.providers.TrackedImages()
	if err != nil {
		return nil, err
	}

	events := []types.Event{}

	// This contains all tracked images that share the same imageIdentifier and thus, the same watcher
	allRelatedTrackedImages := getRelatedTrackedImages(j.details.trackedImage, trackedImages)
	platformCache := make(map[string][]types.Platform)
	platformErrorCache := make(map[string]error)
	diagnosedCandidates := make(map[string]bool)

	for _, trackedImage := range allRelatedTrackedImages {
		if trackedImage.PlatformErr != types.PlatformErrorNone || len(trackedImage.Platforms) == 0 {
			reason := trackedImage.PlatformErr
			if reason == types.PlatformErrorNone {
				reason = types.PlatformErrorWorkloadMetadata
			}
			fields := log.Fields{
				"image":  trackedImage.Image.Repository(),
				"reason": reason,
			}
			if reason == types.PlatformErrorNodeMetadata {
				fields["remediation"] = "verify Keel's service account can list core/v1 nodes and that each node reports status.nodeInfo.operatingSystem and architecture"
			}
			log.WithFields(fields).Warn("trigger.poll.WatchRepositoryTagsJob: skipping workload because its eligible platforms could not be established")
			continue
		}

		filteredTags := tags

		// The fact that they are related, does not mean they share the exact same Policy configuration, so wee need
		// to calculate the tags here for each image.
		filteredTags = trackedImage.Policy.Filter(tags)

		for _, tag := range filteredTags {

			update, err := trackedImage.Policy.ShouldUpdate(trackedImage.Image.Tag(), tag)
			if err != nil {
				continue
			}
			if update == false {
				continue
			}
			// When using tags watcher we rely completely on tag names to deal with updates.
			if trackedImage.Image.Tag() == tag {
				break
			}

			platforms, resolved := platformCache[tag]
			platformErr, failed := platformErrorCache[tag]
			if !resolved && !failed {
				platforms, platformErr = j.candidatePlatforms(trackedImage, tag)
				if platformErr != nil {
					platformErrorCache[tag] = platformErr
				} else {
					platformCache[tag] = platforms
				}
			}
			if platformErr != nil {
				if !diagnosedCandidates[tag] {
					log.WithFields(log.Fields{
						"error": platformErr,
						"image": trackedImage.Image.Repository(),
						"tag":   tag,
					}).Warn("trigger.poll.WatchRepositoryTagsJob: skipping candidate because its platform could not be established")
					diagnosedCandidates[tag] = true
				}
				continue
			}
			if !supportsRelatedWorkloads(platforms, tag, allRelatedTrackedImages) {
				if !diagnosedCandidates[tag] {
					log.WithFields(log.Fields{
						"candidate_platforms": platforms,
						"eligible_platforms":  relatedPlatforms(allRelatedTrackedImages),
						"image":               trackedImage.Image.Repository(),
						"tag":                 tag,
					}).Warn("trigger.poll.WatchRepositoryTagsJob: skipping candidate because it is incompatible with a related workload platform")
					diagnosedCandidates[tag] = true
				}
				continue
			}
			if !exists(tag, events) {
				event := types.Event{
					Repository: types.Repository{
						Name:             trackedImage.Image.Repository(),
						Tag:              tag,
						Platforms:        platforms,
						PlatformVerified: true,
					},
					TriggerName: types.TriggerTypePoll.String(),
				}
				events = append(events, event)
				break
			}
		}
	}

	log.WithFields(log.Fields{
		"current_tag": j.details.trackedImage.Image.Tag(),
		"image_name":  j.details.trackedImage.Image.Remote(),
	}).Debug("trigger.poll.WatchRepositoryTagsJob: events: ", events)

	return events, nil
}

func relatedPlatforms(trackedImages []*types.TrackedImage) []types.Platform {
	var result []types.Platform
	seen := make(map[types.Platform]struct{})
	for _, trackedImage := range trackedImages {
		for _, platform := range trackedImage.Platforms {
			if _, ok := seen[platform]; ok {
				continue
			}
			seen[platform] = struct{}{}
			result = append(result, platform)
		}
	}
	return result
}

func (j *WatchRepositoryTagsJob) candidatePlatforms(trackedImage *types.TrackedImage, tag string) ([]types.Platform, error) {
	opts := registry.Opts{
		Registry: trackedImage.Image.Scheme() + "://" + trackedImage.Image.Registry(),
		Name:     trackedImage.Image.ShortName(),
		Tag:      tag,
	}
	if creds, err := credentialshelper.GetCredentials(trackedImage); err == nil {
		opts.Username = creds.Username
		opts.Password = creds.Password
	}
	return j.registryClient.Platforms(opts)
}

func supportsRelatedWorkloads(candidatePlatforms []types.Platform, candidateTag string, trackedImages []*types.TrackedImage) bool {
	for _, trackedImage := range trackedImages {
		update, err := trackedImage.Policy.ShouldUpdate(trackedImage.Image.Tag(), candidateTag)
		if err != nil || !update || trackedImage.Image.Tag() == candidateTag {
			continue
		}
		if trackedImage.PlatformErr != types.PlatformErrorNone || len(trackedImage.Platforms) == 0 {
			return false
		}
		if !types.PlatformsSupportAll(candidatePlatforms, trackedImage.Platforms) {
			return false
		}
	}
	return true
}

func exists(tag string, events []types.Event) bool {
	for _, e := range events {
		if tag == e.Repository.Tag {
			return true
		}
	}
	return false
}

func getRelatedTrackedImages(ours *types.TrackedImage, all []*types.TrackedImage) []*types.TrackedImage {
	b := make([]*types.TrackedImage, 0, len(all))
	for _, x := range all {
		// A repository watcher may be shared, but webhook consumers must not
		// influence the tag selected by its polling job.
		if x.Trigger == types.TriggerTypePoll &&
			getImageIdentifier(x.Image, x.Policy.KeepTag()) == getImageIdentifier(ours.Image, ours.Policy.KeepTag()) {
			b = append(b, x)
		}
	}
	return b
}

// processTags submits an event per newer tag. The outcome's tag is the first
// submitted tag, or the tracked image's current tag when nothing is newer.
func (j *WatchRepositoryTagsJob) processTags(tags []string) pollOutcome {

	events, err := j.computeEvents(tags)
	if err != nil {
		return pollOutcome{err: err}
	}

	outcome := pollOutcome{tag: j.details.trackedImage.Image.Tag()}
	var submitErrs []string
	for i, e := range events {
		if i == 0 {
			outcome.tag = e.Repository.Tag
		}
		err = j.providers.Submit(e)
		if err != nil {
			log.WithFields(log.Fields{
				"repository": j.details.trackedImage.Image.Repository(),
				"new_tag":    e.Repository.Tag,
				"error":      err,
			}).Error("trigger.poll.WatchRepositoryTagsJob: error while submitting an event")
			submitErrs = append(submitErrs, fmt.Sprintf("%s: %s", e.Repository.Tag, err))
			continue
		}
		outcome.submitted = true
	}
	if len(submitErrs) > 0 {
		outcome.err = fmt.Errorf("failed to submit event(s): %s", strings.Join(submitErrs, ", "))
	}
	return outcome
}
