package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	simulatedBuildDuration    string
	abortOnNewCommit          bool
	simulatedCommitRate       string
	manifestKustomizeFilePath string
	skipGitOperations         bool
	commitSHAsCache           []string
	commitSHAsMutex           sync.Mutex
)

type SimulationStats struct {
	mu                    sync.Mutex
	totalCommits          int
	queuedCommits         int
	completedBuilds       int
	currentBuildStartTime *time.Time
	currentBuildCommitID  int
	abortedBuilds         int
}

type CommitEvent struct {
	timestamp time.Time
	id        int
}

type Kustomization struct {
	APIVersion        string            `yaml:"apiVersion"`
	Kind              string            `yaml:"kind"`
	Resources         []string          `yaml:"resources,omitempty"`
	Patches           []interface{}     `yaml:"patches,omitempty"`
	Images            []interface{}     `yaml:"images,omitempty"`
	CommonAnnotations map[string]string `yaml:"commonAnnotations,omitempty"`
	OpenAPI           map[string]string `yaml:"openapi,omitempty"`
	Configurations    []string          `yaml:"configurations,omitempty"`
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "promoter-demo-generator",
		Short: "Simulates a CI/CD pipeline with Docker builds and Kubernetes manifest updates",
		Long: `A CLI tool that simulates the behavior of a CI/CD pipeline where commits trigger
Docker image builds, and completed builds result in Kubernetes manifest updates.`,
		RunE: runSimulation,
	}

	rootCmd.Flags().StringVar(&simulatedBuildDuration, "simulatedBuildDuration", "15m",
		"Duration for simulated Docker build (e.g., 15m, 30s, 1h)")
	rootCmd.Flags().BoolVar(&abortOnNewCommit, "abortOnNewCommit", false,
		"If true, restart build on new commit; if false, queue commits")
	rootCmd.Flags().StringVar(&simulatedCommitRate, "simulatedCommitRate", "1m",
		"Commit rate: fixed (e.g., '1m'), random range (e.g., '1m-5m'), or pattern (developer, burst, steady, sporadic, rapid)")
	rootCmd.Flags().StringVar(&manifestKustomizeFilePath, "manifestKustomizeFilePath", "",
		"Path to the kustomization.yaml file to modify")
	rootCmd.Flags().BoolVar(&skipGitOperations, "skipGitOperations", false,
		"If true, skip git commit and push operations")

	rootCmd.MarkFlagRequired("manifestKustomizeFilePath")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runSimulation(cmd *cobra.Command, args []string) error {
	// Parse build duration
	buildDuration, err := time.ParseDuration(simulatedBuildDuration)
	if err != nil {
		return fmt.Errorf("invalid simulatedBuildDuration: %w", err)
	}

	// Validate manifest file exists
	if _, err := os.Stat(manifestKustomizeFilePath); os.IsNotExist(err) {
		return fmt.Errorf("manifest file does not exist: %s", manifestKustomizeFilePath)
	}

	// Fetch commit SHAs from GitHub
	fmt.Println("🔍 Fetching commit SHAs from gitops-promoter repository...")
	if err := fetchCommitSHAs(); err != nil {
		fmt.Printf("⚠️  Warning: Could not fetch commits from GitHub: %v\n", err)
		fmt.Println("   Using fallback commit SHA")
		commitSHAsCache = []string{"9d5ccef278218dea4caa903bb6abb9ed974a1d90"}
	} else {
		fmt.Printf("✅ Loaded %d commit SHAs from repository\n", len(commitSHAsCache))
	}

	stats := &SimulationStats{}
	commitQueue := make(chan CommitEvent, 100)
	buildControl := make(chan bool, 1)
	done := make(chan bool)

	fmt.Println("🚀 Starting CI/CD Pipeline Simulation")
	fmt.Println("=====================================")
	fmt.Printf("Build Duration: %v\n", buildDuration)
	fmt.Printf("Abort on New Commit: %v\n", abortOnNewCommit)
	fmt.Printf("Commit Rate: %s\n", simulatedCommitRate)
	fmt.Printf("Manifest File: %s\n", manifestKustomizeFilePath)
	fmt.Println("=====================================")

	// Start commit generator
	go generateCommits(simulatedCommitRate, commitQueue, stats)

	// Start build processor
	go processBuildQueue(buildDuration, commitQueue, buildControl, stats, done)

	// Monitor and print stats
	go printStats(stats, done)

	// Send initial commit to start building immediately
	fmt.Println()
	fmt.Println("📝 Initial commit detected: #1 (timestamp: " + time.Now().Format("15:04:05") + ")")
	initialCommit := CommitEvent{
		timestamp: time.Now(),
		id:        1,
	}
	stats.mu.Lock()
	stats.totalCommits++
	stats.queuedCommits++
	stats.mu.Unlock()
	commitQueue <- initialCommit

	// Wait for interrupt
	select {}
}

