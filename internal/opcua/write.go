package opcua

import (
	"context"
	"fmt"

	"github.com/gopcua/opcua/ua"
)

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
