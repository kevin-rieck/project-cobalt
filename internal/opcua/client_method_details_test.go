package opcua

import (
	"strings"
	"testing"

	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/ua"
)

func TestBrowsedMethodCarriesOwningObjectNodeID(t *testing.T) {
	refs := []*ua.ReferenceDescription{{
		NodeID:      ua.NewExpandedNodeID(ua.NewStringNodeID(2, "SharedReset"), "", 0),
		DisplayName: ua.NewLocalizedText("Reset"),
		BrowseName:  &ua.QualifiedName{NamespaceIndex: 2, Name: "Reset"},
		NodeClass:   ua.NodeClassMethod,
	}}

	nodes := addressNodesFromReferences("ns=2;s=MotorA", refs)

	if len(nodes) != 1 || nodes[0].ParentNodeID != "ns=2;s=MotorA" || nodes[0].NodeID != "ns=2;s=SharedReset" || nodes[0].NodeClass != "Method" {
		t.Fatalf("browsed Method = %#v, want owning Object Node identity", nodes)
	}
}

func TestMethodAttributesExposeDescriptionAndExecutableState(t *testing.T) {
	details := MethodDetails{}
	attrs := []*ua.DataValue{
		{Status: ua.StatusOK, Value: ua.MustVariant(ua.NewLocalizedText("Adds 2 unsigned integers"))},
		{Status: ua.StatusOK, Value: ua.MustVariant(true)},
		{Status: ua.StatusOK, Value: ua.MustVariant(false)},
	}

	if err := applyMethodAttributes(&details, attrs); err != nil {
		t.Fatalf("applyMethodAttributes() error = %v", err)
	}
	if details.Description != "Adds 2 unsigned integers" || !details.Executable || details.UserExecutable {
		t.Fatalf("Method attributes = %#v, want description, executable, and user not executable", details)
	}
}

func TestMethodAttributesAllowUnsupportedOptionalDescription(t *testing.T) {
	details := MethodDetails{}
	attrs := []*ua.DataValue{
		{Status: ua.StatusBadAttributeIDInvalid},
		{Status: ua.StatusOK, Value: ua.MustVariant(true)},
		{Status: ua.StatusOK, Value: ua.MustVariant(true)},
	}

	if err := applyMethodAttributes(&details, attrs); err != nil {
		t.Fatalf("applyMethodAttributes() error = %v, want unsupported optional Description to be ignored", err)
	}
	if details.Description != "" || !details.Executable || !details.UserExecutable {
		t.Fatalf("Method attributes = %#v, want empty description and executable state", details)
	}
}

func TestMethodArgumentsPreserveDeclaredOrderAndMetadata(t *testing.T) {
	value := ua.MustVariant([]*ua.ExtensionObject{
		ua.NewExtensionObject(&ua.Argument{
			Name:            "Summand1",
			DataType:        ua.NewNumericNodeID(0, id.UInt32),
			ValueRank:       -1,
			ArrayDimensions: []uint32{},
			Description:     ua.NewLocalizedText("First summand"),
		}),
		ua.NewExtensionObject(&ua.Argument{
			Name:            "Summand2",
			DataType:        ua.NewNumericNodeID(0, id.UInt32),
			ValueRank:       -1,
			ArrayDimensions: []uint32{},
			Description:     ua.NewLocalizedText("Second summand"),
		}),
	})

	arguments, err := methodArgumentsFromVariant(value)
	if err != nil {
		t.Fatalf("methodArgumentsFromVariant() error = %v", err)
	}
	if len(arguments) != 2 || arguments[0].Name != "Summand1" || arguments[1].Name != "Summand2" {
		t.Fatalf("arguments = %#v, want declared order", arguments)
	}
	first := arguments[0]
	if first.DataTypeID != "i=7" || first.DataType != "UInt32" || first.ValueRank != "Scalar" || first.Description != "First summand" || !first.Supported {
		t.Fatalf("first argument = %#v, want scalar UInt32 metadata", first)
	}
	if first.ArrayDimensions == nil || len(first.ArrayDimensions) != 0 {
		t.Fatalf("ArrayDimensions = %#v, want present empty dimensions", first.ArrayDimensions)
	}
}

func TestMethodArgumentsMarkArraysAndCustomDataTypesUnsupported(t *testing.T) {
	value := ua.MustVariant([]*ua.ExtensionObject{
		ua.NewExtensionObject(&ua.Argument{Name: "Values", DataType: ua.NewNumericNodeID(0, id.Double), ValueRank: 1, ArrayDimensions: []uint32{4}}),
		ua.NewExtensionObject(&ua.Argument{Name: "Recipe", DataType: ua.NewStringNodeID(2, "RecipeType"), ValueRank: -1}),
	})

	arguments, err := methodArgumentsFromVariant(value)
	if err != nil {
		t.Fatalf("methodArgumentsFromVariant() error = %v", err)
	}
	if arguments[0].Supported || arguments[0].ValueRank != "One-dimensional array" || len(arguments[0].ArrayDimensions) != 1 || arguments[0].ArrayDimensions[0] != 4 {
		t.Fatalf("array argument = %#v, want unsupported one-dimensional array", arguments[0])
	}
	if arguments[1].Supported || arguments[1].DataTypeID != "ns=2;s=RecipeType" || arguments[1].DataType != "ns=2;s=RecipeType" {
		t.Fatalf("custom argument = %#v, want unsupported custom DataType", arguments[1])
	}
}

func TestMalformedMethodArgumentPropertyIsMetadataError(t *testing.T) {
	_, err := methodArgumentsFromVariant(ua.MustVariant([]string{"not an Argument"}))
	if err == nil || !strings.Contains(err.Error(), "ua.Argument") {
		t.Fatalf("methodArgumentsFromVariant() error = %v, want malformed ua.Argument metadata error", err)
	}
}

func TestUnreadableMethodArgumentPropertyIsMetadataError(t *testing.T) {
	arguments, err := methodArgumentsFromProperty("InputArguments", nil, ua.StatusBadSecurityModeInsufficient)
	if err == nil || !strings.Contains(err.Error(), "BadSecurityModeInsufficient") {
		t.Fatalf("methodArgumentsFromProperty() error = %v, want secure-channel metadata error", err)
	}
	if arguments != nil {
		t.Fatalf("methodArgumentsFromProperty() = %#v, want no empty signature on metadata failure", arguments)
	}
}
