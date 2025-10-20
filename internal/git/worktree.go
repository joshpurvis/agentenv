package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CreateWorktree creates a new git worktree at the specified path
// repoPath: path to the main repository
// worktreePath: path where the worktree should be created
// branch: branch name to checkout (will be created if it doesn't exist)
func CreateWorktree(repoPath, worktreePath, branch string) error {
	// First, check if the branch exists
	branchExists, err := CheckBranchExists(repoPath, branch)
	if err != nil {
		return fmt.Errorf("failed to check if branch exists: %w", err)
	}

	// Check if worktree path already exists
	if _, err := os.Stat(worktreePath); err == nil {
		return fmt.Errorf("worktree path already exists: %s", worktreePath)
	}

	var cmd *exec.Cmd
	if branchExists {
		// Branch exists, checkout existing branch
		cmd = exec.Command("git", "worktree", "add", worktreePath, branch)
	} else {
		// Branch doesn't exist, create new branch
		cmd = exec.Command("git", "worktree", "add", "-b", branch, worktreePath)
	}
	cmd.Dir = repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create worktree: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// RemoveWorktree removes a git worktree and cleans up
// repoPath: path to the main repository
// worktreePath: path to the worktree to remove
// force: if true, removes even if there are uncommitted changes
func RemoveWorktree(repoPath, worktreePath string, force bool) error {
	// First, check if the worktree exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		// Worktree directory doesn't exist, but git might still track it
		// Try to prune it from git's tracking
		return pruneWorktree(repoPath, worktreePath)
	}

	// Build the git worktree remove command
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, worktreePath)

	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to remove worktree: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// pruneWorktree removes stale worktree entries from git's tracking
func pruneWorktree(repoPath, worktreePath string) error {
	cmd := exec.Command("git", "worktree", "prune")
	cmd.Dir = repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to prune worktrees: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// CheckBranchExists checks if a branch exists in the repository
// Returns true if the branch exists (locally or remotely), false otherwise
func CheckBranchExists(repoPath, branch string) (bool, error) {
	// Check local branches
	cmd := exec.Command("git", "branch", "--list", branch)
	cmd.Dir = repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to list branches: %w", err)
	}

	// If output is not empty, branch exists locally
	if len(strings.TrimSpace(string(output))) > 0 {
		return true, nil
	}

	// Check remote branches
	cmd = exec.Command("git", "branch", "--list", "-r", "*/"+branch)
	cmd.Dir = repoPath

	output, err = cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to list remote branches: %w", err)
	}

	// If output is not empty, branch exists remotely
	if len(strings.TrimSpace(string(output))) > 0 {
		return true, nil
	}

	return false, nil
}

// ListWorktrees returns a list of all worktrees in the repository
func ListWorktrees(repoPath string) ([]WorktreeInfo, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	return parseWorktreeList(string(output)), nil
}

// WorktreeInfo contains information about a git worktree
type WorktreeInfo struct {
	Path   string
	Branch string
	Commit string
	Bare   bool
}

// parseWorktreeList parses the output of 'git worktree list --porcelain'
func parseWorktreeList(output string) []WorktreeInfo {
	var worktrees []WorktreeInfo
	lines := strings.Split(strings.TrimSpace(output), "\n")

	var current WorktreeInfo
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			// Empty line indicates end of worktree entry
			if current.Path != "" {
				worktrees = append(worktrees, current)
				current = WorktreeInfo{}
			}
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		switch key {
		case "worktree":
			current.Path = value
		case "branch":
			// Remove 'refs/heads/' prefix
			current.Branch = strings.TrimPrefix(value, "refs/heads/")
		case "HEAD":
			current.Commit = value
		case "bare":
			current.Bare = true
		}
	}

	// Add the last entry if exists
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees
}

// GetCurrentBranch returns the current branch name of a repository
func GetCurrentBranch(repoPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// GetRepoRoot returns the root directory of the git repository
func GetRepoRoot(path string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = path

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get repo root: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// IsGitRepo checks if the given path is inside a git repository
func IsGitRepo(path string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = path

	err := cmd.Run()
	return err == nil
}

// GetWorktreeParentDir returns the parent directory name for worktrees
// Given /home/user/projects/myapp, returns /home/user/projects
func GetWorktreeParentDir(repoPath string) (string, error) {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	return filepath.Dir(absPath), nil
}

// GenerateWorktreePath generates a worktree path based on convention
// Given /home/user/projects/myapp and agent name "claude1",
// returns /home/user/projects/myapp-claude1
func GenerateWorktreePath(repoPath, agentName string) (string, error) {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	parentDir := filepath.Dir(absPath)
	baseName := filepath.Base(absPath)
	worktreeName := fmt.Sprintf("%s-%s", baseName, agentName)

	return filepath.Join(parentDir, worktreeName), nil
}
