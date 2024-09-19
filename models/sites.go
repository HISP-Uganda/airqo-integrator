package models

import (
	"airqo-integrator/clients"
	"airqo-integrator/config"
	"airqo-integrator/db"
	"encoding/json"
	"github.com/buger/jsonparser"
	log "github.com/sirupsen/logrus"
	"time"
)

type Site struct {
	ID               int64     `json:"id,omitempty" db:"id"`
	UID              string    `json:"_id,omitempty" db:"uid"`
	Name             string    `json:"name,omitempty" db:"name"`
	SearchName       string    `json:"search_name,omitempty" db:"search_name"`
	LocationName     string    `json:"location_name,omitempty" db:"location_name"`
	Country          string    `json:"country,omitempty" db:"country"`
	City             string    `json:"city,omitempty" db:"city"`
	District         string    `json:"district,omitempty" db:"district"`
	County           string    `json:"county,omitempty" db:"county"`
	SubCounty        string    `json:"sub_county,omitempty" db:"sub_county"`
	Region           string    `json:"region,omitempty" db:"region"`
	Longitude        float64   `json:"longitude,omitempty" db:"longitude"`
	Latitude         float64   `json:"latitude,omitempty" db:"latitude"`
	CurrentSubcounty int64     `json:"current_subcounty,omitempty" db:"current_subcounty"`
	Created          time.Time `json:"created,omitempty" db:"created"`
	Updated          time.Time `json:"updated,omitempty" db:"updated"`
	Devices          []Device  `json:"devices,omitempty" db:"devices"`
}

const insertSiteSQL = `
INSERT INTO sites(uid, name, search_name, location_name, country, city, district, county, 
sub_county, region, longitude, latitude, current_subcounty, created, updated)
VALUES(:uid, :name, :search_name, :location_name, :country, :city, :district, :county, 
:sub_county, :region, :longitude, :latitude, :current_subcounty, NOW(), NOW()) RETURNING id
`

// Insert is a method that inserts a new site
func (s *Site) Insert() (int64, error) {
	dbConn := db.GetDB()
	res, err := dbConn.NamedExec(insertSiteSQL, s)
	if err != nil {
		log.WithError(err).Error("Failed to insert site")
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		log.WithError(err).Error("Failed to get last insert ID")
		return 0, err
	}
	return id, nil
}

// Update is a method that updates an existing site
func (s *Site) Update() error {
	dbConn := db.GetDB()
	_, err := dbConn.NamedExec(`
    UPDATE sites SET name = :name, search_name = :search_name, location_name = :location_name, 
    country = :country, city = :city, district = :district, county = :county, sub_county = :sub_county, 
    region = :region, longitude = :longitude, latitude = :latitude, current_subcounty = :current_subcounty, 
    updated = NOW() WHERE id = :id`, s)
	if err != nil {
		log.WithError(err).Error("Failed to update site")
		return err
	}
	return nil
}

// Delete is a method that deletes a site
func (s *Site) Delete() error {
	dbConn := db.GetDB()
	_, err := dbConn.Exec("DELETE FROM sites WHERE id = $1", s.ID)
	if err != nil {
		log.WithError(err).Error("Failed to delete site")
		return err
	}
	return nil
}

// GetSitesByGridUID returns a list of sites in a given grid. use a Join on grid_sites
func GetSitesByGridUID(gridUID string) ([]Site, error) {
	var sites []Site
	dbConn := db.GetDB()
	err := dbConn.Select(&sites, `
    SELECT s.id, s.uid, s.name, s.search_name, s.location_name, s.country, s.city, s.district, s.county, 
    s.sub_county, s.region, s.longitude, s.latitude, s.current_subcounty, s.created, s.updated 
    FROM sites s JOIN grid_sites gs ON s.id = gs.site_id 
    WHERE gs.grid_id = (SELECT id FROM grids WHERE uid = $1)`, gridUID)
	if err != nil {
		log.WithError(err).Error("Failed to get sites by grid UID")
		return nil, err
	}
	return sites, nil
}

// GetSiteByUID returns a site by its unique identifier
func GetSiteByUID(uid string) (*Site, error) {
	var site Site
	dbConn := db.GetDB()
	err := dbConn.Get(&site, `
    SELECT id, uid, name, search_name, location_name, country, city, district, county, sub_county, 
    region, longitude, latitude, current_subcounty, created, updated FROM sites WHERE uid = $1`, uid)
	if err != nil {
		return nil, err
	}
	return &site, nil
}

// GetDevices returns a list of devices in a site
func (s *Site) GetDevices() ([]Device, error) {
	var devices []Device
	dbConn := db.GetDB()
	err := dbConn.Select(&devices, `
    SELECT id, uid, name, site_id, created, updated FROM devices WHERE site_id = $1`, s.ID)
	if err != nil {
		log.WithError(err).Error("Failed to get devices in site")
		return nil, err
	}
	return devices, nil
}

// GetDeviceUIDs returns a []string for UIDs of devices in a site
func (s *Site) GetDeviceUIDs() ([]string, error) {
	var deviceUIDs []string
	dbConn := db.GetDB()
	err := dbConn.Select(&deviceUIDs, `
    SELECT uid FROM devices WHERE site_id = $1`, s.ID)
	if err != nil {
		log.WithError(err).Error("Failed to get device UIDs in site")
		return nil, err
	}
	return deviceUIDs, nil
}

// GetGrids return a list of grids matching a site by referencing the grid_sites table
// check in the grid_sites table for grid_id matching the site_id field. Making a Join
func (s *Site) GetGrids() ([]Grid, error) {
	var grids []Grid
	dbConn := db.GetDB()
	err := dbConn.Select(&grids, `
    SELECT g.* FROM grids g 
    INNER JOIN grid_sites gs ON g.id = gs.grid_id WHERE gs.site_id = $1`, s.ID)
	if err != nil {
		log.WithError(err).Error("Failed to get grids for site")
		return nil, err
	}
	return grids, nil
}

func fetchSitesFromAPI() ([]Site, error) {
	params := map[string]string{
		"token": config.AirQoIntegratorConf.API.AIRQOToken,
		// "limit": "1",
		// "page":  "1",
	}
	resp, err := clients.AirQoClient.GetResource("/devices/sites", params)
	if err != nil {
		log.WithError(err).Error("Failed to get sites")
		return nil, err
	}

	v, _, _, err := jsonparser.Get(resp.Body(), "sites")
	if err != nil {
		log.WithError(err).Error("json parser failed to get sites key")
		return nil, err
	}
	var sites []Site
	err = json.Unmarshal(v, &sites)
	if err != nil {
		log.WithError(err).Error("Error unmarshalling response body:")
		return nil, err
	}
	// log.WithFields(log.Fields{"Grids": grids}).Info("Fetched Grids")
	// resp, err := client.
	return sites, nil
}
