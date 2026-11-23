package netd

import (
	"cmp"
	"net/netip"
	"slices"
)

type ActionKind int

const (
	ActionStop ActionKind = iota
	ActionStart
	ActionSetPolicy
	ActionDeletePeer
	ActionAddPeer
	ActionDeletePath
	ActionAddPath
)

func (k ActionKind) String() string {
	switch k {
	case ActionStop:
		return "stop"
	case ActionStart:
		return "start"
	case ActionSetPolicy:
		return "set-policy"
	case ActionDeletePeer:
		return "delete-peer"
	case ActionAddPeer:
		return "add-peer"
	case ActionDeletePath:
		return "delete-path"
	case ActionAddPath:
		return "add-path"
	}
	return "unknown"
}

type Action struct {
	Kind   ActionKind
	Peer   Peer
	Prefix netip.Prefix
}

func (a Action) String() string {
	switch a.Kind {
	case ActionDeletePeer, ActionAddPeer:
		return a.Kind.String() + " " + a.Peer.Address.String()
	case ActionDeletePath, ActionAddPath:
		return a.Kind.String() + " " + a.Prefix.String()
	}
	return a.Kind.String()
}

func Reconcile(d Desired, o Observed) []Action {
	var actions []Action
	if o.Started && (o.ASN != d.ASN || o.RouterID != d.RouterID) {
		actions = append(actions, Action{Kind: ActionStop})
		o = Observed{}
	}
	if !o.Started {
		actions = append(actions, Action{Kind: ActionStart})
		o = Observed{}
	}
	if !o.PolicyOK {
		actions = append(actions, Action{Kind: ActionSetPolicy})
	}

	wantPeers := make(map[netip.Addr]Peer, len(d.Peers))
	for _, p := range d.Peers {
		wantPeers[p.Address] = p
	}
	havePeers := make(map[netip.Addr]Peer, len(o.Peers))
	for _, p := range sortedPeers(o.Peers) {
		havePeers[p.Address] = p
		if w, ok := wantPeers[p.Address]; !ok || !samePeer(w, p) {
			actions = append(actions, Action{Kind: ActionDeletePeer, Peer: p})
		}
	}
	for _, p := range sortedPeers(d.Peers) {
		if h, ok := havePeers[p.Address]; !ok || !samePeer(p, h) {
			actions = append(actions, Action{Kind: ActionAddPeer, Peer: p})
		}
	}

	wantPaths := make(map[netip.Prefix]bool, len(d.Paths))
	for _, p := range d.Paths {
		wantPaths[p] = true
	}
	havePaths := make(map[netip.Prefix]bool, len(o.Paths))
	for _, p := range sortedPrefixes(o.Paths) {
		havePaths[p] = true
		if !wantPaths[p] {
			actions = append(actions, Action{Kind: ActionDeletePath, Prefix: p})
		}
	}
	for _, p := range sortedPrefixes(d.Paths) {
		if !havePaths[p] {
			actions = append(actions, Action{Kind: ActionAddPath, Prefix: p})
		}
	}
	return actions
}

func samePeer(a, b Peer) bool {
	return a.Address == b.Address && a.ASN == b.ASN && a.Multihop == b.Multihop && a.Local == b.Local
}

func sortedPeers(in []Peer) []Peer {
	out := slices.Clone(in)
	slices.SortFunc(out, func(a, b Peer) int {
		return a.Address.Compare(b.Address)
	})
	return out
}

func sortedPrefixes(in []netip.Prefix) []netip.Prefix {
	out := slices.Clone(in)
	slices.SortFunc(out, func(a, b netip.Prefix) int {
		if c := a.Addr().Compare(b.Addr()); c != 0 {
			return c
		}
		return cmp.Compare(a.Bits(), b.Bits())
	})
	return out
}
