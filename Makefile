.PHONY: build stop

# Build the images and start every container in the background.
build:
	docker compose up -d --build

# Stop and remove the containers (volumes survive).
stop:
	docker compose down
