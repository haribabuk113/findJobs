-- Audit trail for the Blue Tick workflow: every state transition and who
-- triggered it (edit -> revoke, admin approve/reject, re-request, etc.)
CREATE TABLE IF NOT EXISTS timeline_verification_events (
    event_id             BIGSERIAL PRIMARY KEY,
    timeline_id          BIGINT NOT NULL REFERENCES candidate_timeline(timeline_id) ON DELETE CASCADE,
    event_type           VARCHAR(30) NOT NULL CHECK (event_type IN
                            ('SUBMITTED','APPROVED','REJECTED','REVOKED_ON_EDIT','RE_REQUESTED')),
    actor_type           VARCHAR(20) NOT NULL CHECK (actor_type IN ('CANDIDATE','COMPANY_ADMIN','SYSTEM')),
    actor_id             BIGINT NULL,
    notes                TEXT NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Raw event log (source of truth; aggregates below are derived/cached from this)
CREATE TABLE IF NOT EXISTS profile_analytics_events (
    event_id              BIGSERIAL PRIMARY KEY,
    candidate_id          BIGINT NOT NULL REFERENCES candidates(candidate_id) ON DELETE CASCADE,
    event_type            VARCHAR(30) NOT NULL CHECK (event_type IN
                             ('PROFILE_VIEW_SEARCH','PROFILE_VIEW_DIRECT','RESUME_VIEW','RESUME_DOWNLOAD')),
    viewer_session_id      VARCHAR(100) NULL,
    viewer_user_id         BIGINT NULL,
    occurred_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Daily rollup used to power the 7-day trend chart and KPI cards without
-- re-aggregating the raw event log on every page load.
CREATE TABLE IF NOT EXISTS profile_analytics_daily (
    candidate_id           BIGINT NOT NULL REFERENCES candidates(candidate_id) ON DELETE CASCADE,
    activity_date           DATE NOT NULL,
    profile_views_search    INTEGER NOT NULL DEFAULT 0,
    profile_views_direct    INTEGER NOT NULL DEFAULT 0,
    resume_views             INTEGER NOT NULL DEFAULT 0,
    resume_downloads         INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (candidate_id, activity_date)
);

CREATE INDEX idx_analytics_events_candidate_time ON profile_analytics_events(candidate_id, occurred_at);
CREATE INDEX idx_analytics_daily_candidate_date ON profile_analytics_daily(candidate_id, activity_date);
