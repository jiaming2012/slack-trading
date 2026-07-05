package safety

import (
	"errors"
	"fmt"
	"sync"
)

// ErrHalted is the sentinel that every halt rejection wraps. Callers at the
// Broker seam and in the RPC layer test for it with errors.Is(err, ErrHalted)
// to recognise the single new "rejected-because-halted" failure mode without
// depending on the concrete HaltError type.
var ErrHalted = errors.New("order submission halted by kill switch")

// ErrAckRequired is returned by Release when an auto-halt is still awaiting the
// operator's cooldown acknowledgment. It enforces the rule that an auto-halt
// never clears without an explicit acknowledgment first.
var ErrAckRequired = errors.New("cooldown acknowledgment required before release")

// HaltError is the concrete error returned by AllowOrder while engaged. It
// carries the reason and source so the rejection is self-describing, and it
// reports errors.Is(err, ErrHalted) == true.
type HaltError struct {
	Reason string
	Source Source
}

func (e *HaltError) Error() string {
	return fmt.Sprintf("%v (source=%s): %s", ErrHalted, e.Source, e.Reason)
}

// Is lets errors.Is(err, ErrHalted) succeed for any HaltError.
func (e *HaltError) Is(target error) bool {
	return target == ErrHalted
}

// HaltController is the single source of truth for whether order submission is
// permitted. The manual kill switch and every anomaly guard converge on this
// one object, so there is exactly one chokepoint (AllowOrder, consulted at the
// Broker seam) and one status surface. All transitions are mutex-guarded and
// persisted through a HaltStateStore, so a restart resumes the prior state.
type HaltController struct {
	mu    sync.Mutex
	state HaltState
	store HaltStateStore

	// listener, when set, is invoked with a copy of the new state after every
	// transition (engage, auto-engage, acknowledge, release) — the hook the
	// halt-engaged telemetry gauge hangs off. It is called while the
	// controller mutex is held, so it MUST NOT call back into the controller.
	listener func(HaltState)
}

// SetTransitionListener installs the transition listener. The listener is
// invoked under the controller mutex and must not call controller methods.
// Passing nil removes it.
func (c *HaltController) SetTransitionListener(fn func(HaltState)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.listener = fn
}

// notifyLocked invokes the transition listener with the current state.
// Callers hold c.mu.
func (c *HaltController) notifyLocked() {
	if c.listener != nil {
		c.listener(c.state)
	}
}

// NewHaltController constructs a controller whose initial state is loaded from
// store. A store with nothing persisted yields a clear controller. A store that
// fails to load (e.g. a corrupt file) returns an error rather than silently
// starting clear — refusing to start is safer than losing a halt.
func NewHaltController(store HaltStateStore) (*HaltController, error) {
	state, err := store.Load()
	if err != nil {
		return nil, fmt.Errorf("NewHaltController: load persisted state: %w", err)
	}
	return &HaltController{state: state, store: store}, nil
}

// AllowOrder returns nil when order submission is permitted and a *HaltError
// (matching ErrHalted) when the controller is engaged. It is the check consulted
// at the Broker seam before any order in any Mode reaches a Broker.
func (c *HaltController) AllowOrder() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state.Engaged {
		return &HaltError{Reason: c.state.Reason, Source: c.state.Source}
	}
	return nil
}

// Engage moves the controller to the engaged state as a manual halt. A manual
// halt carries no cooldown requirement and may be released directly.
//
// A manual engage MUST NOT weaken an active auto-halt. If an auto-halt is still
// awaiting acknowledgment, Engage retains the cooldown-ack requirement, keeps
// the source automatic, and preserves the auto reason (appending the manual
// reason) rather than laundering the anomaly into a freely-releasable manual
// halt. This is the mirror of EngageAuto escalating over a manual halt: neither
// direction ever transitions an auto-halt back toward clear without an explicit
// acknowledgment, per the halt-cooldown-protocol spec. Engaging over a manual
// halt (or while clear) overwrites the reason and marks the halt manual.
func (c *HaltController) Engage(reason string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state.Engaged && c.state.Source == SourceAuto && c.state.AckRequired {
		// Preserve the auto-halt's ack requirement and source; retain the auto
		// reason and append the manual engage so no context is destroyed.
		c.state.Reason = fmt.Sprintf("%s; manual engage: %s", c.state.Reason, reason)
		c.notifyLocked()
		return c.persist()
	}
	c.state = HaltState{
		Engaged:     true,
		Reason:      reason,
		Source:      SourceManual,
		AckRequired: false,
	}
	c.notifyLocked()
	return c.persist()
}

// EngageAuto moves the controller to the engaged state as an automatic halt and
// sets the cooldown-acknowledgment requirement. It is idempotent while an
// auto-halt is already awaiting acknowledgment (repeated guard trips do not
// thrash persistence). An auto-halt over an existing manual halt escalates the
// halt to require acknowledgment — an anomaly is never quietly cleared.
func (c *HaltController) EngageAuto(reason string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state.Engaged && c.state.Source == SourceAuto && c.state.AckRequired {
		return nil
	}
	c.state = HaltState{
		Engaged:     true,
		Reason:      reason,
		Source:      SourceAuto,
		AckRequired: true,
	}
	c.notifyLocked()
	return c.persist()
}

// Acknowledge clears the cooldown-acknowledgment requirement without releasing
// the halt. It is the operator's explicit "I have seen the anomaly" step that
// must precede Release for an auto-halt. Acknowledging while clear is an error.
func (c *HaltController) Acknowledge() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.state.Engaged {
		return fmt.Errorf("Acknowledge: controller is not engaged")
	}
	if !c.state.AckRequired {
		return nil
	}
	c.state.AckRequired = false
	c.notifyLocked()
	return c.persist()
}

// Release moves the controller to the clear state. It refuses (ErrAckRequired)
// while an auto-halt still needs acknowledgment. Releasing while already clear
// is a no-op. The controller never returns to clear by any other path — it does
// not self-clear when an anomaly subsides.
func (c *HaltController) Release() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.state.Engaged {
		return nil
	}
	if c.state.AckRequired {
		return ErrAckRequired
	}
	c.state = HaltState{}
	c.notifyLocked()
	return c.persist()
}

// Status returns a copy of the current state for the status surface.
func (c *HaltController) Status() HaltState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// persist writes the current state through the store. Callers hold c.mu.
func (c *HaltController) persist() error {
	if c.store == nil {
		return nil
	}
	if err := c.store.Save(c.state); err != nil {
		return fmt.Errorf("HaltController: persist state: %w", err)
	}
	return nil
}
