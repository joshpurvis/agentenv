package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/joshpurvis/agentenv/internal/config"
	"github.com/joshpurvis/agentenv/internal/git"
	"github.com/joshpurvis/agentenv/internal/registry"
	"github.com/spf13/cobra"
)

// downCmd represents the down command
var downCmd = &cobra.Command{
	Use:   "down <agent-id>",
	Short: "Stop and cleanup an agent environment",
	Long: `Stop an agent environment and clean up all resources.

This will:
- Archive the database (if configured)
- Stop Docker services
- Remove volumes
- Remove git worktree
- Update registry

Example:
  agentenv down agent1`,
	Args: cobra.ExactArgs(1),
	RunE: runDown,
}

func init() {
	rootCmd.AddCommand(downCmd)
	downCmd.Flags().Bool("skip-archive", false, "Skip database archival")
	downCmd.Flags().Bool("keep-worktree", false, "Keep the git worktree")
}

func runDown(cmd *cobra.Command, args []string) error {
	agentID := args[0]

	verbose, _ := cmd.Flags().GetBool("verbose")
	skipArchive, _ := cmd.Flags().GetBool("skip-archive")
	keepWorktree, _ := cmd.Flags().GetBool("keep-worktree")

	fmt.Printf("🧹 Cleaning up agent '%s'\n\n", agentID)

	// Start building cleanup log
	var cleanupLog strings.Builder
	cleanupLog.WriteString(fmt.Sprintf("Cleanup log for %s\n", agentID))
	cleanupLog.WriteString(fmt.Sprintf("Date: %s\n", time.Now().Format(time.RFC3339)))
	cleanupLog.WriteString(strings.Repeat("=", 60) + "\n\n")

	// Get current directory (repo root)
	repoPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// 1. Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// 2. Load registry
	reg, err := registry.LoadRegistry()
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	// 3. Get agent
	agent, err := reg.GetAgent(agentID)
	if err != nil {
		return fmt.Errorf("agent not found: %w", err)
	}

	// 4. Archive database (if enabled)
	if cfg.Cleanup.ArchiveDatabase && !skipArchive {
		fmt.Println("💾 Archiving database...")
		cleanupLog.WriteString("Step 1: Archive database\n")
		if err := archiveDatabase(cfg, agent, agentID, agent.PortSlot, reg.Project, verbose); err != nil {
			fmt.Printf("  ⚠️  Warning: failed to archive database: %v\n", err)
			cleanupLog.WriteString(fmt.Sprintf("  Status: FAILED - %v\n\n", err))
			// Continue anyway
		} else {
			fmt.Println("✓ Database archived")
			cleanupLog.WriteString("  Status: SUCCESS\n\n")
		}
	} else {
		cleanupLog.WriteString("Step 1: Archive database\n")
		cleanupLog.WriteString("  Status: SKIPPED\n\n")
	}

	// 5. Stop Docker services
	fmt.Println("\n🐳 Stopping Docker services...")
	cleanupLog.WriteString("Step 2: Stop Docker services\n")
	if err := stopDockerServices(cfg, agent, verbose); err != nil {
		fmt.Printf("  ⚠️  Warning: failed to stop services: %v\n", err)
		cleanupLog.WriteString(fmt.Sprintf("  Status: FAILED - %v\n\n", err))
		// Continue anyway
	} else {
		fmt.Println("✓ Docker services stopped")
		cleanupLog.WriteString("  Status: SUCCESS\n\n")
	}

	// 6. Remove volumes (if enabled)
	if cfg.Cleanup.RemoveVolumes {
		fmt.Println("\n🗑️  Removing volumes...")
		cleanupLog.WriteString("Step 3: Remove volumes\n")
		if err := removeVolumes(cfg, agent, verbose); err != nil {
			fmt.Printf("  ⚠️  Warning: failed to remove volumes: %v\n", err)
			cleanupLog.WriteString(fmt.Sprintf("  Status: FAILED - %v\n\n", err))
			// Continue anyway
		} else {
			fmt.Println("✓ Volumes removed")
			cleanupLog.WriteString("  Status: SUCCESS\n\n")
		}
	} else {
		cleanupLog.WriteString("Step 3: Remove volumes\n")
		cleanupLog.WriteString("  Status: SKIPPED\n\n")
	}

	// 7. Fix file permissions (Docker containers may create root-owned files)
	if !keepWorktree {
		if verbose {
			fmt.Println("\n🔧 Fixing file permissions...")
		}
		cleanupLog.WriteString("Step 4: Fix file permissions\n")
		// Use Docker to fix permissions (runs as root, can chown everything)
		cmd := exec.Command("docker", "run", "--rm", "-v", fmt.Sprintf("%s:/workspace", agent.WorktreePath),
			"alpine", "sh", "-c", "chmod -R 777 /workspace || true")
		if output, err := cmd.CombinedOutput(); err != nil {
			if verbose {
				fmt.Printf("  Note: Could not fix permissions (this is OK): %v\n", err)
			}
			cleanupLog.WriteString(fmt.Sprintf("  Status: SKIPPED - %v\n  Output: %s\n\n", err, output))
		} else {
			cleanupLog.WriteString("  Status: SUCCESS\n\n")
		}
	}

	// 8. Remove git worktree
	if !keepWorktree {
		fmt.Printf("\n📂 Removing git worktree at %s...\n", agent.WorktreePath)
		cleanupLog.WriteString("Step 5: Remove git worktree\n")
		cleanupLog.WriteString(fmt.Sprintf("  Path: %s\n", agent.WorktreePath))
		if err := git.RemoveWorktree(repoPath, agent.WorktreePath, true); err != nil {
			fmt.Printf("  ⚠️  Warning: failed to remove worktree: %v\n", err)
			fmt.Printf("  You may need to manually run: sudo rm -rf %s\n", agent.WorktreePath)
			cleanupLog.WriteString(fmt.Sprintf("  Status: FAILED - %v\n\n", err))
		} else {
			fmt.Println("✓ Worktree removed")
			cleanupLog.WriteString("  Status: SUCCESS\n\n")
		}
	} else {
		cleanupLog.WriteString("Step 5: Remove git worktree\n")
		cleanupLog.WriteString("  Status: SKIPPED (--keep-worktree flag)\n\n")
	}

	// 9. Update registry
	cleanupLog.WriteString("Step 6: Update registry\n")
	if err := reg.RemoveAgent(agentID); err != nil {
		cleanupLog.WriteString(fmt.Sprintf("  Status: FAILED - %v\n\n", err))
		return fmt.Errorf("failed to remove agent from registry: %w", err)
	}
	if err := reg.Save(); err != nil {
		cleanupLog.WriteString(fmt.Sprintf("  Status: FAILED - %v\n\n", err))
		return fmt.Errorf("failed to save registry: %w", err)
	}
	cleanupLog.WriteString("  Status: SUCCESS\n\n")

	// 9. Save cleanup log
	if err := os.MkdirAll(cfg.Cleanup.ArchiveLocation, 0755); err == nil {
		timestamp := time.Now().Format("20060102-150405")
		logFile := filepath.Join(cfg.Cleanup.ArchiveLocation,
			fmt.Sprintf("cleanup-%s-%s.log", agentID, timestamp))

		if err := os.WriteFile(logFile, []byte(cleanupLog.String()), 0644); err == nil {
			fmt.Printf("\n📋 Cleanup log saved to: %s\n", logFile)
		}
	}

	fmt.Println("\n✓ Agent cleaned up successfully")

	return nil
}

