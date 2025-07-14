# ------------------------------------------------------------------------------
# Phony targets and set up
# ------------------------------------------------------------------------------
# List of known targets to exclude
KNOWN_TARGETS := help \
	setup cleanup \
	install uninstall freeze \
	runserver \
	lint test \
	db-up db-down db-shell db-setup migrate upgrade

# Declare known targets (not KNOWN_TARGETS itself) as phony
.PHONY: $(KNOWN_TARGETS)

# ARGS are everything passed to make, except known targets
ARGS := $(filter-out $(KNOWN_TARGETS),$(MAKECMDGOALS))

DB_COMPOSE_FILE := ./infra/local/docker-compose.db.yaml

# Default target
.DEFAULT_GOAL := help


# ------------------------------------------------------------------------------
# help: show this help text
# ------------------------------------------------------------------------------
help:
	@echo "Makefile for managing the project"
	@echo ""
	@echo "  help        - Show this help text"
	@echo ""
	@echo "Environment targets:"
	@echo "  setup       - Set up the environment"
	@echo "  cleanup     - Clean up environment artifacts"
	@echo "  install     - Install dependencies (or specific packages via ARGS)"
	@echo "  uninstall   - Uninstall dependencies (or specific packages via ARGS)"
	@echo "  freeze      - Freeze current environment into requirements.txt"
	@echo ""
	@echo "Server targets:"
	@echo "  runserver   - Run the web server"
	@echo ""
	@echo "Quality targets:"
	@echo "  lint        - Run code quality checks"
	@echo "  test        - Run tests"
	@echo ""
	@echo "Database targets:"
	@echo "  db-up       - Start the database container"
	@echo "  db-down     - Stop and remove the database container"
	@echo "  db-shell    - Open a psql shell in the database container"
	@echo "  db-setup    - Ensure DB is running and apply migrations"
	@echo "  migrate     - Create a new Alembic migration (usage: make migrate m=\"description\")"
	@echo "  upgrade     - Apply all pending Alembic migrations"
	@echo ""


# ------------------------------------------------------------------------------
# Environment Related Targets
# ------------------------------------------------------------------------------
setup:
	chmod +x ./bin/setup.sh
	./bin/setup.sh

cleanup:
	@echo "Cleaning up directories..."
	chmod +x ./bin/cleanup.sh
	./bin/cleanup.sh


# ------------------------------------------------------------------------------
# Package Related Targets
# ------------------------------------------------------------------------------
install:
	@pip install --upgrade pip
ifneq ($(ARGS),)
	@echo "Installing new package(s): $(ARGS)"
	@pip install $(ARGS)
else
	@echo "Installing from requirements.txt"
	@pip install -r requirements.txt
endif
	@$(MAKE) freeze

uninstall:
ifneq ($(ARGS),)
	@echo "Uninstalling package(s): $(ARGS)"
	@pip uninstall -y $(ARGS)
else
	@echo "Uninstalling everything in requirements.txt"
	@pip uninstall -y -r requirements.txt || true
endif
	@$(MAKE) freeze

freeze:
	@echo "Freezing installed packages into requirements.txt"
	@pip freeze > requirements.txt


# ------------------------------------------------------------------------------
# Server Related Targets
# ------------------------------------------------------------------------------
runserver:
	@echo "Running server..."
	uvicorn app.main:app --reload


# ------------------------------------------------------------------------------
# Quality Related Targets
# ------------------------------------------------------------------------------
test:
	@echo "Running tests with pytest"
	@pytest
	@echo "Removing coverage data files"
	@rm -rf .coverage

lint:
	@echo "Running flake8…"
	@flake8 app tests --max-line-length=88
	@echo "✔️  Lint passed"


# ------------------------------------------------------------------------------
# Database Related Targets
# ------------------------------------------------------------------------------
db-up:  ## docker compose up -d db
	docker compose -f $(DB_COMPOSE_FILE) up -d db

db-down:  ## docker compose down
	docker compose -f $(DB_COMPOSE_FILE) down

db-shell:  ## psql inside the running container
	docker compose -f $(DB_COMPOSE_FILE) exec db psql -U "postgres" "postgres"

db-setup:  ## Start DB (if needed) & run alembic upgrade head
	chmod +x bin/migrate.sh
	bin/migrate.sh

migrate:   ## Create a new autogen revision: make migrate m="add_users_table"
ifndef m
	$(error Usage: make migrate m="your_message")
endif
	alembic revision --autogenerate -m "$(m)"

upgrade:   ## Apply all pending migrations
	alembic upgrade head


# ------------------------------------------------------------------------------
# Pass-through for ARGS
# ------------------------------------------------------------------------------
$(ARGS):
	@: