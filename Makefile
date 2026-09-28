.PHONY: help migrate-up migrate-down migrate-version

MIGRATE_IMAGE ?= migrate/migrate:v4.18.3
MIGRATIONS_TABLE ?= interview_schema_migrations
DATABASE_URL ?= postgres://user:password@localhost:5432/interverse?sslmode=disable
NETWORK ?= host
STEPS ?= 1

help: ## Show targets
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

migrate-up: ## Apply all pending migrations
	@echo "Migrating interview (table=$(MIGRATIONS_TABLE))..."
	@case "$(DATABASE_URL)" in *\?*) DB_URL="$(DATABASE_URL)&x-migrations-table=$(MIGRATIONS_TABLE)";; *) DB_URL="$(DATABASE_URL)?x-migrations-table=$(MIGRATIONS_TABLE)";; esac; \
	docker run --rm --network "$(NETWORK)" \
		-v "$(CURDIR)/migrations:/migrations:ro" \
		$(MIGRATE_IMAGE) \
		-path=/migrations \
		-database "$$DB_URL" \
		up

migrate-down: ## Roll back STEPS migrations (default 1)
	@case "$(DATABASE_URL)" in *\?*) DB_URL="$(DATABASE_URL)&x-migrations-table=$(MIGRATIONS_TABLE)";; *) DB_URL="$(DATABASE_URL)?x-migrations-table=$(MIGRATIONS_TABLE)";; esac; \
	docker run --rm --network "$(NETWORK)" \
		-v "$(CURDIR)/migrations:/migrations:ro" \
		$(MIGRATE_IMAGE) \
		-path=/migrations \
		-database "$$DB_URL" \
		down $(STEPS)

migrate-version: ## Show current migration version
	@case "$(DATABASE_URL)" in *\?*) DB_URL="$(DATABASE_URL)&x-migrations-table=$(MIGRATIONS_TABLE)";; *) DB_URL="$(DATABASE_URL)?x-migrations-table=$(MIGRATIONS_TABLE)";; esac; \
	docker run --rm --network "$(NETWORK)" \
		-v "$(CURDIR)/migrations:/migrations:ro" \
		$(MIGRATE_IMAGE) \
		-path=/migrations \
		-database "$$DB_URL" \
		version
