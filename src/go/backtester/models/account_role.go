package models

import "fmt"

// AccountRole is the INTERNAL per-order / per-account tag behind the Broker
// seam, renamed from the legacy LiveAccountType with the same five persisted
// string values (migrate-crossed-enums design amendment). It drives:
//
//   - order routing: AccountRoleReconcilation selects ReconcileTrades /
//     ReconcileOrderID instead of Trades / OrderID
//   - trade linking: AccountRoleSimulator short-circuits trade→order linking
//   - broker selection: AccountRoleMock selects the mock broker, and the
//     Tradier env-var set is chosen per role (LiveAccountVariables)
//   - store map keys: CreateAccountRequestSource identity
//
// AccountRole is NOT operator-facing: it never appears in RPC surfaces or
// playground-creation flows, which speak Mode (mode.go). The persisted
// columns order_records.account_type, live_accounts.account_type and
// playground_sessions.live_account_type carry these exact strings and stay
// byte-identical across the migration.
type AccountRole string

const (
	AccountRolePaper         AccountRole = "paper"
	AccountRoleMargin        AccountRole = "margin"
	AccountRoleReconcilation AccountRole = "reconcilation"
	AccountRoleMock          AccountRole = "mock"
	AccountRoleSimulator     AccountRole = "simulator"
)

// Validate accepts exactly the five legacy-compatible role values.
func (r AccountRole) Validate() error {
	switch r {
	case AccountRolePaper, AccountRoleMargin, AccountRoleReconcilation, AccountRoleMock, AccountRoleSimulator:
		return nil
	default:
		return fmt.Errorf("AccountRole: unsupported account role: %s", r)
	}
}
