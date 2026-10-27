CREATE TABLE IF NOT EXISTS daily_records (
    contract_id TEXT NOT NULL,
    year INTEGER NOT NULL,
    month INTEGER NOT NULL,
    day INTEGER NOT NULL,
    start_hour INTEGER NOT NULL,
    start_minute INTEGER NOT NULL,
    end_hour INTEGER NOT NULL,
    end_minute INTEGER NOT NULL,
    break_start_hour INTEGER,
    break_start_minute INTEGER,
    break_end_hour INTEGER,
    break_end_minute INTEGER,
    PRIMARY KEY (contract_id, year, month, day)
)
