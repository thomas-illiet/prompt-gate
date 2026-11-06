# Development Guide

This guide covers local development for Prompt Gate Backend.

## Local Stack With Docker Compose

Run the full local stack:

```sh
docker compose up --build
```

Compose starts:

- PostgreSQL on `localhost:5432`
- Redis on `localhost:6379`
- Keycloak on `http://keycloak.localhost:8082`
- database migrations
- a seed job for a local Ollama provider
- the API on `http://localhost:8080`
- the proxy on `http://localhost:8081`
- the worker
- the scheduler

Local credentials:

- Keycloak admin: `admin` / `admin`
- Prompt Gate test user: `admin` / `admin`

The seeded provider is named `ollama` and points at
`http://host.docker.internal:11434/v1`.

Compose mounts `deploy/config/promptgate.compose.yaml` read-only into every
Prompt Gate process. That committed file contains development-only credentials;
edit or replace it for local experiments, then restart the affected services.

### Local Login Troubleshooting

The first browser user synced into an empty Prompt Gate database becomes
`admin`. Human users are matched by the exact OIDC `preferred_username` claim.
If Keycloak is reset while the PostgreSQL volume is kept, signing in with the
same username refreshes the stored OIDC subject while preserving the local
account and its application role.

When `admin` signs in but sees **Access denied** in local development:

- verify that Keycloak emits the expected `preferred_username` claim, then
- promote the matching account from the User management page or directly in
  PostgreSQL for local recovery.

## Source-Based Setup

Create a local runtime configuration:

```sh
cp promptgate.example.yaml promptgate.yaml
```

Fill the required database, Redis, OIDC, URL, and secret values. The Makefile
uses `CONFIG_FILE=promptgate.yaml` by default; set `CONFIG_FILE=/other/path.yaml`
to select another file.

Install dependencies and run checks:

```sh
make deps
make fmt-check
make vet
make staticcheck
make test
make build
```

Integration-test service endpoints are read from
[`testdata/promptgate.integration.yaml`](../testdata/promptgate.integration.yaml).
The PostgreSQL- and Redis-specific tests run when those disposable services are
available and otherwise skip locally. CI starts matching services for the
endpoints in this file, so the integration coverage always runs there.

Run the backend processes:

```sh
make migrate
make run-api
make run-proxy
make run-worker
make run-schedule
```

For convenience, run migrations, worker, scheduler, API, and proxy from one terminal:

```sh
make run-all
```

## Configuration Loading

Every runtime requires an explicit YAML path:

```sh
go run . --config promptgate.yaml api
go run . --config promptgate.yaml proxy
go run . --config promptgate.yaml worker
go run . --config promptgate.yaml migrate
go run . --config promptgate.yaml schedule
```

The loader does not inspect `.env`, expand environment placeholders, or accept
environment overrides. It rejects unknown YAML keys and invalid types.

## Common Make Targets

| Target | Purpose |
| --- | --- |
| `make deps` | Download Go modules. |
| `make fmt` | Format Go files. |
| `make fmt-check` | Check Go formatting without rewriting files. |
| `make vet` | Run `go vet`. |
| `make staticcheck` | Run the pinned Staticcheck version. |
| `make test` | Run backend tests. |
| `make coverage` | Run backend tests and enforce at least 80% aggregate Go statement coverage. |
| `make build` | Build `bin/promptgate`. |
| `make migrate` | Run database migrations. |
| `make run-api` | Run the API process. |
| `make run-proxy` | Run the proxy process. |
| `make run-worker` | Run the generic worker process. |
| `make run-schedule` | Run the scheduler process. |
| `make run-all` | Run migrations, worker, scheduler, API, and proxy together. |
| `make clean` | Remove local binaries. |

## Frontend Notes

The production Dockerfile builds the Nuxt frontend and copies generated static
assets into `/app/public`. Configure that path in the mounted YAML:

