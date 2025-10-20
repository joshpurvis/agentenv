package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the .agentenv.yml configuration
type Config struct {
	Docker         DockerConfig        `yaml:"docker"`
	EnvFiles       []EnvFile           `yaml:"env_files"`
	Database       DatabaseConfig      `yaml:"database"`
	SetupCommands  []SetupCommand      `yaml:"setup_commands"`
	AgentLaunch    AgentLaunchConfig   `yaml:"agent_launch"`
	Cleanup        CleanupConfig       `yaml:"cleanup"`
}

// DockerConfig contains Docker Compose configuration
type DockerConfig struct {
	ComposeFile string                    `yaml:"compose_file"`
	Services    map[string]ServiceConfig  `yaml:"services"`
}

// ServiceConfig represents a Docker service configuration
type ServiceConfig struct {
	Ports       []PortMapping         `yaml:"ports"`
	Volumes     []string              `yaml:"volumes"`
	Environment map[string]string     `yaml:"environment"`
	DependsOn   []string              `yaml:"depends_on"`
}

// PortMapping represents a port mapping configuration
type PortMapping struct {
	Container int `yaml:"container"`
	HostBase  int `yaml:"host_base"`
}

// EnvFile represents an environment file to patch
type EnvFile struct {
	Path    string      `yaml:"path"`
	Patches []EnvPatch  `yaml:"patches"`
}

// EnvPatch represents a regex replacement in an env file
type EnvPatch struct {
	Pattern string `yaml:"pattern"`
	Replace string `yaml:"replace"`
}

// DatabaseConfig contains database initialization settings
type DatabaseConfig struct {
	Service    string           `yaml:"service"`
	Type       string           `yaml:"type"`
	MainURL    string           `yaml:"main_url"`
	Migrations MigrationsConfig `yaml:"migrations"`
}

// MigrationsConfig contains migration command settings
type MigrationsConfig struct {
	Command    string `yaml:"command"`
	WorkingDir string `yaml:"working_dir"`
}

// SetupCommand represents a custom setup command
type SetupCommand struct {
	Name       string `yaml:"name"`
	Command    string `yaml:"command"`
	WorkingDir string `yaml:"working_dir"`
	When       string `yaml:"when"`
}

// AgentLaunchConfig contains agent launch settings
type AgentLaunchConfig struct {
	Terminal         string `yaml:"terminal"`
	WorkingDirectory string `yaml:"working_directory"`
}

// CleanupConfig contains cleanup settings
type CleanupConfig struct {
	ArchiveDatabase bool   `yaml:"archive_database"`
	ArchiveLocation string `yaml:"archive_location"`
	RemoveVolumes   bool   `yaml:"remove_volumes"`
}

// LoadConfig loads the .agentenv.yml configuration from the current directory
func LoadConfig() (*Config, error) {
	return LoadConfigFromPath(".agentenv.yml")
}

// LoadConfigFromPath loads the configuration from a specific path
func LoadConfigFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if config.Docker.ComposeFile == "" {
		config.Docker.ComposeFile = "docker-compose.yml"
	}
	if config.Cleanup.ArchiveLocation == "" {
		config.Cleanup.ArchiveLocation = "agent-archives"
	}

	return &config, nil
}

// GetServicePort returns the host port for a service given an agent ID
func (c *Config) GetServicePort(serviceName string, agentID int) int {
	service, ok := c.Docker.Services[serviceName]
	if !ok || len(service.Ports) == 0 {
		return 0
	}

	// Return the first port + agent ID
	return service.Ports[0].HostBase + agentID
}

// GetAllPorts returns a map of service names to allocated host ports for an agent
func (c *Config) GetAllPorts(agentID int) map[string]int {
	ports := make(map[string]int)
	for serviceName, service := range c.Docker.Services {
		if len(service.Ports) > 0 {
			ports[serviceName] = service.Ports[0].HostBase + agentID
		}
	}
	return ports
}
