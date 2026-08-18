package main

import (
	"context"
	"testing"

	"opcua-studio/internal/application"
)

func TestAppForwardsApplicationEventsToItsDeliveryAdapter(t *testing.T) {
	studio := application.NewStudio()
	var got application.Event
	app := newApp(studio, func(_ context.Context, event application.Event) { got = event })

	app.forwardEvent(application.SessionSafetyUpdated{View: application.SessionSafetyView{Connected: true}})

	if got.Type() != application.EventSessionSafetyUpdated || got.Payload() != (application.SessionSafetyView{Connected: true}) {
		t.Fatalf("forwarded event = %#v", got)
	}
}
