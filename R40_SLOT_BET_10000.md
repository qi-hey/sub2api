# R40 Slot Bet Limit

Release: `0.1.165-r40`

## Behavior

- Paid slot bet options are `1, 2, 5, 10, 20, 50, 100, 200, 500, 1000, 2000, 5000, 10000` credits.
- The backend rejects requested or configured slot bets above `10000`.
- The game cabinet stops the increase control at `10000`.
- Admin settings validate the same maximum before submitting.
- Free spins remain locked to the configured default bet.

## Verification

- Backend option and setting-boundary tests pass.
- LuckyScene and SettingsView boundary tests pass.
- Production frontend build passes and the deployed arcade asset contains the `10000` option.
- Production health checks return HTTP 200.

## Deployment

- Binary SHA256: `73c519f0aa2244da368286caa84c0fccbef43e15870b4fcf34b98d250f41bcf0`
- R39 rollback backup: `/opt/sub2api/backups/sub2api-r39-before-r40-20260730T123651Z`
