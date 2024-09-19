package models

import (
	"airqo-integrator/db"
	log "github.com/sirupsen/logrus"
	"time"
)

type DeviceGrid struct {
	ID       int64     `json:"id,omitempty" db:"id"`
	DeviceID int64     `json:"device_id,omitempty" db:"device_id"`
	GridID   int64     `json:"grid_id,omitempty" db:"grid_id"`
	Created  time.Time `json:"created,omitempty" db:"created"`
	Updated  time.Time `json:"updated,omitempty" db:"updated"`
}

// Insert ..
func (dg *DeviceGrid) Insert() error {
	dbConn := db.GetDB()
	_, err := dbConn.NamedExec(`
    INSERT INTO grid_devices(device_id, grid_id, created, updated)
    VALUES(:device_id, :grid_id, NOW(), NOW())`, dg)
	if err != nil {
		log.WithError(err).Error("Failed to insert device_grids")
		return err
	}
	return nil
}

// InsertDeviceGrid function to insert given grid_id and device_id
func InsertDeviceGrid(gridID, deviceID int64) error {
	dg := &DeviceGrid{
		DeviceID: deviceID,
		GridID:   gridID,
	}
	return dg.Insert()
}

// Delete ..
func (dg *DeviceGrid) Delete() error {
	dbConn := db.GetDB()
	_, err := dbConn.Exec("DELETE FROM grid_devices WHERE id = $1", dg.ID)
	if err != nil {
		log.WithError(err).Error("Failed to delete device_grids")
		return err
	}
	return nil
}

// DeleteDeviceGrid function to delete given device_id and grid_id
func DeleteDeviceGrid(gridID, deviceID int64) error {
	dg := &DeviceGrid{
		DeviceID: deviceID,
		GridID:   gridID,
	}
	return dg.Delete()
}
