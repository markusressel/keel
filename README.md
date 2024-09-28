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
This fork is intended to provide a more stable and feature rich version of Keel, since the original
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

### Canged Features

- API
    - While the REST API of the original project is still there, it might be extended in the future.
    - Since I do not intend to support a ton of external services like Slack or Discord, this API is the main
      way to interact with Keel and come up with your own service to provide bots and notifications. If you
      would like to create such a service, take a look at the [keel-telegram-bot](https://github.com/markusressel/keel-telegram-bot) project.

### Removed Features

- Notification Providers
    - All of Notification Providers except the Webhook one have been removed from this project.
      The reason for this is that supporting them is out of scope for me and the webhook integration makes it easy to
      provide them using third party containers.
      I am doing the exact same thing for telegram using my [keel-telegram-bot](https://github.com/markusressel/keel-telegram-bot) project.
- Helm Provider
    - While I have not yet removed the support for Helm, I am not using it myself and might remove support for it in the
      future.

# Keel - automated Kubernetes deployments for the rest of us

Keel is a tool for automating [Kubernetes](https://kubernetes.io/) deployment updates. Keel is stateless, robust and
lightweight.

Keel provides several key features:

* __[Kubernetes](https://kubernetes.io/) and [Helm](https://helm.sh) providers__ - Keel has direct integrations with
  Kubernetes and Helm.

* __No CLI/API__ - tired of `f***ctl` for everything? Keel doesn't have one. Gets job done through labels, annotations,
  charts.

* __Semver policies__ - specify update policy for each deployment/Helm release individually.

* __Automatic [Google Container Registry](https://cloud.google.com/container-registry/) configuration__ - Keel
  automatically sets up topic and subscriptions for your deployment images by periodically scanning your environment.

* __[Native, DockerHub, Quay and Azure container registry webhooks](https://keel.sh/docs/#triggers) support__ - once
  webhook is received impacted deployments will be identified and updated.

* __[Polling](https://keel.sh/docs/#polling)__ - when webhooks and pubsub aren't available - Keel can still be useful by
  checking Docker Registry for new tags (if current tag is semver) or same tag SHA digest change (ie: `latest`).

* __Notifications__ - out of the box Keel has Slack, Hipchat, Mattermost and standard webhook notifications, more
  info [here](https://keel.sh/docs/#notifications)

<p align="center">
  <a href="https://keel.sh" target="_blank"><img width="700"src="https://keel.sh/img/keel_high_level.png"></a>
</p>

### Quick Start

<p align="center">
  <a href="https://keel.sh" target="_blank"><img width="700"src="https://keel.sh/img/examples/force-workflow.png"></a>
</p>

A step-by-step guide to install Keel on your Kubernetes cluster is viewable on the Keel website:

[https://keel.sh/examples/#example-1-push-to-deploy](https://keel.sh/examples/#example-1-push-to-deploy)

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

```bash
sudo docker run -t --rm --name keel ghcr.io/markusressel/keel:latest
```

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
