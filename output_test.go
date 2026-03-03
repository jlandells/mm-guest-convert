package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func sampleResult() *ConvertResult {
	return &ConvertResult{
		DryRun: false,
		User: UserInfo{
			ID:          "user-id-001",
			Username:    "jsmith",
			DisplayName: "John Smith",
			CurrentRole: "member",
		},
		TargetTeam:    "acme-corp",
		TargetChannel: "project-alpha",
		WasAlreadyGuest:  false,
		WasAlreadyMember: true,
		TotalMemberships: 4,
		ChannelsRemoved:  2,
		ChannelsRetained: 1,
		ChannelsSkipped:  1,
		RemovalErrors:    0,
		Actions: []ChannelAction{
			{TeamName: "acme-corp", ChannelName: "project-alpha", ChannelID: "ch-target", Action: "retained", Status: "ok"},
			{TeamName: "acme-corp", ChannelName: "town-square", ChannelID: "ch-ts", Action: "removed", Status: "ok"},
			{TeamName: "acme-corp", ChannelName: "off-topic", ChannelID: "ch-ot", Action: "removed", Status: "ok"},
		},
		Errors: []ChannelAction{},
	}
}

func TestWriteTable_Basic(t *testing.T) {
	var buf bytes.Buffer
	result := sampleResult()
	code := writeTable(&buf, result, "v1.0.0")

	if code != ExitSuccess {
		t.Errorf("expected exit code %d, got %d", ExitSuccess, code)
	}

	output := buf.String()

	// Check required elements
	checks := []string{
		"mm-guest-convert v1.0.0",
		"Target user:    jsmith (John Smith)",
		"Current role:   member",
		"Target team:    acme-corp",
		"Target channel: project-alpha",
		"Step 1/6",
		"Step 2/6",
		"Step 3/6",
		"Step 4/6",
		"Step 5/6",
		"Step 6/6",
		"Removed from #town-square",
		"Removed from #off-topic",
		"Summary:",
		"2 channels removed",
		"0 errors",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("table output missing %q", check)
		}
	}
}

func TestWriteTable_DryRun(t *testing.T) {
	var buf bytes.Buffer
	result := sampleResult()
	result.DryRun = true
	writeTable(&buf, result, "v1.0.0")

	output := buf.String()
	if !strings.Contains(output, "DRY RUN") {
		t.Error("dry-run table should contain DRY RUN header")
	}
}

func TestWriteTable_AlreadyGuest(t *testing.T) {
	var buf bytes.Buffer
	result := sampleResult()
	result.WasAlreadyGuest = true
	writeTable(&buf, result, "v1.0.0")

	output := buf.String()
	if !strings.Contains(output, "already a guest") {
		t.Error("table should indicate user was already a guest")
	}
}

func TestWriteTable_Truncation(t *testing.T) {
	var buf bytes.Buffer
	result := sampleResult()
	// Add more than maxTableRemovals actions
	result.Actions = []ChannelAction{
		{TeamName: "acme-corp", ChannelName: "project-alpha", Action: "retained", Status: "ok"},
	}
	for i := 0; i < 15; i++ {
		result.Actions = append(result.Actions, ChannelAction{
			TeamName:    "acme-corp",
			ChannelName: fmt.Sprintf("channel-%02d", i),
			Action:      "removed",
			Status:      "ok",
		})
	}
	result.ChannelsRemoved = 15
	writeTable(&buf, result, "v1.0.0")

	output := buf.String()
	if !strings.Contains(output, "... (5 more)") {
		t.Errorf("expected truncation message '... (5 more)', output:\n%s", output)
	}
}

func TestWriteCSV_Basic(t *testing.T) {
	var buf bytes.Buffer
	result := sampleResult()
	code := writeCSV(&buf, result)

	if code != ExitSuccess {
		t.Errorf("expected exit code %d, got %d", ExitSuccess, code)
	}

	reader := csv.NewReader(&buf)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to parse CSV output: %v", err)
	}

	// Header + 3 data rows
	if len(records) != 4 {
		t.Fatalf("expected 4 CSV records (1 header + 3 data), got %d", len(records))
	}

	// Check header
	expectedHeader := []string{"username", "team_name", "channel_name", "action", "status"}
	for i, h := range expectedHeader {
		if records[0][i] != h {
			t.Errorf("header[%d] = %q, want %q", i, records[0][i], h)
		}
	}

	// Check first data row (retained)
	if records[1][0] != "jsmith" {
		t.Errorf("first data row username = %q, want 'jsmith'", records[1][0])
	}
	if records[1][3] != "retained" {
		t.Errorf("first data row action = %q, want 'retained'", records[1][3])
	}

	// Check removed rows
	if records[2][3] != "removed" {
		t.Errorf("second data row action = %q, want 'removed'", records[2][3])
	}
	if records[3][3] != "removed" {
		t.Errorf("third data row action = %q, want 'removed'", records[3][3])
	}
}

