package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joshpurvis/agentenv/internal/config"
	"github.com/joshpurvis/agentenv/internal/registry"
	"gopkg.in/yaml.v3"
)

// ComposeOverride represents a docker-compose override file structure
type ComposeOverride struct {
	Services map[string]ServiceOverride `yaml:"services"`
	Volumes  map[string]interface{}     `yaml:"volumes,omitempty"`
}

// ServiceOverride represents service-specific overrides
type ServiceOverride struct {
	ContainerName string              `yaml:"container_name,omitempty"`
	Ports         []string            `yaml:"ports,omitempty"`
	Volumes       []string            `yaml:"volumes,omitempty"`
	Environment   map[string]string   `yaml:"environment,omitempty"`
	DependsOn     []string            `yaml:"depends_on,omitempty"`
}

// GenerateOverride creates a docker-compose override file for an agent
// It takes the config, agent details, agent ID, and project name
// Returns the path to the generated override file and any error
func GenerateOverride(cfg *config.Config, agent *registry.Agent, agentID int, projectName string) (string, error) {
	override := ComposeOverride{
		Services: make(map[string]ServiceOverride),
		Volumes:  make(map[string]interface{}),
	}

	// Process each service in the config
	for serviceName, serviceCfg := range cfg.Docker.Services {
		serviceOverride := ServiceOverride{}

		// Set container name with agent ID in the middle for better tab completion
		serviceOverride.ContainerName = fmt.Sprintf("%s-agent%d-%s", projectName, agentID, serviceName)

		// Map ports: "hostPort:containerPort"
		if len(serviceCfg.Ports) > 0 {
			serviceOverride.Ports = make([]string, 0, len(serviceCfg.Ports))
			for _, portMapping := range serviceCfg.Ports {
				hostPort := agent.Ports[serviceName]
				containerPort := portMapping.Container
				serviceOverride.Ports = append(serviceOverride.Ports,
					fmt.Sprintf("%d:%d", hostPort, containerPort))
			}
		}

		// Remap volumes with agent suffix
		if len(serviceCfg.Volumes) > 0 {
			serviceOverride.Volumes = make([]string, 0, len(serviceCfg.Volumes))
			for _, volumeName := range serviceCfg.Volumes {
				// Check if this is a named volume (not a bind mount)
				if !strings.Contains(volumeName, "/") && !strings.HasPrefix(volumeName, ".") {
					newVolumeName := fmt.Sprintf("%s_agent%d", volumeName, agentID)

					// Add to volumes section
					override.Volumes[newVolumeName] = nil

					// Find the mount path in the original volume spec
					// Format: "volumeName:/path/in/container"
					// We need to preserve the container path
					serviceOverride.Volumes = append(serviceOverride.Volumes,
						fmt.Sprintf("%s:%s", newVolumeName, getVolumeMountPath(volumeName)))
				} else {
					// Keep bind mounts as-is
					serviceOverride.Volumes = append(serviceOverride.Volumes, volumeName)
				}
			}
		}

		// Apply environment variable templates
		if len(serviceCfg.Environment) > 0 {
			serviceOverride.Environment = make(map[string]string)
			for key, value := range serviceCfg.Environment {
				// Replace template variables
				replaced := replaceTemplateVars(value, agent, agentID)
				serviceOverride.Environment[key] = replaced
			}
		}

		// Preserve depends_on relationships
		if len(serviceCfg.DependsOn) > 0 {
			serviceOverride.DependsOn = serviceCfg.DependsOn
		}

		override.Services[serviceName] = serviceOverride
	}

	// Generate output file path
	outputPath := filepath.Join(agent.WorktreePath, agent.DockerComposeOverride)

	// Marshal to YAML
	data, err := yaml.Marshal(&override)
	if err != nil {
		return "", fmt.Errorf("failed to marshal override to YAML: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write override file: %w", err)
	}

	return outputPath, nil
}

// getVolumeMountPath extracts the container mount path from a volume name
// This is a simplified version - in a real implementation, you'd need to
// parse the original docker-compose.yml to get the actual mount paths
func getVolumeMountPath(volumeName string) string {
	// Common patterns for volume mount paths
	mountPaths := map[string]string{
		"postgres_data": "/var/lib/postgresql/data",
		"redis_data":    "/data",
		"mongo_data":    "/data/db",
	}

	if path, ok := mountPaths[volumeName]; ok {
		return path
	}

	// Default fallback - you should parse the original compose file instead
	return "/data"
}

// replaceTemplateVars replaces template variables in strings
// Supports: {postgres.port}, {backend.port}, {frontend.port}, {id}, {worktree_path}
func replaceTemplateVars(value string, agent *registry.Agent, agentID int) string {
	result := value

	// Replace port variables: {serviceName.port}
	for serviceName, port := range agent.Ports {
		placeholder := fmt.Sprintf("{%s.port}", serviceName)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%d", port))
	}

	// Replace {id}
	result = strings.ReplaceAll(result, "{id}", fmt.Sprintf("%d", agentID))

	// Replace {worktree_path}
	result = strings.ReplaceAll(result, "{worktree_path}", agent.WorktreePath)

	return result
}

// StartServices starts Docker Compose services with the override file
func StartServices(worktreePath, overridePath string) error {
	// This would execute: docker-compose -f docker-compose.yml -f override.yml up -d
	// Implementation in a separate function or as part of the main command
	return fmt.Errorf("not implemented - use exec.Command to run docker-compose")
}

// StopServices stops Docker Compose services
func StopServices(worktreePath, overridePath string) error {
	// This would execute: docker-compose -f docker-compose.yml -f override.yml down
	return fmt.Errorf("not implemented - use exec.Command to run docker-compose")
}
