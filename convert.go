package main

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/mattermost/mattermost/server/public/model"
)

// ConvertConfig holds the input parameters for the conversion workflow.
type ConvertConfig struct {
	TargetUsername  string
	TeamName        string
	ChannelName     string
	KeepAllChannels bool
	RestrictToTeam  bool
	DryRun          bool
	Workers         int
	Verbose         bool
	ProgressFn      func(msg string) // optional callback for progress updates
}

// UserInfo holds resolved user details for output.
type UserInfo struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	CurrentRole string `json:"current_role"`
}

// ChannelAction records an action taken on a channel.
type ChannelAction struct {
	TeamName    string `json:"team_name"`
	ChannelName string `json:"channel_name"`
	ChannelID   string `json:"channel_id"`
	Action      string `json:"action"` // "retained" or "removed"
	Status      string `json:"status"` // "ok" or "error"
	Error       string `json:"error,omitempty"`
}

// TeamAction records an action taken on a team membership.
type TeamAction struct {
	TeamName string `json:"team_name"`
	TeamID   string `json:"team_id"`
	Action   string `json:"action"` // "retained" or "removed"
	Status   string `json:"status"` // "ok" or "error"
	Error    string `json:"error,omitempty"`
}

// ConvertResult holds all output from the conversion workflow.
type ConvertResult struct {
	DryRun           bool            `json:"dry_run"`
	KeepAllChannels  bool            `json:"keep_all_channels"`
	RestrictToTeam   bool            `json:"restrict_to_team"`
	User             UserInfo        `json:"user"`
	TargetTeam       string          `json:"target_team"`
	TargetChannel    string          `json:"target_channel"`
	WasAlreadyGuest  bool            `json:"was_already_guest"`
	WasAlreadyMember bool            `json:"was_already_member"`
	Actions          []ChannelAction `json:"actions"`
	Errors           []ChannelAction `json:"errors"`
	TotalMemberships int             `json:"total_memberships"`
	ChannelsRemoved  int             `json:"channels_removed"`
	ChannelsRetained int             `json:"channels_retained"`
	ChannelsSkipped  int             `json:"channels_skipped"` // DM/GM count
	RemovalErrors    int             `json:"removal_errors"`
	// Team-level fields (only populated when RestrictToTeam is true)
	TeamActions       []TeamAction `json:"team_actions,omitempty"`
	TeamErrors        []TeamAction `json:"team_errors,omitempty"`
	TotalTeams        int          `json:"total_teams,omitempty"`
	TeamsRemovedCount int          `json:"teams_removed_count,omitempty"`
	TeamsRetained     int          `json:"teams_retained,omitempty"`
	TeamRemovalErrors int          `json:"team_removal_errors,omitempty"`
}

// channelInfo holds details for a channel membership to process.
type channelInfo struct {
	ChannelID   string
	ChannelName string
	ChannelType model.ChannelType
	TeamName    string
}

// removalResult holds the outcome of a single channel removal.
type removalResult struct {
	Info channelInfo
	Err  error
}

