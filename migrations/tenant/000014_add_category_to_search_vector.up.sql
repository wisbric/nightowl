-- Add category to the incident search vector so full-text search covers it.
CREATE OR REPLACE FUNCTION update_incident_search_vector() RETURNS TRIGGER AS $$
BEGIN
    NEW.search_vector :=
        setweight(to_tsvector('english', COALESCE(NEW.title, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(NEW.category, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(NEW.symptoms, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(NEW.root_cause, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(NEW.solution, '')), 'C') ||
        setweight(to_tsvector('english', COALESCE(array_to_string(NEW.error_patterns, ' '), '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(array_to_string(NEW.services, ' '), '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(array_to_string(NEW.tags, ' '), '')), 'C');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Reindex existing rows so category is picked up.
UPDATE incidents SET updated_at = updated_at;
