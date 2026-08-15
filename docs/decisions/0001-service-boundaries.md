# ADR 0001: split extension, manager, and public origin

Status: accepted.

Blueprint owns Pterodactyl session and server authorization. Browser calls only Blueprint client routes. Blueprint sends a bearer token and derived permission to loopback manager API. Go service owns SQLite and canonical packs outside Wings volumes. Separate public listener exposes only releases and content-addressed blobs. This keeps service secrets and draft state outside browser/public trust boundaries.
