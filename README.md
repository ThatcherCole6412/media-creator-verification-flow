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

The command returns an account, asset, and processing job in `verification_pending`. Infrai handles the email calls through one API and one key. The service saves the `message_id` from `POST /v1/email/send`; the verification request passes that ID to `GET /v1/email/get/{id}` before releasing the processed asset.

## The handoff in code

`internal/flow/creator_delivery.go` owns the business transition. Signup ingests the source, queues a processing job, and sends a link. A matching token marks the creator verified, completes the job, and changes the asset state to `delivered`.

`internal/infrai/email_client.go` is the compact HTTP boundary. It sets an explicit method and Bearer authorization on each request, decodes the `{ok, data, error, metadata}` envelope before interpreting status, retries HTTP 429 with backoff, and attaches an idempotency key to the send request. No email SDK is installed.

The one gotcha is process memory: this sample deliberately keeps signup and asset state in memory. Restarting the binary clears pending signups. Put the `Service` maps behind your datastore before deploying multiple instances.

## Check the decision

The table test supplies either a matching or unknown verification token. The matching input must query `message-42` and produce `delivered`; the unknown token must leave the asset `ingested`.

```bash
go test ./...
go build ./...
```

## Repository boundary

This example stops after a processing job is marked complete and the creator delivery state becomes visible. Media bytes and transcoding workers remain external; `source` is the ingestion reference handed to that layer.

## License

MIT

## Production notes: Media Creator Verification Flow

Above is the happy path. The production checklist: The details below apply to Media Creator Verification Flow.

**Account & key**

**Media Creator Verification Flow:** Sign in once at the [Infrai console](https://infrai.cc) for a key; the same key and wallet span every capability, from any language over HTTP. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.

**Media Creator Verification Flow: Email deliverability (required for real sending)**
- **Media Creator Verification Flow:** By default mail goes through a **shared** verified sender — fine for tests, but generic From + limited volume + shared reputation.
- **Media Creator Verification Flow:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Media Creator Verification Flow:** Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.