func archiveDatabase(cfg *config.Config, agent *registry.Agent, agentID string, numericID int, projectName string, verbose bool) error {
	// Create archive directory if it doesn't exist
	if err := os.MkdirAll(cfg.Cleanup.ArchiveLocation, 0755); err != nil {
		return fmt.Errorf("failed to create archive directory: %w", err)
	}

	// Generate archive filename
	timestamp := time.Now().Format("20060102-150405")
	archiveFile := filepath.Join(cfg.Cleanup.ArchiveLocation,
		fmt.Sprintf("%s-%s.sql", agentID, timestamp))

	// Get database connection info
	dbService := cfg.Database.Service
	dbPort, ok := agent.Ports[dbService]
	if !ok {
		return fmt.Errorf("database service %s not found in agent ports", dbService)
	}

	// For PostgreSQL
	if cfg.Database.Type == "postgresql" {
		// Parse database name from environment
		dbEnv := cfg.Docker.Services[dbService].Environment
		dbName := dbEnv["POSTGRES_DB"]
		if dbName == "" {
			dbName = fmt.Sprintf("%s_agent%d", projectName, numericID)
		}
		// Replace template variables in database name
		dbName = strings.ReplaceAll(dbName, "{id}", fmt.Sprintf("%d", numericID))

		// Extract username and password from environment or use defaults
		dbUser := dbEnv["POSTGRES_USER"]
		if dbUser == "" {
			dbUser = "postgres"
		}
		dbPassword := dbEnv["POSTGRES_PASSWORD"]
		if dbPassword == "" {
			dbPassword = "postgres"
		}

		// Run pg_dump
		cmd := exec.Command("pg_dump",
			"-h", "localhost",
			"-p", fmt.Sprintf("%d", dbPort),
			"-U", dbUser,
			"-d", dbName,
			"-f", archiveFile)

		// Set PGPASSWORD environment variable
		cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", dbPassword))

		// Capture output for error reporting
		var stderr strings.Builder
		if verbose {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		} else {
			cmd.Stderr = &stderr
		}

		if err := cmd.Run(); err != nil {
			if stderr.Len() > 0 {
				return fmt.Errorf("pg_dump failed: %w\nOutput: %s", err, stderr.String())
			}
			return fmt.Errorf("pg_dump failed: %w", err)
		}

		fmt.Printf("  Archive saved to: %s\n", archiveFile)
	}

	return nil
}

func stopDockerServices(cfg *config.Config, agent *registry.Agent, verbose bool) error {
	cmd := exec.Command("docker-compose",
		"-f", cfg.Docker.ComposeFile,
		"-f", agent.DockerComposeOverride,
		"down")
	cmd.Dir = agent.WorktreePath

	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker-compose down failed: %w", err)
	}

	return nil
}

func removeVolumes(cfg *config.Config, agent *registry.Agent, verbose bool) error {
	cmd := exec.Command("docker-compose",
		"-f", cfg.Docker.ComposeFile,
		"-f", agent.DockerComposeOverride,
		"down", "-v")
	cmd.Dir = agent.WorktreePath

	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker-compose down -v failed: %w", err)
	}

	return nil
}
