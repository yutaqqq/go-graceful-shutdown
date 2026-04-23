# go-graceful-shutdown

[![CI](https://github.com/yutaqqq/go-graceful-shutdown/actions/workflows/ci.yml/badge.svg)](https://github.com/yutaqqq/go-graceful-shutdown/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/go-1.22-blue.svg)](https://go.dev/)
[![Go Reference](https://pkg.go.dev/badge/github.com/yutaqqq/go-graceful-shutdown.svg)](https://pkg.go.dev/github.com/yutaqqq/go-graceful-shutdown)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Минималистичная библиотека для graceful shutdown Go-приложений. Нулевые зависимости — только стандартная библиотека.

## Установка

```bash
go get github.com/yutaqqq/go-graceful-shutdown
```

## Быстрый старт

```go
s := shutdown.New()

s.Register(shutdown.Handler{Name: "db",   Timeout: 5 * time.Second,  Fn: db.Close})
s.Register(shutdown.Handler{Name: "http", Timeout: 10 * time.Second, Fn: srv.Shutdown})

// Блокирует до SIGTERM / SIGINT / отмены ctx.
// Handlers вызываются в обратном порядке: http → db.
if err := s.Listen(ctx); err != nil {
    log.Printf("shutdown errors: %v", err)
}
```

## API

### `shutdown.New(opts ...Option) *Shutdown`

Создаёт оркестратор. По умолчанию слушает `SIGTERM` и `SIGINT`.

```go
// Изменить сигналы:
s := shutdown.New(shutdown.WithSignals(syscall.SIGTERM))
```

### `s.Register(h Handler) *Shutdown`

Регистрирует handler. Возвращает receiver для цепочки вызовов. Потокобезопасен.

```go
type Handler struct {
    Name    string                           // для сообщений об ошибках
    Timeout time.Duration                    // 0 → 30-секундный дефолт
    Fn      func(ctx context.Context) error
}
```

### `s.Listen(ctx context.Context) error`

Блокирует до сигнала или отмены `ctx`, затем запускает все handlers в порядке LIFO (последний зарегистрированный — первый выполненный, как `defer`). Собирает все ошибки через `errors.Join`.

### `s.Shutdown() error`

Запускает handlers немедленно, без ожидания сигнала. Удобно для тестов.

## Порядок выполнения

Handlers выполняются в **обратном** порядке регистрации — аналогично `defer`:

```
Register: db → cache → http
Shutdown: http → cache → db
```

Это гарантирует корректный порядок при наличии зависимостей: HTTP-сервер останавливается первым (перестаёт принимать запросы), затем освобождаются ресурсы.

## Примеры

- [`examples/http`](examples/http/main.go) — HTTP-сервер
- [`examples/worker`](examples/worker/main.go) — фоновый воркер
