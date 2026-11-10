# 🖖🚀 Narada
> Rapid web development, all in one place

[![Go Report Card](https://goreportcard.com/badge/github.com/m1ome/narada)](https://goreportcard.com/report/github.com/m1ome/narada)
[![GoDoc](https://godoc.org/github.com/m1ome/narada?status.svg)](https://godoc.org/github.com/m1ome/narada)
[![Build Status](https://travis-ci.org/m1ome/narada.svg?branch=master)](https://travis-ci.org/m1ome/narada)
[![Coverage Status](https://coveralls.io/repos/github/m1ome/narada/badge.svg?branch=master)](https://coveralls.io/github/m1ome/narada?branch=master)

## What is under the hood?
Under the hood a lot of different and cool packages such as:
- log/slog (stdlib)
- Go-PG
- Go-redis
- Uber FX
- Viper
- Sentry
- Pprof
- Prometheus

Also it supports:
- Workers using chapsuk workers package
- Http.Handler binding and serving

## Modules

### Config
**Dependency:** `*viper.Viper`  
**Configuration:**  
Configuration file are located at `NARADA_CONFIG` or `config.yml`.  
Environment `BINDING_API` will be replaced to `binding.api`.  


### Logger
**Dependency:** `*slog.Logger`  
**Configuration:**
```yaml
logger: 
    formatter: text # Supported values ['text', 'json']
    level: debug # Supported values ['debug', 'info', 'warn', 'error']
    catch_errors: true # Sending >=error level to sentry
    slack: false # Sending >= error level to slack 
    slack_url: "" # Slack webhook url [Required when slack is true]
    slack_icon: ""
    slack_emoji: ":ghost:"
    slack_username: "<YOUR_APP_NAME>_bot"
    slack_channel: "" #  Slack username [Required when slack is true]
```

### Workers
This module generates

**Dependency:** `*narada.Workers`

### Clients
Here all clients that are provided automatically located.

### Redis
**Dependency:** `*redis.Client`
**Configuration:**  
```yaml
redis:
    addr: "" # Redis address, in e.g.: 127.0.0.1:6379
    db: 0
    password: ""
    pool_size: 10
    idle_timeout: 60s
```

### PostgreSQL
**Dependency:** `*pg.DB`  
**Configuration:**
```yaml
database:
    addr: "" # PostgreSQL address
    user: ""
    password: ""
    database: ""
    pool: 10
    ssl: true
    migrations_lock_timeout: 15m # how long `migrate:up`/`migrate:down` wait for another migrator
```

`migrate:up` and `migrate:down` hold a Postgres session advisory lock (goose's default lock ID) while they run, so migrators started concurrently — e.g. one init container per ECS task — apply each migration exactly once. The rest wait, then find nothing pending. A migrator that can't take the lock within `migrations_lock_timeout` fails. Session advisory locks need a direct connection or session pooling; they don't hold through a transaction-mode pooler.

## Upgrading to v1.8

v1.8 replaces logrus with `log/slog`; go-pg and go-redis v8 stay. Consumers need code changes.

- Injected constructors take `*slog.Logger` instead of `*logrus.Logger`. Replace `log.WithError(err).Error("msg")` with `log.Error("msg", narada.Err(err))`, `log.WithField(k, v)` with `log.With(k, v)`, and `log.Fatal(...)` with `narada.Fatal(log, ...)` (slog has no fatal level — the record is emitted at error level and the process exits).
- `NewLogrusSentryHook`/`NewLogrusSlackHook` became the `slog.Handler` decorators `NewSentryHandler`/`NewSlackHandler`.
- `clients.NewPostgreSQL` takes `*slog.Logger`.
