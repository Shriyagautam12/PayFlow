package payment

import "fmt"

// validTransitions defines the ONLY allowed state movements.
// This is the single source of truth for the payment lifecycle.
// Any transition not in this map is rejected — no exceptions.
var validTransitions = map[PaymentStatus][]PaymentStatus{
	StatusInitiated:  {StatusPending, StatusFailed},
	StatusPending:    {StatusAuthorized, StatusFailed},
	StatusAuthorized: {StatusCompleted, StatusFailed},
	StatusCompleted:  {StatusRefunded},
	StatusFailed:     {}, // terminal — no further transitions
	StatusRefunded:   {}, // terminal — no further transitions
}

// CanTransition checks if moving from → to is a valid state transition.
func CanTransition(from, to PaymentStatus) bool {
	allowed, exists := validTransitions[from]
	if !exists {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// ValidateTransition returns an error if the transition is not allowed.
// Call this in the service layer before every status update.
func ValidateTransition(from, to PaymentStatus) error {
	if !CanTransition(from, to) {
		return fmt.Errorf("invalid transition: cannot move payment from %q to %q", from, to)
	}
	return nil
}

// IsTerminal returns true if the status has no further transitions.
func IsTerminal(status PaymentStatus) bool {
	transitions, exists := validTransitions[status]
	return exists && len(transitions) == 0
}

// IsSuccessful returns true if the payment completed without issues.
func IsSuccessful(status PaymentStatus) bool {
	return status == StatusCompleted || status == StatusRefunded
}
