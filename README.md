# Screening Test: Data Automation & Retrieval Engineer (PostgreSQL / Go)

## Prerequisites

- [Go](https://go.dev/dl/) 1.23+
- [Docker](https://docs.docker.com/get-docker/) and Docker Compose
- [air](https://github.com/air-verse/air) — `go install github.com/air-verse/air@latest`
- [golang-migrate](https://github.com/golang-migrate/migrate) — `brew install golang-migrate`
- [swag](https://github.com/swaggo/swag) (optional, for regenerating API docs) — `go install github.com/swaggo/swag/cmd/swag@latest`

---

## Instructions

### Time Allocation

This test is designed to be completed until **friday, 2 October** (1 week).

### Submission Requirements

1. Create a **public GitHub repository** for your solutions
2. Include a comprehensive `README.md` explaining your approach and decisions
3. Organize your code with a clear folder structure
4. Add comments explaining your logic where appropriate
5. Include any SQL scripts, Go code, and documentation
6. Submit the GitHub repository link when complete

### Evaluation Criteria

- **Complex SQL query writing (PRIMARY FOCUS – 45%)**
- PostgreSQL query optimization and performance
- Go programming proficiency
- Problem-solving approach
- Code quality and structure
- Documentation and communication

> [!IMPORTANT]
> This role requires someone who can write complex SQL queries involving multiple tables, advanced aggregations, window functions, CTEs, and optimization. The SQL portion is weighted heavily in this assessment.

---

## Scenario: E-Commerce Order Analytics System

You are working for an e-commerce company that needs help retrieving and automating order data analysis. The company has a **PostgreSQL** database containing **orders**, **customers**, and **products**. Business teams frequently request complex data extracts and analytical reports.

---

## Part 1

---

## Part 2

---

## Part 3

---

## Running Locally

### 1. Clone and install dependencies

```bash
git clone <repo-url>
cd e-commerce_order_analytics_system
go mod tidy
```

### 2. Configure environment

Copy `.env.example` to `.env` and fill in the required values (database URL, JWT secrets, etc.):

```bash
cp .env.example .env
```

### 3. Provision local secrets for Docker

The `db` service reads its credentials from files in `./secrets/`. Create them once:

```bash
mkdir -p secrets
echo "postgres" > secrets/user.txt
echo "postgres" > secrets/password.txt
```

### 4. Start the infrastructure

Brings up PostgreSQL (`:5432`)

```bash
docker compose up -d
```

### 5. Run database migrations

```bash
make migration-up
```

### 6. Start the services

Pick whichever process you need. Each binary builds independently.

**API server (with hot reload via air):**

```bash
air
```

**API server (without hot reload):**

```bash
go build -o ./bin/api ./cmd/api && ./bin/api
# or
make run-api
```

**CLI:**

```bash
go build -o ./report ./cmd/cli
# or
make build-cli

# then (to see all commands available)
./report -- help
```

**Cron:**

```bash
go build -o ./bin/cron ./cmd/cron && ./bin/cron
# or
make run-cron
```

## API Endpoints

Routes are registered in [`transport/http/router.go`](transport/http/router.go). All versioned endpoints are mounted under `/api`.

Interactive docs:

- Swagger UI — `GET /docs/*any`
- Scalar reference — `GET /reference`
