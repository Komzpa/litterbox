ALTER TABLE cards
    ADD COLUMN at timestamptz,
    ADD COLUMN timed boolean NOT NULL DEFAULT false,
    ADD COLUMN note text NOT NULL DEFAULT '',
    ADD COLUMN note_order integer NOT NULL DEFAULT 0;

