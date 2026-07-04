package opcua

import (
	"testing"

	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/ua"
)

func TestNodeDetailsUsesUserAccessLevelForEffectiveWriteAvailability(t *testing.T) {
	details := NodeDetails{NodeID: "ns=2;s=Level"}

	applyNodeDetailAttributes(&details, nodeDetailAttributeValues(byte(ua.AccessLevelTypeCurrentRead), byte(ua.AccessLevelTypeCurrentRead|ua.AccessLevelTypeCurrentWrite)))

	if !details.Writable {
		t.Fatalf("Writable = false, want true from UserAccessLevel")
	}
	if details.UserAccessLevel != "CurrentRead, CurrentWrite" {
		t.Fatalf("UserAccessLevel = %q, want CurrentRead, CurrentWrite", details.UserAccessLevel)
	}
	if !details.UserAccessLevelAvailable {
		t.Fatalf("UserAccessLevelAvailable = false, want true")
	}
}

func TestNodeDetailsPrefersReadOnlyUserAccessLevelOverWritableAccessLevel(t *testing.T) {
	details := NodeDetails{NodeID: "ns=2;s=Level"}

	applyNodeDetailAttributes(&details, nodeDetailAttributeValues(byte(ua.AccessLevelTypeCurrentRead|ua.AccessLevelTypeCurrentWrite), byte(ua.AccessLevelTypeCurrentRead)))

	if details.Writable {
		t.Fatalf("Writable = true, want false from read-only UserAccessLevel")
	}
	if details.WriteAvailability != "Read-only in this session" {
		t.Fatalf("WriteAvailability = %q, want Read-only in this session", details.WriteAvailability)
	}
}

func TestNodeDetailsFallsBackToAccessLevelWhenUserAccessLevelUnavailable(t *testing.T) {
	details := NodeDetails{NodeID: "ns=2;s=Level"}
	attrs := nodeDetailAttributeValues(byte(ua.AccessLevelTypeCurrentRead|ua.AccessLevelTypeCurrentWrite), byte(ua.AccessLevelTypeNone))
	attrs[5] = &ua.DataValue{Status: ua.StatusBadAttributeIDInvalid}

	applyNodeDetailAttributes(&details, attrs)

	if !details.Writable {
		t.Fatalf("Writable = false, want fallback from writable AccessLevel")
	}
	if details.UserAccessLevelAvailable {
		t.Fatalf("UserAccessLevelAvailable = true, want false")
	}
	if details.WriteAvailability != "Write availability not confirmed for this user" {
		t.Fatalf("WriteAvailability = %q, want fallback warning", details.WriteAvailability)
	}
}

func nodeDetailAttributeValues(accessLevel byte, userAccessLevel byte) []*ua.DataValue {
	return []*ua.DataValue{
		{Status: ua.StatusOK, Value: ua.MustVariant(ua.NewLocalizedText(""))},
		{Status: ua.StatusOK, Value: ua.MustVariant(accessLevel)},
		{Status: ua.StatusOK, Value: ua.MustVariant(ua.NewNumericNodeID(0, id.Double))},
		{Status: ua.StatusOK, Value: ua.MustVariant(int32(-1))},
		{Status: ua.StatusOK, Value: ua.MustVariant([]uint32{})},
		{Status: ua.StatusOK, Value: ua.MustVariant(userAccessLevel)},
	}
}

func TestEngineeringUnitTextUsesDisplayNameFromExtensionObject(t *testing.T) {
	unit := ua.NewExtensionObject(&ua.EUInformation{DisplayName: ua.NewLocalizedText("%")})

	got := engineeringUnitText(unit)

	if got != "%" {
		t.Fatalf("engineering unit = %q, want %%", got)
	}
}
