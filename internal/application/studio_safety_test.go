package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"opcua-studio/internal/connections"
	"opcua-studio/internal/opcua"
)

func TestStudioSerializesVariableNodeWriteWithReadOnlyTransition(t *testing.T) {
	node := opcua.AddressNode{NodeID: "ns=2;s=Level", NodeClass: "Variable"}
	writeStarted := make(chan struct{})
	writeRelease := make(chan struct{})
	client := &blockingWriteClient{recordingClient: recordingClient{
		readDetails: map[string]opcua.NodeDetails{node.NodeID: {NodeID: node.NodeID, DataType: "Double", Writable: true, ValueRank: "Scalar"}},
		readValues:  map[string]opcua.LiveValue{node.NodeID: {NodeID: node.NodeID, Value: "42", Status: "Good"}},
	}, started: writeStarted, release: writeRelease}
	studio := writableStudio(client, OperationTimeouts{Write: time.Second})
	studio.inspections.Select(node)
	studio.inspections.ApplyLiveValue(node.NodeID, opcua.LiveValue{NodeID: node.NodeID, Value: "40", Status: "Good"}, nil)

	writeDone := make(chan error, 1)
	go func() {
		_, err := studio.WriteVariableNodeValue(VariableNodeWriteRequest{NodeID: node.NodeID, TargetValue: "42"})
		writeDone <- err
	}()
	<-writeStarted

	transitionDone := make(chan error, 1)
	go func() { transitionDone <- studio.SetReadOnlyMode(true) }()
	select {
	case err := <-transitionDone:
		t.Fatalf("SetReadOnlyMode(true) completed during Variable Node Write: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(writeRelease)
	if err := <-writeDone; err != nil {
		t.Fatalf("WriteVariableNodeValue() error = %v", err)
	}
	if err := <-transitionDone; err != nil {
		t.Fatalf("SetReadOnlyMode(true) error = %v", err)
	}
}

func TestStudioBoundsAnUncooperativeVariableNodeWrite(t *testing.T) {
	node := opcua.AddressNode{NodeID: "ns=2;s=Level", NodeClass: "Variable"}
	release := make(chan struct{})
	client := &blockingWriteClient{recordingClient: recordingClient{
		readDetails: map[string]opcua.NodeDetails{node.NodeID: {NodeID: node.NodeID, DataType: "Double", Writable: true, ValueRank: "Scalar"}},
	}, started: make(chan struct{}), release: release}
	studio := writableStudio(client, OperationTimeouts{Write: 10 * time.Millisecond})
	studio.inspections.Select(node)
	studio.inspections.ApplyLiveValue(node.NodeID, opcua.LiveValue{NodeID: node.NodeID, Value: "40", Status: "Good"}, nil)

	started := time.Now()
	_, err := studio.WriteVariableNodeValue(VariableNodeWriteRequest{NodeID: node.NodeID, TargetValue: "42"})
	var unknown *UnknownMutationOutcomeError
	if !errors.As(err, &unknown) {
		t.Fatalf("WriteVariableNodeValue() error = %v, want UnknownMutationOutcomeError", err)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("WriteVariableNodeValue() took %s, want bounded timeout", elapsed)
	}
	if safety := studio.GetSessionSafety(); !safety.ReadOnlyMode {
		t.Fatalf("GetSessionSafety() after unknown mutation = %#v, want fail-safe Read-Only Mode", safety)
	}
	if err := studio.SetReadOnlyMode(true); !errors.Is(err, ErrUnknownMutationInFlight) {
		t.Fatalf("SetReadOnlyMode(true) error = %v, want unknown-mutation barrier", err)
	}
	if err := studio.Disconnect(); !errors.Is(err, ErrUnknownMutationInFlight) {
		t.Fatalf("Disconnect() error = %v, want unknown-mutation barrier", err)
	}

	close(release)
	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if !studio.Snapshot().MutationExecutionPending {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if studio.Snapshot().MutationExecutionPending {
		t.Fatal("timed-out write remained marked as executing after it completed")
	}
	if err := studio.Connect(ConnectionRequest{Endpoint: "opc.tcp://other.local:4840", AuthType: opcua.AuthAnonymous}); !errors.Is(err, ErrUnknownMutationInFlight) {
		t.Fatalf("Connect() error = %v, want inspection barrier", err)
	}
	if err := studio.SetReadOnlyMode(false); err != nil {
		t.Fatalf("SetReadOnlyMode(false) after deliberate inspection acknowledgement error = %v", err)
	}
}

func TestStudioCancelsSubscriptionThatCompletesAfterReadDeadline(t *testing.T) {
	subscription := &recordingSubscription{cancelled: make(chan struct{})}
	client := &lateSubscribeClient{started: make(chan struct{}), release: make(chan struct{}), subscription: subscription}
	studio := NewStudio(Options{
		ClientFactory:     func() opcua.Client { return client },
		OperationTimeouts: OperationTimeouts{Read: 10 * time.Millisecond},
	})
	if err := studio.WatchVariableNode(opcua.AddressNode{NodeID: "ns=2;s=Level", NodeClass: "Variable"}); err != nil {
		t.Fatalf("WatchVariableNode() error = %v", err)
	}
	<-client.started
	time.Sleep(25 * time.Millisecond)
	close(client.release)
	select {
	case <-subscription.cancelled:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("late ValueSubscription was not cancelled")
	}
}

func TestStudioReportsUnknownOutcomeWhenVariableNodeWriteTimesOut(t *testing.T) {
	node := opcua.AddressNode{NodeID: "ns=2;s=Level", NodeClass: "Variable"}
	client := &deadlineWriteClient{recordingClient: recordingClient{
		readDetails: map[string]opcua.NodeDetails{node.NodeID: {NodeID: node.NodeID, DataType: "Double", Writable: true, ValueRank: "Scalar"}},
	}}
	studio := writableStudio(client, OperationTimeouts{Write: 10 * time.Millisecond})
	studio.inspections.Select(node)
	studio.inspections.ApplyLiveValue(node.NodeID, opcua.LiveValue{NodeID: node.NodeID, Value: "40", Status: "Good"}, nil)

	_, err := studio.WriteVariableNodeValue(VariableNodeWriteRequest{NodeID: node.NodeID, TargetValue: "42"})
	var unknown *UnknownMutationOutcomeError
	if !errors.As(err, &unknown) {
		t.Fatalf("WriteVariableNodeValue() error = %v, want UnknownMutationOutcomeError", err)
	}
	if unknown.Operation != "Variable Node Write" {
		t.Fatalf("unknown mutation operation = %q", unknown.Operation)
	}
}

func TestStudioReportsUnknownOutcomeForClientMutationTimeout(t *testing.T) {
	node := opcua.AddressNode{NodeID: "ns=2;s=Level", NodeClass: "Variable"}
	client := &recordingClient{
		readDetails: map[string]opcua.NodeDetails{node.NodeID: {NodeID: node.NodeID, DataType: "Double", Writable: true, ValueRank: "Scalar"}},
		writeErrors: map[string]error{node.NodeID: context.DeadlineExceeded},
	}
	studio := writableStudio(client, OperationTimeouts{Write: time.Second})
	studio.inspections.Select(node)
	studio.inspections.ApplyLiveValue(node.NodeID, opcua.LiveValue{NodeID: node.NodeID, Value: "40", Status: "Good"}, nil)

	_, err := studio.WriteVariableNodeValue(VariableNodeWriteRequest{NodeID: node.NodeID, TargetValue: "42"})
	var unknown *UnknownMutationOutcomeError
	if !errors.As(err, &unknown) {
		t.Fatalf("WriteVariableNodeValue() error = %v, want UnknownMutationOutcomeError", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WriteVariableNodeValue() error = %v, want client deadline preserved", err)
	}
}

func TestStudioReportsServerRejectedVariableNodeWriteAsKnownFailure(t *testing.T) {
	node := opcua.AddressNode{NodeID: "ns=2;s=Level", NodeClass: "Variable"}
	rejected := &opcua.WriteRejectedError{NodeID: node.NodeID, Status: "StatusBadUserAccessDenied"}
	client := &recordingClient{
		readDetails: map[string]opcua.NodeDetails{node.NodeID: {NodeID: node.NodeID, DataType: "Double", Writable: true, ValueRank: "Scalar"}},
		writeErrors: map[string]error{node.NodeID: rejected},
	}
	studio := writableStudio(client, OperationTimeouts{Write: time.Second})
	studio.inspections.Select(node)
	studio.inspections.ApplyLiveValue(node.NodeID, opcua.LiveValue{NodeID: node.NodeID, Value: "40", Status: "Good"}, nil)

	_, err := studio.WriteVariableNodeValue(VariableNodeWriteRequest{NodeID: node.NodeID, TargetValue: "42"})
	if !errors.Is(err, rejected) {
		t.Fatalf("WriteVariableNodeValue() error = %v, want server rejection", err)
	}
	var unknown *UnknownMutationOutcomeError
	if errors.As(err, &unknown) {
		t.Fatalf("WriteVariableNodeValue() error = %v, want known failure", err)
	}
	if safety := studio.GetSessionSafety(); !safety.Connected || safety.ReadOnlyMode {
		t.Fatalf("GetSessionSafety() after known rejection = %#v, want active writable session", safety)
	}
}

func TestStudioLimitsWatchlistToOneHundredVariableNodes(t *testing.T) {
	studio := NewStudio(Options{ClientFactory: func() opcua.Client { return &recordingClient{} }})
	for i := 0; i < 100; i++ {
		node := opcua.AddressNode{NodeID: fmt.Sprintf("ns=2;s=Level%d", i), NodeClass: "Variable"}
		if err := studio.WatchVariableNode(node); err != nil {
			t.Fatalf("WatchVariableNode(%d) error = %v", i, err)
		}
	}

	err := studio.WatchVariableNode(opcua.AddressNode{NodeID: "ns=2;s=Level100", NodeClass: "Variable"})
	if err == nil || !strings.Contains(err.Error(), "100") {
		t.Fatalf("101st WatchVariableNode() error = %v, want 100-node limit", err)
	}
	if got := len(studio.GetWatchlist()); got != 100 {
		t.Fatalf("Watchlist size = %d, want 100", got)
	}
}

func TestStudioDiagnosticsRedactVariableNodeWriteValues(t *testing.T) {
	node := opcua.AddressNode{NodeID: "ns=2;s=Secret", NodeClass: "Variable"}
	client := &recordingClient{
		readDetails: map[string]opcua.NodeDetails{node.NodeID: {NodeID: node.NodeID, DataType: "String", Writable: true, ValueRank: "Scalar"}},
		readValues:  map[string]opcua.LiveValue{node.NodeID: {NodeID: node.NodeID, Value: "TOP-SECRET", Status: "Good"}},
	}
	studio := writableStudio(client, OperationTimeouts{})
	studio.inspections.Select(node)
	studio.inspections.ApplyLiveValue(node.NodeID, opcua.LiveValue{NodeID: node.NodeID, Value: "before", Status: "Good"}, nil)

	if _, err := studio.WriteVariableNodeValue(VariableNodeWriteRequest{NodeID: node.NodeID, TargetValue: "TOP-SECRET"}); err != nil {
		t.Fatalf("WriteVariableNodeValue() error = %v", err)
	}
	for _, entry := range studio.GetDiagnosticLogs() {
		if strings.Contains(entry.Message, "TOP-SECRET") || strings.Contains(entry.Message, "before") {
			t.Fatalf("diagnostic leaked Variable Node value: %q", entry.Message)
		}
	}
}

func writableStudio(client opcua.Client, timeouts OperationTimeouts) *Studio {
	studio := NewStudio(Options{
		ClientFactory:     func() opcua.Client { return client },
		OperationTimeouts: timeouts,
	})
	studio.client = client
	studio.connected = true
	studio.readOnlyMode = false
	return studio
}

func TestStudioUsesInjectedTimeForDiagnosticsAndSessionTrend(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	studio := NewStudio(Options{
		ClientFactory: func() opcua.Client { return &recordingClient{} },
		Now:           func() time.Time { return now },
	})
	studio.Start(context.Background())
	if logs := studio.GetDiagnosticLogs(); len(logs) != 1 || logs[0].Timestamp != now.Format(time.RFC3339) {
		t.Fatalf("diagnostic logs = %#v, want injected timestamp", logs)
	}

	node := opcua.AddressNode{NodeID: "ns=2;s=Level", NodeClass: "Variable"}
	studio.inspections.Watch(node)
	studio.inspections.ApplyLiveValue(node.NodeID, opcua.LiveValue{NodeID: node.NodeID, Value: "42", Status: "Good"}, nil)
	trend := studio.GetSessionTrend(node.NodeID)
	if len(trend.Points) != 1 || trend.Points[0].ReceivedAt != now.Format(time.RFC3339Nano) {
		t.Fatalf("Session Trend = %#v, want injected timestamp", trend)
	}
}

func TestStudioUsesInjectedDeadlineFactory(t *testing.T) {
	var deadlines []time.Duration
	studio := NewStudio(Options{
		ClientFactory: func() opcua.Client { return &recordingClient{} },
		WithTimeout: func(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
			deadlines = append(deadlines, timeout)
			return context.WithCancel(parent)
		},
	})
	if _, err := studio.DiscoverEndpoints("opc.tcp://gateway.local:4840"); err != nil {
		t.Fatalf("DiscoverEndpoints() error = %v", err)
	}
	if len(deadlines) != 1 || deadlines[0] != defaultOperationTimeout {
		t.Fatalf("deadline factory calls = %#v, want discovery deadline", deadlines)
	}
}

func TestStudioKeepsActiveConnectionSnapshotWhenSavedConnectionChanges(t *testing.T) {
	store := connections.NewFileStore(t.TempDir() + "/saved-connections.json")
	saved, err := store.Save(connections.SaveRequest{
		Name: "Control Gateway", Endpoint: "opc.tcp://gateway.local:4840", AuthType: string(opcua.AuthAnonymous),
	}, time.Now())
	if err != nil {
		t.Fatalf("seed Saved Connection: %v", err)
	}
	studio := NewStudio(Options{
		ClientFactory:        func() opcua.Client { return &recordingClient{} },
		SavedConnectionStore: store,
	})
	studio.Start(context.Background())
	if err := studio.Connect(ConnectionRequest{
		SavedConnectionID: saved.ID, Name: saved.Name, Endpoint: saved.Endpoint, AuthType: opcua.AuthAnonymous,
	}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = studio.Disconnect() })
	before, ok := studio.ActiveConnection()
	if !ok {
		t.Fatal("ActiveConnection() missing after Connect")
	}

	if _, err := studio.SaveSavedConnection(ConnectionRequest{
		ExistingName: saved.Name, Name: "Packaging Line", Endpoint: "opc.tcp://packaging.local:4840", AuthType: opcua.AuthAnonymous,
	}); err != nil {
		t.Fatalf("SaveSavedConnection() error = %v", err)
	}
	if deleted, err := studio.DeleteSavedConnection(saved.ID); err != nil || !deleted {
		t.Fatalf("DeleteSavedConnection() = (%t, %v), want (true, nil)", deleted, err)
	}
	after, ok := studio.ActiveConnection()
	if !ok || after != before {
		t.Fatalf("ActiveConnection() after Saved Connection changes = %#v, want %#v", after, before)
	}
	if snapshot := studio.Snapshot(); snapshot.ActiveConnection == nil || *snapshot.ActiveConnection != before || !snapshot.Safety.Connected {
		t.Fatalf("Snapshot() = %#v, want active connection and connected safety", snapshot)
	}
	if safety := studio.GetSessionSafety(); !safety.Connected {
		t.Fatalf("GetSessionSafety() = %#v, want active session preserved", safety)
	}
}

func TestStudioBoundsConcurrentSafeReads(t *testing.T) {
	client := &blockingDiscoverClient{started: make(chan struct{}, 3), release: make(chan struct{})}
	studio := NewStudio(Options{
		ClientFactory:      func() opcua.Client { return client },
		MaxConcurrentReads: 2,
		OperationTimeouts:  OperationTimeouts{Discovery: time.Second},
	})
	done := make(chan error, 3)
	for range 3 {
		go func() { _, err := studio.DiscoverEndpoints("opc.tcp://gateway.local:4840"); done <- err }()
	}
	<-client.started
	<-client.started
	select {
	case <-client.started:
		t.Fatal("third safe read reached the OPC UA client before a read slot became available")
	case <-time.After(50 * time.Millisecond):
	}
	close(client.release)
	for range 3 {
		if err := <-done; err != nil {
			t.Fatalf("DiscoverEndpoints() error = %v", err)
		}
	}
}

func TestStudioAppliesConfiguredConnectDeadline(t *testing.T) {
	studio := NewStudio(Options{
		ClientFactory:     func() opcua.Client { return &deadlineConnectClient{} },
		OperationTimeouts: OperationTimeouts{Connect: 10 * time.Millisecond},
	})
	started := time.Now()
	err := studio.Connect(ConnectionRequest{Endpoint: "opc.tcp://gateway.local:4840", AuthType: opcua.AuthAnonymous})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Connect() error = %v, want deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("Connect() took %s, want bounded timeout", elapsed)
	}
}

func TestStudioClosesNoncooperativeConnectOnlyAfterItReturns(t *testing.T) {
	client := &lateConnectClient{started: make(chan struct{}), release: make(chan struct{}), closed: make(chan struct{})}
	studio := NewStudio(Options{
		ClientFactory:     func() opcua.Client { return client },
		OperationTimeouts: OperationTimeouts{Connect: 10 * time.Millisecond},
	})
	err := studio.Connect(ConnectionRequest{Endpoint: "opc.tcp://gateway.local:4840", AuthType: opcua.AuthAnonymous})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Connect() error = %v, want deadline exceeded", err)
	}
	<-client.started
	select {
	case <-client.closed:
		t.Fatal("Connect client was closed before its uncooperative Connect returned")
	case <-time.After(25 * time.Millisecond):
	}
	close(client.release)
	select {
	case <-client.closed:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("late Connect client was not closed after Connect returned")
	}
}

func TestStudioReportsUnknownOutcomeWhenMethodCallTimesOut(t *testing.T) {
	const objectNodeID = "ns=2;s=Object"
	const methodNodeID = "ns=2;s=Method"
	client := &deadlineMethodClient{recordingClient: recordingClient{methodDetails: map[string]opcua.MethodDetails{
		objectNodeID + "\x00" + methodNodeID: {Executable: true, UserExecutable: true},
	}}}
	studio := writableStudio(client, OperationTimeouts{MethodCall: 10 * time.Millisecond})

	_, err := studio.CallMethod(MethodCallRequest{ObjectNodeID: objectNodeID, MethodNodeID: methodNodeID})
	var unknown *UnknownMutationOutcomeError
	if !errors.As(err, &unknown) {
		t.Fatalf("CallMethod() error = %v, want UnknownMutationOutcomeError", err)
	}
	if unknown.Operation != "Method Call" {
		t.Fatalf("unknown mutation operation = %q", unknown.Operation)
	}
}

type lateSubscribeClient struct {
	recordingClient
	started      chan struct{}
	release      chan struct{}
	subscription opcua.ValueSubscription
}

func (c *lateSubscribeClient) SubscribeValue(context.Context, string) (<-chan opcua.LiveValue, opcua.ValueSubscription, error) {
	close(c.started)
	<-c.release
	return nil, c.subscription, nil
}

type recordingSubscription struct{ cancelled chan struct{} }

func (s *recordingSubscription) Cancel(context.Context) error {
	close(s.cancelled)
	return nil
}

type blockingWriteClient struct {
	recordingClient
	started chan struct{}
	release <-chan struct{}
}

func (c *blockingWriteClient) WriteValue(context.Context, string, opcua.ScalarValue) error {
	close(c.started)
	<-c.release
	return nil
}

type deadlineWriteClient struct{ recordingClient }

func (c *deadlineWriteClient) WriteValue(ctx context.Context, _ string, _ opcua.ScalarValue) error {
	<-ctx.Done()
	return ctx.Err()
}

type blockingDiscoverClient struct {
	recordingClient
	started chan struct{}
	release chan struct{}
}

func (c *blockingDiscoverClient) DiscoverEndpoints(ctx context.Context, _ string) ([]opcua.Endpoint, error) {
	c.started <- struct{}{}
	select {
	case <-c.release:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type deadlineConnectClient struct{ recordingClient }

func (c *deadlineConnectClient) Connect(ctx context.Context, _ opcua.ConnectRequest) error {
	<-ctx.Done()
	return ctx.Err()
}

type deadlineMethodClient struct{ recordingClient }

func (c *deadlineMethodClient) CallMethod(ctx context.Context, _ string, _ string, _ []opcua.ScalarValue) (opcua.MethodCallResult, error) {
	<-ctx.Done()
	return opcua.MethodCallResult{}, ctx.Err()
}

type lateConnectClient struct {
	recordingClient
	started chan struct{}
	release chan struct{}
	closed  chan struct{}
}

func (c *lateConnectClient) Connect(context.Context, opcua.ConnectRequest) error {
	close(c.started)
	<-c.release
	return nil
}

func (c *lateConnectClient) Close(context.Context) error {
	close(c.closed)
	return nil
}
