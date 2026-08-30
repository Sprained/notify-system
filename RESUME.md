# Resumo do progresso — notify-system

Snapshot do que já foi feito até 2026-08-30. Complementa [NOTIFY.md](NOTIFY.md) (desenho
geral), [DECISIONS.md](DECISIONS.md) (decisões pontuais com o porquê) e
[TESTS.md](TESTS.md) (checklist de casos de teste por fase).

## Estado geral

Repositório criado, arquitetura DDD montada. **Fases 1 a 5 do roadmap completas**:
backend (schema, ingestão, worker, Telegram) validado ponta a ponta com bot real, e
agora **dashboard web completo** (5 telas, testado e validado no navegador). Imagem de
produção já buildada e publicada no registry — falta só subir no Portainer do Pi e
validar rodando de verdade lá.

## Estrutura do projeto

```
cmd/notify/main.go            — liga tudo: Postgres, migrations automáticas, httpapi,
                                  webui, worker (goroutine), servidor HTTP

internal/
  message/                     — entidade Message (domínio) ✅
    message.go                   — New(...): valida corpo, clampa priority 1–5, gera ULID
    repository.go                  — interface Repository { Insert, Get, Recent }

  delivery/                    — entidade Delivery (domínio) ✅
    delivery.go                   — struct Delivery
    failed_delivery.go              — struct FailedDelivery (view enriquecida p/ dashboard)
    repository.go                  — interface Repository { InsertPending, ListPending,
                                       MarkSent, MarkFailed, CountSentToday,
                                       CountFailedDeliveries, ListFailed }

  subscriber/                  — entidade Subscriber (domínio) ✅
    subscriber.go                  — struct Subscriber (ID, Kind, Config, Label, Enabled)
    repository.go                   — interface Repository { Get, CountActive, List, Create }

  route/                       — entidade Route (domínio) ✅
    route.go                       — struct Route (com SubscriberLabel p/ exibição)
    repository.go                    — interface Repository { MatchingSubscribers, List, Create }

  notifier/
    notifier.go                    — interface Notifier { Kind, Send } ✅
    lognotifier/                     — ✅ implementado, contador configurável de falhas
    telegram/                         — ✅ implementado: texto puro, click_url→botão,
                                          trunca em 4096

  worker/                       — orquestração de envio (Fase 3) ✅
    worker.go                       — Deliver (retry+backoff em memória) + Worker.ProcessPending

  postgres/                     — implementação de storage ✅
    postgres.go                    — Store: message/route/delivery.Repository completas
    subscriber.go                    — SubscriberStore: subscriber.Repository completa
    migrations/                       — schema completo (6 tabelas), embutido via go:embed
    *_test.go                          — migration, constraints, queries, overview, routes,
                                          subscribers, failed_deliveries — tudo contra
                                          Postgres real (testcontainers)

  httpapi/                      — endpoint de ingestão (Fase 2) ✅
    httpapi.go                     — Handler.Publish + RegisterRoutes ("POST /{topic}")

  webui/                        — dashboard web (Fase 5) ✅ NOVO
    webui.go                        — Handler com 5 telas + fragmento htmx + RegisterRoutes
    templates/                        — layout.html (shell puro) + stats.html (parcial
                                         reaproveitado) + overview/messages/failed/routes/
                                         subscribers.html (um `*template.Template` por
                                         página via parsePage, evita colisão de {{define}})
    static/                            — styles.css (tokens do DESIGN.md) + htmx.min.js
                                          (vendorizado, embutidos via go:embed)
    handler_test.go / webui_test.go     — testes com fakes, cobrindo as 5 telas
```

## Progresso por fase

Ver [TESTS.md](TESTS.md) pro checklist detalhado item a item — resumo:

| Fase | O quê | Status |
|---|---|---|
| 1 | Schema (spike do banco) | ✅ Green |
| 2 | Endpoint de ingestão | ✅ Green |
| 3 | Worker e interface de canal | ✅ Green |
| 4 | Integração com Telegram | ✅ Green — validado com bot real |
| 5 | Dashboard web (5 telas) | ✅ Green — validado no navegador |
| Deploy | Docker prod + Portainer | 🟡 imagem buildada e publicada; falta subir no Pi |

## Dashboard web — as 5 telas

- **Visão geral** (`/`) — contadores (pendentes, enviadas hoje, falhadas, assinantes
  ativos) atualizando sozinhos a cada 10s via `htmx` (`GET /partials/stats`), + tabela
  de últimas mensagens
- **Mensagens** (`/mensagens`) — lista todas as mensagens, mais recentes primeiro
- **Entregas falhadas** (`/entregas`) — mensagem + assinante + tentativas + erro de
  cada `delivery` com `status = failed`