```yaml
api:
  static_assets_dir: /app/public
```

When this path is set, the API serves frontend files directly.

## OpenAPI Contract

`internal/transport/httpapi/openapi/openapi.yaml` is the source document loaded
by the protected user explorer at `/documentation`. When a runtime route under
`/api/v1/**`, its request, or its response changes, update this document in the
same change.

The HTTP API tests validate the document, representative schemas, and
bidirectional parity between runtime operations and OpenAPI operations. Run the
targeted checks with:

```sh
go test ./internal/transport/httpapi/...
```

`make test` includes the same checks in the full backend suite.

## Migrations

Local migrations use:

```sh
make migrate
```

The migration command applies all GORM migrations in dependency order. Run it
after changing persistent models and before starting API or proxy processes
against a fresh database.

## Test And Validation Checklist

Before opening a pull request:

```sh
make fmt-check
make vet
make staticcheck
make test
docker compose config --quiet
docker build -t prompt-gate-backend:test .
```

If frontend code changed, also run from `frontend/`:

```sh
npm ci
npm run lint:check
npm run typecheck
npm test -- --run
```

## Audit fonctionnel et couverture — 4 novembre 2026

La couverture Go globale passe de **72,2 % à 80,5 %**. Le périmètre reste celui
de `make coverage` : tous les paquets Go du projet, sans nouvelle exclusion et
sans changement de `coverpkg`. Le seuil obligatoire du Makefile et du script de
validation est maintenant **80 %** ; la CI utilise ce même contrôle. Il s'agit
de la couverture des instructions Go, pas d'un taux de couverture des branches
ni d'un minimum garanti pour chaque paquet.

Les ajouts comprennent **34 scénarios principaux dans 14 nouveaux fichiers de
test**, avec des cas paramétrés supplémentaires. Les tests utilisent SQLite
pour les transactions isolées, PostgreSQL pour les contraintes et les cycles de
vie réels, Redis réel pour les intégrations existantes, et des instances Redis
simulées isolées pour les pannes et expirations déterministes. Les fournisseurs
LLM, OIDC et SMTP sont remplacés par des serveurs locaux contrôlés.

### Vérification fonctionnalité par fonctionnalité

Les pourcentages ci-dessous portent sur les paquets propriétaires de chaque
fonctionnalité. La colonne « contrôles » rassemble les tests existants relancés
et les nouveaux scénarios de cet audit.

