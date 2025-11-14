package envpatch

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/joshpurvis/agentenv/internal/config"
)

// PatchEnvFiles patches all environment files according to the configuration
func PatchEnvFiles(cfg *config.Config, worktreePath string, ports map[string]int, agentID int, agentName string) error {
	// Get current directory (main repo root)
	mainRepoPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	for _, envFile := range cfg.EnvFiles {
		// Source path in main repo
		mainEnvPath := filepath.Join(mainRepoPath, envFile.Path)
		// Destination path in worktree
		worktreeEnvPath := filepath.Join(worktreePath, envFile.Path)

		// Copy env file from main repo to worktree if it exists
		if _, err := os.Stat(mainEnvPath); err == nil {
			// Create directory if needed
			if err := os.MkdirAll(filepath.Dir(worktreeEnvPath), 0755); err != nil {
				return fmt.Errorf("failed to create directory for %s: %w", worktreeEnvPath, err)
			}

			// Copy file
			content, err := os.ReadFile(mainEnvPath)
			if err != nil {
				return fmt.Errorf("failed to read env file %s: %w", mainEnvPath, err)
			}

			if err := os.WriteFile(worktreeEnvPath, content, 0644); err != nil {
				return fmt.Errorf("failed to copy env file to %s: %w", worktreeEnvPath, err)
			}
		} else {
			fmt.Printf("Warning: env file %s does not exist in main repo, skipping\n", mainEnvPath)
			continue
		}

		// Read the copied file
		content, err := os.ReadFile(worktreeEnvPath)
		if err != nil {
			return fmt.Errorf("failed to read env file %s: %w", worktreeEnvPath, err)
		}

		contentStr := string(content)

		// Apply patches
		for _, patch := range envFile.Patches {
			pattern := patch.Pattern
			replacement := patch.Replace

			// Replace template variables in the replacement string
			replacement = replacePlaceholders(replacement, ports, agentID, agentName, worktreePath)

			// Apply regex replacement
			re, err := regexp.Compile(pattern)
			if err != nil {
				return fmt.Errorf("invalid regex pattern '%s': %w", pattern, err)
			}

			contentStr = re.ReplaceAllString(contentStr, replacement)
		}

		// Write the patched file
		if err := os.WriteFile(worktreeEnvPath, []byte(contentStr), 0644); err != nil {
			return fmt.Errorf("failed to write patched env file %s: %w", worktreeEnvPath, err)
		}
	}

	return nil
}

// replacePlaceholders replaces template variables in a string
func replacePlaceholders(str string, ports map[string]int, agentID int, agentName string, worktreePath string) string {
	// Replace {service.port} placeholders
	for serviceName, port := range ports {
		placeholder := fmt.Sprintf("{%s.port}", serviceName)
		str = strings.ReplaceAll(str, placeholder, fmt.Sprintf("%d", port))
	}

	// Replace {id} placeholder (port slot number for backward compatibility)
	str = strings.ReplaceAll(str, "{id}", fmt.Sprintf("%d", agentID))

	// Replace {name} placeholder (agent name)
	str = strings.ReplaceAll(str, "{name}", agentName)

	// Replace {worktree_path} placeholder
	str = strings.ReplaceAll(str, "{worktree_path}", worktreePath)

	return str
}
