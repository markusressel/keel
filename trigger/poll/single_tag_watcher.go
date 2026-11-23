package poll

import (
	"github.com/keel-hq/keel/extension/credentialshelper"
	"github.com/keel-hq/keel/provider"
	"github.com/keel-hq/keel/registry"
	"github.com/keel-hq/keel/types"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

// WatchTagJob - Watch specific tag job
type WatchTagJob struct {
	providers      provider.Providers
	registryClient registry.Client
	details        *watchDetails
}

// NewWatchTagJob - new watch tag job monitors specific tag by checking digest based on specified
// cron style schedule
func NewWatchTagJob(providers provider.Providers, registryClient registry.Client, details *watchDetails) *WatchTagJob {
	return &WatchTagJob{
		providers:      providers,
		registryClient: registryClient,
		details:        details,
	}
}

// Run - main function to check schedule
func (j *WatchTagJob) Run() {
	j.check()
}

// check compares the tag's registry digest with the last known one and
// submits an event when it changed. Scheduled and on-demand polls both run it.
func (j *WatchTagJob) check() pollOutcome {
	j.details.runMu.Lock()
	defer j.details.runMu.Unlock()

	reg := j.details.trackedImage.Image.Scheme() + "://" + j.details.trackedImage.Image.Registry()
	registryOpts := registry.Opts{
		Registry: reg,
		Name:     j.details.trackedImage.Image.ShortName(),
		Tag:      j.details.trackedImage.Image.Tag(),
	}

	creds, err := credentialshelper.GetCredentials(j.details.trackedImage)
	if err == nil {
		registryOpts.Username = creds.Username
		registryOpts.Password = creds.Password
	}

	currentDigest, err := j.registryClient.Digest(registryOpts)

	registriesScannedCounter.With(prometheus.Labels{"registry": j.details.trackedImage.Image.Registry(), "image": j.details.trackedImage.Image.Repository()}).Inc()

	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"image": j.details.trackedImage.Image.String(),
		}).Error("trigger.poll.WatchTagJob: failed to check digest")
		return pollOutcome{err: err}
	}

	log.WithFields(log.Fields{
		"current_digest": j.details.digest,
		"new_digest":     currentDigest,
		"registry_url":   reg,
		"image":          j.details.trackedImage.Image.String(),
	}).Debug("trigger.poll.WatchTagJob: checking digest")

	// checking whether image digest has changed
	if j.details.digest != currentDigest {
		// updating digest
		j.details.digest = currentDigest

		event := types.Event{
			Repository: types.Repository{
				Name:   j.details.trackedImage.Image.Repository(),
				Tag:    j.details.trackedImage.Image.Tag(),
				Digest: currentDigest,
			},
			TriggerName: types.TriggerTypePoll.String(),
		}
		log.WithFields(log.Fields{
			"image":      j.details.trackedImage.Image.String(),
			"new_digest": currentDigest,
		}).Info("trigger.poll.WatchTagJob: digest change detected, submiting event to providers")

		// j.providers.Submit(event)
		err := j.providers.Submit(event)
		if err != nil {
			log.WithFields(log.Fields{
				"repository": j.details.trackedImage.Image.Repository(),
				"digest":     currentDigest,
				"error":      err,
			}).Error("trigger.poll.WatchRepositoryTagsJob: error while submitting an event")
			return pollOutcome{tag: currentDigest, err: err}
		}
		return pollOutcome{submitted: true, tag: currentDigest}
	}

	return pollOutcome{tag: currentDigest}
}
