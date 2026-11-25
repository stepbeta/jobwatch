package model

import "testing"

func TestApplicationStateValid(t *testing.T) {
	tests := []struct {
		name  string
		state ApplicationState
		want  bool
	}{
		{"none", AppNone, true},
		{"saved", AppSaved, true},
		{"applied", AppApplied, true},
		{"interview", AppInterview, true},
		{"offer", AppOffer, true},
		{"accepted", AppAccepted, true},
		{"turned down", AppTurnedDown, true},
		{"negative", AppNegative, true},
		{"removed todo", ApplicationState("todo"), false},
		{"removed responded", ApplicationState("responded"), false},
		{"removed maybe", ApplicationState("maybe"), false},
		{"removed positive", ApplicationState("positive"), false},
		{"empty", ApplicationState(""), false},
		{"unknown", ApplicationState("hired"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.Valid(); got != tt.want {
				t.Errorf("%q.Valid() = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

func TestApplicationStateDefaultIsNone(t *testing.T) {
	if AppNone != "none" {
		t.Errorf("AppNone = %q, want none", AppNone)
	}
	if !AppNone.Valid() {
		t.Error("the default application state must be valid")
	}
}
