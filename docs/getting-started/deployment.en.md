# Deployment Guide

This guide covers production configuration, migrations and persistence. For local development, use the [README](../../README.md#-get-running). The complete configuration reference is [.env.example](../../.env.example).

## Production configuration

On a new deployment, copy the template to `.env` and edit it before starting:

```bash
cp .env.example .env
chmod 600 .env
```

Set these values; generate each secret independently with `openssl rand -hex 32`:

```dotenv
APP_ENV=prod
ACCESS_TOKEN_SECRET=replace-with-generated-access-secret
REFRESH_TOKEN_SECRET=replace-with-generated-refresh-secret
ADMIN_PASSWORD=replace-with-a-strong-admin-password
```

Production rejects built-in JWT secrets. The seed password is used only when creating the first administrator; it does not reset an existing account. Local passwords must contain at least 8 characters and at most 72 UTF-8 bytes.

For Google/GitHub login, configure provider credentials and an independent `IDENTITY_SESSION_SECRET`. Set `IDENTITY_BASE_URL` to the public backend origin and `IDENTITY_REDIRECT_URL` to the frontend `/oauth-callback` URL. When using email invitations, set `INVITATION_ACCEPT_URL` to the public frontend `/accept-invitation` URL and configure the Resend fields.

Environment variables override `.env`. Changing configuration requires restarting the service.

## Docker Compose

The [Dockerfile](../../Dockerfile) builds the frontend, generates Go code and packages a single binary. The following example runs SQLite with persistent bind mounts. Create `compose.yaml` at the repository root:

```yaml
services:
  shadmin:
    build: .
    image: shadmin:local
    restart: unless-stopped
    ports:
      - "127.0.0.1:55667:55667"
    env_file: .env
    volumes:
      - ./.database:/app/.database
      - ./.uploads:/app/.uploads
      - ./.logs:/app/.logs
```

The image runs as UID/GID 1000. On a Linux host, prepare the bind mounts so that user can write to them:

```bash
mkdir -p .database .uploads .logs
sudo chown 1000:1000 .database .uploads .logs

docker compose build
docker compose run --rm shadmin ./shadmin-runner -migrate
docker compose up -d
```

Migration must succeed before starting the service. Inspect startup with `docker compose logs --tail=100 shadmin`. The port is bound to localhost for access through a reverse proxy; use SSH forwarding for remote administrative checks before the proxy is configured.

## Database migrations

Production startup never changes the schema, even if `DB_AUTO_MIGRATE=true`. Outside production, automatic additive schema synchronization is enabled by default and can be disabled with `DB_AUTO_MIGRATE=false`.

`-migrate` applies additive Ent schema changes and exits without starting the server. Run it with the same configuration and database as the serving process:

```bash
# Docker Compose
docker compose run --rm shadmin ./shadmin-runner -migrate

# Native binary
./shadmin -migrate
```

Back up first and test changes against a restored copy. Removing/renaming columns or transforming existing data requires reviewed versioned SQL migrations; `-migrate` does not replace those steps.

For an upgrade:

1. Build the new image or binary.
2. Stop the application and back up the database, `.env` and uploaded files. For SQLite, back up the whole `.database/` directory while the service is stopped.
3. Apply the required migrations using the new version. If they fail, resolve the failure before starting the service.
4. Start the new version and verify health, login and role access. A binary rollback does not reverse database changes.

## Database, storage and cache choices

### PostgreSQL or MySQL

Provision the database and set `DB_TYPE` and `DB_DSN` in `.env`. The Docker example needs no `.database/` mount when using an external database.

```dotenv
# PostgreSQL
DB_TYPE=postgres
DB_DSN=postgres://shadmin:password@database-host:5432/shadmin?sslmode=require

# Or MySQL
DB_TYPE=mysql
DB_DSN=shadmin:password@tcp(database-host:3306)/shadmin?parseTime=true&loc=Local&charset=utf8mb4
```

Use a database host reachable from the container; `localhost` inside the container refers to the container itself. MySQL needs `parseTime=true`. Changing database engines does not transfer existing data; import it separately and run the migration step against the target database.

### File storage

Disk storage defaults to `.uploads/`. If you change `STORAGE_BASE_PATH`, update the corresponding mount.

For object storage, select the bucket with `S3_BUCKET`. Allow listing its objects, reading, writing and deleting them, and aborting multipart uploads. To supply credentials explicitly, set both `S3_ACCESS_KEY` and `S3_SECRET_KEY`. For temporary credentials, also set `S3_TOKEN`. See [.env.example](../../.env.example).

- **AWS S3 (`STORAGE_TYPE=s3`).** Use a pre-created bucket. Set its region with `S3_REGION` or the AWS SDK configuration, such as `AWS_REGION`. The SDK uses its default credential chain, including IAM roles, when explicit credentials are absent.
- **MinIO (`STORAGE_TYPE=minio`).** Supply explicit credentials and set `S3_ADDRESS` to the S3 API endpoint (`host:port`). Use `S3_SECURE=false` for HTTP. The region defaults to `us-east-1`. Set `S3_REGION` for other regions before deploying or upgrading. Uploads create missing buckets if permitted.

| Container path   | Persist                                      |
| ---------------- | -------------------------------------------- |
| `/app/.database` | SQLite database, including journal/WAL files |
| `/app/.uploads`  | Files when using default disk storage        |
| `/app/.logs`     | Rotating application logs                    |

Keep `.env` and backups outside the image. Protect and back up database/object storage independently when using external services.

### Multiple instances

All instances must use the same authoritative database and consistent token secrets. Use shared object storage for uploads and `CACHE_TYPE=redis` with the `REDIS_*` fields to share user-state, Captcha and JWT blacklist caches.

Each system request checks the database authorization generation before using its local Casbin snapshot. A changed generation triggers a rebuild; read/build failures return HTTP 503. Each protected system request therefore adds a database read. Local commit triggers and `AUTHZ_SYNC_POLL_INTERVAL_SECONDS` polling only warm and recover snapshots; Redis does not refresh other instances' Enforcers.

## Reverse proxy and health

Terminate HTTPS at the reverse proxy and forward requests to the local application port. For an existing Nginx TLS server block, the application location can be:

```nginx
location / {
    proxy_pass http://127.0.0.1:55667;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    client_max_body_size 50m;
}
```

Set the upload limit for your use case. If Swagger should be private, block `/swagger/` at the proxy. Check and reload your proxy configuration after changes.

Run the health probe from the host or your monitoring service:

```bash
curl --fail http://127.0.0.1:55667/api/v1/health
```

This endpoint reports that the HTTP service is responding; it does not test database readiness or authorization. The runtime image does not include `curl`, so an in-container probe needs a derived image containing a probe client. Login is a POST endpoint and must not be used as a GET health probe.

## Native binary

Build the frontend and generate Ent as described in the [README](../../README.md#-get-running), then build from the repository root:

```bash
CGO_ENABLED=1 go build -o shadmin .
```

On the target host, place `.env` in the working directory, grant the service user write access to the configured data/log directories, run `./shadmin -migrate`, and start `./shadmin` under your process supervisor. The binary embeds the frontend and Swagger; it still needs its runtime libraries and persistent data. Apply the same migration, backup and proxy steps as for Docker.
