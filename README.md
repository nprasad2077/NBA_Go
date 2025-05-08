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