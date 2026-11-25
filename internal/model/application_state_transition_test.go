package model

import "testing"

func TestNextApplicationState(t *testing.T) {
	allStates := []ApplicationState{
		AppNone, AppSaved, AppApplied, AppInterview,
		AppOffer, AppAccepted, AppTurnedDown, AppNegative,
	}

	tests := []struct {
		name  string
		event EventType
		want  map[ApplicationState]ApplicationState
	}{
		{
			name:  "cv_sent opens none, saved and negative",
			event: EventCVSent,
			want: map[ApplicationState]ApplicationState{
				AppNone:       AppApplied,
				AppSaved:      AppApplied,
				AppApplied:    AppApplied,
				AppInterview:  AppInterview,
				AppOffer:      AppOffer,
				AppAccepted:   AppAccepted,
				AppTurnedDown: AppTurnedDown,
				AppNegative:   AppApplied,
			},
		},
		{
			name:  "rejected closes every state",
			event: EventRejected,
			want: map[ApplicationState]ApplicationState{
				AppNone:       AppNegative,
				AppSaved:      AppNegative,
				AppApplied:    AppNegative,
				AppInterview:  AppNegative,
				AppOffer:      AppNegative,
				AppAccepted:   AppNegative,
				AppTurnedDown: AppNegative,
				AppNegative:   AppNegative,
			},
		},
		{
			name:  "ghosted closes applied and interview only",
			event: EventGhosted,
			want: map[ApplicationState]ApplicationState{
				AppNone:       AppNone,
				AppSaved:      AppSaved,
				AppApplied:    AppNegative,
				AppInterview:  AppNegative,
				AppOffer:      AppOffer,
				AppAccepted:   AppAccepted,
				AppTurnedDown: AppTurnedDown,
				AppNegative:   AppNegative,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, from := range allStates {
				want := tt.want[from]
				if got := NextApplicationState(from, tt.event); got != want {
					t.Errorf("NextApplicationState(%q, %q) = %q, want %q", from, tt.event, got, want)
				}
			}
		})
	}

	// Every non-transition event is a no-op from every state.
	for _, event := range []EventType{
		EventMessageReceived, EventMessageSent, EventInterview, EventOffer, EventOther,
	} {
		for _, from := range allStates {
			if got := NextApplicationState(from, event); got != from {
				t.Errorf("NextApplicationState(%q, %q) = %q, want unchanged", from, event, got)
			}
		}
	}
}
