# Specification

Directory layout:

```
.
├── base
│   ├── beaver.yml
│   ├── odoo.yml
│   ├── ytt/
│   └── postgres.yml
├── prod
│   ├── odoo.yml
│   └── postgres.yml
├── prod-build
│   ├── <namespace>.<type>.<name>.yml
│   ├── prod.statefulset.postgresql.yml
│   └── prod.deployment.odoo.yml
├── test
│   ├── beaver.yml
│   └── odoo.yml
└── vendir.yml
```

- `beaver.yml`: beaver.cloudcrane.io config file.
	- filename is mandatory, cannot use another name (must be uniq per project)
- `<other-files>.yml`: charts (static) values files


Command: `beaver build <namespace>`

Should build charts, exemple:

```sh
helm template postgresql vendor/helm/postgresql \
    --namespace <namespace> \
    -f /tmp/values-above.yaml \
    -f base/postgres.yaml \
    (if ./<namespace>/postgres.yaml then -f ./<namespace>/postgres.yaml fi) \
    > /tmp/resources.yaml
```

if ./base/ytt then
  # TODO: exec ytt patches
fi

if ./<namespace>/beaver.yaml then
  # TODO: exec beaver build
fi

if ./<namespace>/ytt then
  # TODO: exec ytt patches
fi
