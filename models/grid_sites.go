package models

import (
	"airqo-integrator/db"
	log "github.com/sirupsen/logrus"
	"time"
)

type GridSite struct {
	ID      int64     `json:"id,omitempty" db:"id"`
	GridID  int64     `json:"grid_id,omitempty" db:"grid_id"`
	SiteID  int64     `json:"site_id,omitempty" db:"site_id"`
	Created time.Time `json:"created,omitempty" db:"created"`
	Updated time.Time `json:"updated,omitempty" db:"updated"`
}

// Insert ..
func (gs *GridSite) Insert() error {
	dbConn := db.GetDB()
	_, err := dbConn.NamedExec(`
    INSERT INTO grid_sites(grid_id, site_id, created, updated)
    VALUES(:grid_id, :site_id, NOW(), NOW())`, gs)
	if err != nil {
		log.WithError(err).Error("Failed to insert grid site")
		return err
	}
	return nil
}

// Delete ..
func (gs *GridSite) Delete() error {
	dbConn := db.GetDB()
	_, err := dbConn.Exec("DELETE FROM grid_sites WHERE grid_id = $1 AND site_id = $2", gs.GridID, gs.SiteID)
	if err != nil {
		log.WithError(err).Error("Failed to delete grid site")
		return err
	}
	return nil
}

// InsertGridSite ...
func InsertGridSite(gridID, siteID int64) error {
	gs := &GridSite{GridID: gridID, SiteID: siteID}
	return gs.Insert()
}

// DeleteGridSite ...
func DeleteGridSite(gridID, siteID int64) error {
	gs := &GridSite{GridID: gridID, SiteID: siteID}
	return gs.Delete()
}

// GetGridSitesByGridID ...
func GetGridSitesByGridID(gridID int64) ([]GridSite, error) {
	var gridSites []GridSite
	dbConn := db.GetDB()
	err := dbConn.Select(&gridSites, "SELECT * FROM grid_sites WHERE grid_id = $1", gridID)
	if err != nil {
		log.WithError(err).Error("Failed to get grid sites by grid ID")
		return nil, err
	}
	return gridSites, nil
}
