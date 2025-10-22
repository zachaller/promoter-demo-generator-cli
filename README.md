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
| `--simulatedCommitRate` | string | `1m` | Commit frequency: fixed (`1m`), random range (`1m-5m`), or pattern name (see below) |
| `--manifestKustomizeFilePath` | string | *required* | Path to the `kustomization.yaml` file to modify |
| `--skipGitOperations` | bool | `false` | If `true`, skip git commit and push operations |

### Pre-Canned Commit Rate Patterns

Instead of specifying a duration, you can use these realistic patterns:

| Pattern | Description | Behavior |
|---------|-------------|----------|
| `developer` | Realistic developer workflow | Bursts of 3-7 commits (30s-2min apart), then 15-45min pauses |
| `burst` | Frequent small bursts | 2-4 commits (20-60s apart), then 5-10min pauses |
| `steady` | Consistent pace | One commit every 2-5 minutes |
| `sporadic` | Unpredictable timing | Random commits with 1-30 minute gaps |
| `rapid` | High frequency | Continuous commits every 30s-2min |

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

### Example 5: Developer Pattern (Realistic Workflow)

Simulates a real developer with bursts of activity:

```bash
./promoter-demo-generator \
  --manifestKustomizeFilePath=./kustomization.yaml \
  --simulatedBuildDuration=10m \
  --abortOnNewCommit=false \
  --simulatedCommitRate=developer
```

**Behavior**:
- Developer makes 3-7 commits in quick succession (30s-2min apart)
- Then takes a break for 15-45 minutes
- Very realistic for testing how CI handles bursty workflows
- Queue will build up during burst, then drain during pause

### Example 6: Burst Pattern with Abort Mode

Test how abort mode handles frequent bursts:

```bash
./promoter-demo-generator \
  --manifestKustomizeFilePath=./kustomization.yaml \
  --simulatedBuildDuration=15m \
  --abortOnNewCommit=true \
  --simulatedCommitRate=burst
```

**Behavior**:
- Small bursts of 2-4 commits every 5-10 minutes
- With abort mode, builds may frequently restart
- Tests aggressive abort behavior

### Example 7: Skip Git Operations (Testing Mode)

If you want to test without committing to git:

```bash
./promoter-demo-generator \
  --manifestKustomizeFilePath=./kustomization.yaml \
  --simulatedBuildDuration=30s \
  --simulatedCommitRate=rapid \
  --skipGitOperations=true
```

**Behavior**:
- Fast simulation for testing with rapid commits
- Updates the manifest file only
- No git add, commit, or push operations
- Useful when the manifest isn't in a git repository or for quick testing

## How It Works

### Commit Rate Patterns Explained

The simulator supports three ways to specify commit rates:

1. **Fixed Rate**: Use a Go duration (e.g., `1m`, `30s`, `2h`)
   - Commits arrive at exact intervals
   - Example: `--simulatedCommitRate=2m` → commit every 2 minutes

2. **Random Range**: Use two durations separated by a dash (e.g., `1m-5m`)
   - Commits arrive at random intervals within the range
   - Example: `--simulatedCommitRate=1m-10m` → commits between 1-10 minutes apart

3. **Named Patterns**: Use a pattern name for realistic scenarios
   - `developer`: Simulates realistic developer workflow with work bursts and breaks
   - `burst`: Frequent small bursts of commits with medium pauses
   - `steady`: Predictable, consistent commit rate
   - `sporadic`: Highly variable, unpredictable commits
   - `rapid`: Continuous high-frequency commits for stress testing

#### Developer Pattern Deep Dive

The `developer` pattern is particularly useful for realistic testing:

```
Time 0:00   → Burst starts: 5 commits over 4 minutes
Time 0:00   → Commit #2
Time 0:01:30 → Commit #3
Time 0:02:15 → Commit #4  
Time 0:03:00 → Commit #5
Time 0:03:45 → Commit #6
Time 0:04:00 → Developer break: 28 minutes
Time 0:32:00 → Next burst starts...
```

This simulates:
- Morning coding sessions
- Post-lunch development
- Bug fixing sprints
- Breaks for meetings, lunch, code review

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
Argocd-reference-commit-sha: 8f3a9b2c1d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a
Argocd-reference-commit-subject: This change fixes a bug in the code v1.0.232
Argocd-reference-commit-body: "Commit message of the code commit\n\nSigned-off-by: Author Name <author@example.com>"
Argocd-reference-commit-repourl: https://github.com/argoproj-labs/gitops-promoter
Argocd-reference-commit-date: 2025-10-01T08:23:45-04:00
Signed-off-by: Zach Aller <zach_aller@intuit.com>
```

**Dynamic Commit References:**
- The `Argocd-reference-commit-sha` is randomly selected from **real commits** fetched from the [gitops-promoter repository](https://github.com/argoproj-labs/gitops-promoter)
- At startup, the CLI fetches the 100 most recent commits via GitHub API
- Each manifest update uses a different random commit SHA for realistic simulation
- The `Argocd-reference-commit-date` is randomly generated to be 5-35 days in the past
- If GitHub is unavailable, falls back to a static commit SHA

## Output

The CLI provides real-time feedback:

```
🔍 Fetching commit SHAs from gitops-promoter repository...
✅ Loaded 100 commit SHAs from repository
🚀 Starting CI/CD Pipeline Simulation
=====================================
Build Duration: 15m0s
Abort on New Commit: false
Commit Rate: 1m
Manifest File: ./kustomization.yaml
=====================================

📝 Initial commit detected: #1 (timestamp: 14:23:15)
🔨 Starting build for commit #1 (duration: 15m0s)
💥 Developer burst: 4 commits incoming
📝 New commit detected: #2 (timestamp: 14:24:15)
📝 New commit detected: #3 (timestamp: 14:25:30)
📝 New commit detected: #4 (timestamp: 14:26:45)
📝 New commit detected: #5 (timestamp: 14:27:20)
😴 Developer taking a break for 25m

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
- Internet connection (for fetching commit SHAs from GitHub - optional, will use fallback if unavailable)

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

## Use Cases by Pattern

| Use Case | Recommended Pattern | Why |
|----------|-------------------|-----|
| Test queue buildup and drainage | `developer` | Bursts create queues, pauses let them drain |
| Stress test CI system | `rapid` | Continuous high-frequency commits |
| Test abort mode effectiveness | `burst` + `--abortOnNewCommit=true` | Frequent interruptions |
| Baseline performance testing | `steady` | Predictable, consistent load |
| Test edge cases | `sporadic` | Wide variance catches timing issues |
| Simulate real production | `developer` | Most realistic developer behavior |
| Quick functionality test | `30s` + `--skipGitOperations=true` | Fast, simple validation |

## License

MIT

