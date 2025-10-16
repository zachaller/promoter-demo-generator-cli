# Promoter Demo Generator CLI

A Go CLI tool that simulates a CI/CD pipeline where commits trigger Docker image builds, and completed builds result in Kubernetes manifest updates.

## Features

- **Simulated Build Process**: Mimics the time it takes to build a Docker image
- **Commit Simulation**: Generates commits at fixed or random intervals
- **Two Queue Modes**:
  - **Abort Mode**: Restarts the build timer when new commits arrive
  - **Queue Mode**: Queues commits and processes them sequentially
- **Automatic Manifest Updates**: Bumps version numbers in Kustomization files
- **Real-time Statistics**: Displays queue size, completed builds, and progress

## Installation

```bash
go build -o promoter-demo-generator ./cmd/main.go
```

## Usage

### Basic Command

```bash
./promoter-demo-generator --manifestKustomizeFilePath=./kustomization.yaml
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--simulatedBuildDuration` | duration | `15m` | How long each simulated build takes (e.g., `15m`, `30s`, `1h`) |
| `--abortOnNewCommit` | bool | `false` | If `true`, restart build on new commits; if `false`, queue them |
| `--simulatedCommitRate` | string | `1m` | Commit frequency: fixed (`1m`) or random range (`1m-5m`) |
| `--manifestKustomizeFilePath` | string | *required* | Path to the `kustomization.yaml` file to modify |
| `--skipGitOperations` | bool | `false` | If `true`, skip git commit and push operations |

## Examples

### Example 1: Abort Mode with Fast Builds

Simulates a scenario where new commits abort ongoing builds:

```bash
./promoter-demo-generator \
  --manifestKustomizeFilePath=./kustomization.yaml \
  --simulatedBuildDuration=15m \
  --abortOnNewCommit=true \
  --simulatedCommitRate=5m
```

**Behavior**: 
- Commits come every 5 minutes
- Each build takes 15 minutes
- New commits will abort the current build and restart the timer
- Result: Builds may never complete if commits come too frequently!

### Example 2: Queue Mode with Sequential Processing

Processes all commits sequentially without aborting:

```bash
./promoter-demo-generator \
  --manifestKustomizeFilePath=./kustomization.yaml \
  --simulatedBuildDuration=10m \
  --abortOnNewCommit=false \
  --simulatedCommitRate=3m
```

**Behavior**:
- Commits come every 3 minutes
- Each build takes 10 minutes
- Commits queue up and are processed one by one
- The queue will grow over time since commits come faster than builds complete

### Example 3: Random Commit Intervals

Simulates unpredictable commit patterns:

```bash
./promoter-demo-generator \
  --manifestKustomizeFilePath=./kustomization.yaml \
  --simulatedBuildDuration=20m \
  --abortOnNewCommit=false \
  --simulatedCommitRate=5m-15m
```

**Behavior**:
- Commits arrive randomly between 5-15 minutes apart
- Each build takes 20 minutes
- Queue size will fluctuate based on random timing

### Example 4: Fast Builds with Frequent Commits (Queue Mode)

```bash
./promoter-demo-generator \
  --manifestKustomizeFilePath=./kustomization.yaml \
  --simulatedBuildDuration=2m \
  --abortOnNewCommit=false \
  --simulatedCommitRate=1m
```

**Behavior**:
- Commits come every minute
- Builds complete in 2 minutes
- Queue will build up slowly (1 commit per build cycle)

### Example 5: Skip Git Operations (Testing Mode)

If you want to test without committing to git:

```bash
./promoter-demo-generator \
  --manifestKustomizeFilePath=./kustomization.yaml \
  --simulatedBuildDuration=30s \
  --simulatedCommitRate=10s \
  --skipGitOperations=true
```

**Behavior**:
- Fast simulation for testing
- Updates the manifest file only
- No git add, commit, or push operations
- Useful when the manifest isn't in a git repository or for quick testing

## How It Works

### Abort on New Commit (true)

