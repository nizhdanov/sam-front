package domain

type Dataset string

const (
	DatasetInstallations Dataset = "installations"
	DatasetDevices       Dataset = "devices"
	DatasetProducts      Dataset = "products"
	DatasetContracts     Dataset = "contracts"
	DatasetFindings      Dataset = "findings"
)

const (
	MaxQueryRows  = 100
	MaxExportRows = 10_000
)

type DataQuery struct {
	Dataset      Dataset           `json:"dataset"`
	Filters      map[string]string `json:"filters"`
	GroupBy      string            `json:"group_by"`
	Limit        int               `json:"limit"`
	Export       bool              `json:"-"`
	Admin        bool              `json:"-"`
	DeviceSerial string            `json:"-"`
}

type DataTable struct {
	Dataset Dataset  `json:"dataset"`
	GroupBy string   `json:"group_by,omitempty"`
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
	Total   int      `json:"total"`
	Shown   int      `json:"shown"`
}
