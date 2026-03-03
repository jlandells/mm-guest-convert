package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

const maxTableRemovals = 10

// WriteOutput renders the result in the specified format and writes to the given destination.
// If outputPath is non-empty, it writes to that file (falling back to stdout on error).
func WriteOutput(result *ConvertResult, format, outputPath, ver string) int {
	var w io.Writer = os.Stdout

	if outputPath != "" {
		f, err := os.Create(outputPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: unable to write to %s (%v), writing to stdout instead\n", outputPath, err)
			return writeFormatted(os.Stdout, result, format, ver)
		}
		defer f.Close()
		w = f
	}

	return writeFormatted(w, result, format, ver)
}

func writeFormatted(w io.Writer, result *ConvertResult, format, ver string) int {
	switch strings.ToLower(format) {
	case "csv":
		return writeCSV(w, result)
	case "json":
		return writeJSON(w, result)
	default:
		return writeTable(w, result, ver)
	}
}

// writeTable renders the human-readable table format.
func writeTable(w io.Writer, result *ConvertResult, ver string) int {
	if result.DryRun {
		fmt.Fprintln(w, "⚠  DRY RUN — no changes have been made to your Mattermost instance.")
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "mm-guest-convert %s\n\n", ver)

	fmt.Fprintf(w, "Target user:    %s (%s)\n", result.User.Username, result.User.DisplayName)
	fmt.Fprintf(w, "Current role:   %s\n", result.User.CurrentRole)

	if result.KeepAllChannels {
		fmt.Fprintln(w)

		// Step 1
		fmt.Fprintf(w, "Step 1/2  Resolved user %-40s [OK]\n", result.User.Username)

		// Step 2
		if result.WasAlreadyGuest {
			fmt.Fprintf(w, "Step 2/2  User is already a guest — skipped %22s [OK]\n", "")
		} else if result.DryRun {
			fmt.Fprintf(w, "Step 2/2  Would demote %s to guest %21s [DRY RUN]\n", result.User.Username, "")
		} else {
			fmt.Fprintf(w, "Step 2/2  Demoted %s to guest %-23s [OK]\n", result.User.Username, "")
		}

		// Summary
		fmt.Fprintln(w)
		demotedLabel := "1 user demoted"
		if result.WasAlreadyGuest {
			demotedLabel = "already a guest"
		} else if result.DryRun {
			demotedLabel = "1 user would be demoted"
		}
		fmt.Fprintf(w, "Summary: %s · channel memberships unchanged\n", demotedLabel)

		return ExitSuccess
	}

	fmt.Fprintf(w, "Target team:    %s\n", result.TargetTeam)
	fmt.Fprintf(w, "Target channel: %s\n\n", result.TargetChannel)

	// Step 1
	fmt.Fprintf(w, "Step 1/6  Resolved user %-40s [OK]\n", result.User.Username)

	// Step 2
	if result.WasAlreadyGuest {
		fmt.Fprintf(w, "Step 2/6  User is already a guest — skipped %22s [OK]\n", "")
	} else if result.DryRun {
		fmt.Fprintf(w, "Step 2/6  Would demote %s to guest %21s [DRY RUN]\n", result.User.Username, "")
	} else {
		fmt.Fprintf(w, "Step 2/6  Demoted %s to guest %-23s [OK]\n", result.User.Username, "")
	}

	// Step 3
	fmt.Fprintf(w, "Step 3/6  Resolved team %-40s [OK]\n", result.TargetTeam)
	fmt.Fprintf(w, "          Resolved channel %-37s [OK]\n", result.TargetChannel)

	// Step 4
	if result.WasAlreadyMember {
		fmt.Fprintf(w, "Step 4/6  Already a member of #%-33s [OK]\n", result.TargetChannel)
	} else if result.DryRun {
		fmt.Fprintf(w, "Step 4/6  Would add %s to #%-26s [DRY RUN]\n", result.User.Username, result.TargetChannel)
	} else {
		fmt.Fprintf(w, "Step 4/6  Added %s to #%-30s [OK]\n", result.User.Username, result.TargetChannel)
	}

	// Step 5
	fmt.Fprintf(w, "Step 5/6  Found %d channel memberships %-25s [OK]\n", result.TotalMemberships, "")

	// Step 6
	removedActions := filterActions(result.Actions, "removed")
	if len(removedActions) == 0 {
		fmt.Fprintf(w, "Step 6/6  No channels to remove %-30s [OK]\n", "")
	} else {
		actionLabel := "Removing from"
		if result.DryRun {
			actionLabel = "Would remove from"
		}
		fmt.Fprintf(w, "Step 6/6  %s %d channels...\n", actionLabel, len(removedActions))

		shown := 0
		for _, a := range removedActions {
			if shown >= maxTableRemovals {
				remaining := len(removedActions) - maxTableRemovals
				fmt.Fprintf(w, "          ... (%d more)\n", remaining)
				break
			}
			status := "[OK]"
			if a.Status == "error" {
				status = "[ERROR]"
			}
			teamLabel := ""
			if a.TeamName != "" {
				teamLabel = fmt.Sprintf(" (%s)", a.TeamName)
			}
			label := "Removed from"
			if result.DryRun {
				label = "Would remove from"
			}
			fmt.Fprintf(w, "          %s #%s%s %s\n", label, a.ChannelName, teamLabel, status)
			shown++
		}
	}

	// Summary
	fmt.Fprintln(w)
	demotedLabel := "1 user demoted"
	if result.WasAlreadyGuest {
		demotedLabel = "already a guest"
	} else if result.DryRun {
		demotedLabel = "1 user would be demoted"
	}
	fmt.Fprintf(w, "Summary: %s · %d channel retained · %d channels removed · %d errors\n",
		demotedLabel, result.ChannelsRetained, result.ChannelsRemoved, result.RemovalErrors)

	return ExitSuccess
}

