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
3. Changes are committed to git
4. Commit is pushed to remote

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
Current Build Progress: 1m30s elapsed
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
- **Current Build Progress**: Time elapsed for the current build

## Testing

A sample `kustomization.yaml` file is included for testing purposes.

## Requirements

- Go 1.25.1 or later
- Git (for committing manifest changes)
- Write access to the manifest repository (if using git push)

## License

MIT

