# Variables
IMAGE_NAME=vitorvezani/rinha-backend-2025-go
DOCKER_COMPOSE=docker-compose

# Build the Docker image
build:
	docker build -t $(IMAGE_NAME) .

# Run docker-compose up
up:
	$(DOCKER_COMPOSE) up

# Run docker-compose up in detached mode
up-detached:
	$(DOCKER_COMPOSE) up -d

# Stop and remove containers
down:
	$(DOCKER_COMPOSE) down

# Rebuild everything from scratch
rebuild:
	docker build -t $(IMAGE_NAME) .
	$(DOCKER_COMPOSE) up --build

# Rebuild everything from scratch
rebuild-no-cache:
	docker build --no-cache -t $(IMAGE_NAME) .
	$(DOCKER_COMPOSE) up --build

# Clean all docker artifacts (optional and destructive!)
clean:
	docker system prune -af --volumes
