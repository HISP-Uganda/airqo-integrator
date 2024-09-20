CREATE TABLE IF NOT EXISTS grids (
    id bigserial NOT NULL PRIMARY KEY,
    uid VARCHAR(25) NOT NULL,
    name VARCHAR(255) NOT NULL,
    admin_level VARCHAR(36) NOT NULL,
    in_scope BOOLEAN NOT NULL DEFAULT TRUE,
    created     timestamptz DEFAULT CURRENT_TIMESTAMP,
    updated     timestamptz DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (uid)
);
CREATE INDEX grid_uid_idx ON grids (uid);
CREATE INDEX grid_admin_level_idx ON grids (admin_level);
CREATE INDEX grid_in_scope_idx ON grids (in_scope);

CREATE TABLE IF NOT EXISTS sites (
    id bigserial NOT NULL PRIMARY KEY,
    uid VARCHAR(25) NOT NULL,
    name TEXT NOT NULL,
    search_name TEXT NOT NULL,
    location_name TEXT NOT NULL,
    country TEXT NOT NULL DEFAULT '',
    city TEXT NOT NULL DEFAULT '',
    district TEXT NOT NULL DEFAULT '',
    county TEXT NOT NULL DEFAULT '',
    sub_county TEXT NOT NULL DEFAULT '',
    region TEXT NOT NULL DEFAULT '',
    longitude DOUBLE PRECISION NOT NULL,
    latitude  DOUBLE PRECISION NOT NULL,
    dhis2_district BIGSERIAL,
    current_subcounty BIGSERIAL,
    created TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (uid)
);

CREATE INDEX sites_uid_idx ON sites (uid);
CREATE INDEX sites_search_name_idx ON sites (search_name);
CREATE INDEX sites_location_name_idx ON sites (location_name);
CREATE INDEX sites_country_idx ON sites (country);
CREATE INDEX sites_city_idx ON sites (city);
CREATE INDEX sites_district_idx ON sites (district);

CREATE TABLE IF NOT EXISTS grid_sites (
    id      bigserial NOT NULL PRIMARY KEY,
    grid_id bigint REFERENCES grids (id),
    site_id bigint REFERENCES sites (id),
    created TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (grid_id, site_id)
);

CREATE TABLE IF NOT EXISTS devices (
    id bigserial NOT NULL PRIMARY KEY,
    uid VARCHAR(25) NOT NULL,
    name TEXT NOT NULL,
    device_number NUMERIC NOT NULL,
    location_name TEXT NOT NULL DEFAULT '',
    country TEXT NOT NULL DEFAULT '',
    city TEXT NOT NULL DEFAULT '',
    network TEXT NOT NULL DEFAULT '',
    longitude DOUBLE PRECISION NOT NULL,
    latitude  DOUBLE PRECISION NOT NULL,
    current_subcounty BIGSERIAL,
    site_id BIGSERIAL NOT NULL REFERENCES sites (id),
    created TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (uid)
);

CREATE INDEX devices_uid_idx ON devices (uid);
CREATE INDEX devices_device_number_idx ON devices (device_number);
CREATE INDEX devices_site_id_idx ON devices (site_id);
CREATE INDEX devices_country_idx ON devices (country);
CREATE INDEX devices_current_subcounty_idx ON devices (current_subcounty);

CREATE TABLE IF NOT EXISTS grid_devices
(
    id        bigserial NOT NULL PRIMARY KEY,
    grid_id   bigint REFERENCES grids (id),
    device_id bigint REFERENCES devices (id),
    created   TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated   TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (grid_id, device_id)
);
