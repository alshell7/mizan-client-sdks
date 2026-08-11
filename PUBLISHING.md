# Publishing

Releases are owned by `alshell7`. Python releases are built and published automatically from package changes pushed
or merged to `main`; the publishing environment accepts deployments from `main` only.

## Python / PyPI

The distribution name is `mizan-billing`; the import is `mizan`. Read the release version from
`mizan-python/pyproject.toml` and confirm it matches `mizan-python/src/mizan/_version.py`.

1. In PyPI, configure the trusted publisher for repository `alshell7/mizan-client-sdks`, workflow
   `publish-python.yml`, environment `pypi`.
2. In GitHub, create the protected `pypi` environment and limit deployments to `main`. Do not add a required-reviewer
   rule when unattended main-branch publishing is desired.
3. For every publish-triggering Python package change, choose a new immutable version and set it in both
   `mizan-python/pyproject.toml` and `mizan-python/src/mizan/_version.py`.
4. Run the Python test, type-check, build, and `twine check` commands locally and merge only after SDK CI is green.
5. Merge or push the package change to `main`. The publishing workflow rebuilds and checks the distributions, then
   publishes through the main-only `pypi` environment with a short-lived OIDC credential. It has no pull-request
   trigger and stores no long-lived PyPI token.

The workflow is serialized and does not cancel an in-flight publish. PyPI versions are immutable: if a publish
partially succeeds, increment the version; never overwrite or silently skip an existing release.

## Go

The module path is `github.com/alshell7/mizan-client-sdks/mizan-go`. From the repository root, tag the commit with the
subdirectory-aware module tag:

```bash
VERSION=1.8.0
git tag -a "mizan-go/v$VERSION" -m "mizan-go v$VERSION"
git push origin "mizan-go/v$VERSION"
GOPROXY=https://proxy.golang.org go list -m "github.com/alshell7/mizan-client-sdks/mizan-go@v$VERSION"
```

Do not use a root `v$VERSION` tag for this monorepo layout.
If the release workstation has a configured signing key, use `git tag -s` instead of `git tag -a`.
