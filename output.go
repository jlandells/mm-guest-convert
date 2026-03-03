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

	if result.RestrictToTeam {
		return writeTableRestrictToTeam(w, result)
	}

	return writeTableStandard(w, result)
}

// writeTableStandard renders the 6-step standard conversion table.
func writeTableStandard(w io.Writer, result *ConvertResult) int {
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
		writeChannelRemovalLines(w, removedActions, result.DryRun)
	}

	// Summary
	fmt.Fprintln(w)
	demotedLabel := demotedLabelStr(result)
	fmt.Fprintf(w, "Summary: %s · %d channel retained · %d channels removed · %d errors\n",
		demotedLabel, result.ChannelsRetained, result.ChannelsRemoved, result.RemovalErrors)

	return ExitSuccess
}

// writeTableRestrictToTeam renders the 8-step restrict-to-team table.
func writeTableRestrictToTeam(w io.Writer, result *ConvertResult) int {
	// Step 1
	fmt.Fprintf(w, "Step 1/8  Resolved user %-40s [OK]\n", result.User.Username)

	// Step 2
	if result.WasAlreadyGuest {
		fmt.Fprintf(w, "Step 2/8  User is already a guest — skipped %22s [OK]\n", "")
	} else if result.DryRun {
		fmt.Fprintf(w, "Step 2/8  Would demote %s to guest %21s [DRY RUN]\n", result.User.Username, "")
	} else {
		fmt.Fprintf(w, "Step 2/8  Demoted %s to guest %-23s [OK]\n", result.User.Username, "")
	}

	// Step 3
	fmt.Fprintf(w, "Step 3/8  Resolved target team %-33s [OK]\n", result.TargetTeam)

	// Step 4
	fmt.Fprintf(w, "Step 4/8  Found %d team memberships %-28s [OK]\n", result.TotalTeams, "")

	// Step 5 — team removal
	removedTeams := filterTeamActions(result.TeamActions, "removed")
	if len(removedTeams) == 0 {
		fmt.Fprintf(w, "Step 5/8  No teams to remove %-33s [OK]\n", "")
	} else {
		actionLabel := "Removing from"
		if result.DryRun {
			actionLabel = "Would remove from"
		}
		fmt.Fprintf(w, "Step 5/8  %s %d teams...\n", actionLabel, len(removedTeams))

		shown := 0
		for _, a := range removedTeams {
			if shown >= maxTableRemovals {
				remaining := len(removedTeams) - maxTableRemovals
				fmt.Fprintf(w, "          ... (%d more)\n", remaining)
				break
			}
			status := "[OK]"
			if a.Status == "error" {
				status = "[ERROR]"
			}
			label := "Removed from team:"
			if result.DryRun {
				label = "Would remove from team:"
			}
			fmt.Fprintf(w, "          %s %s %s\n", label, a.TeamName, status)
			shown++
		}
	}

	// Step 6
	fmt.Fprintf(w, "Step 6/8  Resolved channel %-37s [OK]\n", result.TargetChannel)
	if result.WasAlreadyMember {
		fmt.Fprintf(w, "          Already a member of #%-33s [OK]\n", result.TargetChannel)
	} else if result.DryRun {
		fmt.Fprintf(w, "          Would add %s to #%-26s [DRY RUN]\n", result.User.Username, result.TargetChannel)
	} else {
		fmt.Fprintf(w, "          Added %s to #%-30s [OK]\n", result.User.Username, result.TargetChannel)
	}

	// Step 7
	fmt.Fprintf(w, "Step 7/8  Found %d remaining channel memberships %-16s [OK]\n", result.TotalMemberships, "")

	// Step 8 — channel removal
	removedChannels := filterActions(result.Actions, "removed")
	if len(removedChannels) == 0 {
		fmt.Fprintf(w, "Step 8/8  No channels to remove %-30s [OK]\n", "")
	} else {
		actionLabel := "Removing from"
		if result.DryRun {
			actionLabel = "Would remove from"
		}
		fmt.Fprintf(w, "Step 8/8  %s %d channels...\n", actionLabel, len(removedChannels))
		writeChannelRemovalLines(w, removedChannels, result.DryRun)
	}

	// Summary
	fmt.Fprintln(w)
	demotedLabel := demotedLabelStr(result)
	fmt.Fprintf(w, "Summary: %s · %d team retained · %d teams removed · %d channel retained · %d channels removed · %d errors\n",
		demotedLabel, result.TeamsRetained, result.TeamsRemovedCount, result.ChannelsRetained, result.ChannelsRemoved,
		result.RemovalErrors+result.TeamRemovalErrors)

	return ExitSuccess
}

