CREATE TABLE rounds (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 name TEXT NOT NULL DEFAULT '',
 started_at TEXT NOT NULL,
 ended_at TEXT
);
CREATE UNIQUE INDEX rounds_one_open ON rounds((1)) WHERE ended_at IS NULL;

-- Keep an existing live list as a named round rather than discarding it.
INSERT INTO rounds(name,started_at)
 SELECT 'Rodada anterior',strftime('%Y-%m-%dT%H:%M:%fZ','now')
 WHERE EXISTS(SELECT 1 FROM participants);

CREATE TABLE participants_next (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 round_id INTEGER NOT NULL REFERENCES rounds(id),
 station_id INTEGER NOT NULL REFERENCES stations(id),
 mode TEXT NOT NULL,
 notes TEXT NOT NULL,
 qso_id INTEGER REFERENCES qsos(id),
 active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0,1)),
 removed_at TEXT
);
INSERT INTO participants_next(id,round_id,station_id,mode,notes,qso_id)
 SELECT id,(SELECT id FROM rounds WHERE ended_at IS NULL),station_id,mode,notes,qso_id FROM participants;
-- Retain the high-water mark even if the highest old ID was already removed.
UPDATE sqlite_sequence SET seq=max(seq,coalesce((SELECT seq FROM sqlite_sequence WHERE name='participants'),0))
 WHERE name='participants_next';
INSERT INTO sqlite_sequence(name,seq)
 SELECT 'participants_next',seq FROM sqlite_sequence WHERE name='participants'
 AND NOT EXISTS(SELECT 1 FROM sqlite_sequence WHERE name='participants_next');
DROP TABLE participants;
ALTER TABLE participants_next RENAME TO participants;
CREATE UNIQUE INDEX participants_live_station ON participants(round_id,station_id) WHERE removed_at IS NULL;
CREATE INDEX participants_round ON participants(round_id);
ALTER TABLE qsos ADD COLUMN round_id INTEGER REFERENCES rounds(id);
UPDATE qsos SET round_id=(SELECT round_id FROM participants WHERE qso_id=qsos.id LIMIT 1)
 WHERE EXISTS(SELECT 1 FROM participants WHERE qso_id=qsos.id);
PRAGMA user_version = 3;