func TestWriteCSV_EmptyActions(t *testing.T) {
	var buf bytes.Buffer
	result := sampleResult()
	result.Actions = nil
	code := writeCSV(&buf, result)

	if code != ExitSuccess {
		t.Errorf("expected exit code %d, got %d", ExitSuccess, code)
	}

	reader := csv.NewReader(&buf)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to parse CSV output: %v", err)
	}

	// Only header
	if len(records) != 1 {
		t.Errorf("expected 1 CSV record (header only), got %d", len(records))
	}
}

func TestWriteJSON_Basic(t *testing.T) {
	var buf bytes.Buffer
	result := sampleResult()
	code := writeJSON(&buf, result)

	if code != ExitSuccess {
		t.Errorf("expected exit code %d, got %d", ExitSuccess, code)
	}

	var out jsonOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if out.DryRun != false {
		t.Error("expected dry_run=false")
	}
	if out.User.Username != "jsmith" {
		t.Errorf("expected username 'jsmith', got %q", out.User.Username)
	}
	if out.User.DisplayName != "John Smith" {
		t.Errorf("expected display_name 'John Smith', got %q", out.User.DisplayName)
	}
	if out.TargetTeam != "acme-corp" {
		t.Errorf("expected target_team 'acme-corp', got %q", out.TargetTeam)
	}
	if out.TargetChannel != "project-alpha" {
		t.Errorf("expected target_channel 'project-alpha', got %q", out.TargetChannel)
	}
	if len(out.ChannelsRemoved) != 2 {
		t.Errorf("expected 2 channels removed, got %d", len(out.ChannelsRemoved))
	}
	if out.Summary.TotalMembershipsFound != 4 {
		t.Errorf("expected total_memberships_found=4, got %d", out.Summary.TotalMembershipsFound)
	}
	if out.Summary.ChannelsRemoved != 2 {
		t.Errorf("expected channels_removed=2, got %d", out.Summary.ChannelsRemoved)
	}
	if out.Summary.ChannelsRetained != 1 {
		t.Errorf("expected channels_retained=1, got %d", out.Summary.ChannelsRetained)
	}
	if out.Summary.Errors != 0 {
		t.Errorf("expected errors=0, got %d", out.Summary.Errors)
	}
	if len(out.Errors) != 0 {
		t.Errorf("expected empty errors array, got %d", len(out.Errors))
	}
}

func TestWriteJSON_DryRun(t *testing.T) {
	var buf bytes.Buffer
	result := sampleResult()
	result.DryRun = true
	writeJSON(&buf, result)

	var out jsonOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if !out.DryRun {
		t.Error("expected dry_run=true in JSON output")
	}
}

func TestWriteJSON_WithErrors(t *testing.T) {
	var buf bytes.Buffer
	result := sampleResult()
	result.Errors = []ChannelAction{
		{TeamName: "acme-corp", ChannelName: "secret-channel", Action: "removed", Status: "error", Error: "permission denied"},
	}
	result.RemovalErrors = 1
	writeJSON(&buf, result)

	var out jsonOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(out.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(out.Errors))
	}
	if out.Errors[0].ChannelName != "secret-channel" {
		t.Errorf("expected error channel 'secret-channel', got %q", out.Errors[0].ChannelName)
	}
	if out.Errors[0].Error != "permission denied" {
		t.Errorf("expected error message 'permission denied', got %q", out.Errors[0].Error)
	}
}

func TestWriteJSON_EmptyArraysNotNull(t *testing.T) {
	var buf bytes.Buffer
	result := sampleResult()
	result.Actions = nil
	result.Errors = nil
	writeJSON(&buf, result)

	raw := buf.String()
	// JSON should have empty arrays, not null
	if strings.Contains(raw, `"channels_removed": null`) {
		t.Error("channels_removed should be [] not null")
	}
	if strings.Contains(raw, `"errors": null`) {
		t.Error("errors should be [] not null")
	}
}

func TestFilterActions(t *testing.T) {
	actions := []ChannelAction{
		{Action: "retained"},
		{Action: "removed"},
		{Action: "removed"},
		{Action: "retained"},
	}

	removed := filterActions(actions, "removed")
	if len(removed) != 2 {
		t.Errorf("expected 2 removed actions, got %d", len(removed))
	}

	retained := filterActions(actions, "retained")
	if len(retained) != 2 {
		t.Errorf("expected 2 retained actions, got %d", len(retained))
	}

	other := filterActions(actions, "other")
	if len(other) != 0 {
		t.Errorf("expected 0 other actions, got %d", len(other))
	}
}

func TestWriteFormatted_InvalidFormat(t *testing.T) {
	var buf bytes.Buffer
	result := sampleResult()
	// Unknown format defaults to table
	code := writeFormatted(&buf, result, "xml", "v1.0.0")
	if code != ExitSuccess {
		t.Errorf("expected exit code %d for unknown format (falls back to table), got %d", ExitSuccess, code)
	}
	if !strings.Contains(buf.String(), "mm-guest-convert") {
		t.Error("expected table output for unknown format")
	}
}
