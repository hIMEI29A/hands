# AGENTS.md

Texas Hold'em poker **library** (no app or binary). Module `hands`, `go 1.20`. Single dependency: `gonum.org/v1/gonum` (used only for combinations).

## Commands

- `go build ./...` and `go vet ./...` are the only meaningful repo-wide checks — there are no `*_test.go` files, so `go test ./...` verifies nothing.
- First build needs `go mod download` (fetches gonum); offline environments fail here.
- Per-package check: `go test ./src/models/` (compiles and vets the package even with no tests).

## Layout

- `src/models` — all domain code: `Card`/`Suite`, `Hand` (ranking + comparisons), `Deck`, `Table`, `Player`, `Pot`, `Bet`, `IdMaker`.
- `src/helpers` — combinations (gonum), random IDs (`crypto/rand`), random ints.
- Imports use `hands/src/...`, not a VCS path. No root package, no `internal/`.
- `cmd/` is gitignored (`cmd`, `cmd/*`). `cmd/test/main.go` is an untracked local scratch playground — not an entrypoint, don't put real code there.

## Quirks

- Card strings are value+suit with no separator: `"10H"`, `"2C"`, `"AS"`. Suits are H/D/S/C only (`Crosses` = clubs). Parsing is regex-based in `card.go` and silently defaults to `Two` / empty suit on bad input, so keep the exact format.
- The suit type is spelled `Suite` everywhere (`Suites`, `SuitesNum`, `CardSuite`). Keep that spelling for consistency.
- `Hand` stores cards in `cardsMap map[string]int` keyed by card string, so iteration order is random; order-dependent logic must sort explicitly (as `HasStraight`/`Max` do).
- `NewDeck()` starts empty. Intended flow: `ShuffleDeck()` → `PreFlop()` → `Flop()` → `Turn()` → `River()`; deal methods on an unshuffled table return `EofError`.
- `Table.GetPlayersHand` → `getMaxHand` requires exactly 7 cards (5 board + 2 pocket); other counts are errors.
- Comments and error messages mix Russian and English. Match the surrounding language instead of translating.

## Known correctness issues

Existing behaviour is not trustworthy; verify before relying on it. Examples:
- `Hand.Define` returns `StraightFlushHand` for a plain straight.
- `Deck.Shuffle` can leave `nil` slots when the random index collides, so a "shuffled" deck may be short/invalid.
- `Pot.Register` errors for unregistered players instead of registering them; `Player.AddCard` mutates under `RLock`.
- `Table.ResolveWinner` panics with fewer than 2 players.

Fixing these is the next workstream; delete entries here as they are fixed.

## Git and releases

`dev` is the integration branch, `master` is release-only. This overrides the global "branch from `origin/master`" default:
- Branch from `origin/dev` and PR into `dev`; back-merge `master` into `dev` when needed.
- Release: PR `dev` → `master`, tag `vX.Y.Z-alpha` on the merge commit, and bump `README.md` "Current version" in that same PR.
- `README.md` currently says `v0.2.0-alpha` while `v0.2.1-alpha` is tagged — expect that drift.
