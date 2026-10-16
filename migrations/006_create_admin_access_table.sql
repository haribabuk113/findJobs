CREATE TABLE IF NOT EXISTS admin_review_queue (
    id                     BIGSERIAL PRIMARY KEY,
    entity_type            VARCHAR(20) NOT NULL CHECK (entity_type IN ('DESIGNATION','SKILL')),
    entity_id              BIGINT NOT NULL,       -- references master_designations or master_skills
    submitted_by_candidate_id BIGINT NOT NULL REFERENCES candidates(candidate_id),
    status                  VARCHAR(20) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','APPROVED','MERGED','REJECTED')),
    reviewed_by_admin_id     BIGINT NULL,
    reviewed_at              TIMESTAMPTZ NULL,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now()
);
