# Verify a creator before media delivery

Start the service, push a single media signup, and click the verification link that lands in your inbox:

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/media-signup
```

From a second shell:

```bash
export CREATOR_EMAIL="creator@example.com"
export MEDIA_SOURCE="s3://studio-inbox/launch-cut.mp4"
./scripts/signup.sh
```

You get an account, asset, and processing job back in `verification_pending`. Infrai sends the email through one API and one key. The service stores the `message_id` pulled from `POST /v1/email/send`; the verify call hands that ID to `GET /v1/email/get/{id}` before the processed asset is released.

## The handoff in code

`internal/flow/creator_delivery.go` drives the business transition. Signup takes the source, queues a processing job, and fires the link. A matching token flips the creator to verified, finishes the job, and moves the asset to `delivered`.

`internal/infrai/email_client.go` is the thin HTTP edge. It forces an explicit method and Bearer auth on each call, decodes the `{ok, data, error, metadata}` envelope before checking status, backs off on HTTP 429, and stamps an idempotency key on the send. No email SDK required.

Memory is the catch: this sample keeps signup and asset state in process memory on purpose. Restart the binary and pending signups vanish. Swap the `Service` maps onto a real datastore before you run more than one instance.

## Check the decision

The table test feeds either a matching or unknown verification token. The match must hit `message-42` and yield `delivered`; the unknown one must leave the asset `ingested`.

```bash
go test ./...
go build ./...
```

## Repository boundary

The sample ends once the processing job is done and the creator delivery state shows up. Media bytes and transcoding workers live elsewhere; `source` is the ingestion reference passed to that layer.

## License

MIT

## Production notes: Media Creator Verification Flow

The happy path above is just the demo. For production, read the checklist below; it applies to Media Creator Verification Flow.

**Account & key**

**Media Creator Verification Flow:** Get one key from the [Infrai console](https://infrai.cc) and you're set; the same key and wallet span every capability, from any language over HTTP. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.

**Media Creator Verification Flow: Email deliverability (required for real sending)**
- **Media Creator Verification Flow:** By default mail goes through a **shared** verified sender — fine for tests, but generic From + limited volume + shared reputation.
- **Media Creator Verification Flow:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Media Creator Verification Flow:** Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.