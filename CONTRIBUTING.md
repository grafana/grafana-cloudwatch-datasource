# Contributing

## Signed commits are required

> [!IMPORTANT]
> All commits must be [signed](https://docs.github.com/en/authentication/managing-commit-signature-verification/signing-commits) (GPG, SSH, or S/MIME) to be merged into this repository. Pull requests with unsigned commits will need to be re-committed with signatures before they can be merged.

# Building and releasing

## How to build the CloudWatch data source plugin locally

## Dependencies

Make sure you have the following dependencies installed first:

- [Git](https://git-scm.com/)
- [Go](https://golang.org/dl/) (see [go.mod](../go.mod#L3) for minimum required version)
- [Mage](https://magefile.org/)
- [Node.js (Long Term Support)](https://nodejs.org)

### Package manager version

This repository defines the required package manager and its exact version in the
`packageManager` field of `package.json`. Enable [Corepack](https://github.com/nodejs/corepack)
to make your terminal automatically use that version:

Corepack is included with many Node.js distributions. Check whether it is
available:

```bash
corepack --version
```

If the command is unavailable, install the standalone Corepack package:

```bash
npm install --global --ignore-scripts corepack
```

Then enable its npm shim:

```bash
corepack enable npm
```

Restart your terminal after enabling Corepack. No directory-change hook is
required: Corepack reads the nearest `package.json` whenever you run `npm`.
You can verify the selected version from the repository directory:

```bash
npm --version
```

Corepack manages the package manager version only; it does not install or select
the Node.js version specified by the `engines` field.

## Frontend

1. Install dependencies

   ```bash
   npm ci
   ```

2. Build plugin in development mode or run in watch mode

   ```bash
   npm run dev
   ```

3. Build plugin in production mode

   ```bash
   npm run build
   ```

## Backend

1. Build the backend binaries

   ```bash
   mage -v
   ```

## E2E Tests

1. `npm exec -- playwright install --with-deps`
1. `npm run server`
1. `npm run e2e`

## Build a release for the CloudWatch data source plugin

You need to have commit rights to the GitHub repository to publish a release.

1. Update the version number in the `package.json` file.
2. Update the `CHANGELOG.md` with the changes contained in the release.
3. Commit the changes to master and push to GitHub.
4. Follow the release process that you can find [here](https://enghub.grafana-ops.net/docs/default/component/grafana-plugins-platform/plugins-ci-github-actions/010-plugins-ci-github-actions/#cd_1)
