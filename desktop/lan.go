package main

import (
	"context"
	"sync/atomic"

	"github.com/sheneyan/tailsend/internal/localsend"
	"github.com/sheneyan/tailsend/internal/tsdrop"
)

func (a *App) startLAN() {
	cfgDir, err := localsend.ConfigDir()
	if err != nil {
		return
	}
	node, err := localsend.Start(a.requestCtx(), localsend.Config{
		SaveDir:   a.DefaultRecvDir(),
		ConfigDir: cfgDir,
		OnPeers: func(peers []localsend.Peer) {
			a.emit("lan-peers", peers)
		},
		OnOffer: func(in localsend.Incoming) {
			a.emit("lan-offer", in)
		},
		OnSaved: func(alias string, count int) {
			a.emit("lan-saved", map[string]any{"alias": alias, "count": count})
		},
	})
	if err != nil {
		a.emit("lan-status", err.Error())
		return
	}
	a.lan = node
}

func (a *App) shutdownLAN(context.Context) {
	if a.lan != nil {
		a.lan.Close()
	}
}

func (a *App) LanState() localsend.State {
	if a.lan == nil {
		return localsend.State{Peers: []localsend.Peer{}}
	}
	return a.lan.State()
}

func (a *App) LanSetAutoSave(v bool) error {
	if a.lan == nil {
		return nil
	}
	return a.lan.SetAutoSave(v)
}

func (a *App) LanAccept(sessionID string) {
	if a.lan != nil {
		a.lan.Accept(sessionID)
	}
}

func (a *App) LanDecline(sessionID string) {
	if a.lan != nil {
		a.lan.Decline(sessionID)
	}
}

func (a *App) SendLan(fingerprint string, paths []string, pin string) error {
	if a.lan == nil {
		return errNoLAN
	}
	if !a.beginSend() {
		return errBusySend
	}
	defer a.endSend()
	var ask func() (string, error)
	if pin != "" {
		ask = func() (string, error) { return pin, nil }
	}
	return a.lan.Send(a.requestCtx(), fingerprint, paths, ask, func(name string, sent, total int64, state, errText string) {
		a.emit("progress", tsdrop.Progress{Name: name, Sent: sent, Total: total, State: state, Err: errText})
	})
}

var (
	errNoLAN    = errStr("LocalSend is not running")
	errBusySend = errStr("a send is already running")
)

type errStr string

func (e errStr) Error() string { return string(e) }

func (a *App) beginSend() bool {
	return atomic.CompareAndSwapInt32(&a.outbound, 0, 1)
}

func (a *App) endSend() {
	atomic.StoreInt32(&a.outbound, 0)
}
