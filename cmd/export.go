package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joshpurvis/agentenv/internal/config"
	"github.com/joshpurvis/agentenv/internal/database"
	"github.com/spf13/cobra"
)

var (
	exportOutputFile string
)

// exportCmd represents the export command
var exportCmd = &cobra.Command{
	Use:   "export <table> <id>",
	Short: "Export database records with dependencies",
	Long: `Export a database record and all its dependencies to SQL file.

This tool recursively exports a record by following foreign key relationships,
ensuring all dependent data is included. The output SQL can be imported into
agent databases for testing.`,
	Example: `  agentenv export report 123 --output test-report.sql
  agentenv export user 1 --output test-user.sql`,
	Args: cobra.ExactArgs(2),
	Run:  runExport,
}

func init() {
	rootCmd.AddCommand(exportCmd)

	exportCmd.Flags().StringVarP(&exportOutputFile, "output", "o", "", "Output file for SQL export (default: stdout)")
}

func runExport(cmd *cobra.Command, args []string) {
	table := args[0]
	idStr := args[1]

	// Try to parse ID as integer, fall back to string
	var id interface{}
	if idInt, err := strconv.Atoi(idStr); err == nil {
		id = idInt
	} else {
		id = idStr
	}

	// Load configuration to get database URL
	cfg, err := config.LoadConfigFromPath(".agentenv.yml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load .agentenv.yml: %v\n", err)
		fmt.Fprintf(os.Stderr, "\nMake sure you're running this command from a directory with .agentenv.yml\n")
		os.Exit(1)
	}

	if cfg.Database.MainURL == "" {
		fmt.Fprintf(os.Stderr, "Error: database.main_url not configured in .agentenv.yml\n")
		os.Exit(1)
	}

	// Create exporter
	fmt.Printf("Connecting to database...\n")
	exporter, err := database.NewExporter(cfg.Database.MainURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer exporter.Close()

	// Export records
	fmt.Printf("Exporting %s record with id=%v...\n", table, id)
	records, err := exporter.Export(table, id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(records) == 0 {
		fmt.Fprintf(os.Stderr, "Error: no records found\n")
		os.Exit(1)
	}

	fmt.Printf("✓ Found %d record(s) (including dependencies)\n", len(records))

	// Summarize what was exported
	tableCounts := make(map[string]int)
	for _, record := range records {
		tableCounts[record.Table]++
	}

	fmt.Println("\nExport summary:")
	for table, count := range tableCounts {
		fmt.Printf("  - %s: %d record(s)\n", table, count)
	}

	// Generate SQL output
	var writer *os.File
	if exportOutputFile != "" {
		f, err := os.Create(exportOutputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to create output file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		writer = f
		fmt.Printf("\nWriting to %s...\n", exportOutputFile)
	} else {
		writer = os.Stdout
		fmt.Println("\n--- SQL Output ---")
	}

	if err := database.GenerateSQL(records, writer); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to generate SQL: %v\n", err)
		os.Exit(1)
	}

	if exportOutputFile != "" {
		fmt.Printf("✓ Export complete: %s\n", exportOutputFile)
		fmt.Println("\nTo import into an agent database:")
		fmt.Printf("  cd ../project-agentX\n")
		fmt.Printf("  psql <database-url> < %s\n", exportOutputFile)
	}
}
