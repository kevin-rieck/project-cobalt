package opcua

import (
	"context"
	"fmt"
	"strings"

	gopcua "github.com/gopcua/opcua"
	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/ua"
)

// MethodArgument is the app-level projection of one declared Method argument.
type MethodArgument struct {
	Name            string
	DataType        string
	DataTypeID      string
	ValueRank       string
	Description     string
	ArrayDimensions []uint32
	Supported       bool
}

// MethodDetails describes a Method Node and its ordered signature.
type MethodDetails struct {
	ObjectNodeID    string
	MethodNodeID    string
	Description     string
	Executable      bool
	UserExecutable  bool
	InputArguments  []MethodArgument
	OutputArguments []MethodArgument
}

// MethodCallResult is the display-safe projection of a Method execution result.
type MethodCallResult struct {
	StatusCode           string
	InputArgumentResults []string
	OutputArguments      []MethodArgumentValue
}

// MethodArgumentValue is one raw, display-only Method output.
type MethodArgumentValue struct {
	DataType string
	Value    string
}

func (c *gopcuaClient) ReadMethodDetails(ctx context.Context, objectNodeID, methodNodeID string) (MethodDetails, error) {
	if c.client == nil {
		return MethodDetails{}, ua.StatusBadServerNotConnected
	}
	if _, err := ua.ParseNodeID(objectNodeID); err != nil {
		return MethodDetails{}, fmt.Errorf("invalid Object Node ID: %w", err)
	}
	parsedMethodNodeID, err := ua.ParseNodeID(methodNodeID)
	if err != nil {
		return MethodDetails{}, fmt.Errorf("invalid Method Node ID: %w", err)
	}

	details := MethodDetails{
		ObjectNodeID:    objectNodeID,
		MethodNodeID:    methodNodeID,
		InputArguments:  []MethodArgument{},
		OutputArguments: []MethodArgument{},
	}
	methodNode := c.client.Node(parsedMethodNodeID)
	attrs, err := methodNode.Attributes(ctx, ua.AttributeIDDescription, ua.AttributeIDExecutable, ua.AttributeIDUserExecutable)
	if err != nil {
		return MethodDetails{}, fmt.Errorf("read Method attributes: %w", err)
	}
	if err := applyMethodAttributes(&details, attrs); err != nil {
		return MethodDetails{}, err
	}
	if err := c.applyMethodArgumentProperties(ctx, methodNode, &details); err != nil {
		return MethodDetails{}, err
	}
	return details, nil
}

func applyMethodAttributes(details *MethodDetails, attrs []*ua.DataValue) error {
	names := []string{"Description", "Executable", "UserExecutable"}
	if len(attrs) != len(names) {
		return fmt.Errorf("read Method attributes: got %d results, want %d", len(attrs), len(names))
	}
	for i, attr := range attrs {
		if attr == nil {
			return fmt.Errorf("read Method %s: empty result", names[i])
		}
		if i == 0 && attr.Status == ua.StatusBadAttributeIDInvalid {
			continue
		}
		if attr.Status != ua.StatusOK {
			return fmt.Errorf("read Method %s: %w", names[i], attr.Status)
		}
		if attr.Value == nil {
			return fmt.Errorf("read Method %s: empty value", names[i])
		}
	}
	if attrs[0].Status == ua.StatusOK {
		details.Description = localizedTextValue(attrs[0].Value.Value())
	}
	var ok bool
	details.Executable, ok = attrs[1].Value.Value().(bool)
	if !ok {
		return fmt.Errorf("read Method Executable: malformed %T value", attrs[1].Value.Value())
	}
	details.UserExecutable, ok = attrs[2].Value.Value().(bool)
	if !ok {
		return fmt.Errorf("read Method UserExecutable: malformed %T value", attrs[2].Value.Value())
	}
	return nil
}

func (c *gopcuaClient) applyMethodArgumentProperties(ctx context.Context, methodNode *gopcua.Node, details *MethodDetails) error {
	refs, err := methodNode.References(ctx, id.HasProperty, ua.BrowseDirectionForward, ua.NodeClassVariable, true)
	if err != nil {
		return fmt.Errorf("browse Method argument properties: %w", err)
	}
	for _, ref := range refs {
		property := addressNodeFromReference(ref)
		name := strings.TrimPrefix(property.BrowseName, "0:")
		if name != "InputArguments" && name != "OutputArguments" {
			continue
		}
		propertyNodeID, err := ua.ParseNodeID(property.NodeID)
		if err != nil {
			return fmt.Errorf("read Method %s: invalid property Node ID: %w", name, err)
		}
		value, readErr := c.client.Node(propertyNodeID).Attribute(ctx, ua.AttributeIDValue)
		arguments, err := methodArgumentsFromProperty(name, value, readErr)
		if err != nil {
			return err
		}
		if name == "InputArguments" {
			details.InputArguments = arguments
		} else {
			details.OutputArguments = arguments
		}
	}
	return nil
}

func methodArgumentsFromProperty(name string, value *ua.Variant, readErr error) ([]MethodArgument, error) {
	if readErr != nil {
		return nil, fmt.Errorf("read Method %s metadata: %w", name, readErr)
	}
	arguments, err := methodArgumentsFromVariant(value)
	if err != nil {
		return nil, fmt.Errorf("read Method %s: %w", name, err)
	}
	return arguments, nil
}

