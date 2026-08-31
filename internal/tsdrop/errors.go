package tsdrop

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrDaemonDown          = errors.New("tailscale daemon is not running")
	ErrNeedsLogin          = errors.New("tailscale needs login")
	ErrFileSharingDisabled = errors.New("file sharing is not enabled on this tailnet")
	ErrOtherUser           = errors.New("target is owned by a different user")
	ErrTagged              = errors.New("target is a tagged node")
	ErrOffline             = errors.New("target is offline")
	ErrNoPeerAPI           = errors.New("target has no PeerAPI")
	ErrPermission          = errors.New("permission denied talking to tailscale")
	ErrNotFound            = errors.New("no such Taildrop target")
)

// ExitCode maps an error to the CLI process status.
// 0 ok, 2 daemon/login, 3 policy/target, 4 IO/other.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	switch {
	case errors.Is(err, ErrDaemonDown), errors.Is(err, ErrNeedsLogin):
		return 2
	case errors.Is(err, ErrFileSharingDisabled), errors.Is(err, ErrOtherUser),
		errors.Is(err, ErrTagged), errors.Is(err, ErrOffline),
		errors.Is(err, ErrNoPeerAPI), errors.Is(err, ErrNotFound),
		errors.Is(err, ErrPermission):
		return 3
	default:
		return 4
	}
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	s := strings.ToLower(err.Error())
	switch {
	case strings.Contains(s, "failed to connect"),
		strings.Contains(s, "connection refused"),
		strings.Contains(s, "no such file or directory"),
		strings.Contains(s, "cannot find the path"):
		return fmt.Errorf("%w: %v", ErrDaemonDown, err)
	case strings.Contains(s, "file sharing not enabled"):
		return fmt.Errorf("%w: %v", ErrFileSharingDisabled, err)
	case strings.Contains(s, "owned by different user"),
		strings.Contains(s, "owned by another user"):
		return fmt.Errorf("%w: %v", ErrOtherUser, err)
	case strings.Contains(s, "tagged"):
		return fmt.Errorf("%w: %v", ErrTagged, err)
	case strings.Contains(s, "offline"):
		return fmt.Errorf("%w: %v", ErrOffline, err)
	case strings.Contains(s, "access denied"):
		return fmt.Errorf("%w: %v", ErrPermission, err)
	default:
		return err
	}
}
