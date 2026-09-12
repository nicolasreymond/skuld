# skuld

Petit backend qui rend tes **notes** et ton **emploi du temps** GAPS (HEIG-VD) exploitables :
une API HTTP (lue par l'app [garmroot-app](https://github.com/nicolasreymond/garmroot-app))
et une **notification ntfy** à chaque nouvelle note.

skuld = la Norne du destin ; ici, celle qui veille sur tes notes.

## Architecture

- **skuld-scraper** — [`gaps-cli`](https://github.com/heig-lherman/gaps-cli) : se connecte à
  GAPS toutes les 3 h, détecte les nouvelles notes et les POST à l'adaptateur.
- **skuld-adapter** — petit service Go qui :
  - republie chaque nouvelle note en **notification ntfy** ;
  - sert une **API JSON** à partir de `grades.json` (notes) et de ton `horaire.ics` (cours).

## Prérequis

- **Docker** + Docker Compose.
- Un compte **GAPS** (HEIG-VD).
- Un canal **ntfy** : le service public [ntfy.sh](https://ntfy.sh) (choisis un topic secret) ou
  ton propre serveur ntfy.
- Ton **emploi du temps** exporté de GAPS (*Horaires → Télécharger au format Outlook/Apple
  Calendar*), placé dans `data/horaire.ics`.

## Démarrage

```bash
cp gaps.env.example gaps.env      # remplis identifiants GAPS, clé partagée, ntfy
cp .env.example .env              # (optionnel) SKULD_BIND pour l'accès Tailscale
# place ton horaire : data/horaire.ics

docker compose up -d --build
curl http://127.0.0.1:8088/healthz     # → ok
```

Le scraper remplit `data/history/grades.json` à sa première exécution ; l'API est alors complète.

## API

Toutes en `GET` (sauf `/api/grade`), servies sur le port **8088** :

| Endpoint | Réponse |
|---|---|
| `/widget` | prochain cours + semestre courant + moyenne + dernière note |
| `/averages` | moyennes par semestre (générale + par cours) |
| `/semester` | cours du semestre **courant** (calendaire), affichés même sans note |
| `/next-class` | prochain cours |
| `/schedule` | cours à venir |
| `/grades` | toutes les notes |
| `/target?course=X&goal=G[&weight=W]` | note à obtenir sur la prochaine épreuve pour atteindre la moyenne visée |
| `/api/grade` | `POST` interne du scraper (auth `ADAPTER_API_KEY`) → notif ntfy |

En plus, un **rappel ntfy « cours dans N min »** est envoyé avant chaque cours
(désactivable via `REMINDER_MINUTES=0`).

## Accès depuis le téléphone

L'API est **privée par défaut** (127.0.0.1). Pour la joindre depuis ton tél, le plus simple et
le plus sûr est **Tailscale** :

1. Installe Tailscale sur cette machine **et** sur le téléphone (même tailnet).
2. Mets l'IP Tailscale de cette machine dans `.env` : `SKULD_BIND=100.x.y.z`, puis
   `docker compose up -d`.
3. Dans l'app, `SKULD_URL=http://100.x.y.z:8088`.

Avec Tailscale actif sur le tél, l'API est joignable **de partout** (4G, autre Wi-Fi…).

## Sécurité

- `gaps.env` contient tes **identifiants GAPS** et le **token ntfy** : jamais commité (gitignoré).
- Par défaut, les endpoints `GET` **ne sont pas authentifiés** : ne les expose **que** sur ton
  tailnet privé. **Pour un accès public** (app utilisable sans Tailscale), mets un `READ_API_KEY`
  dans `gaps.env` et place l'API derrière un reverse proxy HTTPS : les lectures exigeront alors
  `Authorization: Bearer <READ_API_KEY>` (l'app l'envoie via `SKULD_TOKEN`). Sans ce token, l'API
  publique renvoie 401.
- Choisis un **topic ntfy** difficile à deviner (quiconque le connaît reçoit tes notifs).

## Licence

Projet personnel partagé en l'état, sans garantie. Reprends-le et adapte-le librement.
