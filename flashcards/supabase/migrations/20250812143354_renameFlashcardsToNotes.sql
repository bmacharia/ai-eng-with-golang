-- Rename table from flashcards to notes
ALTER TABLE gocourse.flashcards RENAME TO notes;

-- Drop old index and create new one with updated name
DROP INDEX IF EXISTS idx_flashcards_created_at;
CREATE INDEX IF NOT EXISTS idx_notes_created_at ON gocourse.notes(createdAt);
