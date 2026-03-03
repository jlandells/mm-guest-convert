package main

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

// mockClient implements MattermostClient with function fields for per-test overrides.
type mockClient struct {
	getUserByUsernameFn             func(username string) (*model.User, error)
	demoteUserToGuestFn             func(userID string) error
	getTeamByNameFn                 func(name string) (*model.Team, error)
	getChannelByNameFn              func(channelName, teamID string) (*model.Channel, error)
	getChannelMemberFn              func(channelID, userID string) (*model.ChannelMember, error)
	addChannelMemberFn              func(channelID, userID string) (*model.ChannelMember, error)
	getChannelMembersWithTeamDataFn func(userID string, page, perPage int) ([]model.ChannelMemberWithTeamData, error)
	removeUserFromChannelFn         func(channelID, userID string) error
	getChannelFn                    func(channelID string) (*model.Channel, error)
	getTeamsForUserFn               func(userID string) ([]*model.Team, error)
	removeUserFromTeamFn            func(teamID, userID string) error
	getConfigFn                     func() (*model.Config, error)
}

func (m *mockClient) GetUserByUsername(username string) (*model.User, error) {
	if m.getUserByUsernameFn != nil {
		return m.getUserByUsernameFn(username)
	}
	panic("mockClient.GetUserByUsername called but not configured")
}

func (m *mockClient) DemoteUserToGuest(userID string) error {
	if m.demoteUserToGuestFn != nil {
		return m.demoteUserToGuestFn(userID)
	}
	panic("mockClient.DemoteUserToGuest called but not configured")
}

func (m *mockClient) GetTeamByName(name string) (*model.Team, error) {
	if m.getTeamByNameFn != nil {
		return m.getTeamByNameFn(name)
	}
	panic("mockClient.GetTeamByName called but not configured")
}

func (m *mockClient) GetChannelByName(channelName, teamID string) (*model.Channel, error) {
	if m.getChannelByNameFn != nil {
		return m.getChannelByNameFn(channelName, teamID)
	}
	panic("mockClient.GetChannelByName called but not configured")
}

func (m *mockClient) GetChannelMember(channelID, userID string) (*model.ChannelMember, error) {
	if m.getChannelMemberFn != nil {
		return m.getChannelMemberFn(channelID, userID)
	}
	panic("mockClient.GetChannelMember called but not configured")
}

func (m *mockClient) AddChannelMember(channelID, userID string) (*model.ChannelMember, error) {
	if m.addChannelMemberFn != nil {
		return m.addChannelMemberFn(channelID, userID)
	}
	panic("mockClient.AddChannelMember called but not configured")
}

func (m *mockClient) GetChannelMembersWithTeamData(userID string, page, perPage int) ([]model.ChannelMemberWithTeamData, error) {
	if m.getChannelMembersWithTeamDataFn != nil {
		return m.getChannelMembersWithTeamDataFn(userID, page, perPage)
	}
	panic("mockClient.GetChannelMembersWithTeamData called but not configured")
}

func (m *mockClient) RemoveUserFromChannel(channelID, userID string) error {
	if m.removeUserFromChannelFn != nil {
		return m.removeUserFromChannelFn(channelID, userID)
	}
	panic("mockClient.RemoveUserFromChannel called but not configured")
}

func (m *mockClient) GetChannel(channelID string) (*model.Channel, error) {
	if m.getChannelFn != nil {
		return m.getChannelFn(channelID)
	}
	panic("mockClient.GetChannel called but not configured")
}

func (m *mockClient) GetTeamsForUser(userID string) ([]*model.Team, error) {
	if m.getTeamsForUserFn != nil {
		return m.getTeamsForUserFn(userID)
	}
	panic("mockClient.GetTeamsForUser called but not configured")
}

func (m *mockClient) RemoveUserFromTeam(teamID, userID string) error {
	if m.removeUserFromTeamFn != nil {
		return m.removeUserFromTeamFn(teamID, userID)
	}
	panic("mockClient.RemoveUserFromTeam called but not configured")
}

func (m *mockClient) GetConfig() (*model.Config, error) {
	if m.getConfigFn != nil {
		return m.getConfigFn()
	}
	panic("mockClient.GetConfig called but not configured")
}

// --- Test fixtures ---

func boolPtr(b bool) *bool { return &b }

func testUser(roles string) *model.User {
	return &model.User{
		Id:        "user-id-001",
		Username:  "jsmith",
		FirstName: "John",
		LastName:  "Smith",
		Roles:     roles,
	}
}

func testTeam() *model.Team {
	return &model.Team{
		Id:   "team-id-001",
		Name: "acme-corp",
	}
}

func testChannel(id, name string, chType model.ChannelType, teamID string) *model.Channel {
	return &model.Channel{
		Id:     id,
		Name:   name,
		Type:   chType,
		TeamId: teamID,
	}
}

func testConfig(guestEnabled bool) *model.Config {
	return &model.Config{
		GuestAccountsSettings: model.GuestAccountsSettings{
			Enable: boolPtr(guestEnabled),
		},
	}
}

func memberWithTeamData(channelID, teamName string) model.ChannelMemberWithTeamData {
	return model.ChannelMemberWithTeamData{
		ChannelMember: model.ChannelMember{
			ChannelId: channelID,
			UserId:    "user-id-001",
		},
		TeamName: teamName,
	}
}

