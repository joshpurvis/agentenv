package env

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/joshpurvis/agentenv/internal/config"
	"github.com/joshpurvis/agentenv/internal/registry"
)

// PatchEnvFile copies an environment file to the worktree and applies patches
// sourcePath: path to the original .env file in the main repo
// destPath: path where the patched .env file should be written
// patches: list of regex patterns and replacements
// agent: agent configuration for template variable substitution
// agentID: numeric agent ID
func PatchEnvFile(sourcePath, destPath string, patches []config.EnvPatch, agent *registry.Agent, agentID int) error {
	// Read the source file
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read source file %s: %w", sourcePath, err)
	}

	// Convert to string for processing
	result := string(content)

	// Apply each patch
	for i, patch := range patches {
		// First, replace template variables in the replacement string
		replacement := replaceTemplateVars(patch.Replace, agent, agentID)

		// Compile the regex pattern
		re, err := regexp.Compile(patch.Pattern)
		if err != nil {
			return fmt.Errorf("failed to compile regex pattern #%d '%s': %w", i, patch.Pattern, err)
		}

		// Apply the replacement
		result = re.ReplaceAllString(result, replacement)
	}

	// Ensure destination directory exists
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", destDir, err)
	}

	// Write the patched content
	if err := os.WriteFile(destPath, []byte(result), 0644); err != nil {
		return fmt.Errorf("failed to write patched file %s: %w", destPath, err)
	}

	return nil
}

// PatchAllEnvFiles processes all environment files from the config
// mainRepoPath: path to the main repository
// agent: agent configuration
// agentID: numeric agent ID
// envFiles: list of environment files to patch from config
func PatchAllEnvFiles(mainRepoPath string, agent *registry.Agent, agentID int, envFiles []config.EnvFile) error {
	for _, envFile := range envFiles {
		sourcePath := filepath.Join(mainRepoPath, envFile.Path)
		destPath := filepath.Join(agent.WorktreePath, envFile.Path)

		if err := PatchEnvFile(sourcePath, destPath, envFile.Patches, agent, agentID); err != nil {
			return fmt.Errorf("failed to patch %s: %w", envFile.Path, err)
		}
	}

	return nil
}

// replaceTemplateVars replaces template variables in strings
// Supports: {serviceName.port}, {id}, {name}, {worktree_path}
func replaceTemplateVars(value string, agent *registry.Agent, agentID int) string {
	result := value

	// Replace port variables: {serviceName.port}
	for serviceName, port := range agent.Ports {
		placeholder := fmt.Sprintf("{%s.port}", serviceName)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%d", port))
	}

	// Replace {id} with port slot number (for backward compatibility)
	result = strings.ReplaceAll(result, "{id}", fmt.Sprintf("%d", agentID))

	// Replace {name} with agent name (new feature)
	result = strings.ReplaceAll(result, "{name}", agent.Name)

	// Replace {worktree_path}
	result = strings.ReplaceAll(result, "{worktree_path}", agent.WorktreePath)

	return result
}

// ValidateEnvFile checks if an environment file exists and is readable
func ValidateEnvFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("environment file does not exist: %s", path)
		}
		return fmt.Errorf("failed to stat file %s: %w", path, err)
	}

	if info.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", path)
	}

	// Try to read the file to ensure we have permissions
	_, err = os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read file %s: %w", path, err)
	}

	return nil
}

// BackupEnvFile creates a backup of an environment file
func BackupEnvFile(path string) (string, error) {
	backupPath := path + ".backup"

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file for backup: %w", err)
	}

	if err := os.WriteFile(backupPath, content, 0644); err != nil {
		return "", fmt.Errorf("failed to write backup file: %w", err)
	}

	return backupPath, nil
}

// MergeEnvVars merges two sets of environment variables
// Useful for combining base config with agent-specific overrides
func MergeEnvVars(base, override map[string]string) map[string]string {
	result := make(map[string]string)

	// Copy base
	for k, v := range base {
		result[k] = v
	}

	// Apply overrides
	for k, v := range override {
		result[k] = v
	}

	return result
}

// ParseEnvFile parses a .env file into a map
// Simple parser that handles KEY=VALUE format
func ParseEnvFile(path string) (map[string]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read env file: %w", err)
	}

	envVars := make(map[string]string)
	lines := strings.Split(string(content), "\n")

	for lineNum, line := range lines {
		// Trim whitespace
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split on first '='
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid line %d: %s", lineNum+1, line)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		value = strings.Trim(value, "\"'")

		envVars[key] = value
	}

	return envVars, nil
}

// WriteEnvFile writes a map of environment variables to a file
func WriteEnvFile(path string, envVars map[string]string) error {
	var lines []string

	// Sort keys for consistent output (optional)
	for key, value := range envVars {
		// Quote values that contain spaces
		if strings.Contains(value, " ") {
			value = fmt.Sprintf("\"%s\"", value)
		}
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}

	content := strings.Join(lines, "\n") + "\n"

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write env file: %w", err)
	}

	return nil
}
