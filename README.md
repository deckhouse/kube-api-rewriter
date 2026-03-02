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