func defaultMock() *mockClient {
	return &mockClient{
		getUserByUsernameFn: func(username string) (*model.User, error) {
			return testUser("system_user"), nil
		},
		demoteUserToGuestFn: func(userID string) error {
			return nil
		},
		getTeamByNameFn: func(name string) (*model.Team, error) {
			return testTeam(), nil
		},
		getChannelByNameFn: func(channelName, teamID string) (*model.Channel, error) {
			return testChannel("ch-target", "project-alpha", model.ChannelTypeOpen, "team-id-001"), nil
		},
		getChannelMemberFn: func(channelID, userID string) (*model.ChannelMember, error) {
			return &model.ChannelMember{ChannelId: channelID, UserId: userID}, nil
		},
		addChannelMemberFn: func(channelID, userID string) (*model.ChannelMember, error) {
			return &model.ChannelMember{ChannelId: channelID, UserId: userID}, nil
		},
		getChannelMembersWithTeamDataFn: func(userID string, page, perPage int) ([]model.ChannelMemberWithTeamData, error) {
			if page > 0 {
				return []model.ChannelMemberWithTeamData{}, nil
			}
			return []model.ChannelMemberWithTeamData{
				memberWithTeamData("ch-target", "acme-corp"),
				memberWithTeamData("ch-town-square", "acme-corp"),
				memberWithTeamData("ch-off-topic", "acme-corp"),
				memberWithTeamData("ch-dm-1", ""),
				memberWithTeamData("ch-gm-1", ""),
			}, nil
		},
		removeUserFromChannelFn: func(channelID, userID string) error {
			return nil
		},
		getChannelFn: func(channelID string) (*model.Channel, error) {
			switch channelID {
			case "ch-target":
				return testChannel("ch-target", "project-alpha", model.ChannelTypeOpen, "team-id-001"), nil
			case "ch-town-square":
				return testChannel("ch-town-square", "town-square", model.ChannelTypeOpen, "team-id-001"), nil
			case "ch-off-topic":
				return testChannel("ch-off-topic", "off-topic", model.ChannelTypeOpen, "team-id-001"), nil
			case "ch-dm-1":
				return testChannel("ch-dm-1", "dm-channel", model.ChannelTypeDirect, ""), nil
			case "ch-gm-1":
				return testChannel("ch-gm-1", "gm-channel", model.ChannelTypeGroup, ""), nil
			default:
				return nil, configError(fmt.Sprintf("channel %q not found", channelID))
			}
		},
		getConfigFn: func() (*model.Config, error) {
			return testConfig(true), nil
		},
	}
}

func defaultConfig() ConvertConfig {
	return ConvertConfig{
		TargetUsername: "jsmith",
		TeamName:       "acme-corp",
		ChannelName:    "project-alpha",
		DryRun:         false,
		Workers:        5,
		Verbose:        false,
	}
}

// --- Tests ---