When `--abortOnNewCommit=true`:
1. A commit triggers a build
2. Build timer starts (e.g., 15 minutes)
3. If a new commit arrives at minute 10:
   - Current build is **aborted**
   - Timer **resets** to 0
   - New 15-minute build starts for the latest commit
4. Only when no new commits arrive during the full duration will the build complete

### Queue Mode (false)

When `--abortOnNewCommit=false`:
1. First commit triggers a build (15 minutes)
2. Additional commits are **queued**
3. After 15 minutes, first build completes and manifest is updated
4. Next queued commit immediately starts building
5. This continues until the queue is empty

### Manifest Updates

The tool modifies the `commonAnnotations.version` field in your `kustomization.yaml`:

```yaml
commonAnnotations:
    version: "v1.0.231"  # Increments to v1.0.232, v1.0.233, etc.
```

After each successful build:
1. Version number is incremented
2. File is saved
3. Changes are committed to git (unless `--skipGitOperations=true`)
4. Commit is pushed to remote (unless `--skipGitOperations=true`)

### Git Commit Message Format

Each git commit includes ArgoCD gitops-promoter trailers for integration with GitOps workflows:

```
chore: bump version to v1.0.232

Argocd-reference-commit-author: Zach Aller <code@example.com>
Argocd-reference-commit-sha: 9d5ccef278218dea4caa903bb6abb9ed974a1d90
Argocd-reference-commit-subject: This change fixes a bug in the code v1.0.232
Argocd-reference-commit-body: "Commit message of the code commit\n\nSigned-off-by: Author Name <author@example.com>"
Argocd-reference-commit-repourl: https://github.com/argoproj-labs/gitops-promoter
Argocd-reference-commit-date: 2025-10-01T08:23:45-04:00
Signed-off-by: Zach Aller <zach_aller@intuit.com>
```

The `Argocd-reference-commit-date` is randomly generated to be 5-35 days in the past to simulate realistic scenarios.

## Output

The CLI provides real-time feedback:

```
🚀 Starting CI/CD Pipeline Simulation
=====================================
Build Duration: 15m0s
Abort on New Commit: false
Commit Rate: 1m
Manifest File: ./kustomization.yaml
=====================================

📝 New commit detected: #1 (timestamp: 14:23:15)
🔨 Starting build for commit #1 (duration: 15m0s)
📝 New commit detected: #2 (timestamp: 14:24:15)
⏳ Commit #2 queued (current queue size: 2)

📊 === Statistics ===
Total Commits: 2
Queued Commits: 2
Completed Builds: 0
Current Build: Commit #1 (1m30s elapsed)
===================

✅ Build completed for commit #1
📦 Manifest updated and committed for build #1
🔨 Starting build for commit #2 (duration: 15m0s)
```

### Statistics (printed every 10 seconds)

- **Total Commits**: Number of simulated commits generated
- **Queued Commits**: Number of commits waiting to be built
- **Completed Builds**: Number of successful builds
- **Aborted Builds**: Number of builds canceled (abort mode only)
- **Current Build**: Shows the commit ID being built and time elapsed (e.g., `Commit #3 (2m15s elapsed)`)

## Testing

A sample `kustomization.yaml` file is included for testing purposes.

## Requirements

- Go 1.25.1 or later
- Git (for committing manifest changes, unless using `--skipGitOperations`)
- Write access to the manifest repository (if using git push)

## Troubleshooting

### Git Errors

If you see git-related errors like `git add failed: exit status 128`, you have several options:

1. **Check if the manifest is in a git repository**: The manifest file must be in a directory initialized with git
   ```bash
   cd /path/to/manifest
   git status  # Should show it's a git repo
   ```

2. **Use skip flag**: If you don't want git operations:
   ```bash
   ./promoter-demo-generator --manifestKustomizeFilePath=./kustomization.yaml --skipGitOperations=true
   ```

3. **Check error details**: The CLI now shows detailed stderr output from git commands to help diagnose issues

### Initial Commit

The simulator automatically creates an initial commit when it starts, so you don't have to wait for the first `simulatedCommitRate` interval. The first build begins immediately.

## License

MIT

