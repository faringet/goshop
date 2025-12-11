# goshop

## quickstart
### Поднимаем Postgre
```
docker run -d --name goshop-postgres \
-e POSTGRES_USER=goshop \
-e POSTGRES_PASSWORD=goshop \
-e POSTGRES_DB=goshop \
-p 5433:5432 \
-v pgdata:/var/lib/postgresql/data \
postgres:12
```

___

### Накатываем миграции

1. ```go run services/users/cmd/migrate/main.go status```
2. ```go run services/users/cmd/migrate/main.go up```
3. ```go run services/users/cmd/migrate/main.go status (должно показать Applied)```


### Стартуем
```go run services/users/cmd/users/main.go```

___

## API
### Health

* GET ```/live``` → 200 ok
* GET ```/ready``` → 200 ok или 503 db not ready
* GET ```/v1/db/ping``` → 200 {"status":"ok","latency_ms":...} или 503

___

## Users
### Регистрация

* POST ```/v1/users/register```

**Request:**
```json
{
  "email": "torvalds@linux-foundation.org",
  "password": "RustIsAPoison2013!"
}
```

**Response:**
```json

201 Created

{
"id": "b6c1d746-0f8d-4a0a-9352-7b6d5b4e6a9a",
"email": "RustIsAPoison2013!",
"created_at": "2025-09-19T11:47:11Z"
}
```

**Ошибки:**

400 {"error":"invalid email"}

400 {"error":"weak password"} - минимум 8 символов

409 {"error":"email already taken"}

500 {"error":"internal error"}

___

### Логин
* POST ```/v1/users/login```

**Request:**
```json
{
"email": "user@example.com",
"password": "StrongPass123!"
}
```

**Response:**
```json
200 ok

{
  "access_token":  "<JWT_ACCESS>",
  "refresh_token": "<JWT_REFRESH>",
  "token_type":    "Bearer",
  "expires_in":    7200
}
```

**Ошибки:**

401 {"error":"invalid credentials"}

500 {"error":"internal error"}

___

### Текущий юзер
* GET ```/v1/users/me```

**Request:**
```json
Authorization: Bearer <JWT_ACCESS>
```

**Response:**
```json
200 ok

{
  "uid":        "c583756e-a3a4-4c11-a3dd-80d9b1e9bc43",
  "email":      "user@example.com",
  "issuer":     "goshop-users",
  "subject":    "access",
  "issued_at":  "2025-09-22T13:00:00Z",
  "expires_at": "2025-09-22T15:00:00Z"
}
```

**Ошибки:**

401 {"error":"unauthorized"}

401 {"error":"invalid token"}

___

### Обновление токенов 
* POST ```/v1/users/refresh```

**Request:**
```json
  "refresh_token": "<JWT_REFRESH>"
```

**Response:**
```json
200 ok

{
  "access_token":  "<NEW_JWT_ACCESS>",
  "refresh_token": "<NEW_JWT_REFRESH>",
  "token_type":    "Bearer",
  "expires_in":    900
}
```

___

### Выход с одного устройства
* POST ```/v1/users/logout```

**Request:**
```json
  "refresh_token": "<JWT_REFRESH>"
```

**Response:**
```json
204
```

**Поведение**

* инвалидирует одну сессию 

* действующие access-токены этой сессии останутся валидны до ```exp```

___

### Выход со всех устройств
* POST ```/v1/users/logout_all```

**Request:**
```json
Authorization: Bearer <JWT_ACCESS>
```

**Response:**
```json
200 ok

{ "revoked": 3 }
```

```revoked``` - сколько активных сессий отозвано

**Поведение**

* инвалидирует одну сессию

* действующие access-токены этой сессии останутся валидны до ```exp```


## Дефолтные миграции:
```services/users/migrations/```

* статус
```go run services/users/cmd/migrate/main.go status```


* применить всё новое
```go run services/users/cmd/migrate/main.go up```


* откатить последнюю миграцию
```go run services/users/cmd/migrate/main.go down```


* откатить всё
```go run services/users/cmd/migrate/main.go reset```


* перейти на конкретную версию

```go run services/users/cmd/migrate/main.go up-to  20250910121530```


```go run services/users/cmd/migrate/main.go down-to 20250910121530```


* пересоздать последнюю (down+up)
```go run services/users/cmd/migrate/main.go redo```


