package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

// printJSON outputs data as JSON.
func printJSON(data any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// printTable outputs rows as an aligned table.
func printTable(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	fmt.Fprintln(w, strings.Repeat("─\t", len(headers)))
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	w.Flush()
}

// printKV outputs key-value pairs.
func printKV(pairs [][2]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, kv := range pairs {
		fmt.Fprintf(w, "%s:\t%s\n", kv[0], kv[1])
	}
	w.Flush()
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
