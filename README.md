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

Modifying the branch setup.