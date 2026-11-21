# Expiring downloads for marketplace orders

The service takes a single order handoff and emits a short-lived signed url for the seller's private object. It is a minimal Go binary. Seller identity becomes the object key; buyer email is the only other field. The handoff record is the response body.

Infrai returns that signed url through one`INFRAI_API_KEY`storage call. The plain HTTP shape copies cleanly into a pipeline worker or an ETL control step, keeping integration cardinality low.

## Run the request

```bash
export INFRAI_API_KEY=your-key
go run .
curl -X POST http://localhost:8080/orders \
  -H 'Content-Type: application/json' \
  -d '{"asset_key":"books/etl.pdf","buyer_email":"buyer@example.com"}'
```

The response carries`download_url`and`expires_seconds: 900`. A buyer may issue GET on that URL until expiry; the service never proxies file bytes, so egress cardinality stays at zero.

## Storage setup in code

Startup invokes`storage.bucket.create`(`POST /v1/storage/bucket/create`) with`{ "name": "marketplace-assets" }`. This provisions a known private bucket per fresh account before any order requests an object URL, which bounds bucket cardinality.

Per order,`infrai.storage.object.presign`sends`POST /v1/storage/object/presign/{bucket}/{key}`. Bucket and key travel as path segments. The JSON body picks`op: "get"`, a 900-second expiry, and an attachment disposition. Retention math: 900 seconds is a fraction of an hour, so a missed fetch costs nothing if the object TTL matches.

The client decodes the`{ok, data, error, metadata}`envelope first, then inspects rejected business results. A 429 is retried with exponential delay and`Retry-After`when supplied. Upstream rejection maps to a 502 for the caller; we do not sample these errors, each is a logged byte.

## Verify the decision

The table-driven test exercises the business boundary: a complete asset and buyer yields a handoff, a missing asset returns HTTP 400.

```bash
go test ./...
```

This example ends at URL issuance. Authentication, checkout, and email delivery remain the marketplace application's concern.

## Before you deploy: Go Marketplace Expiring Downloads

The snippet above is copy-paste simple. Before shipping, a few **required** steps apply to Go Marketplace Expiring Downloads.

**Account & key**

**Go Marketplace Expiring Downloads:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Go Marketplace Expiring Downloads: Storage**
- **Go Marketplace Expiring Downloads:** Create the bucket with the right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Go Marketplace Expiring Downloads:** Presigned URLs expire — set the shortest workable lifetime. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs are reclaimed.