// RunConversion executes the 6-step guest conversion workflow.
func RunConversion(client MattermostClient, cfg ConvertConfig) (*ConvertResult, error) {
	result := &ConvertResult{
		DryRun:        cfg.DryRun,
		TargetTeam:    cfg.TeamName,
		TargetChannel: cfg.ChannelName,
	}

	progress := cfg.ProgressFn
	if progress == nil {
		progress = func(string) {}
	}

	verbose := func(msg string) {
		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "%s\n", msg)
		}
	}

	// Step 1: Resolve user
	progress("Resolving user...")
	user, err := client.GetUserByUsername(cfg.TargetUsername)
	if err != nil {
		return nil, wrapStepError(err, fmt.Sprintf("user %q", cfg.TargetUsername))
	}
	result.User = UserInfo{
		ID:          user.Id,
		Username:    user.Username,
		DisplayName: displayName(user),
		CurrentRole: classifyRole(user.Roles),
	}
	verbose(fmt.Sprintf("  Resolved user %s (%s), role: %s", user.Username, result.User.DisplayName, result.User.CurrentRole))

	// Step 2: Demote to guest
	if result.User.CurrentRole == "guest" {
		result.WasAlreadyGuest = true
		verbose("  User is already a guest — skipping demotion")
	} else {
		// Check if guest accounts are enabled
		progress("Checking guest accounts configuration...")
		config, configErr := client.GetConfig()
		if configErr != nil {
			verbose("  Warning: could not check guest accounts configuration, proceeding anyway")
		} else if config.GuestAccountsSettings.Enable != nil && !*config.GuestAccountsSettings.Enable {
			return nil, configError("error: guest accounts are not enabled on this instance. Enable them at System Console → Authentication → Guest Access.")
		}

		if cfg.DryRun {
			verbose(fmt.Sprintf("  [DRY RUN] Would demote %s to guest", user.Username))
		} else {
			progress("Demoting user to guest...")
			if err := client.DemoteUserToGuest(user.Id); err != nil {
				return nil, wrapStepError(err, "demoting user to guest")
			}
			verbose(fmt.Sprintf("  Demoted %s to guest", user.Username))
		}
	}

	// Early return for --keep-all-channels mode
	if cfg.KeepAllChannels {
		verbose("  --keep-all-channels: skipping channel management")
		result.KeepAllChannels = true
		return result, nil
	}

	// Branch based on mode
	if cfg.RestrictToTeam {
		result.RestrictToTeam = true
		return runRestrictToTeam(client, cfg, result, user, progress, verbose)
	}

	return runStandardConversion(client, cfg, result, user, progress, verbose)
}

