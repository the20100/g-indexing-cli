package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/the20100/g-indexing-cli/internal/output"
	indexing "google.golang.org/api/indexing/v3"
)

var notifyCmd = &cobra.Command{
	Use:   "notify",
	Short: "Notify Google about URL updates or deletions",
}

// ── notify update ─────────────────────────────────────────────────────────────

var notifyUpdateCmd = &cobra.Command{
	Use:   "update <url> [url2 ...]",
	Short: "Notify Google that one or more URLs have been created or updated",
	Long: `Notify Google that the given URLs have been created or updated.

The page must be publicly accessible and crawlable by Googlebot.

Examples:
  g-indexing notify update https://example.com/new-page
  g-indexing notify update https://example.com/p1 https://example.com/p2`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runNotify(cmd, args, "URL_UPDATED")
	},
}

// ── notify delete ─────────────────────────────────────────────────────────────

var notifyDeleteCmd = &cobra.Command{
	Use:   "delete <url> [url2 ...]",
	Short: "Notify Google that one or more URLs have been permanently removed",
	Long: `Notify Google that the given URLs have been permanently removed.

Use this when a page no longer exists (not for temporary redirects).

Examples:
  g-indexing notify delete https://example.com/old-page
  g-indexing notify delete https://example.com/p1 https://example.com/p2`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runNotify(cmd, args, "URL_DELETED")
	},
}

// ── notify batch ─────────────────────────────────────────────────────────────

var batchConcurrency int

var notifyBatchCmd = &cobra.Command{
	Use:   "batch <file>",
	Short: "Batch notify Google from a file (use - to read from stdin)",
	Long: `Batch notify Google about multiple URLs from a file.

File format (one entry per line):
  update https://example.com/new-page
  delete https://example.com/old-page
  # Lines starting with # and blank lines are ignored.
  # If a line contains only a URL (no prefix), 'update' is assumed.

Examples:
  g-indexing notify batch urls.txt
  echo "update https://example.com/page" | g-indexing notify batch -`,
	Args: cobra.ExactArgs(1),
	RunE: runNotifyBatch,
}

func init() {
	notifyBatchCmd.Flags().IntVar(&batchConcurrency, "concurrency", 5, "Number of concurrent API requests (1–20)")

	notifyCmd.AddCommand(notifyUpdateCmd, notifyDeleteCmd, notifyBatchCmd)
	rootCmd.AddCommand(notifyCmd)
}

// ─── shared logic ─────────────────────────────────────────────────────────────

type notifyResult struct {
	URL      string `json:"url"`
	Type     string `json:"type"`
	Success  bool   `json:"success"`
	NotifyTime string `json:"notify_time,omitempty"`
	Error    string `json:"error,omitempty"`
}

func runNotify(cmd *cobra.Command, urls []string, notifType string) error {
	results := make([]notifyResult, 0, len(urls))

	for _, u := range urls {
		res := notifyOne(u, notifType)
		results = append(results, res)
	}

	if output.IsJSON(cmd) {
		if len(results) == 1 {
			return output.PrintJSON(results[0], output.IsPretty(cmd))
		}
		return output.PrintJSON(results, output.IsPretty(cmd))
	}

	printNotifyResults(results)
	return nil
}

func notifyOne(url, notifType string) notifyResult {
	notification := &indexing.UrlNotification{
		Url:  url,
		Type: notifType,
	}

	resp, err := svc.UrlNotifications.Publish(notification).Do()
	if err != nil {
		return notifyResult{URL: url, Type: notifType, Success: false, Error: err.Error()}
	}

	notifyTime := ""
	if resp.UrlNotificationMetadata != nil {
		if notifType == "URL_UPDATED" && resp.UrlNotificationMetadata.LatestUpdate != nil {
			notifyTime = resp.UrlNotificationMetadata.LatestUpdate.NotifyTime
		} else if notifType == "URL_DELETED" && resp.UrlNotificationMetadata.LatestRemove != nil {
			notifyTime = resp.UrlNotificationMetadata.LatestRemove.NotifyTime
		}
	}

	return notifyResult{URL: url, Type: notifType, Success: true, NotifyTime: notifyTime}
}

