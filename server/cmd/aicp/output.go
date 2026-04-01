package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
)

// printJSON outputs data as JSON.
func printJSON(data any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// printKV outputs key-value pairs.
func printKV(pairs [][2]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, kv := range pairs {
		_, _ = fmt.Fprintf(w, "%s:\t%s\n", kv[0], kv[1])
	}
	_ = w.Flush()
}

// printOutput routes to the appropriate output format.
func printOutput(data any, tableFunc func()) error {
	switch output {
	case "json":
		return printJSON(data)
	default:
		tableFunc()
		return nil
	}
}
