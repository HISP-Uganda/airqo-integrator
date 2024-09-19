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

type Grid struct {
	ID         int64     `json:"id,omitempty" db:"id"`
	UID        string    `json:"_id,omitempty" db:"uid"`
	Name       string    `json:"name,omitempty" db:"name"`
	AdminLevel string    `json:"admin_level,omitempty" db:"admin_level"`
	InScope    bool      `json:"in_scope,omitempty" db:"in_scope"`
	Created    time.Time `json:"created,omitempty" db:"created"`
	Updated    time.Time `json:"updated,omitempty" db:"updated"`
	Sites      []Site    `json:"sites,omitempty"`
	Devices    []Device  `json:"devices,omitempty"`
}

const insertGridSQL = `
INSERT INTO grids(uid, name, admin_level, in_scope, created, updated)
VALUES(:uid, :name, :admin_level, TRUE, NOW(), NOW()) RETURNING  id
`

func (g *Grid) Insert() (int64, error) {
	dbConn := db.GetDB()
	rows, err := dbConn.NamedQuery(insertGridSQL, g)
	if err != nil {
		log.WithError(err).Error("Failed to insert grid")
		return 0, err
	}
	for rows.Next() {
		var gridId int64
		_ = rows.Scan(&gridId)
		g.ID = gridId
	}
	_ = rows.Close()
	return g.ID, nil
}

func (g *Grid) Update() error {
	dbConn := db.GetDB()
	_, err := dbConn.NamedExec(`
	UPDATE grids SET name = :name, admin_level = :admin_level, in_scope = TRUE, 
	updated = NOW() WHERE id = :id`, g)
	if err != nil {
		log.WithError(err).Error("Failed to update grid")
		return err
	}
	return nil
}

// Delete removes a grid from database
func (g *Grid) Delete() error {
	dbConn := db.GetDB()
	_, err := dbConn.Exec(`DELETE FROM grids WHERE id = $1`, g.ID)
	if err != nil {
		log.WithError(err).Error("Failed to delete grid")
		return err
	}
	return nil
}

// GetSites returns a list of sites in a grid by referencing the grid_sites table
// check in the grid_sites table for site_id matching the grid_Id field
func (g *Grid) GetSites() ([]Site, error) {
	var sites []Site
	dbConn := db.GetDB()
	err := dbConn.Select(&sites, `
    SELECT s.* FROM sites s
    JOIN grid_sites gs ON s.id = gs.site_id
    WHERE gs.grid_id = $1`, g.ID)
	if err != nil {
		log.WithError(err).Error("Failed to get sites in grid")
		return nil, err
	}
	return sites, nil
}

// GetGridByUID is a function that returns a Grid matching a UID field
func GetGridByUID(uid string) (*Grid, error) {
	var grid Grid
	dbConn := db.GetDB()
	err := dbConn.Get(&grid, `SELECT * FROM grids WHERE uid = $1`, uid)
	if err != nil {
		return nil, err
	}
	return &grid, nil
}

// GetGridIDByUID is a function that returns ID of a Grid matching a UID field
func GetGridIDByUID(uid string) (int64, error) {
	var gridID int64
	dbConn := db.GetDB()
	err := dbConn.Get(&gridID, `SELECT id FROM grids WHERE uid = $1`, uid)
	if err != nil {
		return 0, err
	}
	return gridID, nil
}

// GetDevices returns devices in a grid
func (g *Grid) GetDevices() ([]Device, error) {
	var devices []Device
	dbConn := db.GetDB()
	err := dbConn.Select(&devices, `
    SELECT d.* FROM devices d
    JOIN grid_devices gd ON d.id = gd.device_id
    WHERE gd.grid_id = $1`, g.ID)
	if err != nil {
		log.WithError(err).Error("Failed to get devices in grid")
		return nil, err
	}
	return devices, nil
}

// GetDeviceUIDs returns []string for UIDs of devices in a grid
func (g *Grid) GetDeviceUIDs() ([]string, error) {
	var deviceUIDs []string
	dbConn := db.GetDB()
	err := dbConn.Select(&deviceUIDs, `
    SELECT uid FROM devices d
    JOIN grid_devices gd ON d.id = gd.device_id
    WHERE gd.grid_id = $1`, g.ID)
	if err != nil {
		log.WithError(err).Error("Failed to get device UIDs in grid")
		return nil, err
	}
	return deviceUIDs, nil
}

// GetSiteUIDs returns []string for UIDs of sites in a grid
func (g *Grid) GetSiteUIDs() ([]string, error) {
	var siteUIDs []string
	dbConn := db.GetDB()
	err := dbConn.Select(&siteUIDs, `
    SELECT uid FROM sites s
    JOIN grid_sites gs ON s.id = gs.site_id
    WHERE gs.grid_id = $1`, g.ID)
	if err != nil {
		log.WithError(err).Error("Failed to get site UIDs in grid")
		return nil, err
	}
	return siteUIDs, nil
}

// GetGridsInScope returns a list of grids in scope
func GetGridsInScope() ([]Grid, error) {
	var grids []Grid
	dbConn := db.GetDB()
	err := dbConn.Select(&grids, `SELECT * FROM grids WHERE in_scope = TRUE`)
	if err != nil {
		log.WithError(err).Error("Failed to get grids in scope")
		return nil, err
	}
	return grids, nil
}

// LoadGrids is a function that fetches grids from an external API and adds them to the database if not already present
func LoadGrids() error {
	log.Info("Loading grids......")
	grids, err := fetchGridsFromAPI()
	if err != nil {
		return err
	}

	// dbConn := db.GetDB()
	for _, grid := range grids {
		// log.Infof("Grid: %d Sites: %v", grid.ID, grid.Sites)
		g, err := GetGridByUID(grid.UID)
		if err == nil && g != nil {
			// Grid already exists, update it
			err = g.Update()
			if err != nil {
				return err
			}
		} else {
			// Grid does not exist, insert it
			id, err := grid.Insert()
			if err != nil {
				return err
			}
			log.Infof("Inserted grid with ID: %d", id)
		}
	}
	return err
}

// fetchGridsFromAPI fetches grids from AirQo API
func fetchGridsFromAPI() ([]Grid, error) {
	params := map[string]string{
		"token": config.AirQoIntegratorConf.API.AIRQOToken,
		// "limit": "1",
		// "page":  "1",
	}
	resp, err := clients.AirQoClient.GetResource("/devices/grids/summary", params)
	if err != nil {
		log.WithError(err).Error("Failed to get grids")
		return nil, err
	}

	v, _, _, err := jsonparser.Get(resp.Body(), "grids")
	if err != nil {
		log.WithError(err).Error("json parser failed to get grids key")
		return nil, err
	}
	var grids []Grid
	err = json.Unmarshal(v, &grids)
	if err != nil {
		log.WithError(err).Error("Error unmarshalling response body:")
		return nil, err
	}
	// log.WithFields(log.Fields{"Grids": grids}).Info("Fetched Grids")
	// resp, err := client.
	return grids, nil
}
