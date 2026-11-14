package domain

import "github.com/google/uuid"

type Dashboard struct {
	Devices    DeviceStats     `json:"devices"`
	Software   SoftwareStats   `json:"software"`
	Licenses   *LicenseStats   `json:"licenses,omitempty"`
	Compliance ComplianceStats `json:"compliance"`
}

type DeviceStats struct {
	Total      int `json:"total"`
	Assigned   int `json:"assigned"`
	Unassigned int `json:"unassigned"`
}

type SoftwareStats struct {
	ProductsTotal        int `json:"products_total"`
	InstallationsTotal   int `json:"installations_total"`
	UnknownInstallations int `json:"unknown_installations"`
	PendingReview        int `json:"pending_review"`
}

type LicenseStats struct {
	ProductsWithDeficit int `json:"products_with_deficit"`
	ProductsUnverified  int `json:"products_unverified"`
	TotalDeficit        int `json:"total_deficit"`
}

type ComplianceStats struct {
	OpenFindings int `json:"open_findings"`
	Critical     int `json:"critical"`
	High         int `json:"high"`
	Medium       int `json:"medium"`
	Low          int `json:"low"`
}

type SoftwareUsage struct {
	ProductID     uuid.UUID    `json:"product_id"`
	Name          string       `json:"name"`
	Publisher     *string      `json:"publisher"`
	PolicyStatus  PolicyStatus `json:"policy_status"`
	Devices       int          `json:"devices"`
	Installations int          `json:"installations"`
}

type DepartmentStats struct {
	Department    string `json:"department"`
	Employees     int    `json:"employees"`
	Devices       int    `json:"devices"`
	Installations int    `json:"installations"`
	OpenFindings  int    `json:"open_findings"`
}
