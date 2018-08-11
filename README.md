# GitLab Merge Request Concourse Resource

Features:
- Check for new merge requests on GitLab and update the merge request pipeline (build) status
- Rebuild expired builds and update the status
- Written entirely in Go

## Source Configuration

```yaml
resource_types:
- name: merge-request
  type: docker-image
  source:
    repository: mastertinner/gitlab-merge-request-resource

resources:
- name: repo-mr
  type: merge-request
  source:
    uri: https://gitlab.com/myname/myproject.git
    private_token: XXX
    username: my_username
    password: xxx
```

* `uri`: The location of the repository (required)
* `private_token`: Your GitLab user's private token (required, can be found in your profile settings)
* `private_key`: The private SSH key for SSH auth when pulling

  Example:

  ```yaml
  private_key: |
    -----BEGIN RSA PRIVATE KEY-----
    MIIEowIBAAKCAQEAtCS10/f7W7lkQaSgD/mVeaSOvSF9ql4hf/zfMwfVGgHWjj+W
    <Lots more text>
    DWiJL+OFeg9kawcUL6hQ8JeXPhlImG6RTUffma9+iGQyyBMCGd1l
    -----END RSA PRIVATE KEY-----
  ```

* `username`: The username for HTTP(S) auth when pulling
* `password`: The password for HTTP(S) auth when pulling
* `no_ssl`: Set to `true` if the GitLab API should be used over HTTP instead of HTTPS
* `skip_ssl_verification`: Optional. Connect to GitLab insecurely - i.e. skip SSL validation. Defaults to false if not provided.
* `concourse_host`: The url:port for concourse web interface (ATC), so that the builds are linked to gitlab pipelines and can be navigated to from gitlab commit.
* `build_expires_after`: A [time duration](https://golang.org/pkg/time/#ParseDuration), which is used to deem an already successful build as expired, hence kicking off new buid automatically. 

> Please note that you have to provide either `private_key` or `username` and `password`.

## Behavior

### `check`: Check for new merge requests, and build-expired merge requests

Checks if there are new merge requests or merge requests with new commits. Also, checks if any of the already built merge requests are expired, according to given `build_expires_after` parameter.

### `in`: Clone merge request source branch and Merge it with master

`git clone`s the source branch of the respective merge request, and then merges it with master branch. Failure in merge results in a failed status.

### `out`: Update a merge request's merge status

Updates the merge request's `merge_status` which displays nicely in the GitLab UI and allows to only merge changes if they pass the test.

#### Parameters

* `repository`: The path of the repository of the merge request's source branch (required)
* `status`: The new status of the merge request (required, can be either `pending`, `running`, `success`, `failed`, or `canceled`)
* `build_label`: The label of the build in GitLab (optional, defaults to `"Concourse"`)

## Example

```yaml
jobs:
- name: test-merge-request
  plan:
  - get: repo
    resource: repo-mr
    trigger: true
    version: every
  - put: repo-mr
    params:
      repository: repo
      status: running
  - task: run-tests
    file: repo/ci/tasks/run-tests.yml
  on_failure:
    put: repo-mr
    params:
      repository: repo
      status: failed
  on_success:
    put: repo-mr
    params:
      repository: repo
      status: success
```