func TestRunConversion_HappyPath(t *testing.T) {
	mock := defaultMock()
	cfg := defaultConfig()

	demoted := false
	mock.demoteUserToGuestFn = func(userID string) error {
		demoted = true
		return nil
	}

	removedChannels := make(map[string]bool)
	mock.removeUserFromChannelFn = func(channelID, userID string) error {
		removedChannels[channelID] = true
		return nil
	}

	result, err := RunConversion(mock, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !demoted {
		t.Error("expected user to be demoted")
	}
	if result.User.Username != "jsmith" {
		t.Errorf("expected username 'jsmith', got %q", result.User.Username)
	}
	if result.User.DisplayName != "John Smith" {
		t.Errorf("expected display name 'John Smith', got %q", result.User.DisplayName)
	}
	if result.WasAlreadyGuest {
		t.Error("expected WasAlreadyGuest=false")
	}
	if !result.WasAlreadyMember {
		t.Error("expected WasAlreadyMember=true since mock returns membership")
	}
	if result.TotalMemberships != 5 {
		t.Errorf("expected 5 total memberships, got %d", result.TotalMemberships)
	}
	if result.ChannelsRemoved != 2 {
		t.Errorf("expected 2 channels removed, got %d", result.ChannelsRemoved)
	}
	if result.ChannelsSkipped != 2 {
		t.Errorf("expected 2 DM/GM skipped, got %d", result.ChannelsSkipped)
	}
	if result.ChannelsRetained != 1 {
		t.Errorf("expected 1 channel retained, got %d", result.ChannelsRetained)
	}
	if result.RemovalErrors != 0 {
		t.Errorf("expected 0 removal errors, got %d", result.RemovalErrors)
	}
	if !removedChannels["ch-town-square"] {
		t.Error("expected town-square to be removed")
	}
	if !removedChannels["ch-off-topic"] {
		t.Error("expected off-topic to be removed")
	}
	if removedChannels["ch-target"] {
		t.Error("target channel should NOT be removed")
	}
	if removedChannels["ch-dm-1"] {
		t.Error("DM channel should NOT be removed")
	}
	if removedChannels["ch-gm-1"] {
		t.Error("GM channel should NOT be removed")
	}
}

func TestRunConversion_AlreadyGuest(t *testing.T) {
	mock := defaultMock()
	mock.getUserByUsernameFn = func(username string) (*model.User, error) {
		return testUser("system_guest"), nil
	}

	demoteCalled := false
	mock.demoteUserToGuestFn = func(userID string) error {
		demoteCalled = true
		return nil
	}
	// GetConfig should not be called when user is already a guest
	mock.getConfigFn = func() (*model.Config, error) {
		t.Error("GetConfig should not be called when user is already a guest")
		return testConfig(true), nil
	}

	cfg := defaultConfig()
	result, err := RunConversion(mock, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if demoteCalled {
		t.Error("DemoteUserToGuest should not be called for existing guest")
	}
	if !result.WasAlreadyGuest {
		t.Error("expected WasAlreadyGuest=true")
	}
	if result.User.CurrentRole != "guest" {
		t.Errorf("expected role 'guest', got %q", result.User.CurrentRole)
	}
}

func TestRunConversion_AlreadyInChannel(t *testing.T) {
	mock := defaultMock()
	cfg := defaultConfig()

	addCalled := false
	mock.addChannelMemberFn = func(channelID, userID string) (*model.ChannelMember, error) {
		addCalled = true
		return nil, nil
	}

	result, err := RunConversion(mock, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if addCalled {
		t.Error("AddChannelMember should not be called when already a member")
	}
	if !result.WasAlreadyMember {
		t.Error("expected WasAlreadyMember=true")
	}
}

func TestRunConversion_NotInChannel_AddsUser(t *testing.T) {
	mock := defaultMock()
	mock.getChannelMemberFn = func(channelID, userID string) (*model.ChannelMember, error) {
		return nil, configError("not found")
	}

	addCalled := false
	mock.addChannelMemberFn = func(channelID, userID string) (*model.ChannelMember, error) {
		addCalled = true
		return &model.ChannelMember{ChannelId: channelID, UserId: userID}, nil
	}

	cfg := defaultConfig()
	result, err := RunConversion(mock, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !addCalled {
		t.Error("expected AddChannelMember to be called")
	}
	if result.WasAlreadyMember {
		t.Error("expected WasAlreadyMember=false")
	}
}

func TestRunConversion_DryRun(t *testing.T) {
	mock := defaultMock()
	mock.getUserByUsernameFn = func(username string) (*model.User, error) {
		return testUser("system_user"), nil
	}

	demoteCalled := false
	mock.demoteUserToGuestFn = func(userID string) error {
		demoteCalled = true
		return nil
	}

	addCalled := false
	mock.addChannelMemberFn = func(channelID, userID string) (*model.ChannelMember, error) {
		addCalled = true
		return nil, nil
	}

	removeCalled := false
	mock.removeUserFromChannelFn = func(channelID, userID string) error {
		removeCalled = true
		return nil
	}

	// Make user not already in the target channel
	mock.getChannelMemberFn = func(channelID, userID string) (*model.ChannelMember, error) {
		return nil, configError("not found")
	}

	cfg := defaultConfig()
	cfg.DryRun = true

	result, err := RunConversion(mock, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if demoteCalled {
		t.Error("DemoteUserToGuest must NOT be called in dry-run mode")
	}
	if addCalled {
		t.Error("AddChannelMember must NOT be called in dry-run mode")
	}
	if removeCalled {
		t.Error("RemoveUserFromChannel must NOT be called in dry-run mode")
	}
	if !result.DryRun {
		t.Error("expected DryRun=true in result")
	}
	if result.ChannelsRemoved != 2 {
		t.Errorf("expected 2 channels would be removed, got %d", result.ChannelsRemoved)
	}
}

func TestRunConversion_UserNotFound(t *testing.T) {
	mock := defaultMock()
	mock.getUserByUsernameFn = func(username string) (*model.User, error) {
		return nil, configError(fmt.Sprintf("error: user %q not found. Please check the username.", username))
	}

	cfg := defaultConfig()
	_, err := RunConversion(mock, cfg)
	if err == nil {
		t.Fatal("expected error for user not found")
	}
	exitErr, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("expected *ExitError, got %T", err)
	}
	if exitErr.Code != ExitConfigError {
		t.Errorf("expected exit code %d, got %d", ExitConfigError, exitErr.Code)
	}
}

func TestRunConversion_TeamNotFound(t *testing.T) {
	mock := defaultMock()
	mock.getTeamByNameFn = func(name string) (*model.Team, error) {
		return nil, configError(fmt.Sprintf("error: team %q not found. Please check the name and try again.", name))
	}

	cfg := defaultConfig()
	_, err := RunConversion(mock, cfg)
	if err == nil {
		t.Fatal("expected error for team not found")
	}
	exitErr, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("expected *ExitError, got %T", err)
	}
	if exitErr.Code != ExitConfigError {
		t.Errorf("expected exit code %d, got %d", ExitConfigError, exitErr.Code)
	}
}

func TestRunConversion_ChannelNotFound(t *testing.T) {
	mock := defaultMock()
	mock.getChannelByNameFn = func(channelName, teamID string) (*model.Channel, error) {
		return nil, configError(fmt.Sprintf("error: channel %q not found in team", channelName))
	}

	cfg := defaultConfig()
	_, err := RunConversion(mock, cfg)
	if err == nil {
		t.Fatal("expected error for channel not found")
	}
	exitErr, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("expected *ExitError, got %T", err)
	}
	if exitErr.Code != ExitConfigError {
		t.Errorf("expected exit code %d, got %d", ExitConfigError, exitErr.Code)
	}
}

func TestRunConversion_GuestAccountsDisabled(t *testing.T) {
	mock := defaultMock()
	mock.getConfigFn = func() (*model.Config, error) {
		return testConfig(false), nil
	}

	cfg := defaultConfig()
	_, err := RunConversion(mock, cfg)
	if err == nil {
		t.Fatal("expected error when guest accounts disabled")
	}
	exitErr, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("expected *ExitError, got %T", err)
	}
	if exitErr.Code != ExitConfigError {
		t.Errorf("expected exit code %d, got %d", ExitConfigError, exitErr.Code)
	}
	if !strings.Contains(exitErr.Message, "guest accounts are not enabled") {
		t.Errorf("expected guest accounts error message, got %q", exitErr.Message)
	}
}

func TestRunConversion_ConfigCheckFails_ContinuesAnyway(t *testing.T) {
	mock := defaultMock()
	mock.getConfigFn = func() (*model.Config, error) {
		return nil, apiError("config unavailable", fmt.Errorf("forbidden"))
	}

	cfg := defaultConfig()
	cfg.Verbose = true

	result, err := RunConversion(mock, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.User.Username != "jsmith" {
		t.Errorf("expected user to still be resolved, got %q", result.User.Username)
	}
}

func TestRunConversion_PartialFailure(t *testing.T) {
	mock := defaultMock()

	mock.removeUserFromChannelFn = func(channelID, userID string) error {
		if channelID == "ch-off-topic" {
			return apiError("removal failed", fmt.Errorf("server error"))
		}
		return nil
	}

	cfg := defaultConfig()
	result, err := RunConversion(mock, cfg)

	exitErr, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("expected *ExitError, got %T (%v)", err, err)
	}
	if exitErr.Code != ExitPartialFailure {
		t.Errorf("expected exit code %d, got %d", ExitPartialFailure, exitErr.Code)
	}
	if result.RemovalErrors != 1 {
		t.Errorf("expected 1 removal error, got %d", result.RemovalErrors)
	}
	if result.ChannelsRemoved != 1 {
		t.Errorf("expected 1 channel removed, got %d", result.ChannelsRemoved)
	}
	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error entry, got %d", len(result.Errors))
	}
}

