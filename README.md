# findJobs

JPortal is a job portal backend built with Go. It follows a **Hexagonal (Ports and Adapters) architecture** so business rules stay independent of frameworks, databases, and external services.

## Architecture

The application is divided into these layers:

- `internal/domain` — Core domain models, validation errors, and business rules. This layer has no framework or infrastructure dependencies.
- `internal/ports` — Interfaces that define contracts:
  - Inbound ports describe what the application can do.
  - Outbound ports describe what infrastructure must provide.
- `internal/application` — Use-case services that implement inbound ports and orchestrate domain logic through outbound ports.
- `internal/adapters` — Concrete adapters:
  - `inbound` — HTTP handlers that translate requests into service calls.
  - `outbound` — Database and external-service implementations of outbound ports.
- `internal/pkg` — Infrastructure concerns such as config, logging, PostgreSQL, and Redis.
- `cmd/main.go` — Composition root that wires adapters to ports and starts the server.

### Dependency direction

Dependencies point inward toward the domain:

- `cmd/main.go` → adapters → application → ports → domain
- Adapters depend on ports/interfaces, not on concrete implementations.
- The domain never imports `internal/application`, `internal/adapters`, or `internal/pkg`.

## API

All responses use `{status, message, error, data}`. Errors never include internal details.

### Auth — `/api/v1/auth` (session cookie `jp_session`, HttpOnly)

| Method | Path | Notes |
| --- | --- | --- |
| POST | `/session` | `{"code"}` from IAM's redirect. Redeems it at IAM, sets the cookie. |
| GET | `/me` | Current user + session `expires_at`. Refreshes IAM tokens when expired. |
| GET | `/token` | Current IAM access token, for the frontend server to call IAM admin APIs. |
| POST | `/logout` | Deletes the session and revokes it at IAM. |

### Jobs — `POST /api/v1/jobs/search`

One consolidated payload, sent when the user presses Search. AND across categories, OR within a list. All fields optional.

```json
{
  "query": "senior engineer",          "location_text": "bengaluru",
  "designations": [], "skills": ["React"], "location_ids": ["in-blr"], "worldwide_remote": false,
  "min_salary_inr": 3000000, "max_salary_inr": 0,
  "notice_periods": ["Immediate"], "relocation": [], "work_modes": ["Remote", "Hybrid"], "work_types": [],
  "categories": [], "levels": [], "tiers": [], "application_methods": ["Easy apply"],
  "industries": [], "company_sizes": [], "benefits": [], "job_ids": [],
  "min_experience": 3, "max_experience": null, "interview_rounds": 3, "interview_rounds_exact": false,
  "posted_within_days": 30, "sort": "newest", "page": 1, "page_size": 20
}
```

- Salary is filtered on the INR baseline; the client converts the user's currency with `rates.json` first.
- `location_ids` are leaf city ids flattened from the location tree (client expands radius searches too).
- `sort`: `newest` (default), `salary`, `title`. Match-score sorting runs client-side.
- `posted_within_days`: 1, 3, 7, 9, 15, 30. Listings older than 30 days, `is_visible = false` or `Free` (unlisted) tier never appear.
- Hidden-salary jobs still match salary filters but come back with `salary_hidden: true` and null amounts.
- With a valid `jp_session` cookie, jobs the user applied to are excluded.
- Response `data`: `{jobs: [...], total, page, page_size}`.

Dev sample data: `docker exec -i jportal-postgres psql -U dbuser -d jportal < migrations/dev_seed/jobs.sql`
