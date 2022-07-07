# Beaver

```
   ____
  | __ )  ___  __ ___   _____ _ __
  |  _ \ / _ \/ _` \ \ / / _ \ '__|
  | |_) |  __/ (_| |\ V /  __/ |
  |____/ \___|\__,_| \_/ \___|_|

```

## Description

Beaver is a tool to build your k8s templates in a descriptive way.

## Features

- template engine:
	- [helm](https://helm.sh/)
	- [ytt](https://carvel.dev/ytt/)
	- [kubectl create](https://kubernetes.io/docs/reference/generated/kubectl/kubectl-commands#create)
- patch:
	- [ytt overlay](https://carvel.dev/ytt/docs/v0.39.0/ytt-overlays/)
- multi environment variables
- sha256 sum for any compiled resource can be used as variable
- inheritance between beaver project
- each built resource is output inside it's own file

## Usage

```
beaver build <path/to/beaver/project>
```

see `beaver build --help` for more options.

## Beaver project

A beaver project consist of a folder with a beaver config file,  either `beaver.yaml` or `beaver.yml`.

## Beaver config file

```yaml
# Namespace used for this project
namespace: default
# an inherited beaver project - which can also inherit another beaver project
inherit: ../../base  # path is relative to this beaver config file
# your project charts
charts:
  postgres:                           # your chart local name
    type: helm                        # can be either elm or ytt
    path: ../.vendor/helm/postgresql  # path to your chart - relative to this file
# beaver variables that can be used inside your charts value files
variables:
- name: tag      # give your variable a name
  value: v1.2.3  # and a value
# generate beaver variables from compiled resource file
sha:
- key: configmap_demo               # use to generate beaver variable name
  resource: ConfigMap.v1.demo.yaml  # compiled resource filename
```

## Value files

Value files use the following format:

```
<chart_local_name>.[yaml,yml]
```

you can provide value files for your chart using its local name, and beaver will
pass this file to your template engine.

If you have a value file with the same name inside an inherited project then
beaver will also pass this one, but prior to your project file. This ensure that
your current values overwrite inherited values.

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

In the example above beaver will automaticaly pass `base/postgres.yml` and then
`environments/demo/postgres.yaml` to then helm command using `.vendor/postgresql`
as the chart folder.

## Beaver variables

Beaver variables can be used inside your value files using the following syntax:

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

Beaver variables are merged during inheritance.

## Output files

Beaver output files have the following format:

```
<kind>.<apiVersion>.<metadata.name>.yaml
```

all `apiVersion` slashes (`/`) are replaced by underscores (`_`).

This convention will help you reviewing merge requests.

By default beaver will store those files inside `./build/<namespace>`, you can
use `-o` or `--output` build flag to specify an output directory.

## sha256 sum variables

Use generated sha256 in your chart value files with the following syntax:

```
<[sha.key]>
```

For example:

```yaml
sha:
- key: configmap_demo
  resource: ConfigMap.v1.demo.yaml
```
Will generate a sha256 hex sum for `ConfigMap.v1.demo.yaml` compiled file.

Then you can use it in your value file using:

```yaml
label:
  configmapSha: <[sha.configmap_demo]>
```

## Patch using YTT overlay

You can patch **all** your compiled resources using
[ytt overlays](https://carvel.dev/ytt/docs/v0.39.0/ytt-overlays/) by providing
`ytt.yaml` or `ytt.yml` or a `ytt` folder inside your beaver project(s).
