# tools/split: moving go-freya into the go-tangra v4 repositories

These tools carry out Phase B and Phase C of the migration plan. The platform goes to
`go-tangra/go-tangra`. Each service goes to its own `go-tangra/go-tangra-<x>` repository,
keeping its go-freya history, with no AI trailers and no binaries.

| File | Purpose |
|---|---|
| `services.yaml` | Per-service facts: target repo, default branch, level, spec dirs, UI dir, binaries and tags, runtime, sdk |
| `cmd/render`, `render.sh` | Writes `Dockerfile`, `.dockerignore` and `.github/workflows/ci.yaml` into a repo root |
| `templates/*.tmpl` | Templates with `[[ ]]` delimiters, so the `${{ }}` / `{{version}}` in GitHub Actions pass through |
| `extract.sh <svc>` | Fresh clone plus `git filter-repo`: keeps the service and its specs, strips junk and AI trailers |
| `extract-platform.sh` | The same for the platform repo |
| `guard.sh <dir> [base]` | Pre-push guard: fails on files over 1 MB, binaries, junk paths, `co-authored-by`/`claude` in messages |

Every script takes `--dry-run`, which prints the commands without running them.
`git filter-repo` is required and is not installed by the scripts. Install it with
`pipx install git-filter-repo` or `pip install --user git-filter-repo`.

## Rules

- **Every outward-facing action needs the user's explicit confirmation, repository by repository:**
  `gh repo create`, every `git push` (branches and tags), `gh pr create`, PR merges,
  `gh workflow disable`, and package settings changes. Stop and ask before each one.
- Nothing creates or pushes a `latest` tag. There is no registry cleanup job. v3 images stay.
- Merge PRs with a **merge commit**. Squash or rebase would throw away the carried history.
- Run `guard.sh` before every push. It must be clean.

## Dependency order

Each level needs only the **published** tags from the levels before it. Within a level, order is free.

```
L0  go-tangra/go-tangra        platform: module v4.0.0 + @go-tangra/ui@4.0.0
L1  sdk/v4.0.0 tags            auth, gateway(portal), lcm sdks (tagged in their L2 repos first)
L2  auth (go-tangra-auth, NEW repo), gateway (go-tangra-portal), lcm
L3  warden, inventory, notification (default branch: master), paperless, deployer
L4  ipam (warden sdk), ticket (warden sdk), asset (inventory sdk)
L5  dns (ipam sdk, warden sdk, lcm sdk)
```

In L2, auth, gateway and lcm depend on each other only through their sdks. So publish the
three `sdk/v4.0.0` tags first (steps 1-5 below, up to the sdk tag), then tidy and release the
three services.

## Per-service runbook (example: ipam)

```bash
F=~/projects/go-freya          # this repo, clean and committed (extract clones HEAD)
S=ipam
REPO=$(cd $F && GOWORK=off go run ./tools/split/cmd/render -service $S -print repo)          # go-tangra/go-tangra-ipam
BR=$(cd $F && GOWORK=off go run ./tools/split/cmd/render -service $S -print default_branch)  # main (notification: master)
```

### 1. Extract history

```bash
$F/tools/split/extract.sh --dry-run $S         # check the filter-repo arguments
$F/tools/split/extract.sh --workdir /tmp/split $S
# -> /tmp/split/go-tangra-ipam: root = services/ipam, plus specs/011-ipam-service; guard runs at the end
```

### 2. Merge into the target repo, keeping v3 (existing repos)

```bash
git clone https://github.com/$REPO /tmp/split/target && cd /tmp/split/target
git branch v3 origin/$BR                   # the v3 line stays reachable; all v3.x tags stay
git switch -c v4 origin/$BR
git fetch /tmp/split/go-tangra-ipam HEAD
EXTRACTED=$(git rev-parse FETCH_HEAD)
git merge --allow-unrelated-histories --no-commit $EXTRACTED || true   # conflicts are expected
git read-tree -u --reset $EXTRACTED        # tree becomes exactly the v4 code (old files, including old workflows, go away)
```

**New repo (auth → go-tangra-auth):** after confirmation, run `gh repo create go-tangra/go-tangra-auth --public`.
Work directly in the extracted clone (`git switch -c main`). There is no v3 branch.

### 3. Standalone go.mod

```bash
rm -f go.work go.work.sum                  # never committed to a service repo
# own sdk: build the service against the in-repo sdk (removes the chicken-and-egg on PR CI)
[ -d sdk ] && go mod edit -replace github.com/go-tangra/go-tangra-$S/sdk/v4=./sdk   # portal: go-tangra-portal
# lower levels: published tags only
GOWORK=off go get github.com/go-tangra/go-tangra/v4@v4.0.0 \
                  github.com/go-tangra/go-tangra-auth/sdk/v4@v4.0.0 \
                  github.com/go-tangra/go-tangra-portal/sdk/v4@v4.0.0 \
                  github.com/go-tangra/go-tangra-lcm/sdk/v4@v4.0.0      # + warden/inventory/ipam sdk as needed
GOWORK=off go mod tidy
[ -d sdk ] && (cd sdk && GOWORK=off go mod tidy)
GOWORK=off go build ./... && GOWORK=off go vet ./... && GOWORK=off go test -race ./...
```

### 4. Front-end: standalone lockfile

The monorepo used one root `package-lock.json`. A service repo needs its own lockfile
(`ui/`, `console/` for auth, or `shell/` for gateway) that resolves `@go-tangra/ui@^4.0.0` from GitHub Packages:

