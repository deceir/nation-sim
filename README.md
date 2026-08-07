# Diplomatia

Diplomatia is a persistent nation simulation game with a Go backend, React/TypeScript interface, and authoritative MySQL game state.

## Run

```bash
docker compose up --build
```

Open http://localhost:3000. The migrator safely upgrades existing MySQL volumes before the API starts.

## Onboarding

1. Create an account with email and password.
2. Create a nation, leader, capital, government, and continent.
3. Enter the game.

Nationless accounts resume at step two and never appear in the nation directory. New nations receive a 30-day Guardian grant.

## Feed development nations

The feeder runs automatically after migrations and before the API starts. It creates five missing uppercase bots based on real nations, repairs their `BOT` classification, and never creates or reserves Japan. Run `./scripts/feed-bots.ps1` only when you want a manual recheck.

## Current systems

- Secure account registration, login, cookie sessions, and resumable nation founding
- Searchable, privacy-limited nation directory and editable cosmetic profiles
- Server-controlled PLAYER, DEV, and BOT nation classifications
- Yen-denominated currency displays
- Extendable Guardian grants
- Cities with improvement slots, escalating expansion, population capacity, and founding cooldowns
- Realistic technology programs with daily and lifetime limits
- Hourly cash, resource, and happiness-driven population turns
- Income and population projections
- Player-set market order book
- Raid and war declaration foundations

## Economic model

The economic engine is deterministic and resolved in 24 hourly turns per day. Each city has independently purchased Infrastructure and Land, one improvement slot per 50 Infrastructure, local population, commerce, power, pollution, disease, and crime. The national layer controls taxation (10–45%), doctrine, Technology, Education, Happiness, treasury, and resource stockpiles.

Balance constants and improvement data live in `cmd/server/economy_model.go`. The Cities and Income tabs expose the same audited calculation used by the turn runner, including tax revenue, upkeep, effective population, power constraints, health and crime multipliers, production, and the target Happiness calculation.
