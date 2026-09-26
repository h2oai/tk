package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lo5/tk/internal/ticket"
	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "new [title]",
	Short: "Create a new ticket",
	Long: `Create a new ticket with the specified title and options.
Prints the generated ticket ID on success.`,
	RunE: runNew,
}

var (
	newID          string
	newDescription string
	newDesign      string
	newAcceptance  string
	newPriority    int
	newType        string
	newAssignee    string
	newExternalRef string
	newParent      string
)

func init() {
	rootCmd.AddCommand(newCmd)

	newCmd.Flags().StringVar(&newID, "id", "", "Explicit ticket ID (defaults to auto-generated; lowercase letters, digits, '-' and '_' only)")
	newCmd.Flags().StringVarP(&newDescription, "description", "d", "", "Description text")
	newCmd.Flags().StringVar(&newDesign, "design", "", "Design notes")
	newCmd.Flags().StringVar(&newAcceptance, "acceptance", "", "Acceptance criteria")
	newCmd.Flags().IntVarP(&newPriority, "priority", "p", 2, "Priority 0-4, 0=highest")
	newCmd.Flags().StringVarP(&newType, "type", "t", "task", "Type (bug|feature|task|epic|chore)")
	newCmd.Flags().StringVarP(&newAssignee, "assignee", "a", "", "Assignee")
	newCmd.Flags().StringVar(&newExternalRef, "external-ref", "", "External reference (e.g., gh-123)")
	newCmd.Flags().StringVar(&newParent, "parent", "", "Parent ticket ID")
}

func runNew(cmd *cobra.Command, args []string) error {
	title := "Untitled"
	if len(args) > 0 {
		title = strings.Join(args, " ")
	}

	// Default assignee from git config
	assignee := newAssignee
	if assignee == "" {
		out, err := exec.Command("git", "config", "user.name").Output()
		if err == nil {
			assignee = strings.TrimSpace(string(out))
		}
	}

	// Validate type
	issueType := ticket.Type(newType)
	if !issueType.IsValid() {
		return fmt.Errorf("invalid type '%s'. Must be one of: bug, feature, task, epic, chore", newType)
	}

	// Validate priority
	if newPriority < 0 || newPriority > 4 {
		return fmt.Errorf("invalid priority '%d'. Must be 0-4", newPriority)
	}

	// Resolve the ticket ID: explicit if provided, otherwise auto-generated
	// with collision detection.
	var id string
	if newID != "" {
		if err := validateCustomID(newID); err != nil {
			return err
		}
		// Use an exact path check rather than store.Get: partial matching
		// would report a false collision when newID is a substring of an
		// existing ID.
		path := filepath.Join(store.Dir(), newID+".md")
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("ticket ID '%s' already exists", newID)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("checking ticket ID '%s': %w", newID, err)
		}
		id = newID
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}

		maxRetries := 10
		for i := 0; i < maxRetries; i++ {
			id = ticket.GenerateID(cwd)
			_, err := store.Get(id)
			if err != nil {
				// ID doesn't exist, we can use it
				break
			}
			// ID exists, retry (unless it's the last attempt)
			if i == maxRetries-1 {
				return fmt.Errorf("failed to generate unique ticket ID after %d attempts", maxRetries)
			}
		}
	}

	// Build body content
	var bodyParts []string
	if newDescription != "" {
		bodyParts = append(bodyParts, newDescription)
	}
	if newDesign != "" {
		bodyParts = append(bodyParts, "## Design\n\n"+newDesign)
	}
	if newAcceptance != "" {
		bodyParts = append(bodyParts, "## Acceptance Criteria\n\n"+newAcceptance)
	}

	body := ""
	if len(bodyParts) > 0 {
		body = strings.Join(bodyParts, "\n\n")
	}

	t := &ticket.Ticket{
		ID:          id,
		Status:      ticket.StatusOpen,
		Deps:        []string{},
		Links:       []string{},
		Created:     time.Now().UTC(),
		Type:        issueType,
		Priority:    newPriority,
		Assignee:    assignee,
		ExternalRef: newExternalRef,
		Parent:      newParent,
		Title:       title,
		Body:        body,
	}

	if err := store.Create(t); err != nil {
		return fmt.Errorf("creating ticket: %w", err)
	}

	fmt.Println(id)
	return nil
}

// validateCustomID checks that a user-supplied ticket ID is safe to use as a
// filename and consistent with the IDs the generator produces. IDs are used
// unescaped as a path component, so anything outside a conservative allowlist
// (lowercase letters, digits, '-' and '_') is rejected.
func validateCustomID(id string) error {
	if id == "" {
		return fmt.Errorf("ticket ID must not be empty")
	}
	if len(id) > 100 {
		return fmt.Errorf("invalid ticket ID '%s': must be 100 characters or fewer", id)
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return fmt.Errorf("invalid ticket ID '%s': only lowercase letters, digits, '-' and '_' are allowed", id)
	}
	return nil
}
