package models

import (
	"airqo-integrator/db"
	log "github.com/sirupsen/logrus"
	"time"
)

type Device struct {
	ID               int64     `json:"id,omitempty" db:"id"`
	UID              string    `json:"_id" db:"uid"`
	Name             string    `json:"name,omitempty" db:"name"`
	DeviceNumber     string    `json:"device_number,omitempty" db:"device_number"`
	LocationName     string    `json:"location_name,omitempty" db:"location_name"`
	Country          string    `json:"country,omitempty" db:"country"`
	City             string    `json:"city,omitempty" db:"city"`
	Network          string    `json:"network,omitempty" db:"network"`
	Longitude        float64   `json:"longitude,omitempty" db:"longitude"`
	Latitude         float64   `json:"latitude,omitempty" db:"latitude"`
	CurrentSubCounty int64     `json:"current_subcounty,omitempty" db:"current_subcounty"`
	SiteID           int64     `json:"site_id,omitempty" db:"site_id"`
	Created          time.Time `json:"created,omitempty" db:"created"`
	Updated          time.Time `json:"updated,omitempty" db:"updated"`
	Grids            []Grid    `json:"grids,omitempty"`
}

const insertDeviceSQL = `
INSERT INTO devices(uid, name, device_number, location_name, country, city, network, 
	longitude, latitude, current_subcounty, site_id, created, updated)
VALUES(:uid, :name, :device_number, :location_name, :country, :city, :network, 
    :longitude, :latitude, :current_subcounty, :site_id, NOW(), NOW()) RETURNING  id
`

// Insert adds a new device
func (d *Device) Insert() (int64, error) {
	dbConn := db.GetDB()
	rows, err := dbConn.NamedQuery(insertDeviceSQL, d)
	if err != nil {
		log.WithError(err).Error("Failed to insert device")
		return 0, err
	}
	for rows.Next() {
		var gridId int64
		_ = rows.Scan(&gridId)
		d.ID = gridId
	}
	_ = rows.Close()
	return d.ID, nil
}

// Update updates a device
func (d *Device) Update() error {
	dbConn := db.GetDB()
	_, err := dbConn.NamedExec(`
    UPDATE devices SET name = :name, device_number = :device_number, location_name = :location_name, 
    country = :country, city = :city, network = :network, longitude = :longitude, 
    latitude = :latitude, current_subcounty = :current_subcounty, site_id = :site_id, 
    updated = NOW() WHERE id = :id`, d)
	if err != nil {
		log.WithError(err).Error("Failed to update device")
		return err
	}
	return nil
}

// Delete removes a device from the database
func (d *Device) Delete() error {
	dbConn := db.GetDB()
	_, err := dbConn.Exec("DELETE FROM devices WHERE id = $1", d.ID)
	if err != nil {
		log.WithError(err).Error("Failed to delete device")
		return err
	}
	return nil
}

// GetDeviceByUID returns a Device by its UID
func GetDeviceByUID(uid string) (*Device, error) {
	var d Device
	dbConn := db.GetDB()
	err := dbConn.Get(&d, `SELECT * FROM devices WHERE uid = $1`, uid)
	if err != nil {
		log.WithError(err).Error("Failed to get device by UID")
		return nil, err
	}
	return &d, nil
}

// GetDevicesBySiteID returns a slice of Devices for a given site ID
func GetDevicesBySiteID(siteID int64) ([]Device, error) {
	var devices []Device
	dbConn := db.GetDB()
	err := dbConn.Select(&devices, `SELECT * FROM devices WHERE site_id = $1`, siteID)
	if err != nil {
		log.WithError(err).Error("Failed to get devices by site ID")
		return nil, err
	}
	return devices, nil
}

// GetDevicesByGridID returns a slice of Devices for a given grid ID
func GetDevicesByGridID(gridID int64) ([]Device, error) {
	var devices []Device
	dbConn := db.GetDB()
	err := dbConn.Select(&devices, `
    SELECT d.* FROM devices d 
    INNER JOIN grid_devices gd ON d.id = gd.device_id WHERE gd.grid_id = $1`, gridID)
	if err != nil {
		log.WithError(err).Error("Failed to get devices by grid ID")
		return nil, err
	}
	return devices, nil
}

// GetDeviceByGridUID returns a slice of Devices for a given grid uid
func GetDeviceByGridUID(gridUID string) ([]Device, error) {
	var devices []Device
	dbConn := db.GetDB()
	err := dbConn.Select(&devices, `
    SELECT d.* FROM devices d 
    INNER JOIN grid_devices gd ON d.id = gd.device_id 
    INNER JOIN grids g ON gd.grid_id = g.id WHERE g.uid = $1`, gridUID)
	if err != nil {
		log.WithError(err).Error("Failed to get devices by grid UID")
		return nil, err
	}
	return devices, nil
}

// GetDeviceBySiteUID returns a slice of Devices for a given site uid
func GetDeviceBySiteUID(siteUID string) ([]Device, error) {
	var devices []Device
	dbConn := db.GetDB()
	err := dbConn.Select(&devices, `
    SELECT d.* FROM devices d 
    INNER JOIN sites s ON d.site_id = s.id WHERE s.uid = $1`, siteUID)
	if err != nil {
		log.WithError(err).Error("Failed to get devices by site UID")
		return nil, err
	}
	return devices, nil
}
