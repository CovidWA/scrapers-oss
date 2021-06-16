package csg

const DOHAvailable = "AVAILABLE"
const DOHUnavailable = "UNAVAILABLE"
const DOHUnknown = "UNKNOWN"
const DOHDateTimeFormat = "2006-01-02T15:04:05.000Z07:00"

type DOHApiResp struct {
	Data DOHApiData `json:"data"`
}

type DOHApiData struct {
	SearchLocations DOHApiSearchLocations `'json:"searchLocations"`
}

type DOHApiSearchLocations struct {
	Locations []DOHApiLocation `json:"locations"`
}

type DOHApiLocation struct {
	LocationId   string  `json:"locationId"`
	ZipCode      string  `json:"zipcode"`
	Lat          float64 `json:"latitude"`
	Lng          float64 `json:"longitude"`
	Availability string  `json:"vaccineAvailability"`
	UpdatedAt    string  `json:"updatedAt"`
}
