package search

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"opcua-studio/internal/opcua"
)

type browseOnlyAdapter struct {
	children map[string][]opcua.AddressNode
}

func (b *browseOnlyAdapter) BrowseChildren(_ context.Context, nodeID string) ([]opcua.AddressNode, error) {
	return b.children[nodeID], nil
}

type recordingBrowser struct {
	mu       sync.Mutex
	children map[string][]opcua.AddressNode
	requests []string
	times    []time.Time
}

func (b *recordingBrowser) BrowseChildren(_ context.Context, nodeID string) ([]opcua.AddressNode, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.requests = append(b.requests, nodeID)
	b.times = append(b.times, time.Now())
	return b.children[nodeID], nil
}

func (b *recordingBrowser) browseRequests() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.requests...)
}

type cancelAwareBrowser struct {
	started chan struct{}
	once    sync.Once
}

func (b *cancelAwareBrowser) BrowseChildren(ctx context.Context, _ string) ([]opcua.AddressNode, error) {
	b.once.Do(func() { close(b.started) })
	<-ctx.Done()
	return nil, ctx.Err()
}

type failingBrowser struct {
	err error
}

func (b *failingBrowser) BrowseChildren(_ context.Context, _ string) ([]opcua.AddressNode, error) {
	return nil, b.err
}

