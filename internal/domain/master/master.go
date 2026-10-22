package masters

import "time"

// MasterKind selects between the two structurally identical lookup tables.
type MasterKind string

const (
	MasterDesignations MasterKind = "designations"
	MasterSkills       MasterKind = "skills"
)

func ParseMasterKind(s string) (MasterKind, bool) {
	switch k := MasterKind(s); k {
	case MasterDesignations, MasterSkills:
		return k, true
	}
	return "", false
}

type MasterEntry struct {
	ID              int64
	Name            string
	IsAdminApproved bool
	CreatedAt       time.Time
}

type Company struct {
	CompanyID    int64
	Name         string
	IsRegistered bool
}

const (
	LevelCountry = "COUNTRY"
	LevelState   = "STATE"
	LevelCity    = "CITY"
)

type Location struct {
	LocationID       int64
	ParentLocationID *int64
	Level            string
	Name             string
}

type LocationFilter struct {
	Q        string
	Level    string
	ParentID *int64
}

type FxRate struct {
	RateDate          string // YYYY-MM-DD
	CurrencyCode      string
	INRConversionRate float64
}

// ---------- designations / skills ----------
type MasterListQuery struct {
	Q     string `form:"q" binding:"max=100"`
	Limit int    `form:"limit" binding:"omitempty,min=1,max=100"`
}

type MasterNameRequest struct {
	Name string `json:"name" binding:"required,max=150"`
}

type MasterEntryResponse struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	IsAdminApproved bool   `json:"is_admin_approved"`
}

// ---------- companies ----------

type CompanySearchQuery struct {
	Q string `form:"q" binding:"max=100"`
}

type CompanyRequest struct {
	Name         string `json:"name" binding:"required,max=200"`
	IsRegistered bool   `json:"is_registered"`
}

type CompanyResponse struct {
	CompanyID    int64  `json:"company_id"`
	Name         string `json:"name"`
	IsRegistered bool   `json:"is_registered"`
}

// ---------- locations ----------

type LocationListQuery struct {
	Q        string `form:"q" binding:"max=100"`
	Level    string `form:"level" binding:"omitempty,oneof=COUNTRY STATE CITY"`
	ParentID *int64 `form:"parent_id" binding:"omitempty,gt=0"`
}

type LocationRequest struct {
	ParentLocationID *int64 `json:"parent_location_id" binding:"omitempty,gt=0"`
	Level            string `json:"level" binding:"required,oneof=COUNTRY STATE CITY"`
	Name             string `json:"name" binding:"required,max=150"`
}

type LocationResponse struct {
	LocationID       int64  `json:"location_id"`
	ParentLocationID *int64 `json:"parent_location_id"`
	Level            string `json:"level"`
	Name             string `json:"name"`
}

// ---------- fx rates ----------

type FxQuery struct {
	Date string `form:"date" binding:"omitempty,datetime=2006-01-02"`
}

type FxRateResponse struct {
	RateDate          string  `json:"rate_date"`
	CurrencyCode      string  `json:"currency_code"`
	INRConversionRate float64 `json:"inr_conversion_rate"`
}

// Rates: {"USD": 83.21, "EUR": 90.5}
type PutFxRatesRequest struct {
	RateDate string             `json:"rate_date" binding:"required,datetime=2006-01-02"`
	Rates    map[string]float64 `json:"rates" binding:"required,min=1,dive,keys,len=3,alpha,endkeys,gt=0"`
}
