# R44 Game Odds Thresholds

Release: `0.1.165-r44`

R44 keeps the global per-game pools from R43 and increases the minimum sample
size required before a pool profile can change. The same profile still applies
to every player in a game; no user ID or individual history is used.

## Thresholds

- Period control: at least 20 rounds and 100,000 bet credits in one 15-minute
  period.
- Lifetime control: at least 100 rounds and 1,000,000 total bet credits.
- Both the round and credit threshold must be reached.
- A single 10,000-credit wager cannot activate either controller.
- A profile still changes by at most one level per period.

The payout-ratio boundaries remain unchanged:

- Period payout ratio above 98% selects at least `cooling`.
- Period payout ratio above 115% selects `tight`.
- Lifetime payout ratio above 96% selects at least `cooling`.
- Lifetime payout ratio above 108% selects `tight`.

This release changes only the shared pool controller. Game displays, paytables,
BONUS rewards, player wallets, gifts, redemption, and history are unchanged.
