-- Keep participation IDs unique across removals, reinsertion and restarts.
-- Existing operator, QSO and participant data are retained in this transaction.
CREATE TABLE participants_next (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 station_id INTEGER NOT NULL UNIQUE REFERENCES stations(id),
 mode TEXT NOT NULL, notes TEXT NOT NULL,
 qso_id INTEGER REFERENCES qsos(id)
);
INSERT INTO participants_next(id,station_id,mode,notes,qso_id)
 SELECT id,station_id,mode,notes,qso_id FROM participants;
DROP TABLE participants;
ALTER TABLE participants_next RENAME TO participants;
PRAGMA user_version = 2;
