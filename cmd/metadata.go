package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/the20100/g-indexing-cli/internal/output"
)

var metadataCmd = &cobra.Command{
	Use:   "metadata",
	Short: "Query notification metadata for URLs",
}

var metadataGetCmd = &cobra.Command{
	Use:   "get <url>",
	Short: "Get the latest notification metadata for a URL",
	Long: `Retrieve the most recent indexing notification metadata for a URL.

Returns the timestamp and type of the latest URL_UPDATED and URL_DELETED
notifications sent to Google for this URL.

NOTE: Only works for URLs that were previously notified via the Indexing API.

Examples:
  g-indexing metadata get https://example.com/page
  g-indexing metadata get https://example.com/page --json`,
	Args: cobra.ExactArgs(1),
	RunE: runMetadataGet,
}

func init() {
	metadataCmd.AddCommand(metadataGetCmd)
	rootCmd.AddCommand(metadataCmd)
}

func runMetadataGet(cmd *cobra.Command, args []string) error {
	url := args[0]

	resp, err := svc.UrlNotifications.GetMetadata().Url(url).Do()
	if err != nil {
		return fmt.Errorf("getting metadata for %q: %w", url, err)
	}

	if output.IsJSON(cmd) {
		return output.PrintJSON(resp, output.IsPretty(cmd))
	}

	fmt.Printf("URL: %s\n\n", resp.Url)

	if resp.LatestUpdate != nil {
		fmt.Println("Latest Update Notification:")
		output.PrintKeyValue([][]string{
			{"  URL:", resp.LatestUpdate.Url},
			{"  Type:", resp.LatestUpdate.Type},
			{"  Notify Time:", resp.LatestUpdate.NotifyTime},
		})
		fmt.Println()
	} else {
		fmt.Println("Latest Update Notification: none")
		fmt.Println()
	}

	if resp.LatestRemove != nil {
		fmt.Println("Latest Delete Notification:")
		output.PrintKeyValue([][]string{
			{"  URL:", resp.LatestRemove.Url},
			{"  Type:", resp.LatestRemove.Type},
			{"  Notify Time:", resp.LatestRemove.NotifyTime},
		})
	} else {
		fmt.Println("Latest Delete Notification: none")
	}

	return nil
}