func printNotifyResults(results []notifyResult) {
	headers := []string{"STATUS", "TYPE", "URL", "NOTIFY TIME"}
	rows := make([][]string, 0, len(results))
	for _, r := range results {
		status := "✓ OK"
		if !r.Success {
			status = "✗ ERR"
		}
		notifyTime := r.NotifyTime
		if notifyTime == "" {
			notifyTime = "-"
		}
		errNote := ""
		if r.Error != "" {
			errNote = "  [" + output.Truncate(r.Error, 60) + "]"
		}
		rows = append(rows, []string{status, r.Type, r.URL + errNote, notifyTime})
	}
	output.PrintTable(headers, rows)

	ok, fail := 0, 0
	for _, r := range results {
		if r.Success {
			ok++
		} else {
			fail++
		}
	}
	fmt.Printf("\n%d succeeded, %d failed\n", ok, fail)
}

func runNotifyBatch(cmd *cobra.Command, args []string) error {
	type entry struct {
		url      string
		notifType string
	}

	var entries []entry

	// Open file or stdin.
	var scanner *bufio.Scanner
	if args[0] == "-" {
		scanner = bufio.NewScanner(os.Stdin)
	} else {
		f, err := os.Open(args[0])
		if err != nil {
			return fmt.Errorf("opening file: %w", err)
		}
		defer f.Close()
		scanner = bufio.NewScanner(f)
	}

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		var url, notifType string

		switch len(parts) {
		case 1:
			// Bare URL — default to update.
			url = parts[0]
			notifType = "URL_UPDATED"
		case 2:
			keyword := strings.ToLower(parts[0])
			switch keyword {
			case "update", "url_updated":
				notifType = "URL_UPDATED"
			case "delete", "url_deleted", "remove", "url_removed":
				notifType = "URL_DELETED"
			default:
				fmt.Fprintf(os.Stderr, "line %d: unknown type %q (expected update|delete), skipping\n", lineNum, parts[0])
				continue
			}
			url = parts[1]
		default:
			fmt.Fprintf(os.Stderr, "line %d: invalid format (expected \"[update|delete] <url>\"), skipping\n", lineNum)
			continue
		}

		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			fmt.Fprintf(os.Stderr, "line %d: %q doesn't look like a URL, skipping\n", lineNum, url)
			continue
		}

		entries = append(entries, entry{url: url, notifType: notifType})
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No valid entries found in input.")
		return nil
	}

	fmt.Fprintf(os.Stderr, "Processing %d URLs (concurrency: %d)…\n", len(entries), batchConcurrency)

	// Semaphore-based concurrency.
	type work struct {
		entry  entry
		result notifyResult
	}

	if batchConcurrency < 1 {
		batchConcurrency = 1
	}
	if batchConcurrency > 20 {
		batchConcurrency = 20
	}

	resultsCh := make(chan notifyResult, len(entries))
	sem := make(chan struct{}, batchConcurrency)

	for _, e := range entries {
		e := e
		sem <- struct{}{}
		go func() {
			defer func() { <-sem }()
			res := notifyOne(e.url, e.notifType)
			// Tiny sleep to avoid hitting rate limits (100 req/min).
			time.Sleep(10 * time.Millisecond)
			resultsCh <- res
		}()
	}

	// Collect results in submission order.
	resultsMap := make(map[string]notifyResult, len(entries))
	for range entries {
		r := <-resultsCh
		resultsMap[r.URL+"|"+r.Type] = r
	}

	allResults := make([]notifyResult, 0, len(entries))
	for _, e := range entries {
		key := e.url + "|" + e.notifType
		if r, ok := resultsMap[key]; ok {
			allResults = append(allResults, r)
		}
	}

	if output.IsJSON(cmd) {
		return output.PrintJSON(allResults, output.IsPretty(cmd))
	}

	printNotifyResults(allResults)
	return nil
}

// jsonMarshal is a helper for embedding raw JSON in results.
var _ = json.Marshal