- **Rotas** (`/rotas`) — lista + formulário de criação (`POST /rotas`, padrão
  Post/Redirect/Get)
- **Assinantes** (`/assinantes`) — lista com `chat_id` mascarado na tela + formulário
  de criação (`POST /assinantes`, restrito a `kind = telegram` por enquanto — único
  canal com `Notifier` implementado)

**Isso substitui o passo manual de SQL** que o `README.md` documentava pra cadastrar
`subscriber`/`route` — agora é só abrir `/assinantes` e `/rotas` no navegador. Dado
pessoal (`chat_id`, token) continua nunca passando pelo código-fonte, só pelo formulário
direto pro banco.

## Entrega ponta a ponta — backend ✅ validado

`cmd/notify/main.go` liga tudo (Postgres real, handler HTTP, webui, worker), migrations
sobem sozinhas (embutidas no binário), bot real criado no BotFather, notificação chegou
no celular de verdade via `curl`.

**Gotchas reais encontrados no processo** (registrados em `DECISIONS.md` com detalhe):
- `route.topic` tem FK pra `topic` — `topic` só existe depois do primeiro publish
  naquele nome.
- `delivery` só é criada no momento do `POST` — mensagens publicadas antes da `route`
  existir ficam órfãs pra sempre.
- Env var só é lida na subida do processo — reiniciar o `go run` é obrigatório depois
  de exportar `TELEGRAM_BOT_TOKEN`.
- `pgx` via `database/sql` genérico não decodifica `TEXT[]` sozinho — precisou de
  `pq.Array(...)` como helper de encode/decode.

## Gotchas do dashboard web

- Dois `{{define "content"}}` em páginas diferentes, parseados juntos num glob só,
  colidem silenciosamente — resolvido com `parsePage` (um `*template.Template`
  independente por página).
- Botão de submit em formulário grid desalinha se não ganhar um `<label>&nbsp;</label>`
  invisível — os outros campos têm rótulo empurrando o input pra baixo, o botão sozinho
  não.
- `curl` sem `-L` não segue redirect — baixou um stub de 48 bytes no lugar do
  `htmx.min.js` de verdade (arquivo "existia" mas não funcionava).
- Esquecido de ligar `Routes: store` no `webui.Handler` do `main.go` ao adicionar o
  campo — `go test` não pegou (fakes cobrem), só o `go run` real revelou o nil pointer.
  Lição: sempre validar no servidor real depois de mudar wiring em `main.go`, não só
  rodar os testes.

## Deploy — estado atual

- `Dockerfile` — build multi-stage, `CGO_ENABLED=0 GOARCH=arm64` (Pi 5), imagem final
  `distroless/static-debian12` (só o binário, sem shell/SO por baixo)
- `docker-compose.prod.yml` — usa a rede externa `homelab_net`, conecta no Postgres
  global do homelab via `postgres:5432`, `DATABASE_URL`/`TELEGRAM_BOT_TOKEN` vêm de
  variável de ambiente (nunca cravados no compose)
- Imagem já buildada (`docker buildx build --platform linux/arm64 ... --push`) e
  publicada em `ghcr.io/sprained/notify-system`
- **Falta:** criar a stack no Portainer do Pi (apontando pro `docker-compose.prod.yml`,
  com `DATABASE_URL`/`TELEGRAM_BOT_TOKEN` configurados na stack), confirmar que sobe e
  conecta no Postgres global, e validar um `curl` real → dashboard real → Telegram real
  rodando no Pi (não só local)

## Dependências instaladas

```
github.com/golang-migrate/migrate/v4
github.com/golang-migrate/migrate/v4/database/postgres
github.com/golang-migrate/migrate/v4/source/iofs
github.com/jackc/pgx/v5
github.com/jackc/pgx/v5/stdlib
github.com/testcontainers/testcontainers-go
github.com/testcontainers/testcontainers-go/modules/postgres
github.com/oklog/ulid/v2
github.com/lib/pq
```

Front-end não trouxe dependência Go nova — `htmx` é JS vendorizado (arquivo estático),
não módulo Go.

Requer Docker Desktop rodando pra `go test ./internal/postgres/... ./internal/httpapi/... ./internal/webui/...`
(os dois primeiros usam testcontainers direto; `webui` usa fakes, não precisa de Docker).

## Próximo passo

Subir a stack no Portainer do Pi e validar ponta a ponta rodando lá (não só local) —
única task aberta em "Deploy" no `NOTIFY.md`. Depois disso, o roadmap original inteiro
(Fases 1–5 + deploy) está fechado; resta só a Fase 6 (App Android), deixada pra depois
por decisão do Gabs.