func TestRunConversion_NoChannelsToRemove(t *testing.T) {
	mock := defaultMock()
	mock.getChannelMembersWithTeamDataFn = func(userID string, page, perPage int) ([]model.ChannelMemberWithTeamData, error) {
		if page > 0 {
			return []model.ChannelMemberWithTeamData{}, nil
		}
		return []model.ChannelMemberWithTeamData{
			memberWithTeamData("ch-target", "acme-corp"),
		}, nil
	}
	mock.getChannelFn = func(channelID string) (*model.Channel, error) {
		if channelID == "ch-target" {
			return testChannel("ch-target", "project-alpha", model.ChannelTypeOpen, "team-id-001"), nil
		}
		return nil, configError("not found")
	}

	removeCalled := false
	mock.removeUserFromChannelFn = func(channelID, userID string) error {
		removeCalled = true
		return nil
	}

	cfg := defaultConfig()
	result, err := RunConversion(mock, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if removeCalled {
		t.Error("RemoveUserFromChannel should not be called with no channels to remove")
	}
	if result.ChannelsRemoved != 0 {
		t.Errorf("expected 0 channels removed, got %d", result.ChannelsRemoved)
	}
	if result.ChannelsRetained != 1 {
		t.Errorf("expected 1 channel retained, got %d", result.ChannelsRetained)
	}
}

func TestRunConversion_EmptyMemberships(t *testing.T) {
	mock := defaultMock()
	mock.getChannelMembersWithTeamDataFn = func(userID string, page, perPage int) ([]model.ChannelMemberWithTeamData, error) {
		return []model.ChannelMemberWithTeamData{}, nil
	}

	cfg := defaultConfig()
	result, err := RunConversion(mock, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalMemberships != 0 {
		t.Errorf("expected 0 total memberships, got %d", result.TotalMemberships)
	}
	if result.ChannelsRemoved != 0 {
		t.Errorf("expected 0 channels removed, got %d", result.ChannelsRemoved)
	}
}

func TestRunConversion_PaginationBoundary(t *testing.T) {
	mock := defaultMock()

	// Return exactly 200 items on page 0 (triggers next page fetch), then empty
	page0Members := make([]model.ChannelMemberWithTeamData, 200)
	for i := 0; i < 200; i++ {
		page0Members[i] = memberWithTeamData(fmt.Sprintf("ch-%03d", i), "acme-corp")
	}
	// Set ch-000 as the target channel
	mock.getChannelByNameFn = func(channelName, teamID string) (*model.Channel, error) {
		return testChannel("ch-000", "project-alpha", model.ChannelTypeOpen, "team-id-001"), nil
	}

	pagesRequested := 0
	mock.getChannelMembersWithTeamDataFn = func(userID string, page, perPage int) ([]model.ChannelMemberWithTeamData, error) {
		pagesRequested++
		if page == 0 {
			return page0Members, nil
		}
		return []model.ChannelMemberWithTeamData{}, nil
	}

	mock.getChannelFn = func(channelID string) (*model.Channel, error) {
		return testChannel(channelID, channelID, model.ChannelTypeOpen, "team-id-001"), nil
	}

	cfg := defaultConfig()
	result, err := RunConversion(mock, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pagesRequested != 2 {
		t.Errorf("expected 2 pages requested (boundary condition), got %d", pagesRequested)
	}
	if result.TotalMemberships != 200 {
		t.Errorf("expected 200 total memberships, got %d", result.TotalMemberships)
	}
	// 200 total - 1 target = 199 removed
	if result.ChannelsRemoved != 199 {
		t.Errorf("expected 199 channels removed, got %d", result.ChannelsRemoved)
	}
}

func TestRunConversion_MidPaginationFailure(t *testing.T) {
	mock := defaultMock()

	page0Members := make([]model.ChannelMemberWithTeamData, 200)
	for i := 0; i < 200; i++ {
		page0Members[i] = memberWithTeamData(fmt.Sprintf("ch-%03d", i), "acme-corp")
	}

	mock.getChannelMembersWithTeamDataFn = func(userID string, page, perPage int) ([]model.ChannelMemberWithTeamData, error) {
		if page == 0 {
			return page0Members, nil
		}
		return nil, apiError("enumeration failed on page 1", fmt.Errorf("server error"))
	}

	cfg := defaultConfig()
	_, err := RunConversion(mock, cfg)
	if err == nil {
		t.Fatal("expected error for mid-pagination failure")
	}
	exitErr, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("expected *ExitError, got %T", err)
	}
	if exitErr.Code != ExitAPIError {
		t.Errorf("expected exit code %d, got %d", ExitAPIError, exitErr.Code)
	}
}

func TestRunConversion_DMGMExclusion(t *testing.T) {
	mock := defaultMock()

	mock.getChannelMembersWithTeamDataFn = func(userID string, page, perPage int) ([]model.ChannelMemberWithTeamData, error) {
		if page > 0 {
			return []model.ChannelMemberWithTeamData{}, nil
		}
		return []model.ChannelMemberWithTeamData{
			memberWithTeamData("ch-target", "acme-corp"),
			memberWithTeamData("ch-dm-1", ""),
			memberWithTeamData("ch-dm-2", ""),
			memberWithTeamData("ch-gm-1", ""),
		}, nil
	}
	mock.getChannelFn = func(channelID string) (*model.Channel, error) {
		switch channelID {
		case "ch-target":
			return testChannel("ch-target", "project-alpha", model.ChannelTypeOpen, "team-id-001"), nil
		case "ch-dm-1", "ch-dm-2":
			return testChannel(channelID, channelID, model.ChannelTypeDirect, ""), nil
		case "ch-gm-1":
			return testChannel(channelID, channelID, model.ChannelTypeGroup, ""), nil
		}
		return nil, configError("not found")
	}

	removeCalled := false
	mock.removeUserFromChannelFn = func(channelID, userID string) error {
		removeCalled = true
		return nil
	}

	cfg := defaultConfig()
	result, err := RunConversion(mock, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if removeCalled {
		t.Error("no channels should be removed — all non-target are DM/GM")
	}
	if result.ChannelsSkipped != 3 {
		t.Errorf("expected 3 DM/GM skipped, got %d", result.ChannelsSkipped)
	}
	if result.ChannelsRemoved != 0 {
		t.Errorf("expected 0 channels removed, got %d", result.ChannelsRemoved)
	}
}

func TestRunConversion_ConcurrentWorkers(t *testing.T) {
	for _, workers := range []int{1, 5} {
		t.Run(fmt.Sprintf("workers=%d", workers), func(t *testing.T) {
			mock := defaultMock()
			cfg := defaultConfig()
			cfg.Workers = workers

			result, err := RunConversion(mock, cfg)
			if err != nil {
				t.Fatalf("unexpected error with %d workers: %v", workers, err)
			}
			if result.ChannelsRemoved != 2 {
				t.Errorf("expected 2 channels removed with %d workers, got %d", workers, result.ChannelsRemoved)
			}
		})
	}
}

func TestRunConversion_AdminUser(t *testing.T) {
	mock := defaultMock()
	mock.getUserByUsernameFn = func(username string) (*model.User, error) {
		return testUser("system_admin system_user"), nil
	}

	cfg := defaultConfig()
	result, err := RunConversion(mock, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.User.CurrentRole != "admin" {
		t.Errorf("expected role 'admin', got %q", result.User.CurrentRole)
	}
}

func TestRunConversion_KeepAllChannels(t *testing.T) {
	mock := &mockClient{
		getUserByUsernameFn: func(username string) (*model.User, error) {
			return testUser("system_user"), nil
		},
		getConfigFn: func() (*model.Config, error) {
			return testConfig(true), nil
		},
		demoteUserToGuestFn: func(userID string) error {
			return nil
		},
		// These must NOT be called — channel management is skipped
		getTeamByNameFn: func(name string) (*model.Team, error) {
			panic("GetTeamByName must not be called in keep-all-channels mode")
		},
		getChannelByNameFn: func(channelName, teamID string) (*model.Channel, error) {
			panic("GetChannelByName must not be called in keep-all-channels mode")
		},
		getChannelMembersWithTeamDataFn: func(userID string, page, perPage int) ([]model.ChannelMemberWithTeamData, error) {
			panic("GetChannelMembersWithTeamData must not be called in keep-all-channels mode")
		},
		removeUserFromChannelFn: func(channelID, userID string) error {
			panic("RemoveUserFromChannel must not be called in keep-all-channels mode")
		},
	}

	cfg := ConvertConfig{
		TargetUsername:  "jsmith",
		KeepAllChannels: true,
		Workers:         5,
	}

	demoted := false
	mock.demoteUserToGuestFn = func(userID string) error {
		demoted = true
		return nil
	}

	result, err := RunConversion(mock, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !demoted {
		t.Error("expected user to be demoted")
	}
	if !result.KeepAllChannels {
		t.Error("expected KeepAllChannels=true in result")
	}
	if result.User.Username != "jsmith" {
		t.Errorf("expected username 'jsmith', got %q", result.User.Username)
	}
	if result.ChannelsRemoved != 0 {
		t.Errorf("expected 0 channels removed, got %d", result.ChannelsRemoved)
	}
	if result.ChannelsRetained != 0 {
		t.Errorf("expected 0 channels retained, got %d", result.ChannelsRetained)
	}
	if result.TotalMemberships != 0 {
		t.Errorf("expected 0 total memberships, got %d", result.TotalMemberships)
	}
}

func TestRunConversion_KeepAllChannels_AlreadyGuest(t *testing.T) {
	mock := &mockClient{
		getUserByUsernameFn: func(username string) (*model.User, error) {
			return testUser("system_guest"), nil
		},
		// These must NOT be called
		getTeamByNameFn: func(name string) (*model.Team, error) {
			panic("GetTeamByName must not be called in keep-all-channels mode")
		},
		getChannelByNameFn: func(channelName, teamID string) (*model.Channel, error) {
			panic("GetChannelByName must not be called in keep-all-channels mode")
		},
		getChannelMembersWithTeamDataFn: func(userID string, page, perPage int) ([]model.ChannelMemberWithTeamData, error) {
			panic("GetChannelMembersWithTeamData must not be called in keep-all-channels mode")
		},
		removeUserFromChannelFn: func(channelID, userID string) error {
			panic("RemoveUserFromChannel must not be called in keep-all-channels mode")
		},
	}

	demoteCalled := false
	mock.demoteUserToGuestFn = func(userID string) error {
		demoteCalled = true
		return nil
	}

	cfg := ConvertConfig{
		TargetUsername:  "jsmith",
		KeepAllChannels: true,
		Workers:         5,
	}

	result, err := RunConversion(mock, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if demoteCalled {
		t.Error("DemoteUserToGuest should not be called for existing guest")
	}
	if !result.WasAlreadyGuest {
		t.Error("expected WasAlreadyGuest=true")
	}
	if !result.KeepAllChannels {
		t.Error("expected KeepAllChannels=true in result")
	}
	if result.User.CurrentRole != "guest" {
		t.Errorf("expected role 'guest', got %q", result.User.CurrentRole)
	}
}

func TestClassifyRole(t *testing.T) {
	tests := []struct {
		roles string
		want  string
	}{
		{"system_guest", "guest"},
		{"system_user", "member"},
		{"system_admin system_user", "admin"},
		{"system_user system_admin", "admin"},
		{"", "member"},
	}
	for _, tt := range tests {
		t.Run(tt.roles, func(t *testing.T) {
			got := classifyRole(tt.roles)
			if got != tt.want {
				t.Errorf("classifyRole(%q) = %q, want %q", tt.roles, got, tt.want)
			}
		})
	}
}

func TestDisplayName(t *testing.T) {
	tests := []struct {
		name      string
		firstName string
		lastName  string
		username  string
		want      string
	}{
		{"both names", "John", "Smith", "jsmith", "John Smith"},
		{"first only", "John", "", "jsmith", "John"},
		{"last only", "", "Smith", "jsmith", "Smith"},
		{"no names", "", "", "jsmith", "jsmith"},
		{"whitespace names", "  ", "  ", "jsmith", "jsmith"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &model.User{
				Username:  tt.username,
				FirstName: tt.firstName,
				LastName:  tt.lastName,
			}
			got := displayName(user)
			if got != tt.want {
				t.Errorf("displayName() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- Restrict-to-team tests ---

func restrictToTeamMock() *mockClient {
	mock := defaultMock()
	mock.getTeamsForUserFn = func(userID string) ([]*model.Team, error) {
		return []*model.Team{
			{Id: "team-id-001", Name: "acme-corp", DeleteAt: 0},
			{Id: "team-id-002", Name: "acme-sales", DeleteAt: 0},
			{Id: "team-id-003", Name: "acme-marketing", DeleteAt: 0},
		}, nil
	}
	mock.removeUserFromTeamFn = func(teamID, userID string) error {
		return nil
	}
	return mock
}

func restrictToTeamConfig() ConvertConfig {
	return ConvertConfig{
		TargetUsername: "jsmith",
		TeamName:       "acme-corp",
		ChannelName:    "project-alpha",
		RestrictToTeam: true,
		DryRun:         false,
		Workers:        5,
		Verbose:        false,
	}
}

func TestRunConversion_RestrictToTeam_HappyPath(t *testing.T) {
	mock := restrictToTeamMock()
	cfg := restrictToTeamConfig()

	var mu sync.Mutex
	removedTeams := make(map[string]bool)
	mock.removeUserFromTeamFn = func(teamID, userID string) error {
		mu.Lock()
		removedTeams[teamID] = true
		mu.Unlock()
		return nil
	}

	removedChannels := make(map[string]bool)
	mock.removeUserFromChannelFn = func(channelID, userID string) error {
		mu.Lock()
		removedChannels[channelID] = true
		mu.Unlock()
		return nil
	}

	result, err := RunConversion(mock, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.RestrictToTeam {
		t.Error("expected RestrictToTeam=true")
	}
	if result.TotalTeams != 3 {
		t.Errorf("expected 3 total teams, got %d", result.TotalTeams)
	}
	if result.TeamsRemovedCount != 2 {
		t.Errorf("expected 2 teams removed, got %d", result.TeamsRemovedCount)
	}
	if result.TeamsRetained != 1 {
		t.Errorf("expected 1 team retained, got %d", result.TeamsRetained)
	}
	if result.TeamRemovalErrors != 0 {
		t.Errorf("expected 0 team removal errors, got %d", result.TeamRemovalErrors)
	}

	mu.Lock()
	defer mu.Unlock()
	// acme-sales and acme-marketing should be removed
	if !removedTeams["team-id-002"] {
		t.Error("expected team acme-sales to be removed")
	}
	if !removedTeams["team-id-003"] {
		t.Error("expected team acme-marketing to be removed")
	}
	if removedTeams["team-id-001"] {
		t.Error("target team acme-corp should NOT be removed")
	}
	// Channel removal should also happen (town-square and off-topic)
	if result.ChannelsRemoved != 2 {
		t.Errorf("expected 2 channels removed, got %d", result.ChannelsRemoved)
	}
}

func TestRunConversion_RestrictToTeam_OnlyTargetTeam(t *testing.T) {
	mock := restrictToTeamMock()
	mock.getTeamsForUserFn = func(userID string) ([]*model.Team, error) {
		return []*model.Team{
			{Id: "team-id-001", Name: "acme-corp", DeleteAt: 0},
		}, nil
	}

	teamRemoveCalled := false
	mock.removeUserFromTeamFn = func(teamID, userID string) error {
		teamRemoveCalled = true
		return nil
	}

	cfg := restrictToTeamConfig()
	result, err := RunConversion(mock, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if teamRemoveCalled {
		t.Error("RemoveUserFromTeam should not be called when user is only in the target team")
	}
	if result.TeamsRemovedCount != 0 {
		t.Errorf("expected 0 teams removed, got %d", result.TeamsRemovedCount)
	}
	if result.TeamsRetained != 1 {
		t.Errorf("expected 1 team retained, got %d", result.TeamsRetained)
	}
}

func TestRunConversion_RestrictToTeam_DryRun(t *testing.T) {
	mock := restrictToTeamMock()

	demoteCalled := false
	mock.demoteUserToGuestFn = func(userID string) error {
		demoteCalled = true
		return nil
	}

	teamRemoveCalled := false
	mock.removeUserFromTeamFn = func(teamID, userID string) error {
		teamRemoveCalled = true
		return nil
	}

	channelRemoveCalled := false
	mock.removeUserFromChannelFn = func(channelID, userID string) error {
		channelRemoveCalled = true
		return nil
	}

	addCalled := false
	mock.addChannelMemberFn = func(channelID, userID string) (*model.ChannelMember, error) {
		addCalled = true
		return nil, nil
	}

	// Make user not in channel to test add is also skipped
	mock.getChannelMemberFn = func(channelID, userID string) (*model.ChannelMember, error) {
		return nil, configError("not found")
	}

	cfg := restrictToTeamConfig()
	cfg.DryRun = true

	result, err := RunConversion(mock, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if demoteCalled {
		t.Error("DemoteUserToGuest must NOT be called in dry-run mode")
	}
	if teamRemoveCalled {
		t.Error("RemoveUserFromTeam must NOT be called in dry-run mode")
	}
	if channelRemoveCalled {
		t.Error("RemoveUserFromChannel must NOT be called in dry-run mode")
	}
	if addCalled {
		t.Error("AddChannelMember must NOT be called in dry-run mode")
	}
	if !result.DryRun {
		t.Error("expected DryRun=true in result")
	}
	if result.TeamsRemovedCount != 2 {
		t.Errorf("expected 2 teams would be removed, got %d", result.TeamsRemovedCount)
	}
}

func TestRunConversion_RestrictToTeam_PartialFailure_Team(t *testing.T) {
	mock := restrictToTeamMock()
	mock.removeUserFromTeamFn = func(teamID, userID string) error {
		if teamID == "team-id-002" {
			return apiError("removal failed", fmt.Errorf("server error"))
		}
		return nil
	}

	cfg := restrictToTeamConfig()
	result, err := RunConversion(mock, cfg)

	exitErr, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("expected *ExitError, got %T (%v)", err, err)
	}
	if exitErr.Code != ExitPartialFailure {
		t.Errorf("expected exit code %d, got %d", ExitPartialFailure, exitErr.Code)
	}
	if result.TeamRemovalErrors != 1 {
		t.Errorf("expected 1 team removal error, got %d", result.TeamRemovalErrors)
	}
	if result.TeamsRemovedCount != 1 {
		t.Errorf("expected 1 team removed, got %d", result.TeamsRemovedCount)
	}
	if len(result.TeamErrors) != 1 {
		t.Errorf("expected 1 team error entry, got %d", len(result.TeamErrors))
	}
}

func TestRunConversion_RestrictToTeam_ChannelEnumerationAfterTeamRemoval(t *testing.T) {
	mock := restrictToTeamMock()
	cfg := restrictToTeamConfig()

	// Track call ordering — team removals are concurrent so we track phases not individual calls
	var mu sync.Mutex
	var callSequence []string
	mock.removeUserFromTeamFn = func(teamID, userID string) error {
		mu.Lock()
		callSequence = append(callSequence, "remove-team:"+teamID)
		mu.Unlock()
		return nil
	}
	mock.getChannelMembersWithTeamDataFn = func(userID string, page, perPage int) ([]model.ChannelMemberWithTeamData, error) {
		mu.Lock()
		callSequence = append(callSequence, fmt.Sprintf("enumerate-channels:page%d", page))
		mu.Unlock()
		if page > 0 {
			return []model.ChannelMemberWithTeamData{}, nil
		}
		return []model.ChannelMemberWithTeamData{
			memberWithTeamData("ch-target", "acme-corp"),
			memberWithTeamData("ch-town-square", "acme-corp"),
		}, nil
	}

	result, err := RunConversion(mock, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	seq := make([]string, len(callSequence))
	copy(seq, callSequence)
	mu.Unlock()

	// Verify team removals happen before channel enumeration
	teamRemovalsSeen := 0
	channelEnumSeen := false
	for _, call := range seq {
		if strings.HasPrefix(call, "remove-team:") {
			if channelEnumSeen {
				t.Errorf("team removal happened after channel enumeration: %v", seq)
				break
			}
			teamRemovalsSeen++
		}
		if strings.HasPrefix(call, "enumerate-channels:") {
			channelEnumSeen = true
		}
	}
	if teamRemovalsSeen != 2 {
		t.Errorf("expected 2 team removals before channel enumeration, got %d", teamRemovalsSeen)
	}
	if result.ChannelsRemoved != 1 {
		t.Errorf("expected 1 channel removed, got %d", result.ChannelsRemoved)
	}
}

func TestRunConversion_RestrictToTeam_LargeTeamList(t *testing.T) {
	mock := restrictToTeamMock()

	// 50 teams: target + 49 others
	teams := make([]*model.Team, 50)
	teams[0] = &model.Team{Id: "team-id-001", Name: "acme-corp", DeleteAt: 0}
	for i := 1; i < 50; i++ {
		teams[i] = &model.Team{Id: fmt.Sprintf("team-id-%03d", i+1), Name: fmt.Sprintf("team-%03d", i), DeleteAt: 0}
	}
	mock.getTeamsForUserFn = func(userID string) ([]*model.Team, error) {
		return teams, nil
	}

	var mu sync.Mutex
	removedCount := 0
	mock.removeUserFromTeamFn = func(teamID, userID string) error {
		mu.Lock()
		removedCount++
		mu.Unlock()
		return nil
	}

	cfg := restrictToTeamConfig()
	result, err := RunConversion(mock, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalTeams != 50 {
		t.Errorf("expected 50 total teams, got %d", result.TotalTeams)
	}
	if result.TeamsRemovedCount != 49 {
		t.Errorf("expected 49 teams removed, got %d", result.TeamsRemovedCount)
	}
	mu.Lock()
	if removedCount != 49 {
		t.Errorf("expected 49 RemoveUserFromTeam calls, got %d", removedCount)
	}
	mu.Unlock()
}

func TestRunConversion_RestrictToTeam_ArchivedTeamsFiltered(t *testing.T) {
	mock := restrictToTeamMock()
	mock.getTeamsForUserFn = func(userID string) ([]*model.Team, error) {
		return []*model.Team{
			{Id: "team-id-001", Name: "acme-corp", DeleteAt: 0},
			{Id: "team-id-002", Name: "acme-sales", DeleteAt: 0},
			{Id: "team-id-004", Name: "archived-team", DeleteAt: 1609459200000}, // archived
		}, nil
	}

	var mu sync.Mutex
	removedTeams := make(map[string]bool)
	mock.removeUserFromTeamFn = func(teamID, userID string) error {
		mu.Lock()
		removedTeams[teamID] = true
		mu.Unlock()
		return nil
	}

	cfg := restrictToTeamConfig()
	result, err := RunConversion(mock, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only 2 active teams (archived is filtered)
	if result.TotalTeams != 2 {
		t.Errorf("expected 2 total active teams, got %d", result.TotalTeams)
	}
	if result.TeamsRemovedCount != 1 {
		t.Errorf("expected 1 team removed, got %d", result.TeamsRemovedCount)
	}
	mu.Lock()
	defer mu.Unlock()
	if removedTeams["team-id-004"] {
		t.Error("archived team should NOT be removed")
	}
	if !removedTeams["team-id-002"] {
		t.Error("expected acme-sales to be removed")
	}
}
