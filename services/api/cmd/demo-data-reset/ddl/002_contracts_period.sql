ALTER TABLE contracts
    ADD COLUMN IF NOT EXISTS period_start_year INTEGER,
    ADD COLUMN IF NOT EXISTS period_start_month INTEGER,
    ADD COLUMN IF NOT EXISTS period_end_year INTEGER,
    ADD COLUMN IF NOT EXISTS period_end_month INTEGER
