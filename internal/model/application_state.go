package model

// NextApplicationState returns the application state after creating an
// event of type eventType from current. Only cv_sent, rejected and ghosted
// change the state; every other event leaves it unchanged. The rule depends
// on the state at creation time, never on the event's occurred_at.
func NextApplicationState(current ApplicationState, eventType EventType) ApplicationState {
	switch eventType {
	case EventCVSent:
		switch current {
		case AppNone, AppSaved, AppNegative:
			return AppApplied
		}
	case EventRejected:
		return AppNegative
	case EventGhosted:
		switch current {
		case AppApplied, AppInterview:
			return AppNegative
		}
	}
	return current
}
