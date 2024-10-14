package models

import (
	"airqo-integrator/clients"
	"airqo-integrator/config"
	"airqo-integrator/db"
	"database/sql"
	"encoding/json"
	"fmt"
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
	Devices          []Device  `json:"devices,omitempty"`
	Grids            []Grid    `json:"grids,omitempty"`
}

const insertSiteSQL = `
INSERT INTO sites(uid, name, search_name, location_name, country, city, district, county, 
sub_county, region, longitude, latitude, current_subcounty, created, updated)
VALUES(:uid, :name, :search_name, :location_name, :country, :city, :district, :county, 
:sub_county, :region, :longitude, :latitude, :current_subcounty, NOW(), NOW()) 
 ON CONFLICT (uid) DO NOTHING RETURNING id
`

// Insert is a method that inserts a new site
func (s *Site) Insert() (int64, error) {
	dbConn := db.GetDB()
	rows, err := dbConn.NamedQuery(insertSiteSQL, s)
	if err != nil {
		log.WithError(err).Error("Failed to insert site")
		return 0, err
	}
	for rows.Next() {
		var gridId int64
		_ = rows.Scan(&gridId)
		s.ID = gridId
	}
	_ = rows.Close()
	return s.ID, nil
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

// InsertOrUpdate is a method that updates an existing site or creates if missing
func (s *Site) InsertOrUpdate() error {
	if s.ID == 0 {
		_, err := s.Insert()
		return err
	}
	s.ID = s.DbID()
	return s.Update()
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

// DbID retrieves the ID from the database for a site given its UID
func (s *Site) DbID() int64 {
	dbConn := db.GetDB()
	var id sql.NullInt64
	err := dbConn.Get(&id, `SELECT id FROM sites WHERE uid = $1`, s.UID)
	if err != nil {
		log.WithError(err).Infof("Failed to get ID of site with UID: %v", s.UID)
	}
	return id.Int64
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
		// log.WithError(err).Error("Failed to get sites by grid UID")
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

// GetDhis2District returns the organisationunit table matching site's district after appending ' District' to site's district field
// and ensuring match is made for hierarchylevel = 3 in organisationunit table. Also the site's country should be = Uganda

func (s *Site) GetDhis2District() (int64, error) {
	dbConn := db.GetDB()
	if s.Country == "Uganda" {
		var district int64
		err := dbConn.Get(&district, `
   	SELECT id FROM organisationunit WHERE name = $1 AND hierarchylevel = 3`, s.District+" District")
		if err != nil {
			return 0, err
		}
		return district, nil
	}
	return 0, fmt.Errorf("Country %s not supported or site's district not found", s.Country)

}

// GetSiteDistricts returns a slice of distinct dhis2_districts from the sites table
func GetSiteDistricts() ([]int64, error) {
	var districts []int64
	dbConn := db.GetDB()
	err := dbConn.Select(&districts, `
    SELECT DISTINCT dhis2_district FROM sites 
	WHERE dhis2_district IS NOT NULL AND dhis2_district > 0`)
	if err != nil {
		return nil, err
	}
	return districts, nil
}

// GetSubCountiesByDhis2District returns a slice of int64 matching current_subcounty given dhis2_district from sites table
func GetSubCountiesByDhis2District(districtID int64) ([]int64, error) {
	var subCounties []int64
	dbConn := db.GetDB()
	err := dbConn.Select(&subCounties, `
    SELECT DISTINCT current_subcounty FROM sites 
		WHERE dhis2_district = $1 AND current_subcounty > 0`, districtID)
	if err != nil {
		return nil, err
	}
	return subCounties, nil
}

// GetSitesByCurrentSubCounty returns a slice of int64 matching current_subcounty
func GetSitesByCurrentSubCounty(subcountyID int64) ([]Site, error) {
	var sites []Site
	dbConn := db.GetDB()
	err := dbConn.Select(&sites, `
    SELECT id,uid FROM sites WHERE current_subcounty = $1`, subcountyID)
	if err != nil {
		return nil, err
	}
	return sites, nil
}

// UpdateDhis2District updates the site's dhis2_district field given an int64 representing district
func (s *Site) UpdateDhis2District(districtID int64) error {
	dbConn := db.GetDB()
	_, err := dbConn.Exec("UPDATE sites SET dhis2_district = $1 WHERE uid = $2", districtID, s.UID)
	if err != nil {
		// log.WithError(err).Errorf("Failed to update dhis2_district in site: %v", s.UID)
		return err
	}
	return nil
}

// UpdateCurrentSubCounty updates the site's current_subcounty field given an int64 representing subcounty
func (s *Site) UpdateCurrentSubCounty(subcountyID int64) error {
	dbConn := db.GetDB()
	_, err := dbConn.Exec("UPDATE sites SET current_subcounty = $1 WHERE uid = $2", subcountyID, s.UID)
	if err != nil {
		// log.WithError(err).Errorf("Failed to update current_subcounty in site: %v", s.UID)
		return err
	}
	return nil
}

// LoadSites ...
func LoadSites() error {
	log.Infof("Loading sites from API...")
	sites, err := fetchSitesFromAPI()
	if err != nil {
		return err
	}
	for _, site := range sites {
		// Insert or update site in the database
		// log.Infof("Read Site: %v", site.UID)

		err = site.InsertOrUpdate()
		if err != nil {
			continue
			// return err
		}
		site.ID = site.DbID()
		for _, device := range site.Devices {
			device.SiteID = site.ID
			err = device.InsertOrUpdate()
			if err != nil {
				continue
				// return err
			}
			device.ID = device.DbID()
			// log.Infof("Site: %v, Device: %v", site.UID, device.UID)

		}
		dhis2District, er := site.GetDhis2District()
		if er == nil {
			// log.Infof("Site: %v, District: %v, DistrictID: %v", site.UID, site.District, dhis2District)
			err = site.UpdateDhis2District(dhis2District)
			if err != nil {
				continue
			}
		}
		if dhis2District > 0 {
			// log.Infof("Site: %v and DistrictID: %v, District: %v", site.UID, dhis2District)
			subCounties, _ := OrgUnitChildren(dhis2District)
			// log.Infof("District: %v Suncounties: %v", dhis2District, subCounties)
			for _, sc := range subCounties {
				isInOu, _ := IsPointInOrganisationUnit(sc, site.Longitude, site.Latitude)
				if isInOu {
					err = site.UpdateCurrentSubCounty(sc)
					if err != nil {
						continue
					}
				}
			}

		}

	}
	log.Infof("Done loading Sites...")
	return nil
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

// FetchSiteMeasurements ...
func FetchSiteMeasurements(site string, startDate, endDate time.Time) (MeasurementResponse, error) {
	// turn startDate into a string
	startDateStr := startDate.Format("2006-01-02")
	endDateStr := endDate.Format("2006-01-02")

	params := map[string]string{
		"token":     config.AirQoIntegratorConf.API.AIRQOToken,
		"startTime": startDateStr,
		"endTime":   endDateStr,
	}

	resp, err := clients.AirQoClient.GetResource("/devices/measurements/sites/"+site+"/historical", params)
	if err != nil {
		log.WithError(err).Error("Failed to get site measurements")
		return MeasurementResponse{}, err
	}
	var mrs MeasurementResponse
	err = json.Unmarshal(resp.Body(), &mrs)
	// log.Infof("Site Measurements: %v", string(resp.Body()))
	return mrs, err
}
