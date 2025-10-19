# How to contribute

Developer documentation for Altinn 3 Kubernetes operators.

Here are some important resources:

* [Team Apps Github board](https://github.com/orgs/Altinn/projects/39/views/2)
* [Altinn Studio docs](https://docs.altinn.studio/)
* Self service API docs: https://docs.digdir.no/docs/idporten/oidc/oidc_api_admin.html
* Self service API dev Swagger UI: https://api.samarbeid.digdir.dev/swagger-ui/index.html?urls.primaryName=External%20OIDC

## Reporting Issues

Open [our Github issue tracker](https://github.com/Altinn/altinn-k8s-operator/issues/new/choose)
and choose an appropriate issue template.

Feel free to query existing issues before creating a new one.

## Contributing changes

### Local development

#### Prerequisites
- go version v1.22.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

#### Run tests

```sh
make # build

make test # runs local tests (doesn't need k8s etc)

make test-e2e # runs e2e tests, requires kind
```

We use [go-snaps](https://github.com/gkampitakis/go-snaps) for snapshot tests.
[Update snapshots](https://github.com/gkampitakis/go-snaps?tab=readme-ov-file#update-snapshots) by running

```sh
UPDATE_SNAPS=true make test
```

#### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/altinn-k8s-operator:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/altinn-k8s-operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

#### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following are the steps to build the installer and distribute this project to users.

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/altinn-k8s-operator:tag
```

NOTE: The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without
its dependencies.

2. Using the installer

Users can just run kubectl apply -f <URL for YAML BUNDLE> to install the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/altinn-k8s-operator/<tag or branch>/dist/install.yaml
```

### Contributing

// TODO: doc

### JSON schema -> Go structs

The Swagger UI is at: https://api.samarbeid.digdir.dev/swagger-ui/index.html#/
OpenAPI spec at: https://api.samarbeid.digdir.dev/v3/api-docs/altinn-admin

Download to `schemas/spec.json`, tell AI to write the models, update the client and fakes.

### Upgrading

If kubebuilder bumps major version, in some cases there is not much to do. Still, it might be worth it to

* Upgrade kubebuilder CLI
* Scaffold a new project
* Generate some CRD that we use
* Inspect diff with this repo
* Case-insensitive search for `version` to make sure hardcoded versions are up to date
* Run all builds, tests, lints etc.. 

That way we don't get stuck on old versions of the scaffold forever..
Example for v3 -> v4 upgrade of CLI:

```sh
mkdir altinn-k8s-operator2
cd altinn-k8s-operator2
kubebuilder init --plugins go/v4 --domain altinn.studio --owner "Altinn" --repo "github.com/Altinn/altinn-k8s-operator" --project-name "altinn-k8s-operator"
kubebuilder create api --group resources --version v1alpha1 --kind MaskinportenClient
make manifests
```
