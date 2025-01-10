<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Ory Hydra Maester](#ory-hydra-maester)
  - [Prerequisites](#prerequisites)
  - [Design](#design)
  - [How to use it](#how-to-use-it)
    - [Command-line flags](#command-line-flags)
  - [Development](#development)
    - [Testing](#testing)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Ory Hydra Maester

⚠️ ⚠️ ⚠️

> Ory Hydra Maester is developed by the Ory community and is not actively
> maintained by Ory core maintainers due to lack of resources, time, and
> knolwedge. As such please be aware that there might be issues with the system.
> If you have ideas for better testing and development principles please open an
> issue or PR!

⚠️ ⚠️ ⚠️

This project contains a Kubernetes controller that uses Custom Resources (CR) to
manage Hydra Oauth2 clients. ORY Hydra Maester watches for instances of
`oauth2clients.hydra.ory.sh/v1alpha1` CR and creates, updates, or deletes
corresponding OAuth2 clients by communicating with ORY Hydra's API.

Visit Hydra-maester's
[chart documentation](https://github.com/ory/k8s/blob/master/docs/helm/hydra-maester.md)
and view [sample OAuth2 client resources](config/samples) to learn more about
the `oauth2clients.hydra.ory.sh/v1alpha1` CR.

The project is based on
[Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder).

## Prerequisites

- recent version of Go language with support for modules (e.g: 1.12.6)
- make
- kubectl
- kustomize
- [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) for running
  tests
- [ginkgo](https://onsi.github.io/ginkgo/) for local integration testing
- access to K8s environment: minikube or a remote K8s cluster
- [mockery](https://github.com/vektra/mockery) to generate mocks for testing
  purposes

## Design

Take a look at [Design Readme](./docs/README.md).

## How to use it

- `make test` to run tests
- `make test-integration` to run integration tests
- `make install` to generate CRD file from go sources and install it on the
  cluster
- `export HYDRA_URL={HYDRA_SERVICE_URL} && make run` to run the controller

To deploy the controller, edit the value of the `--hydra-url` argument in the
[manager.yaml](config/manager/manager.yaml) file and run `make deploy`.

### Command-line flags

| Name                         | Required | Description                                                                                                      | Default value | Example values                           |
| ---------------------------- | -------- | ---------------------------------------------------------------------------------------------------------------- | ------------- | ---------------------------------------- |
| **hydra-url**                | yes      | ORY Hydra's service address                                                                                      | -             | ` ory-hydra-admin.ory.svc.cluster.local` |
| **hydra-port**               | no       | ORY Hydra's service port                                                                                         | `4445`        | `4445`                                   |
| **endpoint**                 | no       | ORY Hydra's client endpoint                                                                                      | `/clients`    | `clients`                                |
| **tls-trust-store**          | no       | TLS cert path for hydra client                                                                                   | `""`          | `/etc/ssl/certs/ca-certificates.crt`     |
| **insecure-skip-verify**     | no       | Skip http client insecure verification                                                                           | `false`       | `true` or `false`                        |
| **namespace**                | no       | Namespace in which the controller should operate. Setting this will make the controller ignore other namespaces. | `""`          | `"my-namespace"`                         |
| **leader-elector-namespace** | no       | Leader elector namespace where controller should be set.                                                         | `""`          | `"my-namespace"`                         |

### Environmental Variables

| Variable name           | Default value       | Example value         |
| :---------------------- | ------------------- | --------------------- |
| `**CLIENT_ID_KEY**`     | `**CLIENT_ID**`     | `**MY_SECRET_NAME**`  |
| `**CLIENT_SECRET_KEY**` | `**CLIENT_SECRET**` | `**MY_SECRET_VALUE**` |

## Development

### Testing

Use mockery to generate mock types that implement existing interfaces. To
generate a mock type for an interface, navigate to the directory containing that
interface and run this command:

```
mockery -name={INTERFACE_NAME}
```
