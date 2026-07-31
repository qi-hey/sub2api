# R43 Per-game Odds Pools

Release: `0.1.165-r43`

## Scope

R43 adds one server-owned odds pool per paid game. It does not split or migrate
the player's existing wallet, check-in, gift, redemption, spin history, or
leaderboard data.

Current pools:

- `fruit_machine`: reference ring fruit machine only.
- `five_reel_slot`: five-reel slot, including its BONUS settlement, only.

The tables contain no user ID. A policy decision can use only aggregate bets,
payouts, round count, and the current game ID, so personalized win/loss control
is structurally unavailable.

## Controller

- Fixed 15-minute policy periods.
- Profiles: `standard`, `cooling`, and `tight`.
- At most one profile-level change per period.
- Sparse periods do not react to one isolated win.
- Every closed period is kept in `game_odds_periods` with the old/new profile
  and aggregate statistics for audit.
- Existing spin, ledger, and payout rows remain immutable.
- If policy lookup fails, the game uses its unchanged standard profile instead
  of becoming unavailable.

## Fruit machine

The physical 24-cell board, stop indexes, symbols, displayed multipliers, inner
reward tables, and `fruit-v2` payout rules remain unchanged. Profiles adjust
only the shared outer special-gate weight:

| Profile | Special-gate weight |
|---|---:|
| `standard` | 10% |
| `cooling` | 7% |
| `tight` | 4% |

Removed special weight is distributed proportionally across all ordinary fruit
cells, keeping the eight doors close to their existing relative balance.

## Five-reel slot

The ordinary paytable, reel symbols, BONUS rewards, free-spin settlement, and
existing multiplier rules remain unchanged. The requested standard BONUS-entry
probability is now exactly `60 / 8000 = 0.75%`.

| Profile | BONUS entry | Low-tier win keep |
|---|---:|---:|
| `standard` | 0.75% | 1/2 |
| `cooling` | 0.5625% | 1/3 |
| `tight` | 0.375% | 1/4 |

## Future games

A new paid game can reuse the same pool without a new pool table. Its immutable
credit-ledger metadata supplies:

```json
{
  "game_id": "new_game_id",
  "pool_direction": "bet"
}
```

`pool_direction` is either `bet` or `payout`. The database creates an isolated
row for that game ID automatically. The game service then resolves the shared
period profile through `GameOddsPoolRepository`.

## Migration

Migration `200_game_odds_pools.sql` backfills current aggregate bets and payouts
from the immutable ledger. Historical fruit and slot values are written only to
their corresponding pools; slot BONUS payouts cannot enter the fruit pool.