func methodArgumentsFromVariant(value *ua.Variant) ([]MethodArgument, error) {
	if value == nil {
		return nil, fmt.Errorf("malformed argument metadata: empty value, want ua.Argument array")
	}

	var extensionObjects []*ua.ExtensionObject
	switch raw := value.Value().(type) {
	case []*ua.ExtensionObject:
		extensionObjects = raw
	case []ua.ExtensionObject:
		extensionObjects = make([]*ua.ExtensionObject, len(raw))
		for i := range raw {
			extensionObjects[i] = &raw[i]
		}
	default:
		return nil, fmt.Errorf("malformed argument metadata: got %T, want ua.Argument array", value.Value())
	}

	arguments := make([]MethodArgument, 0, len(extensionObjects))
	for i, extensionObject := range extensionObjects {
		if extensionObject == nil {
			return nil, fmt.Errorf("malformed argument metadata at index %d: empty ExtensionObject, want ua.Argument", i)
		}
		argument, ok := extensionObject.Value.(*ua.Argument)
		if !ok || argument == nil {
			return nil, fmt.Errorf("malformed argument metadata at index %d: got %T, want ua.Argument", i, extensionObject.Value)
		}
		arguments = append(arguments, methodArgumentFromUA(argument))
	}
	return arguments, nil
}

func methodArgumentFromUA(argument *ua.Argument) MethodArgument {
	dataTypeID := ""
	if argument.DataType != nil {
		dataTypeID = argument.DataType.String()
	}
	dimensions := append([]uint32{}, argument.ArrayDimensions...)
	dataType := dataTypeName(argument.DataType)
	return MethodArgument{
		Name:            argument.Name,
		DataType:        dataType,
		DataTypeID:      dataTypeID,
		ValueRank:       valueRankText(argument.ValueRank),
		Description:     localizedTextValue(argument.Description),
		ArrayDimensions: dimensions,
		Supported:       argument.ValueRank == -1 && supportedMethodDataType(dataType),
	}
}

func supportedMethodDataType(dataType string) bool {
	switch dataType {
	case "Boolean", "SByte", "Byte", "Int16", "UInt16", "Int32", "UInt32", "Int64", "UInt64", "Float", "Double", "String":
		return true
	default:
		return false
	}
}

// CallMethod executes one Method with already parsed scalar inputs.
func (c *gopcuaClient) CallMethod(ctx context.Context, objectNodeID, methodNodeID string, inputs []ScalarValue) (MethodCallResult, error) {
	if c.client == nil {
		return MethodCallResult{}, ua.StatusBadServerNotConnected
	}
	parsedObjectNodeID, err := ua.ParseNodeID(objectNodeID)
	if err != nil {
		return MethodCallResult{}, fmt.Errorf("invalid Object Node ID: %w", err)
	}
	parsedMethodNodeID, err := ua.ParseNodeID(methodNodeID)
	if err != nil {
		return MethodCallResult{}, fmt.Errorf("invalid Method Node ID: %w", err)
	}

	arguments := make([]*ua.Variant, len(inputs))
	for i, input := range inputs {
		arguments[i], err = ua.NewVariant(input.Value)
		if err != nil {
			return MethodCallResult{}, fmt.Errorf("encode Method input argument %d: %w", i+1, err)
		}
	}
	request := &ua.CallMethodRequest{ObjectID: parsedObjectNodeID, MethodID: parsedMethodNodeID, InputArguments: arguments}
	call := c.callMethod
	if call == nil {
		call = c.client.Call
	}
	response, err := call(ctx, request)
	if err != nil {
		return MethodCallResult{}, err
	}
	if response == nil {
		return MethodCallResult{}, fmt.Errorf("call Method returned an empty result")
	}
	return methodCallResultFromUA(response), nil
}

func methodCallResultFromUA(result *ua.CallMethodResult) MethodCallResult {
	projected := MethodCallResult{
		StatusCode:           statusCodeText(result.StatusCode),
		InputArgumentResults: make([]string, len(result.InputArgumentResults)),
		OutputArguments:      make([]MethodArgumentValue, len(result.OutputArguments)),
	}
	for i, status := range result.InputArgumentResults {
		projected.InputArgumentResults[i] = statusCodeText(status)
	}
	for i, output := range result.OutputArguments {
		if output == nil {
			projected.OutputArguments[i] = MethodArgumentValue{DataType: "Null", Value: "<nil>"}
			continue
		}
		projected.OutputArguments[i] = MethodArgumentValue{
			DataType: variantDataTypeName(output.Type()),
			Value:    fmt.Sprintf("%T(%v)", output.Value(), output.Value()),
		}
	}
	return projected
}

func variantDataTypeName(dataType ua.TypeID) string {
	name := strings.TrimPrefix(dataType.String(), "TypeID")
	name = strings.Replace(name, "Uint", "UInt", 1)
	switch name {
	case "GUID":
		return "Guid"
	case "XMLElement":
		return "XmlElement"
	case "NodeID":
		return "NodeId"
	case "ExpandedNodeID":
		return "ExpandedNodeId"
	default:
		return name
	}
}

func statusCodeText(status ua.StatusCode) string {
	if description, ok := ua.StatusCodes[status]; ok {
		return fmt.Sprintf("%s (0x%X)", description.Name, uint32(status))
	}
	return fmt.Sprintf("0x%X", uint32(status))
}
