CREATE TABLE incidents (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title               TEXT NOT NULL,
    fingerprints        TEXT[] NOT NULL DEFAULT '{}',
    severity            TEXT NOT NULL DEFAULT 'warning',
    category            TEXT,
    tags                TEXT[] NOT NULL DEFAULT '{}',
    services            TEXT[] NOT NULL DEFAULT '{}',
    clusters            TEXT[] NOT NULL DEFAULT '{}',
    namespaces          TEXT[] NOT NULL DEFAULT '{}',
    symptoms            TEXT,
    error_patterns      TEXT[] DEFAULT '{}',
    root_cause          TEXT,
    solution            TEXT,
    runbook_id          UUID REFERENCES runbooks(id),
    resolution_count    INTEGER NOT NULL DEFAULT 0,
    last_resolved_at    TIMESTAMPTZ,
    last_resolved_by    UUID REFERENCES users(id),
    avg_resolution_mins FLOAT,
    merged_into_id      UUID REFERENCES incidents(id),
    search_vector       TSVECTOR,
    created_by          UUID REFERENCES users(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_incidents_fingerprints ON incidents USING GIN(fingerprints);
CREATE INDEX idx_incidents_search ON incidents USING GIN(search_vector);
CREATE INDEX idx_incidents_tags ON incidents USING GIN(tags);
CREATE INDEX idx_incidents_severity ON incidents(severity);
CREATE INDEX idx_incidents_merged ON incidents(merged_into_id) WHERE merged_into_id IS NOT NULL;

CREATE OR REPLACE FUNCTION update_incident_search_vector() RETURNS TRIGGER AS $$
BEGIN
    NEW.search_vector :=
        setweight(to_tsvector('english', COALESCE(NEW.title, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(NEW.symptoms, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(NEW.root_cause, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(NEW.solution, '')), 'C') ||
        setweight(to_tsvector('english', COALESCE(array_to_string(NEW.error_patterns, ' '), '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(array_to_string(NEW.services, ' '), '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(array_to_string(NEW.tags, ' '), '')), 'C');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_incidents_search_vector
    BEFORE INSERT OR UPDATE ON incidents
    FOR EACH ROW EXECUTE FUNCTION update_incident_search_vector();
