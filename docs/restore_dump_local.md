# Restore to local container

```bash
docker exec nba_go-postgres-1 pg_dump -U nbago -Fc nba_db > ./2025.12.05_nba_backup.dump
```

Copy file into target DB container

```bash
docker exec nba_postgres psql -U nba_admin -d nba_analytics -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"

pg_restore -U nba_admin -d nba_analytics --no-owner --no-acl ./2025.12.05_nba_backup.dump
```
