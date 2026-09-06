# ValleyVista Backend (PocketBase)

Backend for ValleyVista real estate marketplace.

## Setup Locally

1. Clone repo
2. `go mod tidy`
3. `go run main.go serve`
4. Open `http://127.0.0.1:8090/_/` to access admin UI

## Environment Variables

- `ADMIN_EMAIL`, `ADMIN_PASSWORD` – create an admin user with isAdmin flag on first run.
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
3. Set start command: `./pocketbase serve --http=0.0.0.0:8080`
4. Add a persistent disk mounted at `/app/pb_data` (size 1GB or more).
5. Set environment variables: `ADMIN_EMAIL`, `ADMIN_PASSWORD`.
