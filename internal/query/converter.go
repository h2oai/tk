package query

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/itchyny/gojq"
	"github.com/h2oai/tk/internal/ticket"
)

// TicketJSON represents a ticket in JSON format
type TicketJSON struct {
	ID          string   `json:"id"`
	Status      string   `json:"status"`
	Deps        []string `json:"deps"`
	Links       []string `json:"links"`
	Created     string   `json:"created"`
	Type        string   `json:"type"`
	Priority    string   `json:"priority"`
	Assignee    string   `json:"assignee,omitempty"`
	ExternalRef string   `json:"external-ref,omitempty"`
	Parent      string   `json:"parent,omitempty"`
}

// ToJSON converts a ticket to a JSON string
func ToJSON(t *ticket.Ticket) (string, error) {
	tj := TicketJSON{
		ID:          t.ID,
		Status:      string(t.Status),
		Deps:        t.Deps,
		Links:       t.Links,
		Created:     t.Created.UTC().Format("2006-01-02T15:04:05Z"),
		Type:        string(t.Type),
		Priority:    fmt.Sprintf("%d", t.Priority),
		Assignee:    t.Assignee,
		ExternalRef: t.ExternalRef,
		Parent:      t.Parent,
	}

	// Ensure arrays are not nil
	if tj.Deps == nil {
		tj.Deps = []string{}
	}
	if tj.Links == nil {
		tj.Links = []string{}
	}

	data, err := json.Marshal(tj)
	if err != nil {
		return "", fmt.Errorf("marshaling JSON: %w", err)
	}

	return string(data), nil
}

// Filter applies a jq-style filter to JSON tickets
func Filter(jsonLines []string, filterExpr string) ([]string, error) {
	// Wrap in select() if not already
	if !strings.HasPrefix(filterExpr, "select(") && !strings.HasPrefix(filterExpr, ".") {
		filterExpr = "select(" + filterExpr + ")"
	} else if strings.HasPrefix(filterExpr, ".") && !strings.Contains(filterExpr, "|") {
		// If it's just a field access like ".priority == 0", wrap in select
		filterExpr = "select(" + filterExpr + ")"
	}

	query, err := gojq.Parse(filterExpr)
	if err != nil {
		return nil, fmt.Errorf("parsing filter: %w", err)
	}

	var results []string
	for _, line := range jsonLines {
		var input interface{}
		if err := json.Unmarshal([]byte(line), &input); err != nil {
			continue
		}

		iter := query.Run(input)
		for {
			v, ok := iter.Next()
			if !ok {
				break
			}
			if err, ok := v.(error); ok {
				// Filter returned error (e.g., select returned false)
				_ = err
				continue
			}
			// If we got a result, include this line
			if v != nil {
				results = append(results, line)
				break
			}
		}
	}

	return results, nil
}
