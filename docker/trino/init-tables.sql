-- Create schema
CREATE SCHEMA IF NOT EXISTS hive.causality WITH (location = 's3a://causality-events/');

-- Create events table with Hive partitioning
CREATE TABLE IF NOT EXISTS hive.causality.events (
    id VARCHAR,
    app_id VARCHAR,
    device_id VARCHAR,
    timestamp_ms BIGINT,
    correlation_id VARCHAR,
    event_category VARCHAR,
    event_type VARCHAR,
    platform VARCHAR,
    os_version VARCHAR,
    app_version VARCHAR,
    build_number VARCHAR,
    device_model VARCHAR,
    manufacturer VARCHAR,
    screen_width INTEGER,
    screen_height INTEGER,
    locale VARCHAR,
    timezone VARCHAR,
    network_type VARCHAR,
    carrier VARCHAR,
    is_jailbroken BOOLEAN,
    is_emulator BOOLEAN,
    sdk_version VARCHAR,
    payload_json VARCHAR,
    year INTEGER,
    month INTEGER,
    day INTEGER,
    hour INTEGER
)
WITH (
    format = 'PARQUET',
    partitioned_by = ARRAY['app_id', 'year', 'month', 'day', 'hour'],
    external_location = 's3a://causality-events/events/'
);

-- Sync partitions from S3
CALL system.sync_partition_metadata('causality', 'events', 'FULL');