// runStandardConversion implements the standard 6-step conversion workflow
// (steps 3-6: resolve team/channel, ensure membership, enumerate channels, remove non-target).
func runStandardConversion(client MattermostClient, cfg ConvertConfig, result *ConvertResult,
	user *model.User, progress func(string), verbose func(string)) (*ConvertResult, error) {

	// Step 3: Resolve team and channel
	progress("Resolving team and channel...")
	team, err := client.GetTeamByName(cfg.TeamName)
	if err != nil {
		return nil, wrapStepError(err, fmt.Sprintf("team %q", cfg.TeamName))
	}
	verbose(fmt.Sprintf("  Resolved team %s", cfg.TeamName))

	channel, err := client.GetChannelByName(cfg.ChannelName, team.Id)
	if err != nil {
		exitErr, ok := err.(*ExitError)
		if ok && exitErr.Code == ExitConfigError {
			return nil, configError(fmt.Sprintf("error: channel %q not found in team %q.", cfg.ChannelName, cfg.TeamName))
		}
		return nil, wrapStepError(err, fmt.Sprintf("channel %q in team %q", cfg.ChannelName, cfg.TeamName))
	}
	verbose(fmt.Sprintf("  Resolved channel %s", cfg.ChannelName))

	// Step 4: Ensure target channel membership
	progress("Checking target channel membership...")
	_, memberErr := client.GetChannelMember(channel.Id, user.Id)
	if memberErr != nil {
		// Not a member — add them
		if cfg.DryRun {
			verbose(fmt.Sprintf("  [DRY RUN] Would add %s to #%s", user.Username, cfg.ChannelName))
			result.WasAlreadyMember = false
		} else {
			progress("Adding user to target channel...")
			_, addErr := client.AddChannelMember(channel.Id, user.Id)
			if addErr != nil {
				return nil, wrapStepError(addErr, fmt.Sprintf("adding user to channel %q", cfg.ChannelName))
			}
			verbose(fmt.Sprintf("  Added %s to #%s", user.Username, cfg.ChannelName))
			result.WasAlreadyMember = false
		}
	} else {
		result.WasAlreadyMember = true
		verbose(fmt.Sprintf("  User is already a member of #%s", cfg.ChannelName))
	}

	// Step 5: Enumerate all channel memberships using GetChannelMembersWithTeamData
	progress("Enumerating channel memberships...")
	var allMembers []model.ChannelMemberWithTeamData
	page := 0
	perPage := 200
	for {
		members, err := client.GetChannelMembersWithTeamData(user.Id, page, perPage)
		if err != nil {
			return nil, apiError(fmt.Sprintf("error: failed to enumerate channel memberships (page %d). Halting to avoid partial processing.", page), err)
		}
		allMembers = append(allMembers, members...)
		if len(members) < perPage {
			break
		}
		page++
	}

	result.TotalMemberships = len(allMembers)
	verbose(fmt.Sprintf("  Found %d channel memberships", len(allMembers)))

	// Fetch channel details (for type and name) — GetChannelMembersWithTeamData
	// provides TeamName but not ChannelName or ChannelType.
	progress("Fetching channel details...")
	channels := make([]channelInfo, 0, len(allMembers))
	for i, m := range allMembers {
		if i%20 == 0 {
			progress(fmt.Sprintf("Fetching channel details (%d/%d)...", i, len(allMembers)))
		}
		ch, err := client.GetChannel(m.ChannelId)
		if err != nil {
			return nil, apiError(fmt.Sprintf("error: failed to fetch details for channel %s during enumeration. Halting to avoid partial processing.", m.ChannelId), err)
		}
		channels = append(channels, channelInfo{
			ChannelID:   ch.Id,
			ChannelName: ch.Name,
			ChannelType: ch.Type,
			TeamName:    m.TeamName, // from GetChannelMembersWithTeamData
		})
	}

	// Separate channels into: target (retain), DM/GM (skip), and others (remove)
	var toRemove []channelInfo
	dmGmCount := 0
	for _, ch := range channels {
		if ch.ChannelType == model.ChannelTypeDirect || ch.ChannelType == model.ChannelTypeGroup {
			dmGmCount++
			continue
		}
		if ch.ChannelID == channel.Id {
			result.Actions = append(result.Actions, ChannelAction{
				TeamName:    cfg.TeamName,
				ChannelName: ch.ChannelName,
				ChannelID:   ch.ChannelID,
				Action:      "retained",
				Status:      "ok",
			})
			continue
		}
		toRemove = append(toRemove, ch)
	}
	result.ChannelsSkipped = dmGmCount
	result.ChannelsRetained = 1 // the target channel

	verbose(fmt.Sprintf("  Channels to remove: %d, DM/GM skipped: %d", len(toRemove), dmGmCount))

	// Step 6: Remove from other channels
	if len(toRemove) == 0 {
		verbose("  No channels to remove — operation complete")
		return result, nil
	}

	if cfg.DryRun {
		for _, ch := range toRemove {
			result.Actions = append(result.Actions, ChannelAction{
				TeamName:    ch.TeamName,
				ChannelName: ch.ChannelName,
				ChannelID:   ch.ChannelID,
				Action:      "removed",
				Status:      "ok",
			})
		}
		result.ChannelsRemoved = len(toRemove)
		verbose(fmt.Sprintf("  [DRY RUN] Would remove from %d channels", len(toRemove)))
		return result, nil
	}

	// Concurrent removal with worker pool
	removeChannelResults := removeChannelsConcurrently(client, user.Id, toRemove, cfg.Workers, progress, verbose)
	for _, r := range removeChannelResults {
		action := ChannelAction{
			TeamName:    r.Info.TeamName,
			ChannelName: r.Info.ChannelName,
			ChannelID:   r.Info.ChannelID,
			Action:      "removed",
			Status:      "ok",
		}
		if r.Err != nil {
			action.Status = "error"
			action.Error = r.Err.Error()
			result.Errors = append(result.Errors, action)
			result.RemovalErrors++
			verbose(fmt.Sprintf("  Error removing from #%s: %v", r.Info.ChannelName, r.Err))
		} else {
			result.ChannelsRemoved++
			verbose(fmt.Sprintf("  Removed from #%s (%s)", r.Info.ChannelName, r.Info.TeamName))
		}
		result.Actions = append(result.Actions, action)
	}

	progress("Complete")

	if result.RemovalErrors > 0 {
		return result, newExitError(ExitPartialFailure,
			fmt.Sprintf("completed with %d removal errors", result.RemovalErrors), nil)
	}

	return result, nil
}

