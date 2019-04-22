# 🚌 Tuktuk
> Simple Golang assessment tool for rapid development

## What is under the hood?
Under the hood a lot of different and cool packages such as:
- Logrus
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
Configuration file are located at `TUKTUK_CONFIG` or `config.yml`.  
Environment `BINDING_API` will be replaced to `binding.api`.  


### Logger
**Dependency:** `*logrus.Logger`  
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