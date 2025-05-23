# Beaver

```
   ____
  | __ )  ___  __ ___   _____ _ __
  |  _ \ / _ \/ _` \ \ / / _ \ '__|
  | |_) |  __/ (_| |\ V /  __/ |
  |____/ \___|\__,_| \_/ \___|_|

```

## Description

`beaver` is a tool to build your k8s templates in a descriptive way.

## Features

- template engine:
	- [helm](https://helm.sh/) charts
	- [ytt](https://carvel.dev/ytt/) charts
	- [kubectl create](https://kubernetes.io/docs/reference/generated/kubectl/kubectl-commands#create)
    - [kustomize](https://kustomize.io)
- patch engine:
	- [ytt overlay](https://carvel.dev/ytt/docs/v0.39.0/ytt-overlays/)
    - [kustomize](https://kustomize.io)
- multi-environment variables
- sha256 sum for any compiled resource can be used as variable
- inheritance between `beaver` project
- each built resource outputs its own file

## Usage

```
beaver build <path/to/beaver/project>
```

see `beaver build --help` for more options.

## Beaver project

A `beaver` project consists of a folder with a `beaver` config file,  either `beaver.yaml` or `beaver.yml`.

## Beaver config file

```yaml
# Default namespace used for this project
namespace: default
# the desired beaver version. If the binary you use to process this file has a
# different version number, it will refuse to process the project to avoid
# messing with your resources.
beaverversion: 3.2.3
# an inherited beaver project - which can also inherit another beaver project
# when starting your first beaver project you create a base file without inherit
# then you create some other file that inherits from your base to reflect
# some kind of environment change (production vs dev)
inherit: ../../base  # path is relative to this beaver config file
# you can also inherit from multiple bases at the same time
inherits:
- ../../base1
- ../../base2
# a beaver project is essentially a collection of charts (either helm or ytt)
# your project charts
charts:
  postgres:                           # your chart local name
    type: helm                        # can be either helm or ytt
    path: ../.vendor/helm/postgresql  # path to your chart - relative to this file
    name: pgsql                       # overwrite **helm** application name, cannot be used for ytt charts
    # Keyword `namespace` only available for Helm charts
    namespace: my-namespace           # Set namespace only for the current chart(Optional)


# You can define beaver variables
# they can be used inside your charts value files
# There are two methods
# First method :
# this method was the original one and is here for historical reasons
# but we highly recommend you use the second method which is nicer and
# will lead to better templates
variables:
- name: tag      # give your variable a name
  value: v1.2.3  # and a value


# Second method :
# which is the recommended way to go (implemented later)
variables:
  tag: v1.2.3
  my_dict1:
    my_key1: value1
    my_dict2:
      my_key2: value2
    my_list:
      - elem1
      - elem2


# You can also use inherit to overlay variables
# in project/base1/beaver.yaml :
variables:
  my_dict:
    key1: value1
    key2: value2


# If you want to redefine all the dict
# in project/base2/beaver.yaml :
inherit: ../base1
variables:
  my_dict:
    newKey1: value3

# Or if you want to redefine only part of the dict
inherit: ../base1
variables:
  my_dict.key1: value3

# resulting dict :
variables:
  my_dict:
    key1:value3
    key2: value1

# generate beaver variables from compiled resource file sha256
sha:
- key: configmap_demo               # use to generate beaver variable name
  resource: ConfigMap.v1.demo.yaml  # compiled resource filename

# create some resources using `kubectl create`
create:
- type: configmap       # resource kind as passed to kubectl create
  name: xbus-pipelines  # resource name
  args:                 # kubectl create arguments
  - flag: --from-file
    value: pipelines
```

## Value files

Value files filename uses the following format:

```
<chart_local_name>.[yaml,yml]
```

you can provide a value file for your chart using its local name, and `beaver`
will pass this file to your template engine.

If you have a value file with the same name inside an inherited project then
`beaver` will also pass this one, but prior to your project file. This ensures
that your current values overwrite inherited values.

example:

```
# folder structure
.
├── base
│   ├── beaver.yml
│   └── postgres.yml
└── environments
    └── demo
        ├── beaver.yml
        └── postgres.yaml
```
```yaml
# base/beaver.yml
charts:
  postgres:
    type: postgres
    path: ../.vendor/postgresql
```
```yaml
# environments/demo/beaver.yml
inherit: ../../base
namespace: demo
```

In the example above `beaver` will automaticaly pass `base/postgres.yml` and then
`environments/demo/postgres.yaml` to helm using `.vendor/postgresql` as chart
folder.

## Beaver variables

`beaver` variables can be used inside your value files, using the following syntax:

```
<[variable_name]>
```

example:
```yaml
# base/beaver.yaml
variables:
- name: pg_tag
  value: 14.4-alpine
charts:
  postgres:
    type: postgres
    path: ../.vendor/postgresql
```
```yaml
# base/postgres.yml
image:
  tag: <[pg_tag]>
```

`beaver` variables are merged during inheritance, example:

```yaml
# base/beaver.yaml
variables:
- name: pg_tag
  value: 14.4-alpine
```

```yaml
# environments/demo/beaver.yaml
inherit: ../../base
variables:
- name: pg_tag
  value: 13.7-alpine
```

here `pg_tag` value will be `13.7-alpine` if you run
`beaver build environments/demo`.

### Beaver variables in the beaver namespace itself

You can set some variables in the 'namespace' keyword of a beaver file.

```yaml
# example/base/beaver.yml
namespace: <[myns]>
charts:
  demoytt:
    type: ytt
    path: demoytt.tmpl.yaml
```

in this case this means this base is not useable by itself but can now be adapted by the caller by setting a beaver
variable to fill the slot

```yaml
# example/ns1/beaver.yml
namespace: ns1
inherit: ../base
variables:
- name: myns
  value: ns1yo
```

This is a somewhat warped example but in this case the resulting ouput dir (namespace for beaver) will be example/build/ns1yo

### variables inside the charts.disabled flag

imagine a base with a chart that is disabled by default

```yaml
# example/base/beaver.yml
namespace: example
charts:
  demoytt:
    type: ytt
    path: demoytt.tmpl.yaml
    disabled: <[configmapDisabled]>
variables:
- name: configmapDisabled
  value: true
```

and another file that inherits from this base and wants to have this chart enabled

```yaml
# example/configmapenabled/beaver.yml
namespace: ns1
inherit: ../base
variables:
- name: configmapDisabled
  value: false
```

This can be used to allow for options in some inheritance cases where you want to enable/disable a certain backend like
a Redis server, a Postgresql server.
Your base provides the different options and your inheritance will pick the ones they need.


## Output files

`beaver` output files have the following format:
- if the resource is namespaced :
```
<kind>.<apiVersion>.<metadata.namespace>.<metadata.name>.yaml
```
- if the resource is clusterwide :
```
<kind>.<apiVersion>.<metadata.name>.yaml
```

all `apiVersion` slashes (`/`) are replaced by underscores (`_`).

This convention will help you review merge requests.

By default `beaver` will store those files inside `${PWD}/build/<namespace>`, you
can use `-o` or `--output` to specify an output directory.

## sha256 sum variables

Use generated sha256 sum in your chart value files with the following syntax:

```
<[sha.key]>
```

For example:

```yaml
# base/beaver.yaml
sha:
- key: configmap_demo
  resource: ConfigMap.v1.demo.yaml
```

Will generate a sha256 hex sum for `ConfigMap.v1.demo.yaml` compiled file.

Then you can use it in your value file using:

```yaml
# base/postgres.yml
label:
  configmapSha: <[sha.configmap_demo]>
```

## Patch using YTT overlay

You can patch **all** your compiled resources using
[ytt overlays](https://carvel.dev/ytt/docs/v0.39.0/ytt-overlays/) by providing
`ytt.yaml` or `ytt.yml` files or a `ytt` folder inside your `beaver` project(s).

You can use `beaver` variables inside ytt files (outside ytt folder), because
`beaver` considers those as value files.

## Create resources using kubectl create

example:

```yaml
# base/beaver.yaml
# create some resources using `kubectl create`
create:
- type: configmap       # resource kind as passed to kubectl create
  name: xbus-pipelines  # resource name
  args:                 # kubectl create arguments
  - flag: --from-file
    value: pipelines
```

In the current context we have a `pipelines` folder inside `base` folder with
some files inside it.

`beaver` will run the following command **inside** `base` folder:

```sh
kubectl create configmap xbus-pipelines --from-file pipelines
```

## Kustomize

To use `kustomize` create a `kustomize` folder inside your beaver project and
use `kustomize` as usual.

A special beaver variable is available in your `kustomization.yaml` file:
`<[beaver.build]>` which exposes your beaver build temp directory, so you can
kustomize your previous builds (helm, ytt, etc.).

example:

```
# folder structure
.
└── base
    ├── beaver.yml
    └── kustomize
        └── kustomization.yaml
```

```yaml
resources:  # was previously named `bases`
- <[beaver.build]>
# now kustomize as usual.
patches:
- myPatch.yaml
```
