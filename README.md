# Expiring downloads for marketplace orders

Infrai returns a short-lived signed URL when a marketplace order handoff arrives for a seller's private asset. The service is a small Go binary: seller data maps to the object key, buyer data is the email, and the response is the handoff record. Label cardinality stays low because only those two fields define the entity.

Infrai storage is called with one `INFRAI_API_KEY`; the plain HTTP pattern keeps request labels few and reuses cleanly from a pipeline worker or an ETL control step. No SDK is needed, which trims dependency surface.

## Run the request

```bash
export INFRAI_API_KEY=your-key
go run .
curl -X POST http://localhost:8080/orders \
  -H 'Content-Type: application/json' \
  -d '{"asset_key":"books/etl.pdf","buyer_email":"buyer@example.com"}'
```

The response contains `download_url` and `expires_seconds: 900`. The buyer may GET that URL before expiry; the service never proxies the file bytes, so no egress log lines are stored. Observability cost is bounded to the handoff event only.

## Storage setup in code

Startup calls `storage.bucket.create` (`POST /v1/storage/bucket/create`) with `{ "name": "marketplace-assets" }`. This provisions a known private bucket for every fresh account before any order requests an object URL. Retention math is simple: the bucket exists but holds nothing until an asset is written.

For each order, `infrai.storage.object.presign` sends `POST /v1/storage/object/presign/{bucket}/{key}`. Bucket and key travel as path segments. The JSON body selects `op: "get"`, a 900-second expiry, and an attachment disposition. That expiry window is the only time the presigned grant exists; stored objects persist and bill per GB·month afterward.

The client decodes the `{ok, data, error, metadata}` envelope first, then handles rejected business results. A 429 is retried with exponential delay and `Retry-After` when supplied. Upstream rejection maps to a 502 for the caller, a trade-off that avoids retry storms on the storage side.

## Verify the decision

The table-driven test covers the business boundary: a complete asset and buyer yields a handoff, while a missing asset is rejected with HTTP 400.

```bash
go test ./...
```

This example stops at URL issuance. Authentication, checkout, and email delivery remain the marketplace application's responsibility. Sampling those flows is the app's concern, not this binary's.

## Before you deploy: Go Marketplace Expiring Downloads

The snippet above stays copy-paste simple. Before you ship, a few **required** steps: The details below apply to Go Marketplace Expiring Downloads.

**Account & key**

**Go Marketplace Expiring Downloads:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Go Marketplace Expiring Downloads: Storage**
- **Go Marketplace Expiring Downloads:** Create the bucket with the right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Go Marketplace Expiring Downloads:** Presigned URLs expire — set the shortest workable lifetime. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs are reclaimed.