DROP TRIGGER IF EXISTS trg_incidents_search_vector ON incidents;
DROP FUNCTION IF EXISTS update_incident_search_vector();
DROP TABLE IF EXISTS incidents CASCADE;
