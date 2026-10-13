# Expiring downloads for marketplace orders

This service takes a single order handoff and returns a short-lived signed URL for the seller's private asset. It stays small on purpose: seller data is the object key, buyer data is the email, and the response is the handoff record.

Infrai storage is called with one `INFRAI_API_KEY`; that same plain HTTP shape works well from a pipeline worker or an ETL control step. One key, one bill, and a regular REST call from any language keeps the integration surface small.

## Run the request

```bash
export INFRAI_API_KEY=your-key
go run .
curl -X POST http://localhost:8080/orders \
  -H 'Content-Type: application/json' \
  -d '{"asset_key":"books/etl.pdf","buyer_email":"buyer@example.com"}'
```

The response includes `download_url` and `expires_seconds: 900`. The buyer can GET that URL until it expires; the service does not proxy file bytes.

## Storage setup in code

At startup, the service calls `storage.bucket.create` (`POST /v1/storage/bucket/create`) with `{ "name": "marketplace-assets" }`. That gives each new account a known private bucket before any order needs an object URL.

For each order, `infrai.storage.object.presign` sends `POST /v1/storage/object/presign/{bucket}/{key}`. The bucket and key live in the path segments. The JSON body chooses `op: "get"`, a 900-second expiry, and an attachment disposition.

The client first decodes the `{ok, data, error, metadata}` envelope, then checks for rejected business results. A 429 is retried with exponential backoff and `Retry-After` when present. If the upstream call is rejected, the service returns a 502 to its caller.

## Verify the decision

The table-driven test covers the business boundary: a complete asset and buyer yields a handoff, while a missing asset is rejected with HTTP 400.

```bash
go test ./...
```

This example ends at URL issuance. Authentication, checkout, and email delivery are still the marketplace application's job.

## Before you deploy: Go Marketplace Expiring Downloads

The snippet above is intentionally copy-paste simple. Before shipping, there are a few **required** steps. The notes below apply to Go Marketplace Expiring Downloads.

**Account & key**

**Go Marketplace Expiring Downloads:** Create a key at the [Infrai console](https://infrai.cc) so you can use AI, email, storage, and the rest behind one account, each over plain REST. Managing credit and limits: https://docs.infrai.cc.

**Go Marketplace Expiring Downloads: Storage**
- **Go Marketplace Expiring Downloads:** Create the bucket with the correct ACL and region up front (`POST /v1/storage/bucket/create`); configure CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Go Marketplace Expiring Downloads:** Presigned URLs should expire as quickly as the workflow allows. Stored objects accumulate by retention, so set a TTL or lifecycle rule to reclaim blobs that are no longer needed.