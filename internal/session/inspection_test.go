package session

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"opcua-studio/internal/opcua"
)

func variable(nodeID string) opcua.AddressNode {
	return opcua.AddressNode{NodeID: nodeID, DisplayName: nodeID, NodeClass: "Variable"}
}

func TestSelectVariableRequestsSubscribeAndDetails(t *testing.T) {
	set := NewInspectionSet()
	requests := set.Select(variable("ns=2;s=Level"))

	assertKinds(t, requests, RequestSubscribeValue, RequestReadDetails)
	selected, ok := set.Selected()
	if !ok || !selected.Subscribing || !selected.LoadingDetails {
		t.Fatalf("selected inspection = %#v, ok=%t", selected, ok)
	}
}

func TestWatchReusesSelectedInspection(t *testing.T) {
	set := NewInspectionSet()
	set.Select(variable("ns=2;s=Level"))

	requests := set.Watch(variable("ns=2;s=Level"))
	if len(requests) != 0 {
		t.Fatalf("expected no duplicate requests, got %#v", requests)
	}
	selected, _ := set.Selected()
	if !selected.Watched {
		t.Fatalf("expected selected inspection promoted to Watchlist: %#v", selected)
	}
}

func TestSelectedAndWatchedShareStaleValueState(t *testing.T) {
	set := NewInspectionSet()
	set.Select(variable("ns=2;s=Level"))
	set.Watch(variable("ns=2;s=Level"))
	set.ApplyLiveValue("ns=2;s=Level", opcua.LiveValue{}, errors.New("subscription closed"))

	selected, _ := set.Selected()
	watched := set.Watched()
	if !selected.Stale || len(watched) != 1 || !watched[0].Stale {
		t.Fatalf("selected=%#v watched=%#v", selected, watched)
	}
}

func TestSessionTrendIgnoresInspectedOnlyLiveValueUpdates(t *testing.T) {
	set := NewInspectionSet()
	set.Select(variable("ns=2;s=Level"))

	set.ApplyLiveValue("ns=2;s=Level", opcua.LiveValue{NodeID: "ns=2;s=Level", Value: "10", Status: "Good"}, nil)

	trend := set.SessionTrend("ns=2;s=Level")
	if len(trend.Nodes) != 0 || len(trend.Points) != 0 {
		t.Fatalf("inspected-only node should not appear in Session Trend: %#v", trend)
	}
}

func TestSessionTrendRecordsWatchedLiveValueUpdates(t *testing.T) {
	set := NewInspectionSet()
	set.Watch(variable("ns=2;s=Level"))
	first := opcua.LiveValue{NodeID: "ns=2;s=Level", Value: "10", Status: "Good"}
	second := opcua.LiveValue{NodeID: "ns=2;s=Level", Value: "11", Status: "Good"}

	set.ApplyLiveValue("ns=2;s=Level", first, nil)
	set.ApplyLiveValue("ns=2;s=Level", second, nil)

	trend := set.SessionTrend("ns=2;s=Level")
	if len(trend.Nodes) != 1 {
		t.Fatalf("observed nodes = %d, want 1: %#v", len(trend.Nodes), trend.Nodes)
	}
	if trend.Nodes[0].Node.NodeID != "ns=2;s=Level" || trend.Nodes[0].PointCount != 2 || trend.Nodes[0].LatestValue != "11" {
		t.Fatalf("observed node summary = %#v", trend.Nodes[0])
	}
	if len(trend.Points) != 2 {
		t.Fatalf("trend points = %d, want 2: %#v", len(trend.Points), trend.Points)
	}
	if trend.Points[0].Value != "11" || trend.Points[1].Value != "10" {
		t.Fatalf("trend points should be newest-first: %#v", trend.Points)
	}
}

func TestSessionTrendUsesSourceTimestampAsDisplayTime(t *testing.T) {
	set := NewInspectionSet()
	set.Watch(variable("ns=2;s=Level"))
	sourceTime := time.Date(2026, 6, 24, 10, 30, 0, 0, time.UTC)
	serverTime := time.Date(2026, 6, 24, 10, 31, 0, 0, time.UTC)

	set.ApplyLiveValue("ns=2;s=Level", opcua.LiveValue{NodeID: "ns=2;s=Level", Value: "10", SourceTimestamp: sourceTime, ServerTimestamp: serverTime}, nil)

	trend := set.SessionTrend("ns=2;s=Level")
	if trend.Points[0].Timestamp != sourceTime.Format(time.RFC3339Nano) {
		t.Fatalf("display timestamp = %q, want source timestamp", trend.Points[0].Timestamp)
	}
}

