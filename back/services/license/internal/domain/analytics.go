package domain

type Dashboard struct {
	Devices            int          `json:"devices"`
	NewDevices         int          `json:"new_devices"`
	Installations      int          `json:"installations"`
	Recognized         int          `json:"recognized_installations"`
	RecognizedPercent  float64      `json:"recognized_percent"`
	PendingReview      int          `json:"pending_review"`
	Deficit            SeatStats    `json:"license_deficit"`
	Idle               SeatStats    `json:"idle_licenses"`
	Forbidden          FindingStats `json:"forbidden_software"`
	Outdated           FindingStats `json:"outdated_software"`
	ExpiringContracts  int          `json:"expiring_contracts"`
	ExpiringWithinDays int          `json:"expiring_within_days"`
}

type SeatStats struct {
	Products int `json:"products"`
	Seats    int `json:"seats"`
}

type FindingStats struct {
	Installations int `json:"installations"`
	Devices       int `json:"devices"`
}

type FreeSoftware struct {
	ProductID       string        `json:"product_id"`
	ProductName     string        `json:"product_name"`
	Publisher       *string       `json:"publisher"`
	InBPO           bool          `json:"in_bpo"`
	Status          ProductStatus `json:"status"`
	CurrentVersion  *string       `json:"current_version"`
	Devices         int           `json:"devices"`
	OutdatedDevices int           `json:"outdated_devices"`
}
