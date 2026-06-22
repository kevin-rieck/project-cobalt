package opcua

import (
	"testing"

	"github.com/gopcua/opcua/ua"
)

func TestEngineeringUnitTextUsesDisplayNameFromExtensionObject(t *testing.T) {
	unit := ua.NewExtensionObject(&ua.EUInformation{DisplayName: ua.NewLocalizedText("%")})

	got := engineeringUnitText(unit)

	if got != "%" {
		t.Fatalf("engineering unit = %q, want %%", got)
	}
}
