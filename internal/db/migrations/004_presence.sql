ALTER TABLE agents ADD COLUMN last_seen TEXT DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'));
