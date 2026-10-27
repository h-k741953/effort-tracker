CREATE TABLE IF NOT EXISTS work_months (
    contract_id TEXT NOT NULL,
    year INTEGER NOT NULL,
    month INTEGER NOT NULL,
    lower_bound_hours INTEGER NOT NULL,
    lower_bound_minutes INTEGER NOT NULL,
    upper_bound_hours INTEGER NOT NULL,
    upper_bound_minutes INTEGER NOT NULL,
    state TEXT NOT NULL,
    excess_hours INTEGER,
    excess_minutes INTEGER,
    shortfall_hours INTEGER,
    shortfall_minutes INTEGER,
    PRIMARY KEY (contract_id, year, month)
)
