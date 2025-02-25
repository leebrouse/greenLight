# Test 1 healthcheck: 
healthcheck:
	curl localhost:4000/v1/healthcheck

#Test 2 movies:
movies:
	curl -X POST localhost:4000/v1/movies

# run project:
client:
	go run ./cmd/api

server:
	go run ./cmd/examples/cors/simple

# sql migration:
migrate_up:
	migrate -path=./migrations -database=$GREENLIGHT_DB_DSN up

migrate_down:
	migrate -path=./migrations -database=$GREENLIGHT_DB_DSN down