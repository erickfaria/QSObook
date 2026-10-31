ALTER TABLE app_settings ADD COLUMN radioid_enabled INTEGER NOT NULL DEFAULT 0 CHECK(radioid_enabled IN (0,1));
ALTER TABLE app_settings ADD COLUMN radioid_after INTEGER NOT NULL DEFAULT 0 CHECK(typeof(radioid_after)='integer' AND radioid_after>=0);
ALTER TABLE app_settings ADD COLUMN radioid_sync_after INTEGER NOT NULL DEFAULT 0 CHECK(typeof(radioid_sync_after)='integer' AND radioid_sync_after>=0);
CREATE TABLE radioid_lookups (
 callsign TEXT PRIMARY KEY,
 next_attempt INTEGER NOT NULL DEFAULT 0 CHECK(typeof(next_attempt)='integer' AND next_attempt>=0),
 status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','found','not_found','unavailable')),
 radio_ids TEXT NOT NULL DEFAULT '[]',
 auto_fields INTEGER NOT NULL DEFAULT 0 CHECK(typeof(auto_fields)='integer' AND auto_fields BETWEEN 0 AND 15),
 revision TEXT NOT NULL DEFAULT ''
) WITHOUT ROWID;
-- Freeze an existing historical participation before automatic enrichment.
ALTER TABLE participants ADD COLUMN radioid_snapshot TEXT;
CREATE TRIGGER radioid_station_insert AFTER INSERT ON stations
 WHEN new.deleted_at IS NULL AND (SELECT radioid_enabled FROM app_settings WHERE id=1)=1
 AND (new.name='' OR new.city='' OR new.state='' OR new.country='')
 BEGIN INSERT INTO radioid_lookups(callsign) VALUES(new.callsign) ON CONFLICT DO NOTHING; END;
CREATE TRIGGER radioid_station_update AFTER UPDATE OF name,city,state,country,deleted_at ON stations
 WHEN new.deleted_at IS NULL AND (SELECT radioid_enabled FROM app_settings WHERE id=1)=1
 AND (new.name='' OR new.city='' OR new.state='' OR new.country='')
 BEGIN INSERT INTO radioid_lookups(callsign) VALUES(new.callsign) ON CONFLICT DO NOTHING; END;
PRAGMA user_version=8;
