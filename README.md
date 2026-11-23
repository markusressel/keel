<p align="center">
  <a href="https://keel.sh" target="_blank"><img width="100"src="https://keel.sh/img/logo.png"></a>
</p>

<p align="center">
  <a href="https://goreportcard.com/report/github.com/markusressel/keel">
    <img src="https://goreportcard.com/badge/github.com/markusressel/keel" alt="Go Report">
  </a>
</p>

# Fork Disclaimer

This is a fork of the original Keel project. The original project can be found [here](https://github.com/keel-hq/keel).
This fork is intended to provide a more up-2-date and possibly more feature rich version of Keel, since the original
project seems to have been abandoned a long time ago. To be able to do that the scope of the project has
been limited slightly by removing some features as described below.

Feel free to open issues and PRs to improve this fork, I will try to keep it up to date with the original project.
Keep in mind though that I am not the original author of Keel and I am not affiliated with the original project in any
way. I also do not have the resources to provide support for this fork like a full-blown company, so please do 
not contact me directly.

This README and maybe also other files in this Project still point to the original project, since it provides
a lot of resources like the official documentation. While it might be feasible to migrate this at some point,
most of these links still hold true and relevant information, so they will stay until there is a need to
change something. If that is the case, please open an Issue to let me know, or better yet open a PR to address it.

## Added, Changed and Removed Features

Most if not all changes made to this project are intended to make it easier to maintain for a single person.
Since the project is already quite feature rich, most of the updates to this repo will be focused on stability,
security (through dependency updates) and maintainability.

### Added Features

- CI/CD
    - This project now utilizes GitHub Actions to builds and publish the `ghcr.io/markusressel/keel:latest` Docker container.
- Approvals only for major (or major and minor) updates
    - `keel.sh/approvalsFor` limits the approvals of `keel.sh/approvals` to the larger version changes, see
      [Approvals only for larger updates](#approvals-only-for-larger-updates).
- Poll on demand
    - `POST /v1/poll` checks the registries right away instead of waiting for `keel.sh/pollSchedule`, see
      [Poll on demand](#poll-on-demand).

### Canged Features

- API
    - The REST API of the original project is extended (for example by `POST /v1/poll`), existing endpoints are
      unchanged. All endpoints are described in [docs/swagger.yaml](docs/swagger.yaml).
    - Since I do not intend to support a ton of external services like Slack or Discord, this API is the main
      way to interact with Keel and come up with your own service to provide bots and notifications. If you
      would like to create such a service, take a look at the [keel-telegram-bot](https://github.com/markusressel/keel-telegram-bot) project.

### Removed Features

- Bots
    - All of the Bots have been removed from this project.
      The reason for this is that supporting them is out of scope for me and the REST API and webhooks makes it easy to
      provide them using third party containers. This makes it possible to share the maintenance burden with the community.
      Check out my own Telegram specific bot: [keel-telegram-bot](https://github.com/markusressel/keel-telegram-bot)
- Notification Providers
    - All of Notification Providers except the Webhook one have been removed from this project.
      The reason for this is that supporting them is out of scope for me and the webhook integration makes it easy to
      provide them using third party containers. Check out my Telegram specific one here: [keel-telegram-bot](https://github.com/markusressel/keel-telegram-bot)
- Helm Provider
    - While I have not yet removed the support for Helm, I am not using it myself and might remove support for it in the
      future.

# Keel - automated Kubernetes deployments for the rest of us

Keel is a tool for automating [Kubernetes](https://kubernetes.io/) deployment updates. Keel is stateless, robust and
lightweight.

Keel provides several key features:

* __[Kubernetes](https://kubernetes.io/) and [Helm](https://helm.sh) providers__ - Keel has direct integrations with
  Kubernetes and Helm.

* __REST API__ - Keel provides a REST API for other services to integrate with it, like f.ex. bots or other external interfaces.

* __No CLI/API__ - tired of `f***ctl` for everything? Keel doesn't have one. Gets job done through labels, annotations,
  charts.

* __Semver policies__ - specify update policy for each deployment/Helm release individually.

* __Automatic [Google Container Registry](https://cloud.google.com/container-registry/) configuration__ - Keel
  automatically sets up topic and subscriptions for your deployment images by periodically scanning your environment.

* __[Native, DockerHub, Quay and Azure container registry webhooks](https://keel.sh/docs/#triggers) support__ - once
  webhook is received impacted deployments will be identified and updated.

* __[Polling](https://keel.sh/docs/#polling)__ - when webhooks and pubsub aren't available - Keel can still be useful by
  checking Docker Registry for new tags (if current tag is semver) or same tag SHA digest change (ie: `latest`).

* __Notifications__ - out of the box this Fork has standard webhook support. More sophisticated notifications
  can be provided by third party services integrating with the REST API.

<p align="center">
  <a href="https://keel.sh" target="_blank"><img width="700"src="https://keel.sh/img/keel_high_level.png"></a>
</p>

### Quick Start

<p align="center">
  <a href="https://keel.sh" target="_blank"><img width="700"src="https://keel.sh/img/examples/force-workflow.png"></a>
</p>

A step-by-step guide to install Keel on your Kubernetes cluster is viewable on the Keel website:

[https://keel.sh/examples/#example-1-push-to-deploy](https://keel.sh/examples/#example-1-push-to-deploy)

### Docker

This repository currently only provides a Docker container

```bash
sudo docker run -t --rm --name keel ghcr.io/markusressel/keel:latest
```


### Configuration

Once Keel is deployed, you only need to specify update policy on your deployment file or Helm chart:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: wd
  namespace: default
  labels:
    name: "wd"
  annotations:
    keel.sh/policy: minor # <-- policy name according to https://semver.org/
    keel.sh/trigger: poll # <-- actively query registry, otherwise defaults to webhooks
spec:
  template:
    metadata:
      name: wd
      labels:
        app: wd
    spec:
      containers:
        - image: karolisr/webhook-demo:0.0.8
          imagePullPolicy: Always
          name: wd
          command: [ "/bin/webhook-demo" ]
          ports:
            - containerPort: 8090
```

No additional configuration is required. Enabling continuous delivery for your workloads has never been this easy!

### Approvals only for larger updates

`keel.sh/approvals: "N"` asks for N approvals before every update. Add `keel.sh/approvalsFor` to ask only
for the larger version changes and apply the smaller ones directly:

```yaml
  annotations:
    keel.sh/policy: all
    keel.sh/trigger: poll
    keel.sh/approvals: "1"
    keel.sh/approvalsFor: major
```

| `keel.sh/approvalsFor` | Approvals for                                    | Applied directly                      |
|------------------------|--------------------------------------------------|---------------------------------------|
| unset or `all`         | every update (the behaviour without the setting) | nothing                               |
| `minor`                | major and minor changes                          | patch, fourth number, pre-release     |
| `major`                | major changes                                    | minor, patch, fourth number, pre-release |

- Keel compares the current and the new tag as versions: `X.Y`, `X.Y.Z` or `X.Y.Z.R`, with an optional `v`
  prefix, pre-release and build metadata. `21.0.0.0 -> 21.0.0.1` and `1.2.5 -> 1.2.5.1` are below a patch change.
- A downgrade counts like an upgrade of the same size, `2.0.0 -> 1.9.9` is a major change.
- When either tag is not a version (`latest`, a bare number like `42`, build tags of a `glob:`, `regexp:` or
  `force` policy), the size of the change is unknown, and Keel asks for approvals as without the setting.
- A new digest of the same version tag (`1.2.3 -> 1.2.3`) is not a version change and is applied directly.
  A new digest of `latest` still needs approvals.
- An unknown value is logged as a warning and Keel asks for approvals on every update.
- A direct update goes the same way as one with `keel.sh/approvals: "0"`: no approval request, and the usual
  update notifications (for example the webhook notification "update resource" on success).
- The setting has no effect without `keel.sh/approvals`. Like the other settings, it can be a label or an
  annotation. For Helm charts, set `keel.approvalsFor` next to `keel.approvals` in the values.
- Versions below 1.0.0 are not treated specially: `0.4.0 -> 0.5.0` is a minor change.

### Poll on demand

The poll trigger checks each image on its `keel.sh/pollSchedule`. `POST /v1/poll` runs that check right away,
for example from a bot. It needs the same authentication as the other `/v1` endpoints (basic auth or a token)
and exists only when authentication is enabled.

```shell
# every image of one resource, the identifier as returned by GET /v1/resources
curl -u admin:password -X POST http://keel:9300/v1/poll -d '{"identifier": "deployment/default/wd"}'
# one image repository, with or without tag; docker.io short forms like nginx work too
curl -u admin:password -X POST http://keel:9300/v1/poll -d '{"image": "ghcr.io/music-assistant/server"}'
# every image the poll trigger watches
curl -u admin:password -X POST http://keel:9300/v1/poll
```

Set `identifier` or `image`, not both; an empty body or `{}` polls every watched image. The response lists one
result per watched image:

```json
{"results": [{"image": "ghcr.io/music-assistant/server", "submitted": true, "tag": "2.5.0", "error": ""}]}
```

- `image` is the image as the poll trigger watches it: `registry/name`, plus `:tag` for `force` policies with
  `keel.sh/matchTag`, which watch one tag. Docker Hub images show as `index.docker.io/library/nginx`.
- `submitted` is true when the poll found a newer image and handed it to Keel. Whether that updates the workload
  directly or creates an approval request follows the usual settings.
- `tag` is the newest tag the poll saw (the current tag when nothing is newer), or the digest for watchers of one tag.
- `error` is empty when the check worked, otherwise the registry error, a timeout, or `not watched by the poll trigger`
  for an image of the resource that has no watcher yet.

The check is the one the schedule runs, so policies, `keel.sh/matchTag`, `keel.sh/approvals`,
`keel.sh/approvalsFor` and notifications behave as on a scheduled poll. The request waits for the checks: four images
at a time, each for up to 30 seconds, the whole request up to 60 seconds. Images not checked by then report an error.
A check that runs into its timeout carries on in the background.

| Status | When |
|--------|------|
| 200 | checks done, see `results` |
| 400 | invalid JSON, an unknown field, or both `identifier` and `image` set |
| 401 | not authenticated |
| 404 | the resource or image is not watched by the poll trigger |
| 429 | a poll of every image was requested while the previous one still runs |
| 503 | the poll trigger is disabled (`POLL=false`) |

Error responses are JSON: `{"error": "..."}`.

### (Official) Documentation

Documentation is viewable on the Keel Website:

[https://keel.sh/docs/#introduction](https://keel.sh/docs/#introduction)

### Contributing

Before starting to work on some big or medium features - raise an issue [here](https://github.com/keel-hq/keel/issues)
so we can coordinate our efforts.

We use pull requests, so:

1. Fork this repository
2. Create a branch on your local copy with a sensible name
3. Push to your fork and open a pull request

### Developing Keel

If you wish to work on Keel itself, you will need Go 1.12+ installed. Make sure you put Keel into correct Gopath and
`go build` (dependency management is done through [dep](https://github.com/golang/dep)).

To test Keel while developing:

1. Launch a Kubernetes cluster like Minikube or Docker for Mac with Kubernetes.
2. Change config to use it: `kubectl config use-context docker-for-desktop`
3. Build Keel from `cmd/keel` directory.
4. Start Keel with: `keel --no-incluster`. This will use Kubeconfig from your home.

### Running unit tests

Get a test parser (makes output nice):

```bash
go get github.com/mfridman/tparse
```

To run unit tests:

```bash
make test
```

### Running e2e tests

Prerequisites:

- configured kubectl + kubeconfig
- a running cluster (test suite will create testing namespaces and delete them after tests)
- Go environment (will compile Keel before running)

Once prerequisites are ready:

```bash
make e2e
```