| Fonctionnalité / paquet | Avant → après | Contrôles et preuves principales |
| --- | --- | --- |
| Authentification et sessions | 80,2 → 80,2 % | Sessions partagées entre instances, expiration, déconnexion sans résurrection, état OIDC consommable une seule fois, redirections. Tests dans `internal/domain/auth/`. La découverte OIDC et PKCE sont aussi exercés avec l'application assemblée. |
| Clés virtuelles | 64,6 → 87,0 % | Clés falsifiées, expirées et révoquées ; filtres par statut ; historique administrateur ; notifications sans doublon ; tâche d'expiration réelle. `tokens/lifecycle_contracts_test.go`. |
| Utilisateurs et comptes de service | 69,6 → 81,0 % | Séparation des identités, renommage sans perte des consommations, tri par tokens, collision d'identifiants, rollback si la révocation échoue, expiration des accès. `users/account_lifecycle_test.go`. |
| Groupes et accès aux modèles | 76,6 → 80,4 % | Règles par fournisseur, expressions régulières et exclusions ; remplacement atomique des groupes ; notification uniquement en cas de changement ; rollback des adhésions en cas de panne. `groups/membership_transactions_test.go`. |
| Abonnements, quotas et crédits de reset | 76,1 → 78,3 % | Changement de plan malgré une erreur de projection KPI, remplacement du plan par défaut, refus de supprimer un plan affecté, réservations concurrentes, resets idempotents, crédits plafonnés et ordre des projections. `subscriptions/plan_lifecycle_test.go` et suites de quotas existantes. |
| Ingestion KPI et dashboard | 78,9 → 80,3 % | Totaux après purge, tokens de cache et embeddings, relecture sans doublon, reprise après panne Redis, événements en attente et marqueurs de déduplication préservés. `proxy/ingestion_regression_test.go` et `proxy/retention_schedule_test.go`. |
| Fournisseurs | 85,0 → 85,0 % | Catalogue, secrets chiffrés, validation des configurations, erreurs amont et absence de fuite de clé lors des redirections. Parcours CRUD supplémentaires au niveau HTTP administrateur. |
| Routage et streaming du proxy | 80,7 → 80,7 % | Routes autorisées, requêtes et réponses trop volumineuses, streaming, CA privée, rechargement et récupération après invalidation manquée. Tests dans `internal/runtime/proxy/`. |
| Model Lab | 87,0 → 87,0 % | Tickets liés à une requête et à usage unique, consommation concurrente atomique, limites de flux, catalogue dégradé. Les tests Vue couvrent aussi sélection des modèles, arrêt du flux et cartes de comparaison. |
| Tarification | 66,2 → 80,3 % | Tarifs par modèle et de repli, modèles sans prix, catalogue indisponible distingué d'une configuration complète, immutabilité des cibles. `pricing/catalog_check_test.go`. |
| Notifications | 71,2 → 80,9 % | Composition et assainissement, idempotence, destinataires, historique et relance ; refus SMTP, exigences STARTTLS/authentification, délai croissant entre tentatives, arrêt au maximum et récupération. `notifications/delivery_failures_test.go`. |
| Monitoring | 95,9 → 95,9 % | Création, contrôle immédiat, disponibilité puis panne HTTP 503 persistée, désactivation et suppression. Complété par le parcours HTTP administrateur. |
| FAQ | 94,2 → 94,2 % | Publication/brouillon, édition, réorganisation, suppression et HTML assaini. Complété par les validations et erreurs HTTP. |
| Guides de configuration | 78,7 → 78,7 % | Initialisation, CRUD, réorganisation, langage de syntaxe et validation des modèles ; tests des formulaires et aperçus Vue relancés. |
| API d'administration | 51,9 → 80,4 % | Parcours complets, JSON invalide, filtres interdits, données absentes, erreurs de stockage, notifications indisponibles. `admin/feature_workflows_test.go` et `admin/feature_failures_test.go`. |
| Assemblage API/proxy/planificateur | 52,9 → 86,4 % | Démarrage réel sur PostgreSQL, découverte OIDC, routage authentifié, fermeture des connexions, erreurs de clé/Redis/CA et du service de validation externe, expiration planifiée. `app/lifecycle_integration_test.go`. |
| Redis partagé | 51,7 → 83,2 % | Versions et JSON corrompus, état déjà consommé, expiration, événement malformé suivi d'une invalidation valide, connexion fermée. `redisstore/failure_contracts_test.go`. |
| Connexions PostgreSQL | 42,9 → 91,4 % | Limite du pool, fermeture effective, annulation et URL invalide. `database/lifecycle_integration_test.go`. |
| Commandes et sondes de disponibilité | 21,0 → 62,4 % | Refus de démarrage sans configuration/base, port déjà occupé, tâche bloquée, base indisponible, arrêt demandé. `cmd/startup_failures_test.go`. |

Les chemins abrégés du tableau sont sous `internal/domain/`,
`internal/platform/`, `internal/runtime/` ou `internal/transport/httpapi/`
selon leur domaine. Le profil HTML permet de consulter les instructions
exactement couvertes.

Un scénario d'intégration traverse également toute la chaîne : création de
l'utilisateur et de sa clé, groupe et abonnement, appel HTTP au proxy,
réponse du fournisseur local avec **10 tokens d'entrée + 5 de sortie**, Redis,
worker, agrégation PostgreSQL, puis vérification que le dashboard et la fiche
utilisateur présentent tous deux **15 tokens**.

