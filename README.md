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
