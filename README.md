# helm-hashtag

This plugin stores the docker digest hash of tags locally,
as to avoid max tags limitations on 3rd party registries.

The source of truth has to be a Google Container Registry, it is assumed that
one repository will be used to lookup digests for all overrides.

## Usage

The values file needs to contain docker images and tags in the following format:

```
chart-name-here:
  image:
    repository: eu.gcr.io/foobar/imagename
    tag: foobar
```

The above value will then be overridden with:
```
chart-name-here:
  image:
    repository: eu.gcr.io/foobar/imagename@sha256
    tag: <long digest hash>
```
And stored into a the "hashtag" value file.

Which can then be used as helm values,
`helm install -f <original-values.yaml> -f <hashtag-values.yaml> ...`

```shell
# Install the plugin
$ helm plugin install https://github.com/balboah/helm-hashtag

# Add the repository to the hashtag.yaml
```
chart-name-here:
  image: null
```

# Update tag hash digest values
$ helm hashtag -f values.yaml --tagfile hashtags.yaml --gcp-repo="eu.gcr.io/foobar"

# Use the override values
$ helm install -f values.yaml -f hashtags.yaml ...
```
