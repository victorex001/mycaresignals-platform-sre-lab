package security

// Role is an exact Phase 019 staff/home role. There is deliberately no
// hierarchy: callers must list every role they intend to authorize.
type Role string

const (
	RoleKeyWorker             Role = "key_worker"
	RoleSenior                Role = "senior"
	RoleManager               Role = "manager"
	RoleResponsibleIndividual Role = "responsible_individual"
	RoleAdmin                 Role = "admin"
	RoleDemoViewer            Role = "demo_viewer"
)

func (r Role) Valid() bool {
	switch r {
	case RoleKeyWorker, RoleSenior, RoleManager, RoleResponsibleIndividual, RoleAdmin, RoleDemoViewer:
		return true
	default:
		return false
	}
}