type GitHubCommit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
	} `json:"commit"`
}

func fetchCommitSHAs() error {
	// Fetch recent commits from the gitops-promoter repository
	url := "https://api.github.com/repos/argoproj-labs/gitops-promoter/commits?per_page=100"

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	// Set user agent as required by GitHub API
	req.Header.Set("User-Agent", "promoter-demo-generator")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	var commits []GitHubCommit
	if err := json.NewDecoder(resp.Body).Decode(&commits); err != nil {
		return err
	}

	if len(commits) == 0 {
		return fmt.Errorf("no commits found")
	}

	// Extract SHAs
	commitSHAsMutex.Lock()
	commitSHAsCache = make([]string, len(commits))
	for i, commit := range commits {
		commitSHAsCache[i] = commit.SHA
	}
	commitSHAsMutex.Unlock()

	return nil
}

func getRandomCommitSHA() string {
	commitSHAsMutex.Lock()
	defer commitSHAsMutex.Unlock()

	if len(commitSHAsCache) == 0 {
		return "9d5ccef278218dea4caa903bb6abb9ed974a1d90" // fallback
	}

	return commitSHAsCache[rand.Intn(len(commitSHAsCache))]
}

func generateCommits(rateSpec string, commitQueue chan<- CommitEvent, stats *SimulationStats) {
	commitID := 2 // Start from 2 since initial commit is 1

	// Check for pre-canned patterns
	pattern := strings.ToLower(rateSpec)
	switch pattern {
	case "developer":
		generateDeveloperPattern(commitQueue, stats, &commitID)
	case "burst":
		generateBurstPattern(commitQueue, stats, &commitID)
	case "steady":
		generateSteadyPattern(commitQueue, stats, &commitID)
	case "sporadic":
		generateSporadicPattern(commitQueue, stats, &commitID)
	case "rapid":
		generateRapidPattern(commitQueue, stats, &commitID)
	default:
		// Use custom duration-based pattern
		generateCustomPattern(rateSpec, commitQueue, stats, &commitID)
	}
}

// Developer pattern: Bursts of commits (3-7 commits in quick succession) followed by longer pauses
func generateDeveloperPattern(commitQueue chan<- CommitEvent, stats *SimulationStats, commitID *int) {
	for {
		// Burst: 3-7 commits
		burstSize := 3 + rand.Intn(5)
		fmt.Printf("💥 Developer burst: %d commits incoming\n", burstSize)

		for i := 0; i < burstSize; i++ {
			if i > 0 {
				// Short delay between commits in a burst (30s - 2min)
				time.Sleep(time.Duration(30+rand.Intn(90)) * time.Second)
			}

			sendCommit(commitQueue, stats, commitID)
		}

		// Long pause between bursts (15-45 minutes)
		pauseDuration := time.Duration(15+rand.Intn(31)) * time.Minute
		fmt.Printf("😴 Developer taking a break for %v\n", pauseDuration.Round(time.Second))
		time.Sleep(pauseDuration)
	}
}

// Burst pattern: Frequent short bursts with medium pauses
func generateBurstPattern(commitQueue chan<- CommitEvent, stats *SimulationStats, commitID *int) {
	for {
		// Small burst: 2-4 commits
		burstSize := 2 + rand.Intn(3)

		for i := 0; i < burstSize; i++ {
			if i > 0 {
				time.Sleep(time.Duration(20+rand.Intn(40)) * time.Second)
			}
			sendCommit(commitQueue, stats, commitID)
		}

		// Medium pause (5-10 minutes)
		time.Sleep(time.Duration(5+rand.Intn(6)) * time.Minute)
	}
}

// Steady pattern: Consistent commits every 2-5 minutes
func generateSteadyPattern(commitQueue chan<- CommitEvent, stats *SimulationStats, commitID *int) {
	for {
		time.Sleep(time.Duration(2+rand.Intn(4)) * time.Minute)
		sendCommit(commitQueue, stats, commitID)
	}
}

// Sporadic pattern: Random commits with wide variance (1-30 minutes)
func generateSporadicPattern(commitQueue chan<- CommitEvent, stats *SimulationStats, commitID *int) {
	for {
		time.Sleep(time.Duration(1+rand.Intn(30)) * time.Minute)
		sendCommit(commitQueue, stats, commitID)
	}
}

// Rapid pattern: High frequency commits (30s - 2min)
func generateRapidPattern(commitQueue chan<- CommitEvent, stats *SimulationStats, commitID *int) {
	for {
		time.Sleep(time.Duration(30+rand.Intn(90)) * time.Second)
		sendCommit(commitQueue, stats, commitID)
	}
}

