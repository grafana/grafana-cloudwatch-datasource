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

2. Run the backend tests

   ```bash
   mage test
   ```

## Data Source Configuration Schema

`pkg/schema/dsconfig.json` is the **single source of truth** for the data source's
configuration surface: every field a user can set, where it is stored (`root`, `jsonData`,
`secureJsonData`), its type, validation rules and UI hints. It is consumed by provisioning
tooling, documentation and automation.

The schema format is defined and documented by [`grafana/dsconfig`](https://github.com/grafana/dsconfig/tree/main/dsconfig):

- [README](https://github.com/grafana/dsconfig/tree/main/dsconfig#readme): concepts and a worked example for each field shape (root / jsonData / secret / array / virtual), plus current gaps and limitations.
- [`schema.md`](https://github.com/grafana/dsconfig/blob/main/dsconfig/schema.md): full property reference.
- [`schema.json`](https://github.com/grafana/dsconfig/blob/main/dsconfig/schema.json): the JSON Schema `dsconfig.json` validates against. It is pinned via the `$schema` key at the top of our file, so editors autocomplete from it; bump that URL when you bump `github.com/grafana/dsconfig/dsconfig` and `github.com/grafana/dsconfig/schema` in `go.mod`.

The rest of this section covers only what is specific to this repository.

### Layout

| File in `pkg/schema/` | Description                                                                                                            |
| --------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| `dsconfig.json`       | Source of truth, **edit this**                                                                                         |
| `dsconfig_test.go`    | Wires the schema into the shared conformance suite; also holds `SecureKeys` and the test-only `settingsJSONModel`      |
| `*.gen.json`          | Generated artifacts, **never hand-edit**; `npm run build` copies them into `dist/schema/` via `webpack.config.ts`       |

### AWS SDK fields

The AWS authentication, region, endpoint and proxy fields come from the shared `aws_sdk_settings`
pack, declared under `baseFields` in `dsconfig.json`. They are defined in
[`grafana/dsconfig`](https://github.com/grafana/dsconfig/tree/main/dsconfig/packs) and are not
repeated in this repo; use `exclude` or `patch` on the `baseFields` entry to adjust them. Only
CloudWatch-specific settings are declared in `fields`.

### Adding a new settings option

1. **Declare the field** in `pkg/schema/dsconfig.json` under `fields`, and add its `id` to
   the appropriate `groups[].fieldRefs` entry. Field ids follow the `<target>_<key>`
   convention, e.g. `jsonData_logsTimeout`. Tag fields only the frontend or only the backend
   reads with `frontend-only` / `backend-only`.
2. **Add the matching Go field** to `CloudWatchSettings` in
   `pkg/cloudwatch/models/settings.go` with a json tag equal to the schema `key`, when the
   backend reads it. This parity is enforced in both directions: a field in the schema but not
   in the struct (or vice versa) fails the test suite. Frontend-only fields are added to the
   test-only `settingsJSONModel` in `pkg/schema/dsconfig_test.go` instead. Secrets
   (`target: secureJsonData`) get no struct field, but their key must be added to
   `SecureKeys` in `pkg/schema/dsconfig_test.go`.
3. **Regenerate the artifacts** and commit them with your change:

   ```bash
   go generate ./pkg/schema/...
   ```

4. **Verify**:

   ```bash
   go test ./pkg/schema/...
   ```

### When the conformance suite fails

Most failures are self-explanatory from the assertion message. The three you are most
likely to hit:

- `SchemaArtifactInSync`: a `.gen.json` file has drifted. Run `go generate ./pkg/schema/...` and commit the result.
- `JSONDataMatchesStruct` / `JSONDataTypesMatchStruct`: the schema and `CloudWatchSettings` (plus the shims in `settingsJSONModel`) disagree on keys or types. Update whichever side is behind.
- `SecureValuesMatchLoadSettings`: the schema's `secureJsonData` fields and `SecureKeys` disagree.

## E2E Tests

The E2E test suite uses the CloudWatch data source provisioned for the Data Sources team's AWS test environment. Export its credentials before starting Grafana.

1. Install the Playwright browser:

   ```bash
   npm exec -- playwright install --with-deps
   ```

2. Export the provisioned data source credentials:

   ```bash
   export ACCESS_KEY=<access-key>
   export SECRET_KEY=<secret-key>
   ```

3. Start Grafana:

   ```bash
   npm run server
   ```

4. Run the E2E tests:

   ```bash
   npm run e2e
   ```

### Test quarantine

Quarantine a test when its failure is a test defect, not a product bug. Examples include a test that assumes local provisioning, relies on a hard-coded data source name, or uses a timeout too short for the shared Cloud instance while the plugin itself is healthy. Infrastructure faults such as the shared instance recycling are resumed or retried in Argo, depending on the gate mode, rather than quarantined.

Add the `@quarantine` tag and a comment with the reason and tracking issue:

```ts
// Quarantined: flaky Cloud selector — tracked in #619
test('the flaky test', { tag: '@quarantine' }, async ({ page }) => {
  // ...
});
```

The tag changes only the Cloud runs. Local `npm run e2e` and PR CI still run quarantined tests unfiltered. The nightly `e2e:cloud` run skips them entirely. The in-Argo bench gate runs them in a separate non-gating quarantine suite so failures are visible without blocking the release.

Re-promote a test by removing `{ tag: '@quarantine' }` once the underlying issue is fixed.

See the [test-signal taxonomy](https://github.com/grafana/data-sources/blob/main/docs/testing/test-signal-taxonomy.md) for the full quarantine policy.

## Release the CloudWatch data source plugin

Releases are automated with [release-please](https://github.com/googleapis/release-please). The version number and the changelog both come from commit messages, so there is nothing to edit by hand.

### What you do

Title your pull request as a [Conventional Commit](https://www.conventionalcommits.org/). The `PR Title` check enforces this, and because the repository squash-merges, the PR title becomes the commit subject that release-please reads.

| Prefix | Effect on the next release |
| --- | --- |
| `fix:` | patch version |
| `feat:` | minor version |
| `feat!:`, or a `BREAKING CHANGE:` footer | major version |
| `chore:` | no release, hidden from the changelog |
| `docs:`, `test:`, `build:`, `ci:`, `refactor:`, `perf:`, `revert:` | no version bump, listed in the changelog |

### What happens next

1. release-please opens a `chore(main): release X.Y.Z` pull request and keeps it up to date as more commits land.
2. Merging that pull request creates the tag and the GitHub release, and publishes the plugin to the prod catalog.
3. Every other push to `main` publishes to the dev catalog instead.

### Do not

Do not edit the version in `package.json`, and do not write `CHANGELOG.md` entries by hand. release-please owns both files, and a manual edit puts `package.json` out of step with `.release-please-manifest.json`, which makes the next release pick a wrong version.

To release a specific version, add a `Release-As: X.Y.Z` footer to a commit rather than editing the version.
