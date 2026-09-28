ALTER TABLE cards ADD COLUMN note_updated_at timestamptz NOT NULL DEFAULT now();