// Custom pattern: Original duration-based logic
func generateCustomPattern(rateSpec string, commitQueue chan<- CommitEvent, stats *SimulationStats, commitID *int) {
	for {
		var waitDuration time.Duration

		// Parse rate specification
		if strings.Contains(rateSpec, "-") {
			// Random range: "1m-5m"
			parts := strings.Split(rateSpec, "-")
			if len(parts) != 2 {
				fmt.Printf("⚠️  Invalid commit rate format: %s\n", rateSpec)
				waitDuration = 1 * time.Minute
			} else {
				minDur, err1 := time.ParseDuration(parts[0])
				maxDur, err2 := time.ParseDuration(parts[1])
				if err1 != nil || err2 != nil {
					fmt.Printf("⚠️  Invalid commit rate format: %s\n", rateSpec)
					waitDuration = 1 * time.Minute
				} else {
					randomRange := maxDur - minDur
					waitDuration = minDur + time.Duration(rand.Int63n(int64(randomRange)))
				}
			}
		} else {
			// Fixed rate: "1m"
			var err error
			waitDuration, err = time.ParseDuration(rateSpec)
			if err != nil {
				fmt.Printf("⚠️  Invalid commit rate format: %s\n", rateSpec)
				waitDuration = 1 * time.Minute
			}
		}

		time.Sleep(waitDuration)
		sendCommit(commitQueue, stats, commitID)
	}
}

// Helper function to send a commit
func sendCommit(commitQueue chan<- CommitEvent, stats *SimulationStats, commitID *int) {
	commit := CommitEvent{
		timestamp: time.Now(),
		id:        *commitID,
	}

	stats.mu.Lock()
	stats.totalCommits++
	stats.queuedCommits++
	stats.mu.Unlock()

	fmt.Printf("📝 New commit detected: #%d (timestamp: %s)\n",
		commit.id, commit.timestamp.Format("15:04:05"))

	commitQueue <- commit
	*commitID++
}

