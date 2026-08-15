# Contributor guidance

- Keep Pterodactyl integration inside Blueprint APIs; never patch core.
- Keep canonical packs outside Wings/game volumes.
- Never expose service/provider secrets to React or public routes.
- Use argument arrays for subprocesses; preserve path, upload, SSRF, archive checks.
- Add tests for security-sensitive changes; keep files focused.
