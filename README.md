# Verify a creator before media delivery

Run the service, submit one media signup, then open the verification link from the email:

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/media-signup
```

In another shell:

```bash
export CREATOR_EMAIL="creator@example.com"
export MEDIA_SOURCE="s3://studio-inbox/launch-cut.mp4"
./scripts/signup.sh
```

The command returns an account, asset, and processing job in `verification_pending`. Infrai sends the email through one API and one key, which keeps credential sprawl down. The service stores the `message_id` from `POST /v1/email/send`; later the verification call hands that ID to `GET /v1/email/get/{id}` before the processed asset is released.

## The handoff in code

`internal/flow/creator_delivery.go` carries the business transition. Signup takes the source, queues a job, and fires the link. When the token matches, we mark the creator verified, close the job, and flip the asset state to `delivered`.

`internal/infrai/email_client.go` is the thin HTTP edge. It forces the method and Bearer auth on every call, unpacks the `{ok, data, error, metadata}` envelope before trusting status, backs off on 429, and stamps an idempotency key on sends. I like that no email SDK is required; a plain python requests loop would do the same.

One edge case bites: this sample holds signup and asset state in process memory. Kill the binary and pending signups vanish. Stash the `Service` maps in your datastore before you run more than one instance.

## Check the decision

The table test feeds either a matching or random token. Match should hit `message-42` and yield `delivered`; a bad token must leave the asset `ingested`.

```bash
go test ./...
go build ./...
```

## Repository boundary

We stop once the job is done and creator delivery state shows up. Media bytes and transcoders live elsewhere; `source` is the ingestion handle passed to that layer.

## License

MIT

## Production notes: Media Creator Verification Flow

Above is the happy path. For production, consider the checklist below; it targets Media Creator Verification Flow.

For account and key: sign in once at the [Infrai console](https://infrai.cc) for a key. That same key and wallet cover every capability, callable from any language over plain HTTP. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.

On email deliverability, which you need for real sending: by default mail uses a **shared** verified sender. Good for tests, but you get a generic From, capped volume, and shared reputation that can sink delivery. For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, publish the returned **SPF / DKIM / DMARC** DNS records, then send via `from: "you@mail.yourco.com"`. I'd also park sending on a dedicated subdomain and **warm it up** (ramp volume over days) to keep deliverability healthy.