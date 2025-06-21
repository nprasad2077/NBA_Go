# Implement a Caching Layer with Redis

For even more speed, especially for data that doesn't change every second, a caching layer is the answer. The goal is to avoid hitting the database at all for repeated requests.

* **How it Works:** When a request comes in (e.g., for Houston Rockets 2024 stats), your Go application first checks if the result is already in Redis (an extremely fast in-memory database).
    * If it's in Redis, it returns the cached data immediately (e.g., in <50ms).
    * If it's not in Redis, it queries the PostgreSQL database as normal, returns the result to the user, and saves a copy of that result in Redis for a set amount of time (e.g., 5-10 minutes).
* **Your Advantage:** Your Coolify `docker ps` output shows you already have a **Redis container running** as part of Coolify's core services. You can easily create a new, dedicated Redis database for your application within Coolify and start using it.

You have built an incredibly solid foundation. These optimizations are the natural next steps to take your already impressive project to the next level of performance. Well done!