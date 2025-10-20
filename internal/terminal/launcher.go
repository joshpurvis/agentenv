package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Terminal represents a detected terminal emulator
type Terminal struct {
	Name       string
	Executable string
	Available  bool
}

// DetectTerminal identifies the current terminal emulator
// Priority order: configured > alacritty > gnome-terminal > tmux > fallback
func DetectTerminal() *Terminal {
	// Check for alacritty
	if isCommandAvailable("alacritty") {
		return &Terminal{
			Name:       "alacritty",
			Executable: "alacritty",
			Available:  true,
		}
	}

	// Check for gnome-terminal
	if isCommandAvailable("gnome-terminal") {
		return &Terminal{
			Name:       "gnome-terminal",
			Executable: "gnome-terminal",
			Available:  true,
		}
	}

	// Check if running in tmux
	if os.Getenv("TMUX") != "" {
		return &Terminal{
			Name:       "tmux",
			Executable: "tmux",
			Available:  true,
		}
	}

	// Check for konsole (KDE)
	if isCommandAvailable("konsole") {
		return &Terminal{
			Name:       "konsole",
			Executable: "konsole",
			Available:  true,
		}
	}

	// Check for xterm (fallback)
	if isCommandAvailable("xterm") {
		return &Terminal{
			Name:       "xterm",
			Executable: "xterm",
			Available:  true,
		}
	}

	// No terminal found
	return &Terminal{
		Name:      "unknown",
		Available: false,
	}
}

// LaunchInTerminal opens a new terminal window and executes the given command
// Returns an error if the terminal could not be launched
func LaunchInTerminal(command string, workDir string, title string) error {
	terminal := DetectTerminal()

	if !terminal.Available {
		return printManualInstructions(command, workDir)
	}

	var cmd *exec.Cmd

	switch terminal.Name {
	case "alacritty":
		// alacritty --title <title> --working-directory <path> -e <command>
		cmd = exec.Command("alacritty", "--title", title, "--working-directory", workDir, "-e", "sh", "-c", command)

	case "gnome-terminal":
		// gnome-terminal --title=<title> --working-directory=<path> -- <command>
		cmd = exec.Command("gnome-terminal", "--title", title, fmt.Sprintf("--working-directory=%s", workDir), "--", "sh", "-c", command)

	case "tmux":
		// tmux new-window -n <title> -c <path> <command>
		cmd = exec.Command("tmux", "new-window", "-n", title, "-c", workDir, "sh", "-c", command)

	case "konsole":
		// konsole --title <title> --workdir <path> -e <command>
		cmd = exec.Command("konsole", "--title", title, "--workdir", workDir, "-e", "sh", "-c", command)

	case "xterm":
		// xterm -title <title> -e "cd <path> && <command>"
		fullCommand := fmt.Sprintf("cd %s && %s", workDir, command)
		cmd = exec.Command("xterm", "-title", title, "-e", "sh", "-c", fullCommand)

	default:
		return printManualInstructions(command, workDir)
	}

	// Start the terminal in the background
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch %s: %w", terminal.Name, err)
	}

	// Don't wait for the terminal to exit - it should run independently
	go func() {
		_ = cmd.Wait()
	}()

	fmt.Printf("✓ Launched %s in new %s window\n", command, terminal.Name)
	return nil
}

// isCommandAvailable checks if a command exists in PATH
func isCommandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// printManualInstructions prints instructions for manually launching the agent
func printManualInstructions(command string, workDir string) error {
	fmt.Println("\n⚠️  Could not detect a supported terminal emulator.")
	fmt.Println("\nTo launch the agent manually, run these commands:")
	fmt.Printf("\n  cd %s\n", workDir)
	fmt.Printf("  %s\n\n", command)
	fmt.Println("Supported terminals: alacritty, gnome-terminal, konsole, tmux, xterm")
	fmt.Println()

	return nil
}

// GetTerminalInfo returns information about the detected terminal
func GetTerminalInfo() string {
	terminal := DetectTerminal()
	if !terminal.Available {
		return "No supported terminal detected"
	}
	return fmt.Sprintf("%s (executable: %s)", terminal.Name, terminal.Executable)
}

// ValidateTerminal checks if a specific terminal is available
func ValidateTerminal(name string) error {
	name = strings.ToLower(name)

	if !isCommandAvailable(name) {
		return fmt.Errorf("terminal '%s' is not available in PATH", name)
	}

	// Verify it's a supported terminal
	supported := []string{"alacritty", "gnome-terminal", "tmux", "konsole", "xterm"}
	for _, t := range supported {
		if name == t {
			return nil
		}
	}

	return fmt.Errorf("terminal '%s' is not supported (supported: %s)", name, strings.Join(supported, ", "))
}