### Défauts reproduits et corrigés

1. **Plan enregistré mais ancien plan conservé dans Redis.** Une erreur de
   lecture des consommations, après l'enregistrement du nouveau plan, pouvait
   interrompre la méthode avant l'invalidation du cache. L'invalidation est
   maintenant déclenchée immédiatement après la validation de la transaction,
   avant les lectures nécessaires à la réponse. La régression couvre les deux
   types de compte et le cache préalablement rempli. Un contrôle par mutation
   temporaire a confirmé que retirer le correctif fait échouer les deux cas.
2. **Connexions PostgreSQL conservées après un échec de démarrage ou un arrêt.**
   Les applications et commandes libèrent maintenant leurs pools. L'échec du
   premier ping ferme aussi le pool ; le ping respecte le contexte d'annulation.
   La fermeture de l'application et du proxy est idempotente. Les tests
   interrogent `pg_stat_activity` pour vérifier l'absence de connexion résiduelle.

### Résultats de validation

- Suite Go complète avec `PROMPTGATE_REQUIRE_INTEGRATION=1` : succès,
  couverture globale supérieure à 80 %.
- Détecteur de conflits d'accès simultanés (`go test -race ./...`) : succès.
  Les derniers ajouts d'intégration ont aussi été vérifiés séparément avec `-race`.
- Interface : **516 tests réussis dans 141 fichiers** ; lint, vérification
  TypeScript et génération Nuxt réussis. Aucun pourcentage de couverture
  JavaScript/Vue n'est déduit de ce nombre de tests.
- Formatage Go, `go vet`, Staticcheck et compilation : succès.
- Recherche de vulnérabilités Go et npm : aucune signalée au moment du contrôle.
- Vérification historique avant remplacement du service de politique : Helm rendu et lint réussis ; 20 tests du service de politique réussis et contrôles HTTP
  d'autorisation réussis. Configuration Docker Compose valide.

### Limites et suite du plan de validation

| Priorité | Vérification restante | Critère de réussite |
| --- | --- | --- |
| Avant publication | Construire et démarrer l'image Docker complète. Le moteur Docker local était indisponible pendant cet audit. | Image construite, processus non privilégié, migrations et sondes saines dans la stack de release. |
| Avant déploiement | Parcours navigateur avec le véritable fournisseur OIDC, les LLM, SMTP/TLS et l'ingress de préproduction. | Connexion/déconnexion, comparaison de modèles en streaming, envoi réel et cohérence des totaux sur les comptes de test. |
| Avant montée en charge | Essai prolongé multi-réplicas avec interruptions Redis/PostgreSQL et renouvellement des connexions. | Aucun événement perdu ou compté deux fois ; latence, mémoire et file d'attente dans les budgets d'exploitation choisis. Les tests concurrents locaux ne constituent pas une mesure de capacité. |
| Renforcement continu | Branches restantes : abonnements 78,3 %, API utilisateur 75,2 %, OpenAPI 74,1 %, migrations 70,1 %, configuration 75,8 %, commandes 62,4 %. | Ajouter des scénarios de panne et d'intégration utiles ; conserver le minimum global de 80 % sans réduire le périmètre. |

Pour reproduire le contrôle et générer un rapport consultable :

```sh
mkdir -p coverage
PROMPTGATE_REQUIRE_INTEGRATION=1 COVERAGE_PROFILE=coverage/go.out make coverage
go tool cover -html=coverage/go.out -o coverage/go.html
PROMPTGATE_REQUIRE_INTEGRATION=1 go test -race ./...
```

Les services d'intégration doivent être jetables et configurés dans
`testdata/promptgate.integration.yaml` ; les tests peuvent réinitialiser leurs
données. Aucun compte, abonnement ou KPI de production n'a été modifié pour
cet audit.
