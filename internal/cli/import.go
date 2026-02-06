package cli

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/octasoft-ltd/gmail-exporter/internal/importer"
	"github.com/octasoft-ltd/gmail-exporter/internal/metrics"
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import exported emails into a Gmail account",
	Long: `Import previously exported emails into a Gmail account.
This command takes exported emails and adds them to the authenticated user's mailbox
without sending them as new emails. The emails will appear as if they were received normally.

AUTHENTICATION:
The import command uses separate credentials from export to allow importing into a different
Gmail account. Use --import-credentials and --import-token to specify different authentication
files for the destination account.

	LABELS:
Use --labels to apply Gmail labels to imported emails. Labels are specified as a
comma-separated list of label names (e.g., --labels "tickets,music"). The tool will
automatically resolve label names to their corresponding Gmail label IDs.
When --labels is used, it replaces any labels extracted from the email headers.

Use --add-label to add a single label to all imported emails, in addition to any
labels from the email headers or --labels flag. This is useful for identifying
imported emails (e.g., --add-label="import-2026-0205").

IMPORTANT: The label specified in --add-label must already exist in your Gmail account
before running the import. You must create the label manually in Gmail's web interface
or via the Gmail API first. If the label doesn't exist, the import will still succeed
but the label won't be applied (a warning will be logged).

DEDUPLICATION:
Use --skip-duplicates to avoid importing emails that already exist in Gmail. This checks
for existing messages by Message-ID before importing. Enabling this will skip duplicates
and log the count of skipped messages.

LABEL EXCLUSION:
Use --no-inbox to prevent the "Inbox" label from being applied to imported
emails. This is useful when you want imported emails to inherit their original
labels without automatically adding them to the Inbox folder. When enabled, emails with
"Inbox" in their X-Gmail-Labels header will have that label excluded during import.

Use --no-important to prevent the "Important" label from being applied.
Use --no-starred to prevent the "Starred" label from being applied.

Use --skip-categories to prevent Gmail category labels from being applied. Gmail
categories (Personal, Social, Promotions, Updates, Forums, Purchases) are
system-managed labels that Gmail assigns automatically based on email content. These
cannot be set via API during import and will be ignored by default. Use this flag
to suppress the warning messages.

Use --limit to process only a specific number of messages, which is useful for testing
the import process with a small number of messages before running a full import.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Build import configuration from flags
		importConfig, err := buildImportConfig(cmd)
		if err != nil {
			return fmt.Errorf("failed to build import config: %w", err)
		}

		// Create importer
		imp, err := importer.New(importConfig)
		if err != nil {
			return fmt.Errorf("failed to create importer: %w", err)
		}

		// Run import
		logrus.WithFields(logrus.Fields{
			"input_dir":        importConfig.InputDir,
			"credentials_file": importConfig.CredentialsFile,
			"limit":            importConfig.Limit,
		}).Info("Starting email import")

		result, err := imp.Import()
		if err != nil {
			return fmt.Errorf("import failed: %w", err)
		}

		// Display results
		fmt.Printf("Import completed successfully!\n")
		fmt.Printf("Total files found: %d\n", result.TotalFound)
		fmt.Printf("Total emails imported: %d\n", result.TotalImported)
		if result.TotalSkipped > 0 {
			fmt.Printf("Total emails skipped (duplicates): %d\n", result.TotalSkipped)
		}
		fmt.Printf("Total size: %s\n", metrics.FormatBytes(result.TotalSize))
		fmt.Printf("Duration: %s\n", result.Duration)

		if result.TotalFailed > 0 {
			fmt.Printf("Failed imports: %d (see log for details)\n", result.TotalFailed)
		}

		return nil
	},
}

func init() {
	importCmd.Flags().StringP("input-dir", "i", "", "Input directory containing exported emails")
	importCmd.Flags().String("import-credentials", "", "Gmail API credentials file for destination account (defaults to main credentials)")
	importCmd.Flags().String("import-token", "", "OAuth token file for destination account (defaults to main token)")
	importCmd.Flags().Int("parallel-workers", 3, "Number of parallel workers")
	importCmd.Flags().Bool("preserve-dates", true, "Preserve original email dates")
	importCmd.Flags().IntP("limit", "l", 0, "Limit number of messages to process (0 = no limit, useful for testing)")
	importCmd.Flags().String("labels", "", "Gmail labels to apply to imported emails (comma-separated, replaces email labels)")
	importCmd.Flags().String("add-label", "", "Gmail label to add to all imported emails (label must exist in Gmail first)")
	importCmd.Flags().Bool("skip-duplicates", false, "Skip emails that already exist in Gmail (based on Message-ID)")
	importCmd.Flags().Bool("no-inbox", false, "Exclude Inbox label from imported emails (based on X-Gmail-Labels)")
	importCmd.Flags().Bool("no-important", false, "Exclude Important label from imported emails (based on X-Gmail-Labels)")
	importCmd.Flags().Bool("no-starred", false, "Exclude Starred label from imported emails (based on X-Gmail-Labels)")
	importCmd.Flags().Bool("skip-categories", false, "Skip Gmail category labels (Personal, Social, Promotions, Updates, Forums, Purchases)")
}

func buildImportConfig(cmd *cobra.Command) (*importer.Config, error) {
	// Start with default credentials (same as export)
	credentialsFile := viper.GetString("credentials_file")
	tokenFile := viper.GetString("token_file")

	// Override with import-specific credentials if provided
	if importCreds, _ := cmd.Flags().GetString("import-credentials"); importCreds != "" {
		credentialsFile = importCreds
	}
	if importToken, _ := cmd.Flags().GetString("import-token"); importToken != "" {
		tokenFile = importToken
	}

	config := &importer.Config{
		CredentialsFile: credentialsFile,
		TokenFile:       tokenFile,
	}

	// Get flags
	if inputDir, _ := cmd.Flags().GetString("input-dir"); inputDir != "" {
		config.InputDir = inputDir
	}
	if parallelWorkers, _ := cmd.Flags().GetInt("parallel-workers"); parallelWorkers > 0 {
		config.ParallelWorkers = parallelWorkers
	}
	if preserveDates, _ := cmd.Flags().GetBool("preserve-dates"); preserveDates {
		config.PreserveDates = preserveDates
	}
	if limit, _ := cmd.Flags().GetInt("limit"); limit > 0 {
		config.Limit = limit
	}
	if labels, _ := cmd.Flags().GetString("labels"); labels != "" {
		config.Labels = labels
	}
	if addLabel, _ := cmd.Flags().GetString("add-label"); addLabel != "" {
		config.AddLabel = addLabel
	}
	if skipDuplicates, _ := cmd.Flags().GetBool("skip-duplicates"); skipDuplicates {
		config.SkipDuplicates = skipDuplicates
	}
	if skipInbox, _ := cmd.Flags().GetBool("no-inbox"); skipInbox {
		config.SkipInboxLabel = skipInbox
	}
	if skipImportant, _ := cmd.Flags().GetBool("no-important"); skipImportant {
		config.SkipImportantLabel = skipImportant
	}
	if skipStarred, _ := cmd.Flags().GetBool("no-starred"); skipStarred {
		config.SkipStarredLabel = skipStarred
	}
	if skipCategories, _ := cmd.Flags().GetBool("skip-categories"); skipCategories {
		config.SkipCategories = skipCategories
	}

	// Validate required fields
	if config.InputDir == "" {
		return nil, fmt.Errorf("input directory is required")
	}

	return config, nil
}
