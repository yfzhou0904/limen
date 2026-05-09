# Contributing

## Local dev

```bash
cp .env.example .env   # fill in keys
cd ui && npm install && npm run build && cd ..
go run ./server
```

UI dev with hot reload: `cd ui && npm run dev` (proxies `/api` to the Go server).

## Deploy (Railway)

The repo deploys as a single container via the included `Dockerfile`. The Go binary embeds the built UI.

```bash
railway up --detach -m "<short message>"
```

Set the env vars from `.env.example` in the Railway service. Mount a volume at `/data` so SQLite + blobs persist across deploys.
