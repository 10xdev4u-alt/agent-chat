CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
    content,
    message_id UNINDEXED,
    room_id UNINDEXED
);

CREATE TRIGGER IF NOT EXISTS messages_fts_insert
AFTER INSERT ON messages
WHEN NEW.deleted_at IS NULL
BEGIN
    INSERT INTO messages_fts(content, message_id, room_id)
    VALUES (NEW.body, NEW.id, NEW.room_id);
END;

CREATE TRIGGER IF NOT EXISTS messages_fts_update
AFTER UPDATE ON messages
BEGIN
    DELETE FROM messages_fts WHERE message_id = OLD.id;
    INSERT INTO messages_fts(content, message_id, room_id)
    SELECT NEW.body, NEW.id, NEW.room_id
    WHERE NEW.deleted_at IS NULL;
END;

CREATE TRIGGER IF NOT EXISTS messages_fts_delete
AFTER DELETE ON messages
BEGIN
    DELETE FROM messages_fts WHERE message_id = OLD.id;
END;

INSERT INTO messages_fts(content, message_id, room_id)
SELECT m.body, m.id, m.room_id
FROM messages m
WHERE m.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM messages_fts f WHERE f.message_id = m.id);
