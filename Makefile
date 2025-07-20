# Delivery System Makefile

# Переменные
BINARY_NAME=delivery-server
DOCKER_IMAGE=delivery-system
VERSION=latest

# Go команды
.PHONY: build clean run test deps docker-build docker-run help

# Сборка бинарного файла
build:
	@echo "Building $(BINARY_NAME)..."
	@go build -o $(BINARY_NAME) cmd/server/main.go

# Очистка артефактов сборки
clean:
	@echo "Cleaning..."
	@go clean
	@rm -f $(BINARY_NAME)

# Запуск приложения
run:
	@echo "Running application..."
	@go run cmd/server/main.go

# Запуск тестов
test:
	@echo "Running tests..."
	@go test -v ./...

# Установка зависимостей
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

# Docker команды
docker-build:
	@echo "Building Docker image..."
	@docker build -t $(DOCKER_IMAGE):$(VERSION) .

docker-run:
	@echo "Running Docker container..."
	@docker run -p 8080:8080 $(DOCKER_IMAGE):$(VERSION)

# Docker Compose команды
up:
	@echo "Starting all services..."
	@docker-compose up -d

down:
	@echo "Stopping all services..."
	@docker-compose down

logs:
	@echo "Showing logs..."
	@docker-compose logs -f

# Разработка
dev-up:
	@echo "Starting development environment..."
	@docker-compose up -d postgres redis kafka zookeeper

dev-down:
	@echo "Stopping development environment..."
	@docker-compose down

# Проверка состояния
health:
	@echo "Checking application health..."
	@curl -s http://localhost:8080/health | jq

# Форматирование кода
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Линтинг
lint:
	@echo "Linting code..."
	@golangci-lint run

# Установка инструментов разработки
install-tools:
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Помощь
help:
	@echo "Available commands:"
	@echo "  build        - Build the application binary"
	@echo "  clean        - Clean build artifacts"
	@echo "  run          - Run the application locally"
	@echo "  test         - Run tests"
	@echo "  deps         - Download and tidy dependencies"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run Docker container"
	@echo "  up           - Start all services with docker-compose"
	@echo "  down         - Stop all services"
	@echo "  logs         - Show docker-compose logs"
	@echo "  dev-up       - Start development infrastructure only"
	@echo "  dev-down     - Stop development infrastructure"
	@echo "  health       - Check application health"
	@echo "  fmt          - Format Go code"
	@echo "  lint         - Lint Go code"
	@echo "  install-tools- Install development tools"
	@echo "  help         - Show this help message" 