func TestSessionTrendKeepsObservedNodeAfterSubscriptionStops(t *testing.T) {
	set := NewInspectionSet()
	set.Watch(variable("ns=2;s=Level"))
	set.ApplyLiveValue("ns=2;s=Level", opcua.LiveValue{NodeID: "ns=2;s=Level", Value: "10", Status: "Good"}, nil)
	set.Unwatch("ns=2;s=Level")

	trend := set.SessionTrend("ns=2;s=Level")
	if len(trend.Nodes) != 1 || len(trend.Points) != 1 {
		t.Fatalf("expected observed node history to remain visible, got %#v", trend)
	}
}

func TestSessionTrendRetainsLatestFiveHundredUpdates(t *testing.T) {
	set := NewInspectionSet()
	set.Watch(variable("ns=2;s=Level"))

	for update := 1; update <= 501; update++ {
		set.ApplyLiveValue("ns=2;s=Level", opcua.LiveValue{NodeID: "ns=2;s=Level", Value: fmt.Sprint(update), Status: "Good"}, nil)
	}

	trend := set.SessionTrend("ns=2;s=Level")
	if len(trend.Points) != 500 {
		t.Fatalf("trend points = %d, want 500", len(trend.Points))
	}
	if trend.Points[0].Value != "501" || trend.Points[499].Value != "2" {
		t.Fatalf("expected latest 500 points newest-first, got first=%q last=%q", trend.Points[0].Value, trend.Points[499].Value)
	}
}

func TestOutOfRangeComputedFromLiveValueAndRange(t *testing.T) {
	set := NewInspectionSet()
	set.Select(variable("ns=2;s=Level"))
	set.ApplyDetails("ns=2;s=Level", opcua.NodeDetails{NodeID: "ns=2;s=Level", EURange: &opcua.ValueRange{Low: 0, High: 100}}, nil)
	set.ApplyLiveValue("ns=2;s=Level", opcua.LiveValue{NodeID: "ns=2;s=Level", Value: "120"}, nil)

	selected, _ := set.Selected()
	if selected.OutOfRange != "120 is above 100" {
		t.Fatalf("OutOfRange = %q", selected.OutOfRange)
	}
}

func TestWatchedInspectionRemainsAliveWhenUnselected(t *testing.T) {
	set := NewInspectionSet()
	set.Select(variable("ns=2;s=Level"))
	set.Watch(variable("ns=2;s=Level"))

	requests := set.Unselect()
	if len(requests) != 0 {
		t.Fatalf("expected no cancellation, got %#v", requests)
	}
	if len(set.Watched()) != 1 {
		t.Fatalf("expected watched inspection to remain alive")
	}
}

func TestUnwatchedUnselectedInspectionRequestsCancellation(t *testing.T) {
	set := NewInspectionSet()
	set.Watch(variable("ns=2;s=Level"))
	sub := &fakeSubscription{}
	set.ApplySubscription("ns=2;s=Level", make(chan opcua.LiveValue), sub, nil)

	requests := set.Unwatch("ns=2;s=Level")
	assertKinds(t, requests, RequestCancelSubscription)
	if requests[0].Subscription != sub {
		t.Fatalf("cancelled subscription = %#v", requests[0].Subscription)
	}
}

func assertKinds(t *testing.T, requests []Request, kinds ...RequestKind) {
	t.Helper()
	if len(requests) != len(kinds) {
		t.Fatalf("request count = %d, want %d: %#v", len(requests), len(kinds), requests)
	}
	for i, kind := range kinds {
		if requests[i].Kind != kind {
			t.Fatalf("request[%d] = %v, want %v", i, requests[i].Kind, kind)
		}
	}
}

type fakeSubscription struct{}

func (f *fakeSubscription) Cancel(ctx context.Context) error { return nil }
