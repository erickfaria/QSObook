CREATE TABLE stations (
 id INTEGER PRIMARY KEY,
 callsign TEXT NOT NULL UNIQUE,
 name TEXT NOT NULL DEFAULT '', city TEXT NOT NULL DEFAULT '',
 state TEXT NOT NULL DEFAULT '', country TEXT NOT NULL DEFAULT '', notes TEXT NOT NULL DEFAULT ''
);
CREATE TABLE qsos (
 id INTEGER PRIMARY KEY,
 station_id INTEGER NOT NULL REFERENCES stations(id),
 callsign TEXT NOT NULL, name TEXT NOT NULL, city TEXT NOT NULL,
 state TEXT NOT NULL, country TEXT NOT NULL, notes TEXT NOT NULL,
 mode TEXT NOT NULL, occurred_at TEXT NOT NULL
);
CREATE INDEX qsos_station_time ON qsos(station_id, occurred_at);
CREATE INDEX qsos_time ON qsos(occurred_at);
CREATE TABLE participants (
 id INTEGER PRIMARY KEY,
 station_id INTEGER NOT NULL UNIQUE REFERENCES stations(id),
 mode TEXT NOT NULL, notes TEXT NOT NULL,
 qso_id INTEGER REFERENCES qsos(id)
);
PRAGMA user_version = 1;
