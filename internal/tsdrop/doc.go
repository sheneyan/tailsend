// Package tsdrop talks to Tailscale Taildrop through the local tailscaled API.
//
// It does not embed a Tailscale node. Pass a *local.Client into New (or nil
// to use the platform default socket). Tests inject a fake via tsdroptest.
package tsdrop
