package opcua

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/gopcua/opcua/ua"
)

var plainDecimalIntegerPattern = regexp.MustCompile(`^[+-]?\d+$`)

type ScalarValue struct {
	DataType   string
	Value      any
	Normalized string
}

func ParseScalarValue(dataType string, target string) (ScalarValue, error) {
	switch strings.TrimSpace(dataType) {
	case "Boolean":
		return parseBooleanScalar(target)
	case "SByte":
		return parseSignedIntegerScalar[int8](dataType, target, math.MinInt8, math.MaxInt8)
	case "Int16":
		return parseSignedIntegerScalar[int16](dataType, target, math.MinInt16, math.MaxInt16)
	case "Int32":
		return parseSignedIntegerScalar[int32](dataType, target, math.MinInt32, math.MaxInt32)
	case "Int64":
		return parseSignedIntegerScalar[int64](dataType, target, math.MinInt64, math.MaxInt64)
	case "Byte":
		return parseUnsignedIntegerScalar[byte](dataType, target, math.MaxUint8)
	case "UInt16":
		return parseUnsignedIntegerScalar[uint16](dataType, target, math.MaxUint16)
	case "UInt32":
		return parseUnsignedIntegerScalar[uint32](dataType, target, math.MaxUint32)
	case "UInt64":
		return parseUnsignedIntegerScalar[uint64](dataType, target, math.MaxUint64)
	case "Float":
		value, err := parseFloatScalar(dataType, target, 32)
		if err != nil {
			return ScalarValue{}, err
		}
		return ScalarValue{DataType: dataType, Value: float32(value), Normalized: strconv.FormatFloat(float64(float32(value)), 'g', -1, 32)}, nil
	case "Double":
		value, err := parseFloatScalar(dataType, target, 64)
		if err != nil {
			return ScalarValue{}, err
		}
		return ScalarValue{DataType: dataType, Value: value, Normalized: strconv.FormatFloat(value, 'g', -1, 64)}, nil
	case "String":
		return ScalarValue{DataType: "String", Value: target, Normalized: target}, nil
	default:
		return ScalarValue{}, fmt.Errorf("unsupported Variable Node Write data type: %s", dataType)
	}
}

func parseBooleanScalar(target string) (ScalarValue, error) {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "true", "1", "on", "yes":
		return ScalarValue{DataType: "Boolean", Value: true, Normalized: "true"}, nil
	case "false", "0", "off", "no":
		return ScalarValue{DataType: "Boolean", Value: false, Normalized: "false"}, nil
	default:
		return ScalarValue{}, fmt.Errorf("invalid Boolean target value: %q", target)
	}
}

func parseSignedIntegerScalar[T ~int8 | ~int16 | ~int32 | ~int64](dataType string, target string, min int64, max int64) (ScalarValue, error) {
	trimmed := strings.TrimSpace(target)
	if !plainDecimalIntegerPattern.MatchString(trimmed) {
		return ScalarValue{}, fmt.Errorf("invalid %s target value %q: use plain decimal integer notation", dataType, target)
	}
	value, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || value < min || value > max {
		return ScalarValue{}, fmt.Errorf("%s target value %q is out of range", dataType, target)
	}
	return ScalarValue{DataType: dataType, Value: T(value), Normalized: strconv.FormatInt(value, 10)}, nil
}

func parseUnsignedIntegerScalar[T ~uint8 | ~uint16 | ~uint32 | ~uint64](dataType string, target string, max uint64) (ScalarValue, error) {
	trimmed := strings.TrimSpace(target)
	if !plainDecimalIntegerPattern.MatchString(trimmed) || strings.HasPrefix(trimmed, "-") {
		return ScalarValue{}, fmt.Errorf("invalid %s target value %q: use plain decimal integer notation", dataType, target)
	}
	value, err := strconv.ParseUint(trimmed, 10, 64)
	if err != nil || value > max {
		return ScalarValue{}, fmt.Errorf("%s target value %q is out of range", dataType, target)
	}
	return ScalarValue{DataType: dataType, Value: T(value), Normalized: strconv.FormatUint(value, 10)}, nil
}

func parseFloatScalar(dataType string, target string, bitSize int) (float64, error) {
	trimmed := strings.TrimSpace(target)
	value, err := strconv.ParseFloat(trimmed, bitSize)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("invalid %s target value: %q", dataType, target)
	}
	return value, nil
}

func (c *gopcuaClient) WriteValue(ctx context.Context, nodeID string, value ScalarValue) error {
	if c.client == nil {
		return ua.StatusBadServerNotConnected
	}
	parsedNodeID, err := ua.ParseNodeID(nodeID)
	if err != nil {
		return err
	}
	variant, err := ua.NewVariant(value.Value)
	if err != nil {
		return err
	}
	response, err := c.client.Write(ctx, &ua.WriteRequest{NodesToWrite: []*ua.WriteValue{{
		NodeID:      parsedNodeID,
		AttributeID: ua.AttributeIDValue,
		Value:       &ua.DataValue{EncodingMask: ua.DataValueValue, Value: variant},
	}}})
	if err != nil {
		return err
	}
	if len(response.Results) == 0 {
		return fmt.Errorf("write %s failed: no result", nodeID)
	}
	if response.Results[0] != ua.StatusOK {
		return fmt.Errorf("write %s failed: %s", nodeID, response.Results[0])
	}
	return nil
}
