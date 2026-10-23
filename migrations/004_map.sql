CREATE TABLE app_settings (
 id INTEGER PRIMARY KEY CHECK(id=1),
 map_enabled INTEGER NOT NULL DEFAULT 0 CHECK(map_enabled IN (0,1)),
 geocode_after INTEGER NOT NULL DEFAULT 0
);
INSERT INTO app_settings(id) VALUES(1);
CREATE TABLE city_coordinates (
 city TEXT NOT NULL, state TEXT NOT NULL, country TEXT NOT NULL,
 latitude REAL, longitude REAL, checked_at TEXT NOT NULL,
 PRIMARY KEY(city,state,country),
 CHECK((latitude IS NULL AND longitude IS NULL) OR
       (latitude IS NOT NULL AND longitude IS NOT NULL AND latitude BETWEEN -90 AND 90 AND longitude BETWEEN -180 AND 180))
) WITHOUT ROWID;
CREATE INDEX qsos_city ON qsos(city,state,country);
PRAGMA user_version = 4;
