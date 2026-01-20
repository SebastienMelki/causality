-- Causality Events Hive Table DDL
-- This creates an external table pointing to Parquet files in S3/MinIO

-- Create database if not exists
CREATE DATABASE IF NOT EXISTS causality
  COMMENT 'Causality event analytics database'
  LOCATION 's3a://causality-events/';

USE causality;

-- Create external table for events
-- Partitioned by app_id, year, month, day, hour for efficient querying
CREATE EXTERNAL TABLE IF NOT EXISTS events (
  -- Event envelope fields
  id STRING COMMENT 'Unique event identifier (UUID v7)',
  device_id STRING COMMENT 'Device/session identifier',
  timestamp_ms BIGINT COMMENT 'Event timestamp in milliseconds since Unix epoch',
  correlation_id STRING COMMENT 'Optional correlation ID for request tracing',

  -- Event type information
  event_category STRING COMMENT 'Event category (user, screen, interaction, commerce, system, custom)',
  event_type STRING COMMENT 'Specific event type within category',

  -- Device context fields
  platform STRING COMMENT 'Platform (ios, android, web)',
  os_version STRING COMMENT 'Operating system version',
  app_version STRING COMMENT 'Application version',
  build_number STRING COMMENT 'Application build number',
  device_model STRING COMMENT 'Device model',
  manufacturer STRING COMMENT 'Device manufacturer',
  screen_width INT COMMENT 'Screen width in pixels',
  screen_height INT COMMENT 'Screen height in pixels',
  locale STRING COMMENT 'Device locale',
  timezone STRING COMMENT 'Device timezone',
  network_type STRING COMMENT 'Network connection type',
  carrier STRING COMMENT 'Carrier name (mobile)',
  is_jailbroken BOOLEAN COMMENT 'Whether device is jailbroken/rooted',
  is_emulator BOOLEAN COMMENT 'Whether device is an emulator',
  sdk_version STRING COMMENT 'SDK version used',

  -- Payload as JSON
  payload_json STRING COMMENT 'Event payload serialized as JSON'
)
PARTITIONED BY (
  app_id STRING COMMENT 'Application identifier for multi-tenant isolation',
  year INT COMMENT 'Event year',
  month INT COMMENT 'Event month',
  day INT COMMENT 'Event day',
  hour INT COMMENT 'Event hour'
)
STORED AS PARQUET
LOCATION 's3a://causality-events/events/'
TBLPROPERTIES (
  'parquet.compression' = 'SNAPPY',
  'projection.enabled' = 'true',
  'projection.app_id.type' = 'injected',
  'projection.year.type' = 'integer',
  'projection.year.range' = '2024,2030',
  'projection.month.type' = 'integer',
  'projection.month.range' = '1,12',
  'projection.day.type' = 'integer',
  'projection.day.range' = '1,31',
  'projection.hour.type' = 'integer',
  'projection.hour.range' = '0,23'
);

-- Repair partitions (discovers existing partitions in S3)
-- Run this after data has been written
-- MSCK REPAIR TABLE events;

-- Example queries:

-- Count events by category for a specific app
-- SELECT event_category, COUNT(*) as count
-- FROM events
-- WHERE app_id = 'myapp'
--   AND year = 2024
--   AND month = 1
-- GROUP BY event_category;

-- Get screen views with pagination
-- SELECT id, timestamp_ms, payload_json
-- FROM events
-- WHERE app_id = 'myapp'
--   AND event_category = 'screen'
--   AND event_type = 'view'
-- ORDER BY timestamp_ms DESC
-- LIMIT 100;

-- Analyze device distribution
-- SELECT platform, device_model, COUNT(*) as count
-- FROM events
-- WHERE app_id = 'myapp'
--   AND year = 2024
-- GROUP BY platform, device_model
-- ORDER BY count DESC;

-- Extract data from JSON payload
-- SELECT id, timestamp_ms,
--        get_json_object(payload_json, '$.screen_name') as screen_name
-- FROM events
-- WHERE app_id = 'myapp'
--   AND event_category = 'screen'
--   AND event_type = 'view';
