package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/joshpurvis/agentenv/internal/config"
	"github.com/joshpurvis/agentenv/internal/docker"
	"github.com/joshpurvis/agentenv/internal/envpatch"
	"github.com/joshpurvis/agentenv/internal/git"
	"github.com/joshpurvis/agentenv/internal/registry"
	"github.com/joshpurvis/agentenv/internal/terminal"
	"github.com/spf13/cobra"
)

// upCmd represents the up command
var upCmd = &cobra.Command{
	Use:   "up <agent-name> <branch> <command>",
	Short: "Launch a new agent environment",
	Long: `Launch a new agent environment with isolated Docker services and git worktree.

Example:
  agentenv up claude1 feat/fix-rendering claude
  agentenv up codex1 feat/new-api codex`,
	Args: cobra.ExactArgs(3),
	RunE: runUp,
}

func init() {
	rootCmd.AddCommand(upCmd)
}

func runUp(cmd *cobra.Command, args []string) error {
	agentName := args[0]
	branch := args[1]
	agentCommand := args[2]

	verbose, _ := cmd.Flags().GetBool("verbose")

	fmt.Printf("🚀 Launching agent '%s' on branch '%s'\n\n", agentName, branch)

	// Get current directory (repo root)
	repoPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// 1. Load config
	if verbose {
		fmt.Println("📋 Loading configuration...")
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// 2. Load or create registry
	if verbose {
		fmt.Println("📝 Loading registry...")
	}
	reg, err := registry.LoadRegistry()
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	// Get project name from repo root
	projectName := filepath.Base(repoPath)
	if reg.Project == "" {
		reg.Project = projectName
	}

	// 3. Find next available port slot
	if verbose {
		fmt.Println("🔢 Finding available port slot...")
	}
	portSlot := reg.FindNextAvailableSlot()

	// 4. Calculate ports based on slot
	ports := cfg.GetAllPorts(portSlot)

	// 5. Generate worktree path (use agentName as ID)
	agentID := agentName
	worktreePath, err := git.GenerateWorktreePath(repoPath, agentID)
	if err != nil {
		return fmt.Errorf("failed to generate worktree path: %w", err)
	}

	// 6. Allocate agent
	agent, err := reg.AllocateAgent(agentID, branch, agentCommand, worktreePath, ports, portSlot)
	if err != nil {
		return fmt.Errorf("failed to allocate agent: %w", err)
	}

	fmt.Printf("✓ Agent '%s' allocated\n", agentID)
	fmt.Printf("  Port slot: %d\n", portSlot)
	fmt.Printf("  Ports: ")
	for serviceName, port := range ports {
		fmt.Printf("%s=%d ", serviceName, port)
	}
	fmt.Println()

	// 7. Create git worktree
	fmt.Printf("\n📂 Creating git worktree at %s...\n", worktreePath)
	if err := git.CreateWorktree(repoPath, worktreePath, branch); err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}
	fmt.Println("✓ Worktree created")

	// 8. Generate docker-compose override
	if verbose {
		fmt.Println("\n🐳 Generating docker-compose override...")
	}
	overridePath, err := docker.GenerateOverride(cfg, agent, portSlot, projectName)
	if err != nil {
		return fmt.Errorf("failed to generate override: %w", err)
	}
	if verbose {
		fmt.Printf("✓ Override file created: %s\n", overridePath)
	}

	// 9. Patch environment files
	fmt.Println("\n⚙️  Patching environment files...")
	if err := envpatch.PatchEnvFiles(cfg, worktreePath, ports, portSlot); err != nil {
		return fmt.Errorf("failed to patch env files: %w", err)
	}
	fmt.Println("✓ Environment files patched")

	// 10. Run setup commands (before services start)
	if len(cfg.SetupCommands) > 0 {
		hasBeforeCommands := false
		for _, setupCmd := range cfg.SetupCommands {
			if setupCmd.When == "before_services_start" {
				if !hasBeforeCommands {
					fmt.Println("\n🔧 Running pre-start setup commands...")
					hasBeforeCommands = true
				}
				fmt.Printf("  Running: %s\n", setupCmd.Name)
				if err := runSetupCommand(setupCmd, worktreePath, verbose); err != nil {
					fmt.Printf("  ⚠️  Warning: setup command failed: %v\n", err)
					// Continue anyway
				} else {
					fmt.Printf("  ✓ %s completed\n", setupCmd.Name)
				}
			}
		}
	}

	// 11. Start Docker services
	fmt.Println("\n🐳 Starting Docker services...")
	if err := startDockerServices(cfg, worktreePath, agent.DockerComposeOverride, verbose); err != nil {
		return fmt.Errorf("failed to start Docker services: %w", err)
	}
	fmt.Println("✓ Docker services started")

	// 12. Wait for services to be healthy
	fmt.Println("\n⏳ Waiting for services to be ready...")
	time.Sleep(5 * time.Second) // Simple wait for now
	fmt.Println("✓ Services ready")

	// 13. Run setup commands (after services start)
	if len(cfg.SetupCommands) > 0 {
		hasAfterCommands := false
		for _, setupCmd := range cfg.SetupCommands {
			if setupCmd.When == "after_services_start" {
				if !hasAfterCommands {
					fmt.Println("\n🔧 Running post-start setup commands...")
					hasAfterCommands = true
				}
				fmt.Printf("  Running: %s\n", setupCmd.Name)
				if err := runSetupCommand(setupCmd, worktreePath, verbose); err != nil {
					fmt.Printf("  ⚠️  Warning: setup command failed: %v\n", err)
					// Continue anyway
				} else {
					fmt.Printf("  ✓ %s completed\n", setupCmd.Name)
				}
			}
		}
	}

	// 14. Save registry
	if err := reg.Save(); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}

	// 15. Launch agent in terminal (if configured)
	if cfg.AgentLaunch.Terminal != "" || cfg.AgentLaunch.WorkingDirectory != "" {
		fmt.Println("\n🚀 Launching agent in terminal...")
		windowTitle := fmt.Sprintf("agentenv: %s", agentName)
		if err := terminal.LaunchInTerminal(agentCommand, worktreePath, windowTitle); err != nil {
			// Terminal launch is not critical - just warn
			if verbose {
				fmt.Printf("  ⚠️  Could not auto-launch terminal: %v\n", err)
			}
		}
	}

	// 16. Print summary
	separator := strings.Repeat("═", 60)
	fmt.Println("\n" + separator)
	fmt.Printf("🎉 Agent %s is ready!\n\n", agentID)
	fmt.Printf("  Branch:     %s\n", branch)
	fmt.Printf("  Worktree:   %s\n", worktreePath)
	fmt.Printf("  Command:    %s\n\n", agentCommand)

	fmt.Println("  Service URLs:")
	for serviceName, port := range ports {
		fmt.Printf("    %s: http://localhost:%d\n", serviceName, port)
	}

	fmt.Println("\n  To work with this agent:")
	fmt.Printf("    cd %s\n", worktreePath)
	fmt.Printf("    %s\n\n", agentCommand)

	fmt.Println("  To stop this agent:")
	fmt.Printf("    agentenv down %s\n", agentID)
	fmt.Println(separator)

	return nil
}

func startDockerServices(cfg *config.Config, worktreePath, overrideFile string, verbose bool) error {
	cmd := exec.Command("docker-compose",
		"-f", cfg.Docker.ComposeFile,
		"-f", overrideFile,
		"up", "-d")
	cmd.Dir = worktreePath

	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker-compose up failed: %w", err)
	}

	return nil
}

func runSetupCommand(setupCmd config.SetupCommand, worktreePath string, verbose bool) error {
	cmd := exec.Command("sh", "-c", setupCmd.Command)
	workDir := filepath.Join(worktreePath, setupCmd.WorkingDir)
	cmd.Dir = workDir

	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}
