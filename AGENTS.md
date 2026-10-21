# AGENTS.md

Texas Hold'em poker **library** (no app or binary). Module `hands`, `go 1.20` (no `slices`/`maps` packages, no builtin `min`/`max`). Single dependency: `gonum.org/v1/gonum` (used only for combinations).

## Commands

- The shell exports a stale `GOROOT` (`go1.22.9` while the `go` binary is newer), so plain `go build` fails with "version does not match go tool version". ALWAYS run Go as `env -u GOROOT go ...`. The `Makefile` already handles this via `GO ?= env -u GOROOT go`; CI does not need the workaround because `GOROOT` is unset there.
- Local checks go through `make`: `make build`, `make vet`, `make fmt`, `make test` (with `-race`), `make test-short`, `make lint` (requires `golangci-lint` locally), and `make check` (`fmt build vet test`).
- Tests exist for every package (card, deck, hand, player, pot, table, helpers). `-short` skips the exhaustive C(52,5) hand-category enumeration in `src/models`; the full `-race` run takes ~17s for that package.
- First build needs `go mod download` (fetches gonum); offline environments fail here.
- CI lives in `.github/workflows/ci.yml` and runs gofmt, build, vet, tests on Go `1.20.x` and `stable`, plus non-blocking `golangci-lint` (config in `.golangci.yml`). Only the first four checks must pass before merge; lint findings are advisory.

## Layout

- `src/models` — all domain code: `Card`/`Suit`, `Hand` (ranking + comparisons, single evaluator in `hand_eval.go`), `Deck`, `Table`, `Player`, `Pot`, `Bet`, `IdMaker`.
- `src/helpers` — combinations (gonum), random IDs (`crypto/rand`), random ints.
- Imports use `hands/src/...`, not a VCS path. No root package, no `internal/`.
- `cmd/` is gitignored (`cmd`, `cmd/*`). `cmd/test/main.go` is an untracked local scratch playground — not an entrypoint, don't put real code there.

## API facts

- Card strings are value+suit with no separator: `"10H"`, `"2C"`, `"AS"`. Suits are H/D/S/C only (`Clubs`).
- Parsing is strict: `ParseCard(s) (*Card, error)` validates and uppercases; `NewCardFromString` is deprecated and PANICS on invalid input. `NewHandFromStrings`/`NewHandFromCards` return `(*Hand, error)`.
- Suits: type `Suit`, `CardSuit`, `SuitColor`, `Suits`, `SuitsNum`, field `Card.Suit` (a `*CardSuit`; access the suit itself as `c.Suit.Suit`). `Clubs` is current; `Crosses` is a deprecated alias. `Suite`-named identifiers remain only as deprecated wrappers (`SuiteFromString`, `CompareSuites`).
- `Hand` stores a sorted `[]*Card` (exactly 5 cards). `Hand.Define()` returns the category; `Hand.Compare` ranks two hands (total order incl. kickers, wheel A-2-3-4-5, wheel straight flush, royal flush). `Hand.CountPairs` returns the number of pairs (0..2), not paired cards.
- `NewDeck()` starts empty. `NewDeckWithRand(*rand.Rand)` enables deterministic tests; `Deck.Shuffle()` is Fisher-Yates over 52 unique cards; `Deck.Len()` exists. Intended flow: `NewTable(...).StartHand()` → `PreFlop()` → `Flop()` → `Turn()` → `River()`; deal methods on an unshuffled table return an error.
- `Table.StartHand()` requires >=2 players; it resets board, pot and players, shuffles and advances the dealer. `PreFlop` deals 2 cards to active players with no burn; `Flop`/`Turn`/`River` burn one card and validate board length (0/3/4).
- `Table.Blinds()` requires >=2 active players, skips inactive seats, and in heads-up the dealer posts the small blind.
- `Table.ResolveWinner()` requires exactly 5 board cards, evaluates only active players, returns ALL tied winner IDs sorted alphabetically, and never panics. `GetFirstPosition`, `GetSecondPosition`, `GetDealer` return `(*Player, error)`. `Table.GetPlayersHand` still requires exactly 7 cards (5 board + 2 pocket).
- `Player` uses an unexported mutex, returns copies of pocket cards, validates bets and never goes negative. `Player.ResetForHand()` exists. `Pot.Register` registers new players and errors on duplicates; `Pot.Reset` zeroes the total; negative bets are rejected.
- Comments and error messages mix Russian and English. Match the surrounding language instead of translating.

## Known limitations

- `TableRules` is an empty placeholder (`TODO`).
- No betting-round state machine: `Table.CurrentMove` is unused, there are no per-street bet validations and no turn-order enforcement.
- No side pots / all-in handling; `ResolveWinner` returns winner IDs only and does not distribute chips.
- `cmd/` is still gitignored scratch space.

## Git and releases

`dev` is the integration branch, `master` is release-only. This overrides the global "branch from `origin/master`" default:
- Branch from `origin/dev` and PR into `dev`; back-merge `master` into `dev` when needed.
- Release: PR `dev` → `master`, tag `vX.Y.Z-alpha` on the merge commit, and bump `README.md` "Current version" in that same PR.
- `README.md` is being bumped to `v0.3.0-alpha` on this branch; the previous tag was `v0.2.1-alpha` — expect that drift.
