CREATE TABLE IF NOT EXISTS contracts (
    id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    engineer_id TEXT NOT NULL,
    lower_bound_hours INTEGER NOT NULL,
    lower_bound_minutes INTEGER NOT NULL,
    upper_bound_hours INTEGER NOT NULL,
    upper_bound_minutes INTEGER NOT NULL,
    period_start_year INTEGER,
    period_start_month INTEGER,
    period_end_year INTEGER,
    period_end_month INTEGER
)
