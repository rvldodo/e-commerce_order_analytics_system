package dto

import "time"

type ReportResult struct {
	ReportType  string      `json:"report_type,omitempty"`
	Date        string      `json:"date,omitempty"`
	Data        interface{} `json:"data,omitempty"`
	GeneratedAt time.Time   `json:"generated_at,omitempty"`
}
