-- NULL means no outstanding local provider intent. A confirmed matching
-- provider state consumes the intent; an older snapshot cannot reverse it.
ALTER TABLE cards ADD COLUMN gmail_pending_inbox boolean;
ALTER TABLE cards ADD COLUMN gmail_state_checked boolean NOT NULL DEFAULT false;
ALTER TABLE cards ADD COLUMN gmail_state_reason text NOT NULL DEFAULT '';
ALTER TABLE cards ADD COLUMN gmail_intent_history numeric NOT NULL DEFAULT 0;
ALTER TABLE cards ADD COLUMN gmail_provider_inbox boolean;
