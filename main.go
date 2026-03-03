package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var Version = "dev"

func main() {
	var (
		urlFlag         string
		tokenFlag       string
		usernameFlag    string
		targetUser      string
		teamFlag        string
		channelFlag     string
		keepAllChannels bool
		restrictToTeam  bool
		dryRun          bool
		workers         int
		format          string
		outputPath      string
		verbose         bool
	)

	rootCmd := &cobra.Command{
		Use:   "mm-guest-convert",
		Short: "Convert a Mattermost member to a guest and restrict channel access",
		Long: `mm-guest-convert converts a regular Mattermost member to a guest account and
restricts their channel access to a single specified channel. It automates the
full workflow — demote, ensure target channel membership, remove all other
channel memberships — in a single idempotent command.

IMPORTANT: --team and --channel require the internal name, NOT the display name.

  These are different in Mattermost:
    Display name:  "Engineering Team"      ← what you see in the UI
    Internal name: "engineering-team"      ← what this tool requires

  To find the internal team name:
    In the Mattermost client, the team name appears in the URL after your server address:
    https://mattermost.example.com/engineering-team/channels/town-square
                                   ^^^^^^^^^^^^^^^^

  To find the internal channel name:
    The channel name is the final segment of the channel URL:
    https://mattermost.example.com/engineering-team/channels/project-alpha
                                                             ^^^^^^^^^^^^^
    Or: System Console → User Management → Channels → select channel → Channel URL field.`,
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate required flags
			if targetUser == "" {
				return fmt.Errorf("--username-target (-u) is required")
			}
			if restrictToTeam && keepAllChannels {
				return fmt.Errorf("--restrict-to-team cannot be used with --keep-all-channels")
			}
			if keepAllChannels {
				if teamFlag != "" || channelFlag != "" {
					return fmt.Errorf("--keep-all-channels cannot be used with --team or --channel")
				}
			} else {
				if teamFlag == "" {
					return fmt.Errorf("--team (-t) is required (unless --keep-all-channels is used)")
				}
				if channelFlag == "" {
					return fmt.Errorf("--channel (-c) is required (unless --keep-all-channels is used)")
				}
			}

			// Validate format
			format = strings.ToLower(format)
			if format != "table" && format != "csv" && format != "json" {
				return fmt.Errorf("--format must be one of: table, csv, json")
			}

			// Validate workers
			if workers < 1 {
				return fmt.Errorf("--workers must be at least 1")
			}

			// Resolve authentication
			auth, err := resolveAuth(urlFlag, tokenFlag, usernameFlag)
			if err != nil {
				exitErr, ok := err.(*ExitError)
				if ok {
					fmt.Fprintln(os.Stderr, exitErr.Message)
					os.Exit(exitErr.Code)
				}
				fmt.Fprintln(os.Stderr, err)
				os.Exit(ExitConfigError)
			}

			// Create API client
			client, err := newClient(auth)
			if err != nil {
				exitErr, ok := err.(*ExitError)
				if ok {
					fmt.Fprintln(os.Stderr, exitErr.Message)
					os.Exit(exitErr.Code)
				}
				fmt.Fprintln(os.Stderr, err)
				os.Exit(ExitAPIError)
			}

			// Set up progress reporting
			var progressFn func(string)
			if verbose {
				// In verbose mode, progress is handled by verbose output
				progressFn = nil
			} else {
				// Progress indicator on stderr
				progressCh := make(chan string, 100)
				done := make(chan struct{})
				go func() {
					for msg := range progressCh {
						fmt.Fprintf(os.Stderr, "\r%-70s", msg)
					}
					// Clear progress line
					fmt.Fprintf(os.Stderr, "\r%-70s\r", "")
					close(done)
				}()
				progressFn = func(msg string) {
					progressCh <- msg
				}
				defer func() {
					close(progressCh)
					<-done
				}()
			}

			// Run the conversion
			cfg := ConvertConfig{
				TargetUsername:  targetUser,
				TeamName:        teamFlag,
				ChannelName:     channelFlag,
				KeepAllChannels: keepAllChannels,
				RestrictToTeam:  restrictToTeam,
				DryRun:          dryRun,
				Workers:         workers,
				Verbose:         verbose,
				ProgressFn:      progressFn,
			}

			result, err := RunConversion(client, cfg)

			// Output result even if there was a partial failure
			if result != nil {
				exitCode := WriteOutput(result, format, outputPath, Version)
				if exitCode != ExitSuccess {
					os.Exit(exitCode)
				}
			}

			if err != nil {
				exitErr, ok := err.(*ExitError)
				if ok {
					if exitErr.Code != ExitPartialFailure {
						fmt.Fprintln(os.Stderr, exitErr.Message)
					}
					os.Exit(exitErr.Code)
				}
				fmt.Fprintln(os.Stderr, err)
				os.Exit(ExitAPIError)
			}

			return nil
		},
	}

	// Connection and auth flags
	rootCmd.Flags().StringVar(&urlFlag, "url", "", "Mattermost server URL (env: MM_URL)")
	rootCmd.Flags().StringVar(&tokenFlag, "token", "", "Personal Access Token (env: MM_TOKEN)")
	rootCmd.Flags().StringVar(&usernameFlag, "username", "", "Username for password auth (env: MM_USERNAME)")

	// Required operation flags
	rootCmd.Flags().StringVarP(&targetUser, "username-target", "u", "", "Mattermost username of the user to convert (required)")
	rootCmd.Flags().StringVarP(&teamFlag, "team", "t", "", "Internal team name containing the target channel (required)")
	rootCmd.Flags().StringVarP(&channelFlag, "channel", "c", "", "Internal channel name the user should retain (required)")

	// Optional flags
	rootCmd.Flags().BoolVar(&keepAllChannels, "keep-all-channels", false, "Only demote the user to guest; do not remove any channel memberships")
	rootCmd.Flags().BoolVar(&restrictToTeam, "restrict-to-team", false, "Remove user from all teams except the target team, then clean up channels")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview all actions without making any changes")
	rootCmd.Flags().IntVar(&workers, "workers", 10, "Number of concurrent workers for channel removal")
	rootCmd.Flags().StringVar(&format, "format", "table", "Output format: table, csv, json")
	rootCmd.Flags().StringVar(&outputPath, "output", "", "Write output to this file path")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging to stderr")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(ExitConfigError)
	}
}