// teamRemovalResult holds the outcome of a single team removal.
type teamRemovalResult struct {
	TeamName string
	TeamID   string
	Err      error
}

// runRestrictToTeam implements the 8-step restrict-to-team workflow.
// It removes the user from all teams except the target team, then cleans up
// remaining channel memberships within the target team.
func runRestrictToTeam(client MattermostClient, cfg ConvertConfig, result *ConvertResult,
	user *model.User, progress func(string), verbose func(string)) (*ConvertResult, error) {

	// Step 3: Resolve target team
	progress("Resolving target team...")
	team, err := client.GetTeamByName(cfg.TeamName)
	if err != nil {
		return nil, wrapStepError(err, fmt.Sprintf("team %q", cfg.TeamName))
	}
	verbose(fmt.Sprintf("  Resolved team %s (ID: %s)", cfg.TeamName, team.Id))

	// Step 4: Enumerate user's teams
	progress("Enumerating team memberships...")
	allTeams, err := client.GetTeamsForUser(user.Id)
	if err != nil {
		return nil, wrapStepError(err, "enumerating teams for user")
	}

	// Filter out archived teams (DeleteAt > 0)
	var activeTeams []*model.Team
	for _, t := range allTeams {
		if t.DeleteAt == 0 {
			activeTeams = append(activeTeams, t)
		}
	}

	result.TotalTeams = len(activeTeams)
	verbose(fmt.Sprintf("  Found %d active teams (%d total including archived)", len(activeTeams), len(allTeams)))

	// Separate into target (retain) and others (remove)
	var teamsToRemove []*model.Team
	for _, t := range activeTeams {
		if t.Id == team.Id {
			result.TeamActions = append(result.TeamActions, TeamAction{
				TeamName: t.Name,
				TeamID:   t.Id,
				Action:   "retained",
				Status:   "ok",
			})
			result.TeamsRetained++
			continue
		}
		teamsToRemove = append(teamsToRemove, t)
	}

	verbose(fmt.Sprintf("  Teams to remove: %d, retained: %d", len(teamsToRemove), result.TeamsRetained))

	// Step 5: Remove from non-target teams
	if len(teamsToRemove) > 0 {
		if cfg.DryRun {
			for _, t := range teamsToRemove {
				result.TeamActions = append(result.TeamActions, TeamAction{
					TeamName: t.Name,
					TeamID:   t.Id,
					Action:   "removed",
					Status:   "ok",
				})
			}
			result.TeamsRemovedCount = len(teamsToRemove)
			verbose(fmt.Sprintf("  [DRY RUN] Would remove from %d teams", len(teamsToRemove)))
		} else {
			// Concurrent team removal with worker pool
			workers := cfg.Workers
			if workers <= 0 {
				workers = 10
			}

			sem := make(chan struct{}, workers)
			teamResultsCh := make(chan teamRemovalResult, len(teamsToRemove))
			var wg sync.WaitGroup

			for _, t := range teamsToRemove {
				progress(fmt.Sprintf("Removing from team: %-50s", t.Name))
				wg.Add(1)
				go func(teamName, teamID string) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()
					err := client.RemoveUserFromTeam(teamID, user.Id)
					teamResultsCh <- teamRemovalResult{TeamName: teamName, TeamID: teamID, Err: err}
				}(t.Name, t.Id)
			}

			go func() {
				wg.Wait()
				close(teamResultsCh)
			}()

			for r := range teamResultsCh {
				action := TeamAction{
					TeamName: r.TeamName,
					TeamID:   r.TeamID,
					Action:   "removed",
					Status:   "ok",
				}
				if r.Err != nil {
					action.Status = "error"
					action.Error = r.Err.Error()
					result.TeamErrors = append(result.TeamErrors, action)
					result.TeamRemovalErrors++
					verbose(fmt.Sprintf("  Error removing from team %s: %v", r.TeamName, r.Err))
				} else {
					result.TeamsRemovedCount++
					verbose(fmt.Sprintf("  Removed from team %s", r.TeamName))
				}
				result.TeamActions = append(result.TeamActions, action)
			}
		}
	}

	// Step 6: Resolve target channel and ensure membership
	progress("Resolving target channel...")
	channel, err := client.GetChannelByName(cfg.ChannelName, team.Id)
	if err != nil {
		exitErr, ok := err.(*ExitError)
		if ok && exitErr.Code == ExitConfigError {
			return nil, configError(fmt.Sprintf("error: channel %q not found in team %q.", cfg.ChannelName, cfg.TeamName))
		}
		return nil, wrapStepError(err, fmt.Sprintf("channel %q in team %q", cfg.ChannelName, cfg.TeamName))
	}
	verbose(fmt.Sprintf("  Resolved channel %s", cfg.ChannelName))

	progress("Checking target channel membership...")
	_, memberErr := client.GetChannelMember(channel.Id, user.Id)
	if memberErr != nil {
		if cfg.DryRun {
			verbose(fmt.Sprintf("  [DRY RUN] Would add %s to #%s", user.Username, cfg.ChannelName))
			result.WasAlreadyMember = false
		} else {
			progress("Adding user to target channel...")
			_, addErr := client.AddChannelMember(channel.Id, user.Id)
			if addErr != nil {
				return nil, wrapStepError(addErr, fmt.Sprintf("adding user to channel %q", cfg.ChannelName))
			}
			verbose(fmt.Sprintf("  Added %s to #%s", user.Username, cfg.ChannelName))
			result.WasAlreadyMember = false
		}
	} else {
		result.WasAlreadyMember = true
		verbose(fmt.Sprintf("  User is already a member of #%s", cfg.ChannelName))
	}

	// Step 7: Enumerate remaining channel memberships
	progress("Enumerating remaining channel memberships...")
	var allMembers []model.ChannelMemberWithTeamData
	page := 0
	perPage := 200
	for {
		members, err := client.GetChannelMembersWithTeamData(user.Id, page, perPage)
		if err != nil {
			return nil, apiError(fmt.Sprintf("error: failed to enumerate channel memberships (page %d). Halting to avoid partial processing.", page), err)
		}
		allMembers = append(allMembers, members...)
		if len(members) < perPage {
			break
		}
		page++
	}

	result.TotalMemberships = len(allMembers)
	verbose(fmt.Sprintf("  Found %d remaining channel memberships", len(allMembers)))

	// Fetch channel details
	progress("Fetching channel details...")
	channels := make([]channelInfo, 0, len(allMembers))
	for i, m := range allMembers {
		if i%20 == 0 {
			progress(fmt.Sprintf("Fetching channel details (%d/%d)...", i, len(allMembers)))
		}
		ch, err := client.GetChannel(m.ChannelId)
		if err != nil {
			return nil, apiError(fmt.Sprintf("error: failed to fetch details for channel %s during enumeration. Halting to avoid partial processing.", m.ChannelId), err)
		}
		channels = append(channels, channelInfo{
			ChannelID:   ch.Id,
			ChannelName: ch.Name,
			ChannelType: ch.Type,
			TeamName:    m.TeamName,
		})
	}

	// Separate channels: target (retain), DM/GM (skip), others (remove)
	var toRemove []channelInfo
	dmGmCount := 0
	for _, ch := range channels {
		if ch.ChannelType == model.ChannelTypeDirect || ch.ChannelType == model.ChannelTypeGroup {
			dmGmCount++
			continue
		}
		if ch.ChannelID == channel.Id {
			result.Actions = append(result.Actions, ChannelAction{
				TeamName:    cfg.TeamName,
				ChannelName: ch.ChannelName,
				ChannelID:   ch.ChannelID,
				Action:      "retained",
				Status:      "ok",
			})
			continue
		}
		toRemove = append(toRemove, ch)
	}
	result.ChannelsSkipped = dmGmCount
	result.ChannelsRetained = 1

	verbose(fmt.Sprintf("  Channels to remove: %d, DM/GM skipped: %d", len(toRemove), dmGmCount))

	// Step 8: Remove from non-target channels
	if len(toRemove) == 0 {
		verbose("  No channels to remove — operation complete")
	} else if cfg.DryRun {
		for _, ch := range toRemove {
			result.Actions = append(result.Actions, ChannelAction{
				TeamName:    ch.TeamName,
				ChannelName: ch.ChannelName,
				ChannelID:   ch.ChannelID,
				Action:      "removed",
				Status:      "ok",
			})
		}
		result.ChannelsRemoved = len(toRemove)
		verbose(fmt.Sprintf("  [DRY RUN] Would remove from %d channels", len(toRemove)))
	} else {
		removeResults := removeChannelsConcurrently(client, user.Id, toRemove, cfg.Workers, progress, verbose)
		for _, r := range removeResults {
			action := ChannelAction{
				TeamName:    r.Info.TeamName,
				ChannelName: r.Info.ChannelName,
				ChannelID:   r.Info.ChannelID,
				Action:      "removed",
				Status:      "ok",
			}
			if r.Err != nil {
				action.Status = "error"
				action.Error = r.Err.Error()
				result.Errors = append(result.Errors, action)
				result.RemovalErrors++
				verbose(fmt.Sprintf("  Error removing from #%s: %v", r.Info.ChannelName, r.Err))
			} else {
				result.ChannelsRemoved++
				verbose(fmt.Sprintf("  Removed from #%s (%s)", r.Info.ChannelName, r.Info.TeamName))
			}
			result.Actions = append(result.Actions, action)
		}
	}

	progress("Complete")

	totalErrors := result.RemovalErrors + result.TeamRemovalErrors
	if totalErrors > 0 {
		return result, newExitError(ExitPartialFailure,
			fmt.Sprintf("completed with %d removal errors", totalErrors), nil)
	}

	return result, nil
}