func TestSessionEmitsBackgroundBrowseFailure(t *testing.T) {
	browserErr := errors.New("access denied")
	session := NewSession(context.Background(), &failingBrowser{err: browserErr})
	defer session.Stop()

	var events <-chan Event = session.Events()
	select {
	case event := <-events:
		failure, ok := event.(BackgroundBrowseFailed)
		if !ok {
			t.Fatalf("event = %#v, want BackgroundBrowseFailed", event)
		}
		if failure.NodeID != objectsRootNodeID || !errors.Is(failure.Err, browserErr) {
			t.Fatalf("BackgroundBrowseFailed = %#v, want Objects node and access denied", failure)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for background browse failure event")
	}
}

func TestSessionEmitsIndexingBudgetExhausted(t *testing.T) {
	browser := &recordingBrowser{children: map[string][]opcua.AddressNode{
		objectsRootNodeID: {{NodeID: "ns=2;s=Area", NodeClass: "Object"}},
	}}
	session := NewSession(context.Background(), browser, SessionOptions{BrowseBudget: 1})
	defer session.Stop()

	select {
	case event := <-session.Events():
		exhausted, ok := event.(IndexingBudgetExhausted)
		if !ok {
			t.Fatalf("event = %#v, want IndexingBudgetExhausted", event)
		}
		if exhausted.BrowseCount != 1 {
			t.Fatalf("IndexingBudgetExhausted.BrowseCount = %d, want 1", exhausted.BrowseCount)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for indexing budget exhausted event")
	}
}

type rootBlockingBrowser struct {
	started chan struct{}
	once    sync.Once
}

func (b *rootBlockingBrowser) BrowseChildren(ctx context.Context, nodeID string) ([]opcua.AddressNode, error) {
	if nodeID == objectsRootNodeID {
		b.once.Do(func() { close(b.started) })
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return []opcua.AddressNode{{NodeID: nodeID + "/child", NodeClass: "Object"}}, nil
}

func TestSessionEmitsPriorityQueueOverflow(t *testing.T) {
	browser := &rootBlockingBrowser{started: make(chan struct{})}
	session := NewSession(context.Background(), browser)
	defer session.Stop()
	<-browser.started

	for i := 0; i < 10_000; i++ {
		if _, err := session.BrowseChildren(fmt.Sprintf("ns=2;s=Area%d", i)); err != nil {
			t.Fatalf("BrowseChildren() error = %v", err)
		}
		select {
		case event := <-session.Events():
			overflow, ok := event.(PriorityQueueOverflow)
			if !ok {
				t.Fatalf("event = %#v, want PriorityQueueOverflow", event)
			}
			if overflow.DroppedParentCount != 1 {
				t.Fatalf("PriorityQueueOverflow.DroppedParentCount = %d, want 1", overflow.DroppedParentCount)
			}
			return
		default:
		}
	}
	t.Fatal("priority queue did not emit an overflow event")
}

func TestSessionStartsShallowAddressSpaceIndexing(t *testing.T) {
	browser := &recordingBrowser{children: map[string][]opcua.AddressNode{
		"i=85": {{NodeID: "ns=2;s=Pump", DisplayName: "Feed Pump", BrowseName: "2:FeedPump", NodeClass: "Variable"}},
	}}
	session := NewSession(context.Background(), browser, SessionOptions{BrowseInterval: time.Millisecond, BrowseBudget: 10})
	defer session.Stop()

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if results := session.Search("Feed Pump").Results; len(results) == 1 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("Search() = %#v; browse requests = %#v, want background-indexed Feed Pump", session.Search("Feed Pump"), browser.browseRequests())
}

func TestSessionOptionsControlBrowseRateAndBudget(t *testing.T) {
	browser := &recordingBrowser{children: map[string][]opcua.AddressNode{
		"i=85":         {{NodeID: "ns=2;s=Area1", NodeClass: "Object"}},
		"ns=2;s=Area1": {{NodeID: "ns=2;s=Area2", NodeClass: "Object"}},
		"ns=2;s=Area2": {{NodeID: "ns=2;s=Area3", NodeClass: "Object"}},
	}}
	interval := 20 * time.Millisecond
	session := NewSession(context.Background(), browser, SessionOptions{BrowseInterval: interval, BrowseBudget: 2})
	defer session.Stop()

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) && len(browser.browseRequests()) < 2 {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(2 * interval)
	browser.mu.Lock()
	requests := append([]string(nil), browser.requests...)
	times := append([]time.Time(nil), browser.times...)
	browser.mu.Unlock()
	if len(requests) != 2 {
		t.Fatalf("browse requests = %#v, want session budget of two", requests)
	}
	if elapsed := times[1].Sub(times[0]); elapsed < interval-5*time.Millisecond {
		t.Fatalf("browse interval = %s, want at least about %s", elapsed, interval)
	}
	if status := session.Search("Missing").Status; !strings.Contains(status, "some Address Space areas may not be indexed") {
		t.Fatalf("Search() status = %q, want exhausted coverage message", status)
	}
}

func TestSessionExplicitBrowsePrioritizesDiscoveredParents(t *testing.T) {
	browser := &recordingBrowser{children: map[string][]opcua.AddressNode{
		"i=85":              {{NodeID: "ns=2;s=Background", NodeClass: "Object"}},
		"ns=2;s=ManualArea": {{NodeID: "ns=2;s=ManualSkid", NodeClass: "Object"}},
		"ns=2;s=ManualSkid": nil,
		"ns=2;s=Background": nil,
	}}
	session := NewSession(context.Background(), browser, SessionOptions{BrowseInterval: 40 * time.Millisecond, BrowseBudget: 10})
	defer session.Stop()

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) && len(browser.browseRequests()) == 0 {
		time.Sleep(time.Millisecond)
	}
	if _, err := session.BrowseChildren("ns=2;s=ManualArea"); err != nil {
		t.Fatalf("BrowseChildren() error = %v", err)
	}

	deadline = time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		requests := browser.browseRequests()
		for i, nodeID := range requests {
			if nodeID == "ns=2;s=ManualArea" && i+1 < len(requests) {
				if requests[i+1] != "ns=2;s=ManualSkid" {
					t.Fatalf("browse requests = %#v, want discovered parent prioritized", requests)
				}
				return
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("browse requests = %#v, want prioritized ManualSkid", browser.browseRequests())
}

func TestSessionExplicitBrowsePromotesAlreadyQueuedParent(t *testing.T) {
	browser := &recordingBrowser{children: map[string][]opcua.AddressNode{
		objectsRootNodeID: {
			{NodeID: "ns=2;s=BackgroundFirst", NodeClass: "Object"},
			{NodeID: "ns=2;s=Promoted", NodeClass: "Object"},
		},
		"ns=2;s=ManualArea": {{NodeID: "ns=2;s=Promoted", NodeClass: "Object"}},
	}}
	session := NewSession(context.Background(), browser, SessionOptions{BrowseInterval: 40 * time.Millisecond, BrowseBudget: 10})
	defer session.Stop()

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) && len(browser.browseRequests()) == 0 {
		time.Sleep(time.Millisecond)
	}
	if _, err := session.BrowseChildren("ns=2;s=ManualArea"); err != nil {
		t.Fatalf("BrowseChildren() error = %v", err)
	}

	deadline = time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		requests := browser.browseRequests()
		if len(requests) >= 4 {
			if requests[2] != "ns=2;s=Promoted" || requests[3] != "ns=2;s=BackgroundFirst" {
				t.Fatalf("browse requests = %#v, want queued parent promoted ahead of BackgroundFirst", requests)
			}
			if got := countRequests(requests, "ns=2;s=Promoted"); got != 1 {
				t.Fatalf("browse requests = %#v, want promoted parent browsed once, got %d", requests, got)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("browse requests = %#v, want promoted parent browsed", browser.browseRequests())
}

func countRequests(requests []string, nodeID string) int {
	count := 0
	for _, request := range requests {
		if request == nodeID {
			count++
		}
	}
	return count
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

func TestSessionReportsActiveShallowIndexingStatus(t *testing.T) {
	browser := &cancelAwareBrowser{started: make(chan struct{})}
	session := NewSession(context.Background(), browser)
	defer session.Stop()
	<-browser.started

	active := session.Search("Missing")
	if !strings.Contains(active.Status, "indexed coverage is still expanding") {
		t.Fatalf("active Search() status = %q", active.Status)
	}
}

func TestSessionSearchIsSafeDuringBackgroundAndExplicitBrowsing(t *testing.T) {
	browser := &recordingBrowser{children: map[string][]opcua.AddressNode{
		"i=85":              {{NodeID: "ns=2;s=BackgroundPump", DisplayName: "Background Pump", NodeClass: "Variable"}},
		"ns=2;s=ManualArea": {{NodeID: "ns=2;s=ManualPump", DisplayName: "Manual Pump", NodeClass: "Variable"}},
	}}
	session := NewSession(context.Background(), browser, SessionOptions{BrowseInterval: time.Millisecond})
	defer session.Stop()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			session.Search("Pump")
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			_, _ = session.BrowseChildren("ns=2;s=ManualArea")
		}
	}()
	wg.Wait()

	if results := session.Search("Manual Pump").Results; len(results) != 1 || results[0].Node.NodeID != "ns=2;s=ManualPump" {
		t.Fatalf("Search() results = %#v, want explicitly browsed node", results)
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

func TestStoppingSessionIsIdempotentAndDiscardsLateBrowseResults(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	browser := &lateResultBrowser{started: started, release: release}
	session := NewSession(context.Background(), browser)
	<-started

	session.Stop()
	session.Stop()
	close(release)

	deadline := time.Now().Add(100 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(session.Search("Stale Pump").Results) != 0 {
			t.Fatalf("Search() contains late result after Stop: %#v", session.Search("Stale Pump"))
		}
		time.Sleep(time.Millisecond)
	}
}

type lateResultBrowser struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *lateResultBrowser) BrowseChildren(context.Context, string) ([]opcua.AddressNode, error) {
	b.once.Do(func() { close(b.started) })
	<-b.release
	return []opcua.AddressNode{{NodeID: "ns=2;s=StalePump", DisplayName: "Stale Pump", NodeClass: "Variable"}}, nil
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
