# ValleyVista Backend (PocketBase)

Backend for ValleyVista real estate marketplace.

## Setup Locally

1. Clone repo
2. `go mod tidy`
3. `go run main.go serve`
4. Open `http://127.0.0.1:8090/_/` to access admin UI

## Environment Variables

- `ADMIN_EMAIL`, `ADMIN_PASSWORD` – create an admin user with isAdmin flag on first run.
- `IDRIVE_BUCKET`, `IDRIVE_REGION`, `IDRIVE_ACCESS_KEY_ID`, `IDRIVE_SECRET_ACCESS_KEY` – Litestream backup configuration.
- Allowed origins are set to `*` (modify in code for production).

## Collections

- `users` (built-in, extended with `isAdmin`)
- `properties`
- `contact_requests`
- `property_requests`
- `owner_enquiries`
- `professionals`
- `favorites`

## Deployment on Render

1. Create a new Web Service, connect GitHub repo.
2. Set build command: `docker build -t valleyvista-backend .`
3. Use the Dockerfile `CMD` or set the start command to `./start.sh`. Do not bypass `start.sh` with `./pocketbase serve`, because that skips restore and replication.
4. Add a persistent disk mounted at `/app/pb_data` (size 1GB or more).
5. Set `ADMIN_EMAIL`, `ADMIN_PASSWORD`, `IDRIVE_BUCKET`, `IDRIVE_REGION`, `IDRIVE_ACCESS_KEY_ID`, and `IDRIVE_SECRET_ACCESS_KEY`.

`start.sh` attempts restore on every boot, starts Litestream with verbose logs,
and restarts replication if the process exits. The Render logs include the
database size before/after restore and the Litestream process ID.