func processBuildQueue(buildDuration time.Duration, commitQueue <-chan CommitEvent,
	buildControl chan bool, stats *SimulationStats, done chan bool) {

	var currentBuild *CommitEvent
	var buildTimer *time.Timer

	for {
		if abortOnNewCommit {
			// Abort mode: always listen for new commits
			commit := <-commitQueue

			if currentBuild != nil {
				// Abort current build
				if buildTimer != nil {
					buildTimer.Stop()
				}
				stats.mu.Lock()
				stats.abortedBuilds++
				stats.mu.Unlock()
				fmt.Printf("❌ Build aborted for commit #%d due to new commit #%d\n",
					currentBuild.id, commit.id)
			}

			// Start new build
			currentBuild = &commit
			now := time.Now()
			stats.mu.Lock()
			stats.currentBuildStartTime = &now
			stats.currentBuildCommitID = commit.id
			stats.queuedCommits = 1 // Only current one in "queue"
			stats.mu.Unlock()

			fmt.Printf("🔨 Starting build for commit #%d (duration: %v)\n",
				commit.id, buildDuration)

			buildTimer = time.AfterFunc(buildDuration, func() {
				completeBuild(*currentBuild, stats)
				currentBuild = nil
				stats.mu.Lock()
				stats.currentBuildStartTime = nil
				stats.currentBuildCommitID = 0
				stats.queuedCommits = 0
				stats.mu.Unlock()
			})
		} else {
			// Queue mode: only consume commits when not building
			if currentBuild == nil {
				// No build in progress, wait for a commit
				commit := <-commitQueue
				currentBuild = &commit
				now := time.Now()
				stats.mu.Lock()
				stats.currentBuildStartTime = &now
				stats.currentBuildCommitID = commit.id
				stats.queuedCommits--
				stats.mu.Unlock()

				fmt.Printf("🔨 Starting build for commit #%d (duration: %v)\n",
					commit.id, buildDuration)

				buildTimer = time.AfterFunc(buildDuration, func() {
					completeBuild(*currentBuild, stats)
					currentBuild = nil
					stats.mu.Lock()
					stats.currentBuildStartTime = nil
					stats.currentBuildCommitID = 0
					stats.mu.Unlock()
				})
			} else {
				// Build in progress, just wait a bit
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}

func completeBuild(commit CommitEvent, stats *SimulationStats) {
	fmt.Printf("✅ Build completed for commit #%d\n", commit.id)

	// Update kustomization file
	if err := bumpManifestVersion(); err != nil {
		fmt.Printf("❌ Error updating manifest: %v\n", err)
	} else {
		stats.mu.Lock()
		stats.completedBuilds++
		stats.mu.Unlock()
		fmt.Printf("📦 Manifest updated and committed for build #%d\n", commit.id)
	}
}

func bumpManifestVersion() error {
	// Read the kustomization file
	data, err := os.ReadFile(manifestKustomizeFilePath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	// Parse YAML
	var kust Kustomization
	if err := yaml.Unmarshal(data, &kust); err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Get current version
	currentVersion := kust.CommonAnnotations["version"]

	// Extract number and increment
	re := regexp.MustCompile(`\d+`)
	matches := re.FindAllString(currentVersion, -1)

	if len(matches) == 0 {
		return fmt.Errorf("no version number found in: %s", currentVersion)
	}

	lastNumStr := matches[len(matches)-1]
	lastNum, _ := strconv.Atoi(lastNumStr)
	newNum := lastNum + 1

	newVersion := re.ReplaceAllStringFunc(currentVersion, func(match string) string {
		if match == lastNumStr {
			return strconv.Itoa(newNum)
		}
		return match
	})

	// Update version
	kust.CommonAnnotations["version"] = newVersion

	// Marshal back to YAML
	updatedData, err := yaml.Marshal(&kust)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	// Write back to file
	if err := os.WriteFile(manifestKustomizeFilePath, updatedData, 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	// Git commit and push
	if !skipGitOperations {
		if err := gitCommitAndPush(newVersion); err != nil {
			fmt.Printf("⚠️  Git operations failed: %v\n", err)
			// Don't return error - we still updated the file
		}
	} else {
		fmt.Printf("⚠️  Git operations skipped (--skipGitOperations=true)\n")
	}

	return nil
}

func gitCommitAndPush(version string) error {
	// Get absolute path and directory
	absPath, err := filepath.Abs(manifestKustomizeFilePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	dir := filepath.Dir(absPath)
	fileName := filepath.Base(absPath)

	// Git add
	cmd := exec.Command("git", "add", fileName)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git add failed: %w (stderr: %s)", err, stderr.String())
	}

	// Generate a random date older than 5 days
	daysAgo := 5 + rand.Intn(30) // 5-35 days ago
	hoursAgo := rand.Intn(24)
	minutesAgo := rand.Intn(60)
	secondsAgo := rand.Intn(60)

	randomPastDate := time.Now().AddDate(0, 0, -daysAgo).
		Add(-time.Duration(hoursAgo) * time.Hour).
		Add(-time.Duration(minutesAgo) * time.Minute).
		Add(-time.Duration(secondsAgo) * time.Second)

	formattedDate := randomPastDate.Format("2006-01-02T15:04:05-07:00")

	// Get a random commit SHA from the cache
	randomSHA := getRandomCommitSHA()

	// Git commit with trailers
	commitMsg := fmt.Sprintf(`chore: bump version to %s

Argocd-reference-commit-author: Zach Aller <code@example.com>
Argocd-reference-commit-sha: %s
Argocd-reference-commit-subject: This change fixes a bug in the code %s
Argocd-reference-commit-body: "Commit message of the code commit\n\nSigned-off-by: Author Name <author@example.com>"
Argocd-reference-commit-repourl: https://github.com/argoproj-labs/gitops-promoter
Argocd-reference-commit-date: %s
Signed-off-by: Zach Aller <zach_aller@intuit.com>`, version, randomSHA, version, formattedDate)

	cmd = exec.Command("git", "commit", "-m", commitMsg)
	cmd.Dir = dir
	stderr.Reset()
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git commit failed: %w (stderr: %s)", err, stderr.String())
	}

	// Git push
	cmd = exec.Command("git", "push")
	cmd.Dir = dir
	stderr.Reset()
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git push failed: %w (stderr: %s)", err, stderr.String())
	}

	return nil
}

func printStats(stats *SimulationStats, done chan bool) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			stats.mu.Lock()
			fmt.Println("\n📊 === Statistics ===")
			fmt.Printf("Total Commits: %d\n", stats.totalCommits)
			fmt.Printf("Queued Commits: %d\n", stats.queuedCommits)
			fmt.Printf("Completed Builds: %d\n", stats.completedBuilds)
			if abortOnNewCommit {
				fmt.Printf("Aborted Builds: %d\n", stats.abortedBuilds)
			}

			if stats.currentBuildStartTime != nil {
				elapsed := time.Since(*stats.currentBuildStartTime)
				fmt.Printf("Current Build: Commit #%d (%v elapsed)\n", stats.currentBuildCommitID, elapsed.Round(time.Second))
			} else {
				fmt.Println("Current Build: None")
			}
			fmt.Println("===================")
			stats.mu.Unlock()

		case <-done:
			return
		}
	}
}