* сгенерировать файлы новой миграции (Up/Down в одном .sql)
```go run services/users/cmd/migrate/main.go create add_users_index sql```

___
___
___

# Makefile команды
---

### Всё в Docker

- **`make dev-docker-only`** супер старт

- **`make dev-docker-only-down`**  сносим все нахер

### Гибрид: infra в Docker, сервисы в Kubernetes

- **`make dev-hybrid`** супер старт

- **`make dev-hybrid-down`**  останавливаем


### Гибрид: infra в Docker, сервисы в Kubernetes (kind)

- **`make docker-infra-up`**  
  Поднимает только **инфраструктуру** в Docker:
  БД, Kafka, Ollama, стэк логирования и т.п.  
  Никакие сервисы (users/orders/…) не стартуют.

- **`make docker-infra-down`**  
  Останавливает и удаляет контейнеры и volume’ы, поднятые `docker-infra-up`.

- **`make dev-hybrid`**  
  Комплексный сценарий:
    1. Поднимает инфраструктуру в Docker (`docker-infra-up`),
    2. Создаёт кластер kind (если его нет),
    3. Собирает и загружает все образы в kind,
    4. Применяет все k8s-манифесты сервисов,
    5. Ставит ingress-nginx и Ingress’ы.  
       В итоге: infra/logs в Docker, сервисы — в k8s.

- **`make dev-hybrid-down`**  
  Обратный сценарий:
    1. Чистит namespace `goshop` в кластере (`k8s-clean`),
    2. Останавливает инфраструктуру в Docker (`docker-infra-down`).

---

## Кластер kind

- **`make k8s-kind-create`**  
  Создаёт кластер kind с именем `$(KIND_CLUSTER)` (по умолчанию `goshop`).

- **`make k8s-kind-delete`**  
  Полностью удаляет кластер kind с именем `$(KIND_CLUSTER)`.

---

## Ingress-nginx и Ingress’ы приложения

- **`make k8s-ingress-nginx-download`**  
  Скачивает официальный манифест ingress-nginx для kind в файл  
  `k8s/infra/ingress-nginx-kind.yaml`.

- **`make k8s-ingress-nginx-install`**  
  Устанавливает ingress-nginx в кластер из локального манифеста  
  `k8s/infra/ingress-nginx-kind.yaml`.

- **`make k8s-ingress-nginx-wait`**  
  Ждёт, пока pod контроллера ingress-nginx перейдёт в состояние `Ready`.

- **`make k8s-ingress-apply`**  
  Применяет все Ingress-ресурсы приложения:  
  `k8s/users-ingress.yaml`, `k8s/gateway-ingress.yaml`, `k8s/opsassistant-ingress.yaml`.

- **`make k8s-ingress-bootstrap`**  
  Полный цикл для ingress:
    1. Скачать манифест (если нужно),
    2. Установить ingress-nginx,
    3. Дождаться готовности,
    4. Применить Ingress’ы приложения.

- **`make k8s-portforward-ingress`**  
  Делает `kubectl port-forward` `svc/ingress-nginx-controller` на `localhost:8080`  
  (чтобы ходить в users/gateway/opsassistant через `http://localhost:8080`).

---

## Сервис: users

- **`make users-image`**  
  Собирает Docker-образ `goshop-users:dev` из `services/users/Dockerfile` (target `users-app`).

- **`make users-kind-load`**  
  Загружает образ `goshop-users:dev` в кластер kind `$(KIND_CLUSTER)`.

- **`make users-build-and-load`**  
  Последовательно делает `users-image` + `users-kind-load`.  
  Удобная команда «собрать и загрузить users».

- **`make k8s-users-apply`**  
  Применяет все манифесты для users:
  `namespace-goshop.yaml`, `users-configmap.yaml`, `users-deployment.yaml`, `users-service.yaml`.

- **`make k8s-users-status`**  
  Показывает pods и сервисы users (по `app=users`) в namespace `$(K8S_NAMESPACE)`.

- **`make k8s-users-logs`**  
  Показывает последние логи деплоймента `users` (`kubectl logs deploy/users`).

---

## Сервис: orders

- **`make orders-image`**  
  Собирает Docker-образ `goshop-orders:dev`.

- **`make orders-kind-load`**  
  Загружает `goshop-orders:dev` в kind-кластер.

