package cmd

import (
	"fmt"
	"text/tabwriter"
	"os"

	"github.com/joshpurvis/agentenv/internal/registry"
	"github.com/spf13/cobra"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all active agent environments",
	Long: `Display a table of all active agent environments with their details.

Example:
  agentenv list`,
	RunE: runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	// Load registry
	reg, err := registry.LoadRegistry()
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	if len(reg.Agents) == 0 {
		fmt.Println("No active agents found.")
		fmt.Println("\nTo launch a new agent:")
		fmt.Println("  agentenv <agent-name> <branch> <command>")
		return nil
	}

	fmt.Printf("Active Agents (%d):\n\n", len(reg.Agents))

	// Create tabwriter for aligned output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	
	// Print header
	fmt.Fprintln(w, "ID\tBranch\tCommand\tPorts\tPath")
	fmt.Fprintln(w, "──\t──────\t───────\t─────\t────")

	// Print each agent
	for agentID, agent := range reg.Agents {
		// Format ports
		portsStr := formatPorts(agent.Ports)
		
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			agentID,
			agent.Branch,
			agent.AgentCommand,
			portsStr,
			agent.WorktreePath)
	}

	w.Flush()

	fmt.Println("\nTo stop an agent:")
	fmt.Println("  agentenv down <agent-id>")

	return nil
}

func formatPorts(ports map[string]int) string {
	if len(ports) == 0 {
		return "-"
	}

	// Format as serviceName:port
	result := ""
	count := 0
	for serviceName, port := range ports {
		if count > 0 {
			result += ", "
		}
		result += fmt.Sprintf("%s:%d", serviceName, port)
		count++
		
		// Limit to first 3 services for display
		if count >= 3 {
			if len(ports) > 3 {
				result += "..."
			}
			break
		}
	}

	return result
}
