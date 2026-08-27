// Package docaudit implements the repository-only documentation/source graph
// audit. It never renders documentation or changes product state.
package docaudit

type Grade string

const (
	GradeExecuted   Grade = "executed"
	GradeGenerated  Grade = "generated"
	GradeClaimed    Grade = "claimed"
	GradeUnverified Grade = "unverified"
)

type Registry struct {
	Schema      string              `json:"schema"`
	Claims      []RegisteredClaim   `json:"claims"`
	Executables []RegisteredExample `json:"executables"`
	Journeys    []RegisteredJourney `json:"journeys"`
}

type RegisteredClaim struct {
	ID          string `json:"id"`
	CheckAnchor string `json:"check_anchor"`
}

type RegisteredExample struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Section  string `json:"section"`
	Language string `json:"language"`
	SHA256   string `json:"sha256"`
	State    Grade  `json:"state"`
	Check    string `json:"check_anchor,omitempty"`
}

type RegisteredJourney struct {
	ID              string `json:"id"`
	Owner           string `json:"owner"`
	Path            string `json:"path"`
	Section         string `json:"section"`
	ProposedActions int    `json:"proposed_actions"`
	State           Grade  `json:"state"`
	Check           string `json:"check_anchor,omitempty"`
}

type Inventory struct {
	Schema        string              `json:"schema"`
	Documents     []InventoryDocument `json:"documents"`
	Surfaces      []InventorySurface  `json:"surfaces"`
	GradeBaseline struct {
		Denominator int `json:"denominator"`
	} `json:"grade_baseline"`
}

type InventoryDocument struct {
	Path     string             `json:"path"`
	Sections []InventorySection `json:"sections"`
}

type InventorySection struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type InventorySurface struct {
	ID               string             `json:"id"`
	Count            int                `json:"count"`
	Unit             string             `json:"unit"`
	OperationIDs     *int               `json:"operation_ids,omitempty"`
	Owner            string             `json:"owner"`
	Finding          string             `json:"finding,omitempty"`
	MeasurementState string             `json:"measurement_state,omitempty"`
	Journeys         []InventoryJourney `json:"journeys,omitempty"`
}

type InventoryJourney struct {
	ID              string `json:"id"`
	Owner           string `json:"owner"`
	ProposedActions int    `json:"proposed_actions"`
}

type Fragment struct {
	ID          string
	Audience    string
	Anchor      string
	Topic       string
	Section     string
	Enumeration string
	ClaimID     string
	ClaimCheck  string
	Prose       string
}

type ExampleCandidate struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Section  string `json:"section"`
	Language string `json:"language"`
	SHA256   string `json:"sha256"`
}

type TopicReport struct {
	Topic     string         `json:"topic"`
	Path      string         `json:"path"`
	Counts    map[Grade]int  `json:"counts"`
	Sections  []SectionGrade `json:"sections"`
	Fragments []UnitGrade    `json:"fragments,omitempty"`
	Examples  []UnitGrade    `json:"examples,omitempty"`
	Journeys  []UnitGrade    `json:"journeys,omitempty"`
}

type SectionGrade struct {
	ID    string `json:"id"`
	Grade Grade  `json:"grade"`
}

type UnitGrade struct {
	ID      string `json:"id"`
	Section string `json:"section"`
	Grade   Grade  `json:"grade"`
}

type Report struct {
	Schema         string          `json:"schema"`
	Counts         map[Grade]int   `json:"counts"`
	Denominator    int             `json:"denominator"`
	ManualSections int             `json:"manual_sections"`
	Fragments      int             `json:"fragments"`
	Claims         int             `json:"claims"`
	Executables    int             `json:"executables"`
	Journeys       int             `json:"journeys"`
	Surfaces       []SurfaceReport `json:"surfaces"`
	TopicReports   []TopicReport   `json:"topics"`
}

type SurfaceReport struct {
	ID               string `json:"id"`
	Count            int    `json:"count"`
	Unit             string `json:"unit"`
	OperationIDs     *int   `json:"operation_ids,omitempty"`
	Owner            string `json:"owner"`
	Finding          string `json:"finding,omitempty"`
	MeasurementState string `json:"measurement_state,omitempty"`
}

type Options struct {
	Root          string
	InventoryPath string
	RegistryPath  string
	TSResolver    func(root string, anchors []string) error
}
