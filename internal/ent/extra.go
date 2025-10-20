package ent

func (r *Role) IsSystemRole() bool {
	return r.ProjectID == nil || *r.ProjectID == 0
}
