package search

import (
	"context"
	"errors"
	"strings"
	"testing"

	"opcua-studio/internal/opcua"
)

type browseOnlyAdapter struct {
	children map[string][]opcua.AddressNode
}

func (b *browseOnlyAdapter) BrowseChildren(_ context.Context, nodeID string) ([]opcua.AddressNode, error) {
	return b.children[nodeID], nil
}

type cancelAwareBrowser struct {
	started chan struct{}
}

func (b *cancelAwareBrowser) BrowseChildren(ctx context.Context, _ string) ([]opcua.AddressNode, error) {
	close(b.started)
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestSessionBrowseChildrenIngestsMetadataForSearch(t *testing.T) {
	browser := &browseOnlyAdapter{children: map[string][]opcua.AddressNode{
		"ns=2;s=Area": {{NodeID: "ns=2;s=Pump", DisplayName: "Feed Pump", BrowseName: "2:FeedPump", NodeClass: "Variable"}},
	}}
	session := NewSession(context.Background(), browser)
	defer session.Stop()

	children, err := session.BrowseChildren("ns=2;s=Area")
	if err != nil {
		t.Fatalf("BrowseChildren() error = %v", err)
	}
	if len(children) != 1 || children[0].NodeID != "ns=2;s=Pump" {
		t.Fatalf("BrowseChildren() = %#v, want Feed Pump", children)
	}

	view := session.Search("Feed Pump")
	if len(view.Results) != 1 || view.Results[0].Score != 500 {
		t.Fatalf("Search() = %#v, want exact explicitly browsed result", view)
	}
	if !strings.Contains(view.Status, "browsed and shallow-indexed Address Space metadata") {
		t.Fatalf("Search() status = %q, want hybrid metadata status", view.Status)
	}
}

func TestSessionOwnsShallowIndexingStatus(t *testing.T) {
	session := NewSession(context.Background(), &browseOnlyAdapter{})
	defer session.Stop()

	active := session.Search("Missing")
	if !strings.Contains(active.Status, "indexed coverage is still expanding") {
		t.Fatalf("active Search() status = %q", active.Status)
	}

	session.FinishShallowIndexing(true)
	exhausted := session.Search("Missing")
	if !strings.Contains(exhausted.Status, "some Address Space areas may not be indexed") {
		t.Fatalf("exhausted Search() status = %q", exhausted.Status)
	}
}

func TestStoppedSessionRejectsBrowsing(t *testing.T) {
	session := NewSession(context.Background(), &browseOnlyAdapter{})
	session.Stop()

	_, err := session.BrowseChildren("i=85")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("BrowseChildren() error = %v, want context canceled", err)
	}
}

func TestStoppingSessionCancelsActiveBrowse(t *testing.T) {
	browser := &cancelAwareBrowser{started: make(chan struct{})}
	session := NewSession(context.Background(), browser)
	browseDone := make(chan error, 1)
	go func() {
		_, err := session.BrowseChildren("i=85")
		browseDone <- err
	}()
	<-browser.started

	session.Stop()

	if err := <-browseDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("active BrowseChildren() error = %v, want context canceled", err)
	}
}
