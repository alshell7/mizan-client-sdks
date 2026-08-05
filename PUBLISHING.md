# Publishing

Releases are owned by `alshell7`. Publish only from a clean `main` commit after SDK CI is green.

## Python / PyPI

The distribution name is `mizan-billing`, version `1.6.0`; the import is `mizan`.

1. In PyPI, add a pending trusted publisher for repository `alshell7/mizan-client-sdks`, workflow
   `publish-python.yml`, environment `pypi`.
2. In GitHub, create the protected `pypi` environment and restrict it to release tags.
3. Confirm `pyproject.toml` and `mizan/_version.py` have the same version.
4. Run the build and `twine check` locally.
5. Push tag `mizan-python-v1.6.0`. GitHub Actions builds once and publishes that artifact via OIDC;
   no long-lived PyPI token is stored.

PyPI versions are immutable. If a publish partially succeeds, increment the version; never overwrite it.

## Go

The module path is `github.com/alshell7/mizan-client-sdks/mizan-go`. From the repository root, tag the commit with the
subdirectory-aware module tag:

```bash
git tag -s mizan-go/v1.6.0 -m "mizan-go v1.6.0"
git push origin mizan-go/v1.6.0
GOPROXY=https://proxy.golang.org go list -m github.com/alshell7/mizan-client-sdks/mizan-go@v1.6.0
```

Do not use a root `v1.6.0` tag for this monorepo layout.
