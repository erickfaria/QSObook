-- Deletion is recoverable; links, snapshots and replay keys remain intact.
ALTER TABLE stations ADD COLUMN deleted_at TEXT;
ALTER TABLE qsos ADD COLUMN deleted_at TEXT;
CREATE INDEX stations_deleted ON stations(deleted_at);
CREATE INDEX qsos_deleted ON qsos(deleted_at);
PRAGMA user_version=6;
