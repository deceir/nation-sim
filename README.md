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

Run `./scripts/feed-bots.ps1` to create five idempotent, uppercase bot nations. The feeder never creates or reserves Japan.

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
