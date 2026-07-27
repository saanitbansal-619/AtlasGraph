package recommendations

// Priority is a qualitative urgency band for ranked recommendations.
type Priority string

const (
	PriorityCritical Priority = "Critical"
	PriorityHigh     Priority = "High"
	PriorityMedium   Priority = "Medium"
	PriorityLow      Priority = "Low"
)

// Category groups recommendations into analyst-facing mitigation themes.
type Category string

const (
	CategorySupplierDiversification Category = "Supplier Diversification"
	CategoryInventoryIncrease       Category = "Inventory Increase"
	CategoryStrategicStockpiling    Category = "Strategic Stockpiling"
	CategoryAlternativeSourcing     Category = "Alternative Sourcing"
	CategoryLogisticsRerouting      Category = "Logistics Rerouting"
	CategorySupplierMonitoring      Category = "Supplier Monitoring"
	CategoryCountryRiskMonitoring   Category = "Country Risk Monitoring"
	CategoryContractHedging         Category = "Contract Hedging"
	CategoryDemandReduction         Category = "Demand Reduction"
	CategoryDualSourcing            Category = "Dual Sourcing"
)

// Difficulty and Confidence use the same qualitative bands.
type Difficulty string
type Confidence string

const (
	DifficultyLow    Difficulty = "Low"
	DifficultyMedium Difficulty = "Medium"
	DifficultyHigh   Difficulty = "High"

	ConfidenceLow    Confidence = "Low"
	ConfidenceMedium Confidence = "Medium"
	ConfidenceHigh   Confidence = "High"
)

// SupportingMetrics holds numeric evidence cited by a recommendation.
type SupportingMetrics map[string]float64

// Recommendation is a single deterministic mitigation action.
type Recommendation struct {
	ID                       string            `json:"id"`
	Title                    string            `json:"title"`
	Description              string            `json:"description"`
	Priority                 Priority          `json:"priority"`
	Category                 Category          `json:"category"`
	Reason                   string            `json:"reason"`
	ExpectedImpact           string            `json:"expected_impact"`
	ImplementationDifficulty Difficulty        `json:"implementation_difficulty"`
	Confidence               Confidence        `json:"confidence"`
	SupportingMetrics        SupportingMetrics `json:"supporting_metrics"`
}

// PriorityDistribution counts recommendations by urgency band.
type PriorityDistribution struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
}

// ExecutiveActionPlan is the ranked mitigation output for a scenario.
type ExecutiveActionPlan struct {
	Summary                 string               `json:"summary"`
	Recommendations         []Recommendation     `json:"recommendations"`
	PriorityDistribution    PriorityDistribution `json:"priority_distribution"`
	EstimatedBusinessImpact string               `json:"estimated_business_impact"`
	SupportingEvidence      []string             `json:"supporting_evidence"`
}

// Input holds the simulation and analytics signals used by rule evaluation.
type Input struct {
	Source              string
	Commodity           string
	ShockType           string
	DropPercent         float64
	Depth               int
	HHI                 *float64
	SupplierShare       *float64
	TradeConcentration  string
	EventRisk           float64
	MacroExposure       float64
	FragilityScore      float64
	InventoryBufferDays *int
	AffectedPaths       int
	AffectedNodes       int
}
