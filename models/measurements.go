package models

import "time"

type MeasurementValue struct {
	Value *float64 `json:"value"` // pointer to handle null values
}

type Measurement struct {
	Device      string           `json:"device,omitempty"`
	DeviceID    string           `json:"device_id"`
	SiteID      string           `json:"site_id"`
	Time        time.Time        `json:"time,omitempty"`
	PM25        MeasurementValue `json:"pm2_5"`
	PM10        MeasurementValue `json:"pm10"`
	Frequency   string           `json:"frequency,omitempty"`
	NO2         MeasurementValue `json:"no2,omitempty"`
	SiteDetails SiteDetails      `json:"siteDetails"`
}

type SiteDetails struct {
	ID              string  `json:"_id"`
	Description     string  `json:"description,omitempty"`
	Country         string  `json:"country,omitempty"`
	District        string  `json:"district,omitempty"`
	SubCounty       string  `json:"sub_county,omitempty"`
	Parish          string  `json:"parish,omitempty"`
	County          string  `json:"county,omitempty"`
	Name            string  `json:"name,omitempty"`
	City            string  `json:"city,omitempty"`
	FormattedName   string  `json:"formatted_name,omitempty"`
	Region          string  `json:"region,omitempty"`
	Street          string  `json:"street,omitempty"`
	Town            string  `json:"town,omitempty"`
	Village         string  `json:"village,omitempty"`
	SearchName      string  `json:"search_name,omitempty"`
	LocationName    string  `json:"location_name,omitempty"`
	ApproximateLat  float64 `json:"approximate_latitude,omitempty"`
	ApproximateLong float64 `json:"approximate_longitude,omitempty"`
	DataProvider    string  `json:"data_provider,omitempty"`
}

type Meta struct {
	Total     int       `json:"total"`
	Skip      int       `json:"skip"`
	Limit     int       `json:"limit"`
	Page      int       `json:"page"`
	Pages     int       `json:"pages"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
}

type MeasurementResponse struct {
	Success      bool          `json:"success"`
	IsCache      bool          `json:"isCache,omitempty"`
	Message      string        `json:"message"`
	Meta         Meta          `json:"meta,omitempty"`
	Measurements []Measurement `json:"measurements"` // Reusing the Measurements type from earlier
}
