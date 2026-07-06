package models

import (
	"fmt"
	"strings"
)

// CompanionStopTag marks broker orders as companion stops (broker-held
// protective exits placed by the live fill pipeline). Every companion stop's
// tag is either exactly this value or CompanionStopTagForEntry's
// "companion-stop-<entryOrderID>" form. The tag is the recursion guard (a
// companion stop's own fill must never spawn another companion stop) and the
// durable per-entry idempotency association, so the prefix is RESERVED:
// client-supplied orders carrying it are rejected at every order-placement
// ingress (CreateOrderRequest.Validate) and at the PlaceOrder RPC boundary.
//
// Canonical home is the models package so the request-validation seam can
// enforce the reservation without an import cycle; the safety package
// re-exports these for its callers.
const CompanionStopTag = "companion-stop"

// CompanionStopTagForEntry returns the tag carrying the durable association
// between a companion stop and the entry order it protects.
func CompanionStopTagForEntry(entryOrderID uint) string {
	return fmt.Sprintf("%s-%d", CompanionStopTag, entryOrderID)
}

// IsCompanionStopOrderTag reports whether a tag identifies a companion-stop
// order (either the bare tag or the per-entry form).
func IsCompanionStopOrderTag(tag string) bool {
	return tag == CompanionStopTag || strings.HasPrefix(tag, CompanionStopTag+"-")
}
