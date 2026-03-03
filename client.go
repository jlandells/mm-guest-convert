package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"golang.org/x/term"
)

// MattermostClient abstracts all Mattermost API calls for testability.
type MattermostClient interface {
	GetUserByUsername(username string) (*model.User, error)
	DemoteUserToGuest(userID string) error
	GetTeamByName(name string) (*model.Team, error)
	GetChannelByName(channelName, teamID string) (*model.Channel, error)
	GetChannelMember(channelID, userID string) (*model.ChannelMember, error)
	AddChannelMember(channelID, userID string) (*model.ChannelMember, error)
	GetChannelMembersWithTeamData(userID string, page, perPage int) ([]model.ChannelMemberWithTeamData, error)
	RemoveUserFromChannel(channelID, userID string) error
	GetChannel(channelID string) (*model.Channel, error)
	GetConfig() (*model.Config, error)
}

// apiClient wraps model.Client4 and implements MattermostClient.
type apiClient struct {
	api *model.Client4
}

func (c *apiClient) GetUserByUsername(username string) (*model.User, error) {
	user, resp, err := c.api.GetUserByUsername(context.Background(), username, "")
	if err != nil {
		return nil, wrapAPIError(resp, err, fmt.Sprintf("user %q", username))
	}
	return user, nil
}

func (c *apiClient) DemoteUserToGuest(userID string) error {
	resp, err := c.api.DemoteUserToGuest(context.Background(), userID)
	if err != nil {
		return wrapAPIError(resp, err, "demoting user to guest")
	}
	return nil
}

func (c *apiClient) GetTeamByName(name string) (*model.Team, error) {
	team, resp, err := c.api.GetTeamByName(context.Background(), name, "")
	if err != nil {
		return nil, wrapAPIError(resp, err, fmt.Sprintf("team %q", name))
	}
	return team, nil
}

func (c *apiClient) GetChannelByName(channelName, teamID string) (*model.Channel, error) {
	channel, resp, err := c.api.GetChannelByName(context.Background(), channelName, teamID, "")
	if err != nil {
		return nil, wrapAPIError(resp, err, fmt.Sprintf("channel %q", channelName))
	}
	return channel, nil
}

func (c *apiClient) GetChannelMember(channelID, userID string) (*model.ChannelMember, error) {
	member, resp, err := c.api.GetChannelMember(context.Background(), channelID, userID, "")
	if err != nil {
		return nil, wrapAPIError(resp, err, "channel membership")
	}
	return member, nil
}

func (c *apiClient) AddChannelMember(channelID, userID string) (*model.ChannelMember, error) {
	member, resp, err := c.api.AddChannelMember(context.Background(), channelID, userID)
	if err != nil {
		return nil, wrapAPIError(resp, err, "adding channel member")
	}
	return member, nil
}

func (c *apiClient) GetChannelMembersWithTeamData(userID string, page, perPage int) ([]model.ChannelMemberWithTeamData, error) {
	members, resp, err := c.api.GetChannelMembersWithTeamData(context.Background(), userID, page, perPage)
	if err != nil {
		return nil, wrapAPIError(resp, err, "channel memberships")
	}
	return members, nil
}

func (c *apiClient) RemoveUserFromChannel(channelID, userID string) error {
	resp, err := c.api.RemoveUserFromChannel(context.Background(), channelID, userID)
	if err != nil {
		return wrapAPIError(resp, err, fmt.Sprintf("removing user from channel %s", channelID))
	}
	return nil
}

func (c *apiClient) GetChannel(channelID string) (*model.Channel, error) {
	channel, resp, err := c.api.GetChannel(context.Background(), channelID)
	if err != nil {
		return nil, wrapAPIError(resp, err, fmt.Sprintf("channel %s", channelID))
	}
	return channel, nil
}

func (c *apiClient) GetConfig() (*model.Config, error) {
	config, resp, err := c.api.GetConfig(context.Background())
	if err != nil {
		return nil, wrapAPIError(resp, err, "server configuration")
	}
	return config, nil
}

// wrapAPIError translates HTTP status codes into user-facing error messages.
func wrapAPIError(resp *model.Response, err error, ctx string) *ExitError {
	if resp == nil {
		return apiError(fmt.Sprintf("error: unable to connect to the server while accessing %s. Check the URL and network connectivity.", ctx), err)
	}
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return newExitError(ExitConfigError, "error: authentication failed. Check your token or credentials.", err)
	case http.StatusForbidden:
		return newExitError(ExitConfigError, "error: permission denied. This operation requires a System Administrator account.", err)
	case http.StatusNotFound:
		return newExitError(ExitConfigError, fmt.Sprintf("error: %s not found. Please check the name and try again.", ctx), err)
	default:
		if resp.StatusCode >= 500 {
			return apiError(fmt.Sprintf("error: the Mattermost server returned an unexpected error (HTTP %d). Check server logs for details.", resp.StatusCode), err)
		}
		return apiError(fmt.Sprintf("error: unexpected API response (HTTP %d) while accessing %s.", resp.StatusCode, ctx), err)
	}
}

// AuthConfig holds the resolved authentication parameters.
type AuthConfig struct {
	ServerURL string
	Token     string
	Username  string
	Password  string
}

// resolveAuth determines which authentication method to use based on flags and
// environment variables. It returns an AuthConfig or an error if nothing is provided.
func resolveAuth(urlFlag, tokenFlag, usernameFlag string) (*AuthConfig, error) {
	serverURL := firstNonEmpty(urlFlag, os.Getenv("MM_URL"))
	if serverURL == "" {
		return nil, configError("error: server URL is required. Use --url or set the MM_URL environment variable.")
	}
	serverURL = strings.TrimRight(serverURL, "/")

	token := firstNonEmpty(tokenFlag, os.Getenv("MM_TOKEN"))
	if token != "" {
		return &AuthConfig{ServerURL: serverURL, Token: token}, nil
	}

	username := firstNonEmpty(usernameFlag, os.Getenv("MM_USERNAME"))
	if username != "" {
		password, err := obtainPassword()
		if err != nil {
			return nil, configError(fmt.Sprintf("error: failed to obtain password: %v", err))
		}
		return &AuthConfig{ServerURL: serverURL, Username: username, Password: password}, nil
	}

	return nil, configError("error: authentication required. Use --token (or MM_TOKEN) for token auth, or --username (or MM_USERNAME) for password auth.")
}

// obtainPassword gets the password from MM_PASSWORD env var or an interactive prompt.
// Priority: MM_PASSWORD first (for automation), then interactive prompt if stdin is a TTY.
func obtainPassword() (string, error) {
	if password := os.Getenv("MM_PASSWORD"); password != "" {
		return password, nil
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", fmt.Errorf("no password available: stdin is not a terminal and MM_PASSWORD is not set")
	}

	fmt.Print("Password: ")
	passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("reading password: %w", err)
	}
	return string(passwordBytes), nil
}

// newClient creates a MattermostClient from the resolved auth config.
func newClient(auth *AuthConfig) (MattermostClient, error) {
	api := model.NewAPIv4Client(auth.ServerURL)

	if auth.Token != "" {
		api.SetToken(auth.Token)
		return &apiClient{api: api}, nil
	}

	user, resp, err := api.Login(context.Background(), auth.Username, auth.Password)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return nil, configError("error: authentication failed. Check your token or credentials.")
		}
		return nil, apiError("error: unable to connect to the server. Check the URL and network connectivity.", err)
	}
	_ = user
	return &apiClient{api: api}, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
