-- Create database for events if not exists
CREATE DATABASE IF NOT EXISTS causality;

-- Create external table for Parquet events
CREATE EXTERNAL TABLE IF NOT EXISTS causality.events (
    id STRING,
    app_id STRING,
    device_id STRING,
    timestamp_ms BIGINT,
    correlation_id STRING,
    event_type STRING,
    payload STRING,
    platform STRING,
    os_version STRING,
    app_version STRING,
    device_model STRING,
    is_jailbroken BOOLEAN,
    is_emulator BOOLEAN,
    year INT,
    month INT,
    day INT,
    hour INT
)
PARTITIONED BY (year, month, day, hour)
STORED AS PARQUET
LOCATION 's3a://causality-events/';

-- Enable automatic partition discovery
MSCK REPAIR TABLE causality.events;
