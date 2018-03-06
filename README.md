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

Add the repository to the hashtag.yaml
```
chart-name-here:
  image: null
```

```shell
# Install the plugin
$ helm plugin install https://github.com/balboah/helm-hashtag --version master

# Update tag hash digest values
$ helm hashtag -f values.yaml --tagfile hashtags.yaml --resolver="https://foobar/path"

# Use the override values
$ helm install -f values.yaml -f hashtags.yaml ...
```

## Resolver format:

The resolver is any http server which serves the correct format.

#### http GET `<resolver-url>/<image>/<tag>`

```
some-other-registry.com/foobar/my-image@sha256:7639a940c07f15c0f842faccd2f0973da9875f02ac1139b72a704e206bdc4e8c
eu.gcr.io/foobar/my-image@sha256:3867e93f2ad17b12cda5e4dede5c21311826de2b022ab65804e53de3c401b7a1
```

This file content can be generated with:

`docker inspect eu.gcr.io/foobar/my-image:tagname -f '{{range .RepoDigests}}{{. | printf "%s\n"}}{{end}}'`

Which could be saved in Google Cloud Storage to be used as the resolver:

```
docker inspect eu.gcr.io/foobar/my-image:tagname -f '{{range .RepoDigests}}{{. | printf "%s\n"}}{{end}}' \
  | gsutil cp - gs://your-bucket-of-tags/my-image/tagname
```

This will typically be ran in the CI platform that creates the original image.
