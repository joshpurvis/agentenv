package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"
)

const registryFile = ".agentenv/registry.json"

// Registry represents the agent registry
type Registry struct {
	Project       string            `json:"project"`
	ConfigVersion string            `json:"config_version"`
	NextID        int               `json:"next_id,omitempty"` // Deprecated, kept for backward compat
	Agents        map[string]*Agent `json:"agents"`
}

// Agent represents an active agent instance
type Agent struct {
	Name                  string         `json:"name"`              // Deprecated, same as ID now
	Branch                string         `json:"branch"`
	AgentCommand          string         `json:"agent_command"`
	WorktreePath          string         `json:"worktree_path"`
	Ports                 map[string]int `json:"ports"`
	PortSlot              int            `json:"port_slot"`         // Which port slot (1, 2, 3...)
	CreatedAt             time.Time      `json:"created_at"`
	DockerComposeOverride string         `json:"docker_compose_override"`
	PID                   int            `json:"pid,omitempty"`
}

// LoadRegistry loads the agent registry from the current directory
func LoadRegistry() (*Registry, error) {
	data, err := os.ReadFile(registryFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Create new registry
			return &Registry{
				ConfigVersion: "1.0",
				NextID:        1,
				Agents:        make(map[string]*Agent),
			}, nil
		}
		return nil, fmt.Errorf("failed to read registry file: %w", err)
	}

	var registry Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, fmt.Errorf("failed to parse registry file: %w", err)
	}

	if registry.Agents == nil {
		registry.Agents = make(map[string]*Agent)
	}

	return &registry, nil
}

// Save saves the registry to disk
func (r *Registry) Save() error {
	// Ensure .agentenv directory exists
	if err := os.MkdirAll(".agentenv", 0755); err != nil {
		return fmt.Errorf("failed to create .agentenv directory: %w", err)
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registry: %w", err)
	}

	if err := os.WriteFile(registryFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write registry file: %w", err)
	}

	return nil
}

// FindNextAvailableSlot finds the next available port slot
func (r *Registry) FindNextAvailableSlot() int {
	// Get all used slots
	usedSlots := make(map[int]bool)
	for _, agent := range r.Agents {
		usedSlots[agent.PortSlot] = true
	}

	// Find first available slot starting from 1
	slot := 1
	for {
		if !usedSlots[slot] {
			return slot
		}
		slot++
	}
}

// AllocateAgent creates a new agent with the given ID
// Returns error if agent ID already exists
func (r *Registry) AllocateAgent(agentID, branch, agentCommand, worktreePath string, ports map[string]int, portSlot int) (*Agent, error) {
	// Check if agent already exists
	if _, exists := r.Agents[agentID]; exists {
		return nil, fmt.Errorf("agent '%s' already exists", agentID)
	}

	agent := &Agent{
		Name:                  agentID, // Name = ID now
		Branch:                branch,
		AgentCommand:          agentCommand,
		WorktreePath:          worktreePath,
		Ports:                 ports,
		PortSlot:              portSlot,
		CreatedAt:             time.Now(),
		DockerComposeOverride: fmt.Sprintf("docker-compose.%s.override.yml", agentID),
	}

	r.Agents[agentID] = agent
	return agent, nil
}

// GetAgent returns an agent by ID
func (r *Registry) GetAgent(agentID string) (*Agent, error) {
	agent, ok := r.Agents[agentID]
	if !ok {
		return nil, fmt.Errorf("agent %s not found", agentID)
	}
	return agent, nil
}

// RemoveAgent removes an agent from the registry
func (r *Registry) RemoveAgent(agentID string) error {
	if _, ok := r.Agents[agentID]; !ok {
		return fmt.Errorf("agent %s not found", agentID)
	}
	delete(r.Agents, agentID)
	return nil
}

// GetAgentNumericID returns the port slot for an agent
// Deprecated: Use agent.PortSlot instead
func GetAgentNumericID(agentID string) int {
	// For backward compatibility during migration
	var id int
	fmt.Sscanf(agentID, "agent%d", &id)
	return id
}

// ListAgentsBySlot returns agents sorted by port slot
func (r *Registry) ListAgentsBySlot() []*Agent {
	agents := make([]*Agent, 0, len(r.Agents))
	for _, agent := range r.Agents {
		agents = append(agents, agent)
	}

	sort.Slice(agents, func(i, j int) bool {
		return agents[i].PortSlot < agents[j].PortSlot
	})

	return agents
}