```bash
cd ui    # console / shell
printf '@go-tangra:registry=https://npm.pkg.github.com\n' > .npmrc     # committed (no token in it)
npm install --no-audit --no-fund "--//npm.pkg.github.com/:_authToken=$(gh auth token)"   # token needs read:packages; not written to disk
npm run lint && npm run test:unit && npm run build
cd ..
```

In the `@go-tangra/ui` package settings, go to "Manage Actions access" and give this repo **read**
access. Otherwise `GITHUB_TOKEN` cannot install the kit in CI or in the Docker build.

### 5. Render the Dockerfile and CI into the repo root

```bash
$F/tools/split/render.sh $S .             # Dockerfile, .dockerignore, .github/workflows/ci.yaml (replaces the monorepo ones)
DOCKER_BUILDKIT=1 NODE_AUTH_TOKEN=$(gh auth token) docker buildx build \
  --secret id=npm_token,env=NODE_AUTH_TOKEN --build-arg APP_VERSION=4.0.0-local -t $S:local .
```

`main.version` is not declared in any service main package today (only in `inventory/cmd/inventory-agent`).
Until a repo adds `var version = "dev"` to `cmd/<x>svc/main.go`, the `-X main.version=` flag does nothing.

### 6. Commit and run the guard

```bash
git add -A
git commit -m "v4.0.0: rebuilt on the go-tangra v4 platform"     # no trailers
$F/tools/split/guard.sh . origin/$BR       # new repo: guard.sh .
```

### 7. Push and open a PR (confirm each step first)

```bash
git push origin v3                                    # CONFIRM
git push -u origin v4                                 # CONFIRM
gh pr create --repo $REPO --base $BR --head v4 \
  --title "v4.0.0: go-tangra v4 platform" --body "..." # CONFIRM
```

CI on the PR runs go vet/test (service + sdk), UI lint + vitest, buf lint (if `buf.yaml`), and
a docker build **without push**. Before merging, record the old images:

```bash
gh api --paginate /orgs/go-tangra/packages/container/${REPO#*/}/versions --jq '.[].metadata.container.tags' > /tmp/split/$S-images-before.json
```

Until the merge, the old default-branch workflow (with the `cleanup-ghcr` job, `keep-n-tagged: 3`,
weekly schedule) is still live. Merge soon after CI is green, or (with confirmation)
`gh workflow disable` it first.

### 8. Merge, then tag the sdk and then the service (confirm each step)

```bash
gh pr merge --repo $REPO --merge v4                   # CONFIRM - merge commit, never squash
git fetch origin && git switch $BR && git pull
[ -d sdk ] && git tag -a sdk/v4.0.0 -m "sdk v4.0.0" && git push origin sdk/v4.0.0   # CONFIRM (no image: 'v*' does not match 'sdk/v*')
git tag -a v4.0.0 -m "v4.0.0" && git push origin v4.0.0                            # CONFIRM -> image 4.0.0, 4.0, 4, sha-xxxxxxx
```

### 9. Verify

```bash
docker pull ghcr.io/$REPO:4.0.0
gh api --paginate /orgs/go-tangra/packages/container/${REPO#*/}/versions --jq '.[].metadata.container.tags'   # all 3.x tags still present
GOWORK=off GOFLAGS=-mod=mod go list -m github.com/go-tangra/go-tangra-$S/v4@v4.0.0   # proxy sees the module
```

Then switch `deploy/stack` in the platform repo to `image: ghcr.io/$REPO:4.0.0` and run a stack smoke test
(`registered:true`, health 200).

## Platform (L0, go-tangra/go-tangra, new repo)

```bash
$F/tools/split/extract-platform.sh --dry-run
$F/tools/split/extract-platform.sh --workdir /tmp/split      # drops services/, .claude/, spec-router*, non-platform specs, go.work
cd /tmp/split/go-tangra
```

Follow-up edits in the extracted tree:

- Root `package.json`: keep only `ui/kit` in `workspaces`, then regenerate `package-lock.json` with `npm install`.
- Contrib modules: without `go.work`, add `replace github.com/go-tangra/go-tangra/v4 => ../..` to each `contrib/*/go.mod`
  (this is ignored by consumers), or tag the root first.
- Replace `.github/workflows/ci.yml` with `$F/tools/split/render.sh platform .` (writes `ci.yaml`).
- `deploy/stack/compose.yaml`: services now use `image: ghcr.io/go-tangra/<repo>:${TAG}` (plan Phase B).
- Guard, then create the repo and push `main` (**CONFIRM**). Tag `v4.0.0` (**CONFIRM**). CI publishes `@go-tangra/ui@4.0.0`
  (the tag must match `ui/kit/package.json`).

## Known guard findings in go-freya today (2026-09-24)

- `go.work`, `go.work.sum`: monorepo only. The extraction drops them.
- `services/gateway/node_modules/.vite/vitest/.../results.json` is **tracked**. The extraction drops it.
  Untrack it in go-freya as well.
- `services/asset/assetsvc` (62 MB) is in history. `--strip-blobs-bigger-than 1M` and the `*svc` filter drop it.
- `ui/kit/catalogue/tests/__screenshots__/phone-320/catalogue.spec.ts/data-freya-{dark,light}.png` are about 1.2 MB.
  The 1 MB blob strip **removes them from the platform history**, and the catalogue visual test would then lose its baselines.
  Shrink or regenerate them under 1 MB before extracting the platform.
- 24 `Co-Authored-By` trailers in history. The message callback removes them.
