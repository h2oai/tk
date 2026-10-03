package cmd

import (
	"fmt"
	"strings"

	"github.com/h2oai/tk/internal/ticket"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status <id> <status>",
	Short: "Update ticket status",
	Long:  fmt.Sprintf("Update the status of a ticket.\nValid statuses: %s", strings.Join(statusNames(), ", ")),
	Args:  cobra.ExactArgs(2),
	RunE:  runStatus,
}

var startCmd = &cobra.Command{
	Use:   "start <id>",
	Short: "Set ticket status to in_progress",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return setStatus(args[0], ticket.StatusInProgress)
	},
}

var closeCmd = &cobra.Command{
	Use:   "close <id>",
	Short: "Set ticket status to closed",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return setStatus(args[0], ticket.StatusClosed)
	},
}

var reopenCmd = &cobra.Command{
	Use:   "reopen <id>",
	Short: "Set ticket status to open",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return setStatus(args[0], ticket.StatusOpen)
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(closeCmd)
	rootCmd.AddCommand(reopenCmd)
}

func statusNames() []string {
	names := make([]string, len(ticket.ValidStatuses))
	for i, s := range ticket.ValidStatuses {
		names[i] = string(s)
	}
	return names
}

func runStatus(cmd *cobra.Command, args []string) error {
	id := args[0]
	status := ticket.Status(args[1])

	if !status.IsValid() {
		return fmt.Errorf("invalid status '%s'. Must be one of: %s", status, strings.Join(statusNames(), ", "))
	}

	return setStatus(id, status)
}

func setStatus(partial string, status ticket.Status) error {
	id, err := store.UpdateField(partial, "status", string(status))
	if err != nil {
		return err
	}

	fmt.Printf("Updated %s -> %s\n", id, status)
	return nil
}
