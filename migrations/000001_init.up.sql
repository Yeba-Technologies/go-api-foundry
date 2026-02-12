-- Initial schema

-- Matches internal/models/waitlist.go (WaitlistEntry) using GORM defaults.

CREATE TABLE IF NOT EXISTS waitlist_entries (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,

    email TEXT NOT NULL,
    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL
);


ALTER TABLE waitlist_entries ADD CONSTRAINT uq_waitlist_entries_email UNIQUE (email);
CREATE INDEX IF NOT EXISTS idx_waitlist_entries_deleted_at ON waitlist_entries (deleted_at);
