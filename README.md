# kube-api-rewriter

## Description

Sometimes you need to use different versions of one controller (or operator) in one cluster.
This is problematic due to CRD differences. Or, you want to enable multitenancy on the CRD level.
The first solution is to rewrite CRD definitions and re-compile controller for each version (or tenant).
Often this is time-consuming and error-prone.

This project offers the proxy sidecar container that sits between the controller and a Kubernetes API and
rewrites CRDs on the fly.

## Install

TODO Improve

1. Create rules for your CRDs. Use loader package to add rules at runtime.
2. Compile kube-api-rewriter with additional Go file in cmd/kube-api-rewriter.
3. Re-compile controller with "only JSON payload" setting for go-client.
4. Change webhook services.
5. Add sidecar to the controller Pod
6. Configure go-client to use localhost as a Kubernetes API address.

## Features

It can rewrite:

1. Discovery requests.
2. CRDs.
3. CRs.
4. Internal Kubernetes resources (i.e. Pod, Deployments, etc.).
5. References in resources (i.e. ownerReferences, etc.).
6. Admission webhook payloads.
7. GET/UPDATE payloads.
8. Patches.
9. Payloads in watch streams.

## Configuration

Default method is to use environment variables.

### Client proxy

#### Listen settings

**CLIENT_PROXY_ADDRESS** — address to listen for incoming requests from the controller. Default is 127.0.0.1

**CLIENT_PROXY_PORT** — port to listen for incoming requests from the controller. Default is 23915.

**CLIENT_PROXY** — flag to disable client proxy. Set to "no" for testing purposes.

#### Target settings

Target is a Kubernetes API server. Use go-client environment variables, or in-cluster client will be initialized.

At least, set api-server address with the `KUBERNETES_MASTER` env.

### Webhook proxy

#### Listen settings

**WEBHOOK_PROXY_ADDRESS** — address to listen for incoming requests from the Kubernetes API server. Default is 0.0.0.0

**WEBHOOK_PROXY_PORT** — port to listen for incoming requests from the Kubernetes API server. Default is 24192.

#### Target settings

**WEBHOOK_ADDRESS** — address of the webhook in the controller. Webhook proxy is disabled if this address is empty.

**WEBHOOK_SERVER_NAME** — server name to use in TLS client.

**WEBHOOK_CERT_FILE** — file name with the certificate of the webhook server.

**WEBHOOK_KEY_NAME** — file name with the private key for the certificate.

### Logging

**LOG_LEVEL** — set logging level: debug, info, warn, error. Default is "info".

**LOG_FORMAT** — set logging format: json, text, or pretty. Default is "json".

**LOG_OUTPUT** — set logging output: stdout, stderr, or discard. Default is "stdout".

### Other

**MONITORING_BIND_ADDRESS** — address of the metrics server. Default is :9090.

**PPROF_BIND_ADDRESS** — address of the pprof server. Pprof is disabled if empty.

## History

**02.03.2026**

Extracted from [deckhouse/virtualization](https://github.com/deckhouse/kube-api-rewriter)
repo into a separate project.

**02.11.2024**

Initially created to use KubeVirt as a part of Deckhouse Virtualization Platform without
interfering with the original KubeVirt installation.

## Known limitations

1. No rewrite for grpc payloads.
2. Needs to write Go structures for configuring rewrites. 
3. Needs to re-compile target controller.
4. Still needs more sophisticated logging.

