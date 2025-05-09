# NBA_Go

### First‑time bootstrap

```bash
# 1. build + run
docker-compose up --build -d

# 2. create API key (ADMIN_SECRET is loaded from .env)
curl -XPOST http://localhost:8080/admin/keys \
  -H "X-Admin-Secret: $ADMIN_SECRET" \
  -d '{"label":"local-test"}'
# → { "id":1, "apiKey":"ab12cd…" }

# 3. call a protected endpoint
curl http://localhost:8080/api/playeradvancedstats \
     -H "X-API-Key: ab12cd…"

```

### Swagger Initiate Docs

```bash
swag init -g main.go -o docs
```

### Test

```bash
go run loadtest.go -n 100 -c 10 -url "http://127.0.0.1:8080/api/playeradvancedstats?page=1&pageSize=20" -log results.log -key "xxx"
```
