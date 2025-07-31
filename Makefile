.PHONY: runserver swagger

runserver:
	go run ./cmd/service-io

# Regenerates the swagger documentation
swagger:
	swag init -g ./cmd/service-io/main.go