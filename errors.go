package main

import "fmt"

// Exit codes — consistent with the Mattermost Admin Utilities family.
const (
	ExitSuccess        = 0 // Success (or dry run completed successfully)
	ExitConfigError    = 1 // Missing flags, invalid input, auth failure, not found, guest accounts disabled
	ExitAPIError       = 2 // Connection failure, unexpected API response
	ExitPartialFailure = 3 // Operation completed but one or more channel removals failed
	ExitOutputError    = 4 // Unable to write output file
)

// ExitError carries an exit code alongside the error so main.go can extract it
// without business logic calling os.Exit directly.
type ExitError struct {
	Code    int
	Message string
	Err     error
}

func (e *ExitError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *ExitError) Unwrap() error {
	return e.Err
}

func newExitError(code int, msg string, err error) *ExitError {
	return &ExitError{Code: code, Message: msg, Err: err}
}

func configError(msg string) *ExitError {
	return &ExitError{Code: ExitConfigError, Message: msg}
}

func apiError(msg string, err error) *ExitError {
	return &ExitError{Code: ExitAPIError, Message: msg, Err: err}
}
