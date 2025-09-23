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

### Users

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

400 {"error":"invalid email"} - невалидная почта

400 {"error":"weak password"} - слишком короткий пароль (минимум 8)

409 {"error":"email already taken"} - email уже занят

500 {"error":"internal error"} - иное

___

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

401 {"error":"invalid credentials"} — неверная почта/пароль/пользователь

500 {"error":"internal error"} — внутренняя ошибка

___

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

401 {"error":"unauthorized"} - отсутствует/неверный/просроченный токен

401 {"error":"invalid token"} - битый формат и т.п.


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