// writeChannelRemovalLines writes the individual channel removal lines for table output.
func writeChannelRemovalLines(w io.Writer, removedActions []ChannelAction, dryRun bool) {
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
		if dryRun {
			label = "Would remove from"
		}
		fmt.Fprintf(w, "          %s #%s%s %s\n", label, a.ChannelName, teamLabel, status)
		shown++
	}
}

// demotedLabelStr returns the appropriate demotion label for summary lines.
func demotedLabelStr(result *ConvertResult) string {
	if result.WasAlreadyGuest {
		return "already a guest"
	}
	if result.DryRun {
		return "1 user would be demoted"
	}
	return "1 user demoted"
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

	// Emit team rows first (only when restrict-to-team is active)
	if result.RestrictToTeam {
		for _, a := range result.TeamActions {
			action := "team-removed"
			if a.Action == "retained" {
				action = "team-retained"
			}
			row := []string{
				result.User.Username,
				a.TeamName,
				"", // empty channel_name for team rows
				action,
				a.Status,
			}
			if err := writer.Write(row); err != nil {
				fmt.Fprintf(os.Stderr, "error: failed to write CSV row: %v\n", err)
				return ExitOutputError
			}
		}
	}

	// Emit channel rows
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
	RestrictToTeam  bool              `json:"restrict_to_team"`
	User            jsonUser          `json:"user"`
	TargetTeam      string            `json:"target_team"`
	TargetChannel   string            `json:"target_channel"`
	ChannelsRemoved []jsonChannelInfo `json:"channels_removed"`
	Summary         jsonSummary       `json:"summary"`
	Errors          []jsonChannelErr  `json:"errors"`
	// Team-level fields (only when restrict_to_team is true)
	TeamsRemoved *[]jsonTeamInfo  `json:"teams_removed,omitempty"`
	TeamSummary  *jsonTeamSummary `json:"team_summary,omitempty"`
	TeamErrors   *[]jsonTeamErr   `json:"team_errors,omitempty"`
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

type jsonTeamInfo struct {
	TeamName string `json:"team_name"`
}

type jsonTeamSummary struct {
	TotalTeams    int `json:"total_teams"`
	TeamsRemoved  int `json:"teams_removed"`
	TeamsRetained int `json:"teams_retained"`
	Errors        int `json:"errors"`
}

type jsonTeamErr struct {
	TeamName string `json:"team_name"`
	Error    string `json:"error"`
}

// writeJSON renders the JSON format.
func writeJSON(w io.Writer, result *ConvertResult) int {
	out := jsonOutput{
		DryRun:          result.DryRun,
		KeepAllChannels: result.KeepAllChannels,
		RestrictToTeam:  result.RestrictToTeam,
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

	// Populate team-level fields when restrict-to-team is active
	if result.RestrictToTeam {
		teamsRemoved := make([]jsonTeamInfo, 0)
		for _, a := range result.TeamActions {
			if a.Action == "removed" && a.Status == "ok" {
				teamsRemoved = append(teamsRemoved, jsonTeamInfo{
					TeamName: a.TeamName,
				})
			}
		}
		out.TeamsRemoved = &teamsRemoved

		teamSummary := jsonTeamSummary{
			TotalTeams:    result.TotalTeams,
			TeamsRemoved:  result.TeamsRemovedCount,
			TeamsRetained: result.TeamsRetained,
			Errors:        result.TeamRemovalErrors,
		}
		out.TeamSummary = &teamSummary

		teamErrors := make([]jsonTeamErr, 0)
		for _, e := range result.TeamErrors {
			teamErrors = append(teamErrors, jsonTeamErr{
				TeamName: e.TeamName,
				Error:    e.Error,
			})
		}
		out.TeamErrors = &teamErrors
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

func filterTeamActions(actions []TeamAction, action string) []TeamAction {
	var result []TeamAction
	for _, a := range actions {
		if a.Action == action {
			result = append(result, a)
		}
	}
	return result
}
