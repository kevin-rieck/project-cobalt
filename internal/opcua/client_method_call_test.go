package opcua

import (
	"context"
	"errors"
	"strings"
	"testing"

	gopcua "github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

func TestClientCallMethodSendsTypedArgumentsAndProjectsRawResult(t *testing.T) {
	var sent *ua.CallMethodRequest
	client := &gopcuaClient{
		client: &gopcua.Client{},
		callMethod: func(_ context.Context, request *ua.CallMethodRequest) (*ua.CallMethodResult, error) {
			sent = request
			return &ua.CallMethodResult{
				StatusCode:           ua.StatusOK,
				InputArgumentResults: []ua.StatusCode{ua.StatusOK, ua.StatusOK},
				OutputArguments:      []*ua.Variant{ua.MustVariant(uint32(42)), ua.MustVariant("done")},
			}, nil
		},
	}

	result, err := client.CallMethod(context.Background(), "ns=3;s=Methods", "ns=3;s=MethodIO", []ScalarValue{
		{DataType: "UInt32", Value: uint32(20), Normalized: "20"},
		{DataType: "UInt32", Value: uint32(22), Normalized: "22"},
	})
	if err != nil {
		t.Fatalf("CallMethod() error = %v", err)
	}
	if sent == nil || sent.ObjectID.String() != "ns=3;s=Methods" || sent.MethodID.String() != "ns=3;s=MethodIO" || len(sent.InputArguments) != 2 {
		t.Fatalf("sent request = %#v, want selected Object, Method, and two inputs", sent)
	}
	if got := sent.InputArguments[0].Value(); got != uint32(20) {
		t.Fatalf("first input = %#v, want uint32(20)", got)
	}
	if result.StatusCode != "StatusGood (0x0)" || len(result.InputArgumentResults) != 2 {
		t.Fatalf("CallMethod() result = %#v, want Good status and per-input results", result)
	}
	if len(result.OutputArguments) != 2 || result.OutputArguments[0].DataType != "UInt32" || result.OutputArguments[0].Value != "uint32(42)" || result.OutputArguments[1].DataType != "String" || result.OutputArguments[1].Value != "string(done)" {
		t.Fatalf("raw outputs = %#v, want stable DataTypes and strings", result.OutputArguments)
	}
}

func TestClientCallMethodReturnsNonGoodResultAndTransportErrors(t *testing.T) {
	client := &gopcuaClient{
		client: &gopcua.Client{},
		callMethod: func(context.Context, *ua.CallMethodRequest) (*ua.CallMethodResult, error) {
			return &ua.CallMethodResult{StatusCode: ua.StatusBadInvalidArgument}, nil
		},
	}
	result, err := client.CallMethod(context.Background(), "i=85", "ns=2;s=Method", nil)
	if err != nil || !strings.Contains(result.StatusCode, "BadInvalidArgument") {
		t.Fatalf("CallMethod() = (%#v, %v), want non-Good display result", result, err)
	}

	client.callMethod = func(context.Context, *ua.CallMethodRequest) (*ua.CallMethodResult, error) {
		return nil, errors.New("connection lost")
	}
	if _, err := client.CallMethod(context.Background(), "i=85", "ns=2;s=Method", nil); err == nil || !strings.Contains(err.Error(), "connection lost") {
		t.Fatalf("CallMethod() transport error = %v, want connection lost", err)
	}
}

func TestClientCallMethodRejectsInvalidNodeIDsBeforeSending(t *testing.T) {
	called := false
	client := &gopcuaClient{
		client: &gopcua.Client{},
		callMethod: func(context.Context, *ua.CallMethodRequest) (*ua.CallMethodResult, error) {
			called = true
			return nil, nil
		},
	}

	if _, err := client.CallMethod(context.Background(), "ns=invalid;i=1", "ns=2;s=Method", nil); err == nil || !strings.Contains(err.Error(), "Object Node ID") {
		t.Fatalf("CallMethod() Object Node ID error = %v", err)
	}
	if _, err := client.CallMethod(context.Background(), "i=85", "ns=invalid;i=1", nil); err == nil || !strings.Contains(err.Error(), "Method Node ID") {
		t.Fatalf("CallMethod() Method Node ID error = %v", err)
	}
	if called {
		t.Fatal("CallMethod() sent request with an invalid Node ID")
	}
}