// writeCSV renders the CSV format.
func writeCSV(w io.Writer, result *ConvertResult) int {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	header := []string{"username", "team_name", "channel_name", "action", "status"}
	if err := writer.Write(header); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to write CSV header: %v\n", err)
		return ExitOutputError
	}

	for _, a := range result.Actions {
		row := []string{
			result.User.Username,
			a.TeamName,
			a.ChannelName,
			a.Action,
			a.Status,
		}
		if err := writer.Write(row); err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to write CSV row: %v\n", err)
			return ExitOutputError
		}
	}

	return ExitSuccess
}

// jsonOutput is the top-level JSON structure.
type jsonOutput struct {
	DryRun          bool              `json:"dry_run"`
	KeepAllChannels bool              `json:"keep_all_channels"`
	User            jsonUser          `json:"user"`
	TargetTeam      string            `json:"target_team"`
	TargetChannel   string            `json:"target_channel"`
	ChannelsRemoved []jsonChannelInfo `json:"channels_removed"`
	Summary         jsonSummary       `json:"summary"`
	Errors          []jsonChannelErr  `json:"errors"`
}

type jsonUser struct {
	Username        string `json:"username"`
	DisplayName     string `json:"display_name"`
	WasAlreadyGuest bool   `json:"was_already_guest"`
}

type jsonChannelInfo struct {
	TeamName    string `json:"team_name"`
	ChannelName string `json:"channel_name"`
}

type jsonSummary struct {
	TotalMembershipsFound int `json:"total_memberships_found"`
	ChannelsRemoved       int `json:"channels_removed"`
	ChannelsRetained      int `json:"channels_retained"`
	Errors                int `json:"errors"`
}

type jsonChannelErr struct {
	TeamName    string `json:"team_name"`
	ChannelName string `json:"channel_name"`
	Error       string `json:"error"`
}

// writeJSON renders the JSON format.
func writeJSON(w io.Writer, result *ConvertResult) int {
	out := jsonOutput{
		DryRun:          result.DryRun,
		KeepAllChannels: result.KeepAllChannels,
		User: jsonUser{
			Username:        result.User.Username,
			DisplayName:     result.User.DisplayName,
			WasAlreadyGuest: result.WasAlreadyGuest,
		},
		TargetTeam:    result.TargetTeam,
		TargetChannel: result.TargetChannel,
		Summary: jsonSummary{
			TotalMembershipsFound: result.TotalMemberships,
			ChannelsRemoved:       result.ChannelsRemoved,
			ChannelsRetained:      result.ChannelsRetained,
			Errors:                result.RemovalErrors,
		},
	}

	out.ChannelsRemoved = make([]jsonChannelInfo, 0)
	for _, a := range result.Actions {
		if a.Action == "removed" && a.Status == "ok" {
			out.ChannelsRemoved = append(out.ChannelsRemoved, jsonChannelInfo{
				TeamName:    a.TeamName,
				ChannelName: a.ChannelName,
			})
		}
	}

	out.Errors = make([]jsonChannelErr, 0)
	for _, e := range result.Errors {
		out.Errors = append(out.Errors, jsonChannelErr{
			TeamName:    e.TeamName,
			ChannelName: e.ChannelName,
			Error:       e.Error,
		})
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to marshal JSON: %v\n", err)
		return ExitOutputError
	}
	fmt.Fprintln(w, string(data))
	return ExitSuccess
}

func filterActions(actions []ChannelAction, action string) []ChannelAction {
	var result []ChannelAction
	for _, a := range actions {
		if a.Action == action {
			result = append(result, a)
		}
	}
	return result
}
