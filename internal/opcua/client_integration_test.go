package opcua

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestIntegrationBrowseObjectsRoot(t *testing.T) {
	endpoint := os.Getenv("TERMUA_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("set TERMUA_TEST_ENDPOINT to run OPC UA integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client := NewClient()
	endpoints, err := client.DiscoverEndpoints(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	if len(endpoints) == 0 {
		t.Fatal("expected at least one endpoint")
	}
	selected := endpoints[0]
	for _, candidate := range endpoints {
		if candidate.SecurityMode == "None" && candidate.SecurityPolicy == "None" {
			selected = candidate
			break
		}
	}
	if err := client.Connect(ctx, ConnectRequest{Endpoint: endpoint, SecurityMode: selected.SecurityMode, SecurityPolicy: selected.SecurityPolicy, AuthType: AuthAnonymous}); err != nil {
		t.Fatal(err)
	}
	defer client.Close(context.Background())

	children, err := client.BrowseChildren(ctx, "i=85")
	if err != nil {
		t.Fatal(err)
	}
	if len(children) == 0 {
		t.Fatal("expected Objects root to have children")
	}
}

func TestIntegrationReadMethodIOMetadata(t *testing.T) {
	endpoint := os.Getenv("TERMUA_METHOD_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("set TERMUA_METHOD_TEST_ENDPOINT to run Method metadata integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client := NewClient()
	endpoints, err := client.DiscoverEndpoints(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	if len(endpoints) == 0 {
		t.Fatal("expected at least one endpoint")
	}
	selected := endpoints[0]
	for _, candidate := range endpoints {
		if candidate.SecurityMode == "None" && candidate.SecurityPolicy == "None" {
			selected = candidate
			break
		}
	}
	if err := client.Connect(ctx, ConnectRequest{Endpoint: endpoint, SecurityMode: selected.SecurityMode, SecurityPolicy: selected.SecurityPolicy, AuthType: AuthAnonymous}); err != nil {
		t.Fatal(err)
	}
	defer client.Close(context.Background())

	const objectNodeID = "ns=3;s=Demo.CTT.Methods"
	const methodNodeID = "ns=3;s=Demo.CTT.Methods.MethodIO"
	children, err := client.BrowseChildren(ctx, objectNodeID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, child := range children {
		if child.NodeID == methodNodeID {
			found = true
			if child.ParentNodeID != objectNodeID || child.NodeClass != "Method" {
				t.Fatalf("MethodIO Address Node = %#v, want owning Object Node and Method class", child)
			}
		}
	}
	if !found {
		t.Fatalf("MethodIO not found below %s", objectNodeID)
	}

	details, err := client.ReadMethodDetails(ctx, objectNodeID, methodNodeID)
	if err != nil {
		t.Fatal(err)
	}
	if !details.Executable || !details.UserExecutable {
		t.Fatalf("MethodIO executable state = %#v, want executable for current user", details)
	}
	if len(details.InputArguments) != 2 || details.InputArguments[0].Name != "Summand1" || details.InputArguments[1].Name != "Summand2" {
		t.Fatalf("MethodIO inputs = %#v, want ordered Summand1 and Summand2", details.InputArguments)
	}
	for _, argument := range details.InputArguments {
		if argument.DataType != "UInt32" || argument.DataTypeID != "i=7" || argument.ValueRank != "Scalar" {
			t.Fatalf("MethodIO input = %#v, want scalar UInt32", argument)
		}
	}
	if len(details.OutputArguments) != 1 || details.OutputArguments[0].Name != "Sum" || details.OutputArguments[0].DataType != "UInt32" || details.OutputArguments[0].ValueRank != "Scalar" {
		t.Fatalf("MethodIO outputs = %#v, want scalar UInt32 Sum", details.OutputArguments)
	}
}

func TestIntegrationDiscoverEndpoints(t *testing.T) {
	endpoint := os.Getenv("TERMUA_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("set TERMUA_TEST_ENDPOINT to run OPC UA integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client := NewClient()
	endpoints, err := client.DiscoverEndpoints(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	if len(endpoints) == 0 {
		t.Fatal("expected at least one endpoint")
	}
}
