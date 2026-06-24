package search

import (
	"testing"

	"opcua-studio/internal/opcua"
)

func TestEmptyQueryReturnsNoResults(t *testing.T) {
	service := NewService()
	service.AddNodes([]opcua.AddressNode{{NodeID: "ns=2;s=PumpA", DisplayName: "Pump A", BrowseName: "2:PumpA", NodeClass: "Variable"}})

	view := service.Search("   ")

	if len(view.Results) != 0 {
		t.Fatalf("expected no results for empty query, got %d", len(view.Results))
	}
}

func TestDeduplicatesNodeIDs(t *testing.T) {
	service := NewService()
	service.AddNodes([]opcua.AddressNode{
		{NodeID: "ns=2;s=PumpA", DisplayName: "Pump A", BrowseName: "2:PumpA", NodeClass: "Variable"},
		{NodeID: "ns=2;s=PumpA", DisplayName: "Pump A Duplicate", BrowseName: "2:PumpADuplicate", NodeClass: "Variable"},
	})

	view := service.Search("Pump")

	if len(view.Results) != 1 {
		t.Fatalf("expected duplicate NodeID once, got %d", len(view.Results))
	}
	if view.Results[0].Node.NodeID != "ns=2;s=PumpA" {
		t.Fatalf("expected PumpA result, got %q", view.Results[0].Node.NodeID)
	}
}

func TestExactDisplayNameRanksAboveNodeIDSubstring(t *testing.T) {
	service := NewService()
	service.AddNodes([]opcua.AddressNode{
		{NodeID: "ns=2;s=Pump", DisplayName: "Compressor", BrowseName: "2:Compressor", NodeClass: "Variable"},
		{NodeID: "ns=2;s=Exact", DisplayName: "Pump", BrowseName: "2:Exact", NodeClass: "Variable"},
	})

	view := service.Search("Pump")

	if len(view.Results) < 2 {
		t.Fatalf("expected two results, got %d", len(view.Results))
	}
	if got := view.Results[0].Node.DisplayName; got != "Pump" {
		t.Fatalf("expected exact DisplayName first, got %q", got)
	}
	if view.Results[0].MatchKind != string(MatchDisplayName) {
		t.Fatalf("expected DisplayName match, got %q", view.Results[0].MatchKind)
	}
}

func TestBrowseNameMatchWorks(t *testing.T) {
	service := NewService()
	service.AddNodes([]opcua.AddressNode{{NodeID: "ns=2;s=Motor1", DisplayName: "Main Motor", BrowseName: "2:DriveMotor", NodeClass: "Variable"}})

	view := service.Search("DriveMotor")

	if len(view.Results) != 1 {
		t.Fatalf("expected one BrowseName result, got %d", len(view.Results))
	}
	if view.Results[0].MatchKind != string(MatchBrowseName) {
		t.Fatalf("expected BrowseName match, got %q", view.Results[0].MatchKind)
	}
}

func TestNodeClassMatchWorks(t *testing.T) {
	service := NewService()
	service.AddNodes([]opcua.AddressNode{{NodeID: "ns=2;s=Area1", DisplayName: "Area 1", BrowseName: "2:Area1", NodeClass: "Object"}})

	view := service.Search("Object")

	if len(view.Results) != 1 {
		t.Fatalf("expected one NodeClass result, got %d", len(view.Results))
	}
	if view.Results[0].MatchKind != string(MatchNodeClass) {
		t.Fatalf("expected NodeClass match, got %q", view.Results[0].MatchKind)
	}
}

func TestResetClearsResults(t *testing.T) {
	service := NewService()
	service.AddNodes([]opcua.AddressNode{{NodeID: "ns=2;s=PumpA", DisplayName: "Pump A", BrowseName: "2:PumpA", NodeClass: "Variable"}})
	service.Reset()

	view := service.Search("Pump")

	if len(view.Results) != 0 {
		t.Fatalf("expected no results after reset, got %d", len(view.Results))
	}
}
