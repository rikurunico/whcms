package install

// CreateAdminRequest is the first-run admin account creation request
// (docs/CONTRACTS.md §15).
type CreateAdminRequest struct {
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

// SiteSettingsRequest is the optional first-run site settings step. Any
// field left blank is simply not written - existing/default values stand.
type SiteSettingsRequest struct {
	CompanyName    string `json:"company_name" validate:"max=150"`
	CompanyEmail   string `json:"company_email" validate:"omitempty,email"`
	CompanyAddress string `json:"company_address" validate:"max=200"`
}

// StatusResponse reports installation state to the frontend wizard. Mirrors
// the bootstrap phase's own status shape (internal/installer) so the wizard
// can treat both phases uniformly - ConfigReady is always true here, since
// this module only ever mounts once config/DB are up.
type StatusResponse struct {
	ConfigReady bool `json:"config_ready"`
	Installed   bool `json:"installed"`
}
