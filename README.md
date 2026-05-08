# Comment Tree Service

REST API сервис для работы с древовидными комментариями с поддержкой:

* вложенных комментариев;
* пагинации;
* полнотекстового поиска;
* soft delete;
* Redis-кеширования;
* Swagger документации;
* graceful shutdown;
* логирования и middleware.

---

# Stack

* Go 1.22+
* Gin
* PostgreSQL
* Redis
* pgx
* Squirrel
* Swagger
* Docker / Docker Compose

---

# Архитектура

Проект построен по принципам layered architecture:

```text
internal/
├── app/            # Инициализация приложения
├── config/         # Конфигурация
├── entity/         # Доменные сущности и ошибки
├── repository/     # Работа с PostgreSQL и Redis
├── service/        # Бизнес-логика
└── transport/http/ # HTTP handlers + middleware
```

---

# Возможности

## Комментарии

* создание корневых комментариев;
* создание вложенных комментариев;
* ограничение максимальной глубины;
* древовидная структура;
* получение поддерева комментариев.

## Поиск

Полнотекстовый поиск по:

* автору;
* содержимому комментария.

Используется PostgreSQL Full Text Search.

## Удаление

Soft delete:

```sql
is_deleted = true
```

Удаление каскадно применяется ко всем потомкам.

## Кеширование

Redis используется для:

* кеширования комментариев;
* кеширования деревьев комментариев;
* ускорения чтения.

---

# Структура таблицы

```sql
CREATE TABLE comments (
    id UUID PRIMARY KEY,
    parent_id UUID REFERENCES comments(id) ON DELETE SET NULL,
    author VARCHAR(100) NOT NULL,
    content TEXT NOT NULL,
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
    path TEXT NOT NULL,
    depth INT NOT NULL DEFAULT 0 CHECK (depth >= 0),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ
);
```

---

# API

## Create Comment

### POST `/comments`

Создание комментария.

### Request

```json
{
  "author": "admin",
  "content": "hello world"
}
```

или вложенный комментарий:

```json
{
  "parent_id": "550e8400-e29b-41d4-a716-446655440001",
  "author": "user",
  "content": "reply"
}
```

### Response

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440001",
  "author": "admin",
  "content": "hello world",
  "depth": 0
}
```

---

## Get Comments Tree

### GET `/comments`

### Query params

| Param     | Description        |
| --------- | ------------------ |
| page      | Номер страницы     |
| page_size | Размер страницы    |
| parent_id | Получить поддерево |

### Example

```bash
GET /comments?page=1&page_size=20
```

---

## Search Comments

### GET `/comments/search`

### Query params

| Param     | Description      |
| --------- | ---------------- |
| q         | Поисковый запрос |
| page      | Номер страницы   |
| page_size | Размер страницы  |

### Example

```bash
GET /comments/search?q=hello
```

---

## Delete Comment

### DELETE `/comments/{id}`

Soft delete комментария и всех дочерних элементов.

---

## Healthcheck

### GET `/health`

```json
{
  "status": "ok"
}
```

---

# Swagger

Swagger UI доступен по адресу:

```text
http://localhost:8080/swagger/index.html
```

---

# Конфигурация

Все настройки задаются через ENV.

## Основные переменные

### HTTP

```env
HTTP_HOST=0.0.0.0
HTTP_PORT=8080
```

### PostgreSQL

```env
DB_DSN=postgres://user:pass@localhost:5432/ctree?sslmode=disable
```

### Redis

```env
CACHE_ADDR=localhost:6379
CACHE_DB=0
```

### Service

```env
SERVICE_MAX_DEPTH=10
SERVICE_DEFAULT_PAGE_SIZE=20
SERVICE_MAX_PAGE_SIZE=100
```

---

# Запуск

## Локально

### 1. Клонировать проект

```bash
git clone <repo>
cd ctree
```

### 2. Запустить PostgreSQL и Redis

Можно через Docker:

```bash
docker compose up -d
```

### 3. Выполнить миграции

```bash
migrate -path migrations -database "$DB_DSN" up
```

### 4. Запустить приложение

```bash
go run cmd/main.go
```

---

# Docker

## Build

```bash
docker build -t ctree .
```

## Run

```bash
docker run -p 8080:8080 ctree
```

---

# Индексы

Используются индексы:

```sql
CREATE INDEX idx_comments_path
ON comments USING btree (path varchar_pattern_ops);

CREATE INDEX idx_comments_fts
ON comments USING GIN (
    to_tsvector('english', author || ' ' || content)
);

CREATE INDEX idx_comments_root
ON comments (id DESC)
WHERE parent_id IS NULL AND is_deleted = FALSE;
```

---

# Особенности реализации

## Materialized Path

Для построения дерева используется `path`.

Пример:

```text
/root
/root/child
/root/child/subchild
```

Это позволяет:

* быстро получать поддеревья;
* эффективно сортировать дерево;
* просто выполнять каскадное удаление.

---

## Кеширование дерева

Кешируются:

* root comments;
* subtree;
* страницы.

Ключи:

```text
tree:root:p1_s20
tree:<parent_id>:p1_s20
```

---

# Graceful Shutdown

Приложение корректно завершает работу:

* HTTP server shutdown;
* закрытие PostgreSQL pool;
* закрытие Redis connections.

---

# Middleware

Используются middleware:

* request id;
* logging;
* CORS;
* panic recovery.

---

# Логирование

Логируются:

* HTTP запросы;
* длительные операции;
* ошибки;
* запуск/остановка сервиса.

---

# Ограничения

| Ограничение      | Значение     |
| ---------------- | ------------ |
| Max depth        | configurable |
| Max request body | 1 MB         |
| Max page size    | 100          |

---

# Пример дерева

```text
Comment 1
├── Reply 1
│   ├── Reply 1.1
│   └── Reply 1.2
└── Reply 2
```

---

# TODO

* JWT authentication
* Rate limiting
* WebSocket updates
* Metrics (Prometheus)
* OpenTelemetry tracing
* Unit tests / integration tests
* Cursor pagination
* Kubernetes deployment

---

# License

MIT
