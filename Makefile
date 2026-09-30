.PHONY: dev build test run clean

# Run backend (:8080) and Vite dev server (:5173) side by side.
dev:
	@trap 'kill 0' INT TERM; \
	go run ./cmd/server & \
	cd web && npm run dev

# Frontend production build + server binary.
build:
	cd web && npm run build
	go build -o bin/server ./cmd/server

test:
	go test ./internal/... -timeout 90s

# Single-binary mode: serve the built frontend + ws on :8080.
run: build
	./bin/server

clean:
	rm -rf bin web/dist
