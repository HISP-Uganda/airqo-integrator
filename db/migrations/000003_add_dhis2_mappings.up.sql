CREATE TABLE dhis2_mappings (
    id SERIAL PRIMARY KEY NOT NULL,
    uid TEXT NOT NULL DEFAULT  generate_uid(),
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    dataset TEXT NOT NULL DEFAULT '',
    dhis2_name TEXT NOT NULL DEFAULT '',
    dataelement TEXT NOT NULL,
    category_option_combo TEXT,
    created TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_dhis2_mappings_name ON dhis2_mappings (name);
CREATE INDEX idx_dhis2_mappings_uid ON dhis2_mappings (uid);
CREATE INDEX idx_dhis2_mappings_dataset ON dhis2_mappings (dataset);
CREATE INDEX idx_dhis2_mappings_dhis2_name ON dhis2_mappings (dhis2_name);
CREATE INDEX idx_dhis2_mappings_dataelement ON dhis2_mappings (dataelement);

INSERT INTO dhis2_mappings (name, description, dataset, dhis2_name, dataelement, category_option_combo)
    VALUES
        ('Average PM 10', '', 'hKBjahBED0H','ENV - Average PM 10 (Airqo)',
            'm8pEcIa4lYQ', 'HllvX50cXC0'),
        ('Average PM 2.5', '', 'hKBjahBED0H', 'ENV - Average PM 2.5 (Airqo)',
            'lz5jUCxfZPJ', 'HllvX50cXC0'),
        ('Max PM 10', '', 'hKBjahBED0H','ENV - Max PM 10 (Airqo)',
            'xIMRUoqiNyq', 'HllvX50cXC0'),
        ('Max PM 2.5', '', 'hKBjahBED0H','ENV - Max PM 2.5 (Airqo)',
            'EmSLaOe7fCV', 'HllvX50cXC0'),
        ('Min PM 10', '', 'hKBjahBED0H','ENV - Max PM 10 (Airqo)',
            'MCpq7yczGh1', 'HllvX50cXC0'),
        ('Min PM 2.5', '', 'hKBjahBED0H','ENV - Min PM 2.5 (Airqo)',
            'jo4B1xI8gYy', 'HllvX50cXC0');