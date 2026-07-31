# R42 Game Credit Gifts

Release: `0.1.165-r42`

## Scope

R42 adds user-to-user game point gifts by recipient login email. It preserves the
complete R41 fruit machine, all game balances, ledgers, leaderboards, reward codes,
and every earlier custom feature.

## User flow

- The game center contains a Chinese `赠送积分` section and quick link.
- The sender enters the recipient account email and a positive integer amount.
- A second confirmation shows the exact normalized email and amount before sending.
- On success, the displayed wallet balance and point ledger refresh immediately.
- Ledger labels distinguish `赠送积分` (`gift_sent`) and `收到赠送`
  (`gift_received`).

## API contract

`POST /api/v1/user/game-wallet/gifts`

Header: `Idempotency-Key: <unique key>`

```json
{
  "recipient_email": "friend@example.com",
  "credits": "25"
}
```

- Recipient lookup is trimmed and case-insensitive.
- Invalid email, missing recipient, self-gifting, non-positive amount, insufficient
  balance, and recipient overflow are rejected without moving points.
- The route is limited to 10 requests per minute per authenticated user and fails
  closed when the rate-limit store is unavailable.

## Persistence and safety

- Migration `199_game_credit_gifts.sql` adds immutable
  `game_credit_transfers` rows and the `gift_sent` / `gift_received` ledger types.
- The ledger constraint explicitly preserves the production `admin_grant` history
  type; all 7 existing administrator grant rows remained intact during deployment.
- Sender deduction, recipient credit, transfer record, and both ledger rows share one
  database transaction.
- Users and wallets are locked in deterministic user-ID order so opposite-direction
  transfers cannot deadlock each other.
- Idempotent replay returns the original result without moving points again. Reusing
  the same key for a different email or amount returns a conflict.
- The client keeps the same idempotency key after uncertain network failures and
  blocks duplicate submits while a request is running.

## Verification

- Focused service, repository, handler, route, and migration tests passed.
- Repository tests cover atomic paired ledger writes, idempotent replay, and rollback
  when the sender has insufficient points.
- Frontend API and game-center suites: 19 tests passed.
- Frontend TypeScript check and production build passed.
- Embedded-frontend root/SPA tests passed with the required `-tags embed` build tag.
- Full repository, handler, route, and migration suites passed.
- The broader service suite still contains a pre-existing live-upstream test failure;
  all R42-focused service tests pass.

## Build

- Linux amd64 binary: `build/sub2api-0.1.165-r42-linux-amd64`
- SHA256: `119a6f9a38f8ea8be1a711d71d1e1398b32547b90df9dc6fc94b3313f94758f0`
- Production builds must use `go build -tags embed`; omitting the tag creates an
  API-only binary whose frontend routes return 404.

## Deployment

- R41 rollback backup:
  `/opt/sub2api/backups/sub2api-r41-before-r42-20260731T084655Z`.
- Migration `199_game_credit_gifts.sql` is recorded in `schema_migrations` and the
  immutable transfer table exists with zero rows before user testing.
- Production reports `Sub2API 0.1.165-r42`; binary SHA256 matches the local build.
- Service state is `active/running`, `NRestarts=0` after the successful start.
- Private and public `/health` checks return HTTP 200; the unauthenticated gift route
  returns HTTP 401 rather than 404.
