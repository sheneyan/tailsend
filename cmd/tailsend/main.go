package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/sheneyan/tailsend/internal/tsdrop"
)

const usage = `tailsend — send files over Tailscale Taildrop

Requires the official Tailscale client to be installed and logged in.
Enable Send Files in the tailnet admin console. Tagged nodes cannot
receive Taildrop unless ACL grants allow it.

Usage:
  tailsend status
  tailsend list [--json]
  tailsend send <file-or-dir...> <device>:
  tailsend inbox
  tailsend recv [dir] [--watch] [--conflict=skip|overwrite|rename]

The send destination must end with a colon (like scp / tailscale file cp).
Device names come from 'tailsend list'; you do not need an IP address.
`

func main() {
	os.Exit(run(os.Args[1:], tsdrop.New(nil), os.Stdout, os.Stderr))
}

func run(args []string, c *tsdrop.Client, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprint(stderr, usage)
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	ctx := context.Background()
	switch args[0] {
	case "status":
		return cmdStatus(ctx, c, stdout, stderr)
	case "list":
		return cmdList(ctx, c, args[1:], stdout, stderr)
	case "send":
		return cmdSend(ctx, c, args[1:], stdout, stderr)
	case "inbox":
		return cmdInbox(ctx, c, stdout, stderr)
	case "recv", "get":
		return cmdRecv(ctx, c, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

func cmdStatus(ctx context.Context, c *tsdrop.Client, stdout, stderr io.Writer) int {
	st, self, err := c.Probe(ctx)
	if err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintf(stdout, "State    %s\n", st)
	if self.Hostname != "" {
		fmt.Fprintf(stdout, "Self     %s\n", self.Hostname)
		if self.DNSName != "" {
			fmt.Fprintf(stdout, "DNS      %s\n", self.DNSName)
		}
		fmt.Fprintf(stdout, "OS       %s\n", self.OS)
		if len(self.IPs) > 0 {
			var ips []string
			for _, ip := range self.IPs {
				ips = append(ips, ip.String())
			}
			fmt.Fprintf(stdout, "Address  %s\n", strings.Join(ips, " "))
		}
	}
	if st != tsdrop.Running {
		fmt.Fprintf(stderr, "Tailscale is not Running. Open the Tailscale app and sign in.\n")
		if st == tsdrop.NeedsLogin {
			return tsdrop.ExitCode(tsdrop.ErrNeedsLogin)
		}
	}
	return 0
}

func cmdList(ctx context.Context, c *tsdrop.Client, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "print sendable targets as JSON (mobile pairing)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *asJSON {
		b, err := c.ExportTargetsJSON(ctx)
		if err != nil {
			return fail(stderr, err)
		}
		fmt.Fprintln(stdout, string(b))
		return 0
	}
	targets, err := c.Targets(ctx)
	if err != nil {
		return fail(stderr, err)
	}
	if len(targets) == 0 {
		fmt.Fprintln(stdout, "No devices visible. Is Tailscale Running?")
		return 0
	}
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tOS\tONLINE\tTAILDROP")
	for _, t := range targets {
		online := "no"
		if t.Online {
			online = "yes"
		}
		taildrop := "yes"
		if t.Reason != "" {
			taildrop = t.Reason
		}
		name := t.Hostname
		if name == "" {
			name = t.DNSName
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", name, t.OS, online, taildrop)
	}
	tw.Flush()
	return 0
}

func cmdSend(ctx context.Context, c *tsdrop.Client, args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintf(stderr, "usage: tailsend send <file-or-dir...> <device>:\n")
		return 2
	}
	dest := args[len(args)-1]
	files := args[:len(args)-1]
	if !strings.HasSuffix(dest, ":") {
		fmt.Fprintf(stderr, "destination %q must end with ':' (example: pixel:)\n", dest)
		return 2
	}
	target, err := c.Lookup(ctx, dest)
	if err != nil {
		return fail(stderr, err)
	}
	var items []tsdrop.SendItem
	for _, f := range files {
		items = append(items, tsdrop.SendItem{Path: f})
	}
	err = c.Send(ctx, target.StableID, items, func(p tsdrop.Progress) {
		switch p.State {
		case "done":
			if p.Total > 0 {
				fmt.Fprintf(stderr, "%s  %s\n", p.Name, iec(p.Sent))
			} else {
				fmt.Fprintf(stderr, "%s  done\n", p.Name)
			}
		case "failed":
			fmt.Fprintf(stderr, "%s  failed: %s\n", p.Name, p.Err)
		}
	})
	if err != nil {
		fmt.Fprintln(stderr)
		return fail(stderr, err)
	}
	fmt.Fprintf(stdout, "sent %d item(s) to %s\n", len(items), target.Hostname)
	return 0
}

func cmdInbox(ctx context.Context, c *tsdrop.Client, stdout, stderr io.Writer) int {
	files, err := c.Inbox(ctx)
	if err != nil {
		return fail(stderr, err)
	}
	if len(files) == 0 {
		fmt.Fprintln(stdout, "inbox empty")
		return 0
	}
	for _, f := range files {
		fmt.Fprintf(stdout, "%s\t%s\n", f.Name, iec(f.Size))
	}
	return 0
}

func cmdRecv(ctx context.Context, c *tsdrop.Client, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("recv", flag.ContinueOnError)
	fs.SetOutput(stderr)
	watch := fs.Bool("watch", false, "wait for files and receive them as they arrive")
	conflictStr := fs.String("conflict", "rename", "skip, overwrite, or rename")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	conflict, err := tsdrop.ParseConflict(*conflictStr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	dir := "."
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
	}
	if *watch {
		return recvWatch(ctx, c, dir, conflict, stdout, stderr)
	}
	written, err := c.Receive(ctx, dir, conflict)
	for _, p := range written {
		fmt.Fprintln(stdout, p)
	}
	if err != nil {
		return fail(stderr, err)
	}
	if len(written) == 0 {
		fmt.Fprintln(stdout, "inbox empty")
	}
	return 0
}

func recvWatch(ctx context.Context, c *tsdrop.Client, dir string, conflict tsdrop.Conflict, stdout, stderr io.Writer) int {
	ch, err := c.WatchInbox(ctx)
	if err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintf(stderr, "watching inbox → %s\n", dir)
	for range ch {
		written, err := c.Receive(ctx, dir, conflict)
		for _, p := range written {
			fmt.Fprintln(stdout, p)
		}
		if err != nil {
			fmt.Fprintln(stderr, err)
		}
	}
	return 0
}

func fail(stderr io.Writer, err error) int {
	fmt.Fprintln(stderr, err)
	return tsdrop.ExitCode(err)
}

func iec(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
