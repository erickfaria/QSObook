-- Remember only explicit roundtable choices; listening sessions stay unnamed.
ALTER TABLE rounds ADD COLUMN edition TEXT NOT NULL DEFAULT '';
ALTER TABLE app_settings ADD COLUMN last_round_name TEXT NOT NULL DEFAULT '';
ALTER TABLE app_settings ADD COLUMN last_round_edition TEXT NOT NULL DEFAULT '';
PRAGMA user_version=7;
