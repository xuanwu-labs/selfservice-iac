package cmdutil

import (
	"encoding/json"
	"fmt"
	"io"
)

// Print writes data to w in the specified format (table|json|yaml).
// This is the shared output dispatcher — every command uses it for --output.
func Print(w io.Writer, format string, v interface{}) error {
	switch format {
	case "json":
		return printJSON(w, v)
	case "yaml":
		// TODO: add yaml support (gopkg.in/yaml.v3)
		return printJSON(w, v) // fallback to json for now
	case "table", "":
		return printTable(w, v)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func printJSON(w io.Writer, v interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printTable(w io.Writer, v interface{}) error {
	// TODO: implement table formatting (text/tabwriter or tablewriter)
	// For骨架 phase, just JSON-pretty-print as placeholder
	return printJSON(w, v)
}
