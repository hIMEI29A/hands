# AGENTS.md

Texas Hold'em poker **library** (no app or binary). Module `hands`, `go 1.20` (no `slices`/`maps` packages, no builtin `min`/`max`). Single dependency: `gonum.org/v1/gonum` (used only for combinations).

## Commands

- The shell exports a stale `GOROOT` (`go1.22.9` while the `go` binary is newer), so plain `go build` fails with "version does not match go tool version". ALWAYS run Go as `env -u GOROOT go ...`. The `Makefile` already handles this via `GO ?= env -u GOROOT go`; CI does not need the workaround because `GOROOT` is unset there.
- Local checks go through `make`: `make build`, `make vet`, `make fmt`, `make test` (with `-race`), `make test-short`, `make lint` (requires `golangci-lint` locally), and `make check` (`fmt build vet test`).
- Tests exist for every package (card, deck, hand, player, pot, table, helpers). `-short` skips the exhaustive C(52,5) hand-category enumeration in `src/models`; the full `-race` run takes ~17s for that package.
- First build needs `go mod download` (fetches gonum); offline environments fail here.
- CI lives in `.github/workflows/ci.yml` and runs gofmt, build, vet, tests on Go `1.20.x` and `stable`, plus non-blocking `golangci-lint` (config in `.golangci.yml`). Only the first four checks must pass before merge; lint findings are advisory.

## Layout

- `src/models` — all domain code: `Card`/`Suit`, `Hand` (ranking + comparisons, single evaluator in `hand_eval.go`), `Deck`, `Table` (betting state machine in `betting.go`, side pots in `sidepots.go`, rules in `rules.go`, ante posting and blind position in `antes.go`), `Player`, `Pot`, legacy `Bet`, `IdMaker`.
- `src/helpers` — combinations (gonum), random IDs (`crypto/rand`), random ints.
- Imports use `hands/src/...`, not a VCS path. No root package, no `internal/`.
- `cmd/` is gitignored (`cmd`, `cmd/*`). `cmd/test/main.go` is an untracked local scratch playground — not an entrypoint, don't put real code there.

## API facts

- Card strings are value+suit with no separator: `"10H"`, `"2C"`, `"AS"`. Suits are H/D/S/C only (`Clubs`).
- Parsing is strict: `ParseCard(s) (*Card, error)` validates and uppercases; `NewCardFromString` is deprecated and PANICS on invalid input. `NewHandFromStrings`/`NewHandFromCards` return `(*Hand, error)`.
- Suits: type `Suit`, `CardSuit`, `SuitColor`, `Suits`, `SuitsNum`, field `Card.Suit` (a `*CardSuit`; access the suit itself as `c.Suit.Suit`). `Clubs` is current; `Crosses` is a deprecated alias. `Suite`-named identifiers remain only as deprecated wrappers (`SuiteFromString`, `CompareSuites`).
- `Hand` stores a sorted `[]*Card` (exactly 5 cards). `Hand.Define()` returns the category; `Hand.Compare` ranks two hands (total order incl. kickers, wheel A-2-3-4-5, wheel straight flush, royal flush). `Hand.CountPairs` returns the number of pairs (0..2), not paired cards.
- `NewDeck()` starts empty. `NewDeckWithRand(*rand.Rand)` enables deterministic tests; `Deck.Shuffle()` resets to 52 unique cards and shuffles; `Deck.Len()` exists. `StartHand` shuffles afresh every hand.
- Betting flow: `NewTableWithRules(...)` (or the legacy `NewTable`) → `StartHand()` → `Act(...)` until the street completes → `NextStreet()` → repeat → `Showdown()`. `StartHand` resets state, sits out empty stacks, moves the dealer, posts antes and blinds, deals pockets and opens preflop betting. `Act` enforces turn order, min raises, short all-ins and the raise cap.
- `Action{PlayerID, Type, Amount}`. `Amount` for `bet`/`raise` is the player's total street commitment ("raise to"). There is no explicit all-in action: using the rest of the stack makes the action all-in. `LegalActions` returns the available types; `ToCall` returns the missing amount.
- `Street`: `preflop`, `flop`, `turn`, `river`, `showdown`. `HandPhase`: `waiting`, `betting`, `street-complete`, `showdown`, `complete`. `NextStreet` only runs in `street-complete`; after the river it moves to `showdown`. If every remaining player is all-in, `NextStreet` runs the board out automatically.
- `TableRules` (in `rules.go`) has `MinPlayers`, `SmallBlind`, `BigBlind`, `Ante`, `AnteMode`, `MinRaise` (0 → big blind), `MaxRaisesPerStreet` (0 → unlimited; counts all aggressive actions) and `OddChipRule` (`OddChipLeftOfDealer`). `Ante == 0` disables antes. `AnteMode` has `AntePerPlayer` (`"per-player"`, default when empty) and `AnteBigBlind` (`"big-blind"`). Only no-limit Hold'em is implemented.
- Antes (`src/models/antes.go`): `StartHand` and the deprecated `Blinds()` helper post antes before the blinds. Per-player mode posts from every active player; big-blind mode posts only from the big blind. Antes are dead money: they go into the pot but not into `streetBet`, `currentBet`, `ToCall`, minimum raise or raise-cap calculations. A short stack posts `min(ante, stack)`, becomes all-in, stays active, gets pocket cards, cannot act and can only win pots it is eligible for. `Table.Ante() Chips` returns the configured ante.
- Side pots: `Showdown` builds layers from total contributions, refunds unmatched chips, excludes folded players, evaluates each layer's eligible players, splits ties and assigns odd chips left of the dealer. `ResolvePots` is the read-only preview; `LastResults` returns the last payouts. `ResolveWinner` is deprecated legacy behavior.
- `Table.Blinds`, `PreFlop`, `Flop`, `Turn`, `River`, and `Player.Check/Call/Raise/Fall` are deprecated low-level helpers that bypass the state machine. `Player.ResetForHand()` activates only players with chips; folded players keep their cards for history.
- `Table.ResolveWinner()` requires exactly 5 board cards, evaluates only active players, returns ALL tied winner IDs sorted alphabetically, and never panics. `GetFirstPosition`, `GetSecondPosition`, `GetDealer` return `(*Player, error)`. `Table.GetPlayersHand` still requires exactly 7 cards (5 board + 2 pocket).
- `Player` uses an unexported mutex, returns copies of pocket cards, validates bets and never goes negative. `Pot.Register` registers new players and errors on duplicates; `Pot.Reset` zeroes the total; negative bets are rejected. `Pot.RemoveChips` backs refunds/payouts.
- Comments and error messages mix Russian and English. Match the surrounding language instead of translating.

## Known limitations

- No-limit Hold'em only; fixed-limit and pot-limit are not implemented.
- No straddles, rebuys, late registration or tournament payout/ICM logic.
- Hand history/action log is not persisted; `LastResults` only keeps the latest payouts.
- `ResolveWinner` remains a deprecated, non-pot-aware convenience.
- `cmd/` is still gitignored scratch space.

## Git and releases

`dev` is the integration branch, `master` is release-only. This overrides the global "branch from `origin/master`" default:
- Branch from `origin/dev` and PR into `dev`; back-merge `master` into `dev` when needed.
- Release: PR `dev` → `master`, tag `vX.Y.Z-alpha` on the merge commit, and bump `README.md` "Current version" in that same PR.
- The current released tag is `v0.4.0-alpha`. Bump `README.md` "Current version" in the release PR.
