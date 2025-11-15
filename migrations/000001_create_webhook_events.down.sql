-- Drop trigger and function first
DROP TRIGGER IF EXISTS trg_prevent_webhook_events_modification ON webhook_events;
DROP FUNCTION IF EXISTS prevent_webhook_events_modification();

-- Drop table
DROP TABLE IF EXISTS webhook_events;
