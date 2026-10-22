package interactor

import "github.com/h-k741953/effort-tracker/services/api/internal/usecase/port"

// authorizeOwnerEngineer は「Engineer かつ本人」を要求する（実装設計 AC-8-12）。
// 未認証は ErrUnauthenticated、ロールが Engineer でない／識別子不一致は ErrNotOwner。
func authorizeOwnerEngineer(actor port.Actor, engineerID string) error {
	if !actor.Authenticated {
		return port.ErrUnauthenticated
	}
	if actor.Role != port.RoleEngineer || actor.ID != engineerID {
		return port.ErrNotOwner
	}
	return nil
}