- **`make orders-build-and-load`**  
  Собрать + загрузить образ orders в kind.

- **`make k8s-orders-apply`**  
  Применяет манифесты orders: configmap, deployment, service.

- **`make k8s-orders-status`**  
  Статус pods и service для orders в namespace `$(K8S_NAMESPACE)`.

- **`make k8s-orders-logs`**  
  Логи деплоймента `orders`.

---

## Сервис: outboxer

- **`make outboxer-image`**  
  Собирает Docker-образ `goshop-outboxer:dev`.

- **`make outboxer-kind-load`**  
  Загружает `goshop-outboxer:dev` в kind.

- **`make outboxer-build-and-load`**  
  Собрать + загрузить образ outboxer.

- **`make k8s-outboxer-apply`**  
  Применяет манифесты outboxer: configmap, deployment.

- **`make k8s-outboxer-status`**  
  Статус pods outboxer в namespace `$(K8S_NAMESPACE)`.

- **`make k8s-outboxer-logs`**  
  Логи деплоймента `outboxer`.

---

## Сервис: payments

- **`make payments-image`**  
  Собирает Docker-образ `goshop-payments:dev`.

- **`make payments-kind-load`**  
  Загружает `goshop-payments:dev` в kind.

- **`make payments-build-and-load`**  
  Собрать + загрузить образ payments.

- **`make k8s-payments-apply`**  
  Применяет манифесты payments: configmap, deployment, service.

- **`make k8s-payments-status`**  
  Статус pods/service для payments.

- **`make k8s-payments-logs`**  
  Логи деплоймента `payments`.

---

## Сервис: gateway

- **`make gateway-image`**  
  Собирает Docker-образ `goshop-gateway:dev`.

- **`make gateway-kind-load`**  
  Загружает `goshop-gateway:dev` в kind.

- **`make gateway-build-and-load`**  
  Собрать + загрузить образ gateway.

- **`make k8s-gateway-apply`**  
  Применяет манифесты gateway: configmap, deployment, service.

- **`make k8s-gateway-status`**  
  Статус pods/service для gateway.

- **`make k8s-gateway-logs`**  
  Логи деплоймента `gateway`.

---

## Сервис: opsassistant

- **`make opsassistant-image`**  
  Собирает Docker-образ `goshop-opsassistant:dev`.

- **`make opsassistant-kind-load`**  
  Загружает `goshop-opsassistant:dev` в kind.

- **`make opsassistant-build-and-load`**  
  Собрать + загрузить образ opsassistant.

- **`make k8s-opsassistant-apply`**  
  Применяет манифесты opsassistant: configmap, deployment, service.

- **`make k8s-opsassistant-status`**  
  Статус pods/service для opsassistant.

- **`make k8s-opsassistant-logs`**  
  Логи деплоймента `opsassistant`.

---

## Агрегированные команды для k8s

- **`make k8s-build-and-load-all`**  
  Собирает и загружает в kind ВСЕ сервисы:
  users, orders, payments, outboxer, gateway, opsassistant.

- **`make k8s-apply-all`**  
  Применяет все манифесты сервисов:
  users, orders, payments, outboxer, gateway, opsassistant.

- **`make k8s-bootstrap`**  
  Полный bootstrap k8s (без Docker-инфры):
    1. Создать кластер kind,
    2. Собрать и загрузить все образы,
    3. Применить все манифесты сервисов,
    4. Поставить ingress-nginx и Ingress’ы.

- **`make k8s-status-all`**  
  Печатает сводную информацию:
    - Pods/Services/Ingress в namespace `$(K8S_NAMESPACE)`,
    - Pods/Services в namespace `ingress-nginx`.

---

## Очистка и redeploy

- **`make k8s-clean`**  
  Чистит namespace `$(K8S_NAMESPACE)`:
    - удаляет все `deployments`, `services`, `pods`, `jobs` и т.п.,
    - удаляет все `ingress`,
    - удаляет все `configmap`.  
      Кластер kind и ingress-nginx не трогаются.

- **`make k8s-redeploy-all`**  
  Полный redeploy всех сервисов:
    1. `users-build-and-load`, `orders-build-and-load`, `payments-build-and-load`,
       `outboxer-build-and-load`, `gateway-build-and-load`, `opsassistant-build-and-load`,
    2. `k8s-apply-all`.  
       Удобно после массовых изменений в коде.