// removeChannelsConcurrently removes a user from multiple channels using a worker pool.
// It returns a slice of removalResult (one per channel).
func removeChannelsConcurrently(client MattermostClient, userID string, toRemove []channelInfo,
	workerCount int, progress func(string), verbose func(string)) []removalResult {

	workers := workerCount
	if workers <= 0 {
		workers = 10
	}

	sem := make(chan struct{}, workers)
	resultsCh := make(chan removalResult, len(toRemove))
	var wg sync.WaitGroup

	for i, ch := range toRemove {
		if i%5 == 0 {
			progress(fmt.Sprintf("Removing from channels (%d/%d)...", i, len(toRemove)))
		}
		wg.Add(1)
		go func(info channelInfo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			err := client.RemoveUserFromChannel(info.ChannelID, userID)
			resultsCh <- removalResult{Info: info, Err: err}
		}(ch)
	}

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	var results []removalResult
	for r := range resultsCh {
		results = append(results, r)
	}
	return results
}

// wrapStepError returns the original *ExitError if it is one, or wraps it as an API error.
func wrapStepError(err error, context string) error {
	if exitErr, ok := err.(*ExitError); ok {
		return exitErr
	}
	return apiError(fmt.Sprintf("error accessing %s", context), err)
}

// classifyRole returns "guest", "admin", or "member" based on the user's Roles string.
func classifyRole(roles string) string {
	if strings.Contains(roles, "system_guest") {
		return "guest"
	}
	if strings.Contains(roles, "system_admin") {
		return "admin"
	}
	return "member"
}

// displayName returns a formatted display name for the user.
func displayName(user *model.User) string {
	first := strings.TrimSpace(user.FirstName)
	last := strings.TrimSpace(user.LastName)
	if first != "" && last != "" {
		return first + " " + last
	}
	if first != "" {
		return first
	}
	if last != "" {
		return last
	}
	return user.Username
}
