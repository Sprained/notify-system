# notify-system

Central de notificações própria pro homelab. Recebe eventos por HTTP e roteia pra
múltiplos canais (Telegram por enquanto) conforme regras de tópico e prioridade.
Inspirado no [ntfy](https://ntfy.sh), construído do zero em Go + Postgres.

Desenho completo, modelo de dados e roadmap: [NOTIFY.md](NOTIFY.md).

## Requisitos

- Go 1.26+
- Docker (só pro Postgres local — via `docker-compose.yml`)

## Configuração

### Variáveis de ambiente

| Variável             | Default                                                        | Descrição                                                 |
| -------------------- | --------------------------------------------------------------- | ---------------------------------------------------------- |
| `DATABASE_URL`         | `postgres://notify:notify@localhost:5432/notify?sslmode=disable` | String de conexão com o Postgres                            |
| `HTTP_ADDR`             | `:8080`                                                           | Endereço/porta onde o servidor HTTP sobe                    |
| `TELEGRAM_BOT_TOKEN`     | *(vazio)*                                                          | Token do bot (BotFather). Sem isso, o canal `telegram` fica desregistrado — só o `log` funciona |

Exemplo de `.env` (não versionado — se criar, adicionar no `.gitignore`):
```
DATABASE_URL=postgres://notify:notify@localhost:5432/notify?sslmode=disable
HTTP_ADDR=:8080
TELEGRAM_BOT_TOKEN=123456789:ABCdefGhIJKlmNoPQRsTUVwxyz
```

## Como rodar

```bash
docker compose up -d
go run ./cmd/notify
```

O binário sobe as migrations automaticamente na inicialização (embutidas via
`go:embed`, não precisa rodar nada à parte).

## Como testar

```bash
go test ./...
```

Os testes de `internal/postgres` e `internal/httpapi` usam
[testcontainers-go](https://golang.testcontainers.org/) — sobem um Postgres real em
container a cada execução, então **precisa do Docker rodando**.

## ⚠️ Configuração manual necessária (não automatizada de propósito)

Sem isso, o serviço aceita a ingestão normalmente, mas **nenhuma notificação é
entregue** — a `message` é gravada, porém zero `delivery` é criada por falta de rota.

**1. Criar o bot no Telegram**
Conversar com [@BotFather](https://t.me/BotFather), `/newbot`, guardar o token (vai no
`TELEGRAM_BOT_TOKEN`).

**2. Descobrir o `chat_id`**
Mandar `/start` (ou qualquer mensagem) pro bot recém-criado, depois:
```bash
curl https://api.telegram.org/bot<TOKEN>/getUpdates
```
O `chat_id` está em `result[0].message.chat.id`.

**3. Cadastrar o assinante e a rota pelo dashboard**
Com o servidor rodando, abrir `http://localhost:8080/assinantes` e cadastrar o
assinante (rótulo + o `chat_id` do passo 2). Depois `http://localhost:8080/rotas` pra
ligar um tópico a esse assinante. Dado pessoal nunca passa pelo código-fonte — só pelo
formulário, direto pro banco (por isso não existe migration de seed pra isso, ver
[DECISIONS.md](DECISIONS.md)).

## Uso — enviando uma notificação

```bash
curl -X POST http://localhost:8080/alertas \
  -H "X-Title: Título da notificação" \
  -H "X-Priority: 4" \
  -d "Corpo da mensagem"
```

Headers opcionais, seguindo a convenção do ntfy: `X-Title`, `X-Priority` (1–5, default
3), `X-Tags` (separado por vírgula), `X-Click` (URL).

## Documentação do projeto

- [NOTIFY.md](NOTIFY.md) — desenho geral, modelo de dados, roadmap completo
- [DECISIONS.md](DECISIONS.md) — decisões pontuais de implementação, com o porquê
- [TESTS.md](TESTS.md) — checklist de casos de teste por fase
- [RESUME.md](RESUME.md) — snapshot do progresso atual
