# CRDT-Engine — Project Summary

## Что это

Легковесный движок для совместного редактирования текста в реальном времени. Написан на Go. Ключевое свойство: конкурентные правки от разных пользователей никогда не перемешиваются посимвольно (гарантия алгоритма Fugue).

Полный стек: CRDT-ядро → WebSocket-транспорт → Redis Pub/Sub → демо-сервер.

---

## Структура проекта

```
CRDT-Engine/
├── pkg/crdt/             # CRDT-ядро (публичная библиотека)
│   ├── engine.go         # Интерфейсы Operation, CRDTNode[State], тип OpID
│   ├── fugue.go          # Главный CRDT: дерево Fugue для текста
│   ├── vclock.go         # Vector Clock (каузальный порядок)
│   ├── gset.go           # Grow-only Set
│   ├── lww_register.go   # Last-Writer-Wins Register
│   ├── twopset.go        # Two-Phase Set
│   └── doc.go            # godoc overview + примеры
│
├── internal/
│   ├── websocket/        # WebSocket-транспорт
│   │   ├── hub.go        # Hub: Fan-In/Fan-Out, реестр клиентов
│   │   ├── client.go     # Один WebSocket-клиент
│   │   ├── handler.go    # HTTP → WebSocket upgrade
│   │   └── message.go    # Тип Message
│   ├── redis/            # Redis Pub/Sub адаптер
│   │   ├── publisher.go  # Публикация дельт в Redis
│   │   └── subscriber.go # Подписка на каналы документов
│   ├── worker/
│   │   └── pool.go       # Worker Pool с graceful shutdown
│   ├── repository/
│   │   ├── repository.go # Интерфейс DocumentRepository
│   │   └── inmemory.go   # In-memory реализация
│   ├── usecase/
│   │   ├── sync.go       # SyncUseCase — оркестратор дельт
│   │   └── document.go   # DocumentUseCase — загрузка/создание документов
│   └── integration/
│       └── two_nodes_test.go  # Интеграционный тест: два узла через Redis
│
├── cmd/demo-app/
│   ├── main.go           # Сборка всего стека, HTTP-сервер, dispatcher
│   └── index.go          # HTML/JS клиент (встроен в бинарник)
│
└── docs/
    ├── adr/              # Architecture Decision Records (ADR-0001 — ADR-0008)
    └── c4/               # C4-диаграммы (Context, Container, Component)
```

---

## Что реализовано

### CRDT-ядро (`pkg/crdt/`)

| Тип | Назначение |
|-----|-----------|
| `FugueTree` | Sequence CRDT для текста. Дерево с in-order обходом. Гарантирует отсутствие переплетения при конкурентных вставках. |
| `VectorClock` | Отслеживает каузальный порядок операций. Нужен для Epoch-based GC (запланирован). |
| `GSet[T]` | Grow-only Set. Только добавление, Merge = union. |
| `LWWRegister[T]` | Last-Writer-Wins Register. Конфликт решается по timestamp + ReplicaID как tiebreaker. |
| `TwoPSet[T]` | Two-Phase Set. Удалённый элемент нельзя добавить снова (remove-wins). |

Все типы потокобезопасны (`sync.RWMutex`), реализуют `CRDTNode[State]`, 60 тестов — все PASS.

### Транспортный слой

| Компонент | Что делает |
|-----------|-----------|
| `Hub` | Fan-In: все входящие WS-сообщения → один канал. Fan-Out: `Broadcast` рассылает всем клиентам документа кроме отправителя. Медленные клиенты отключаются. |
| `worker.Pool` | N горутин читают из канала Hub и обрабатывают операции параллельно. Graceful shutdown через `context`. |
| `redis.Publisher/Subscriber` | Публикует дельты в `crdt:doc:<id>`. Подписывается и отдаёт канал `chan Delta`. Эхо от собственных публикаций фильтруется по `OriginNodeID`. |

### Прикладной слой (`internal/usecase/`)

`SyncUseCase.HandleDelta` — единая точка входа для всех дельт:
1. Загрузить (или создать) дерево документа из репозитория.
2. Десериализовать payload → `crdt.Operation`.
3. Применить к дереву.
4. WebSocket Broadcast (всегда).
5. Redis Publish (только для локальных дельт, чтобы не зацикливаться).

Слой работает с портами `Broadcaster` и `Publisher`, не импортируя конкретные транспорты — тестируется с mock-ами.

### Демо-сервер (`cmd/demo-app/`)

```
HTTP/WebSocket → Hub → Worker Pool → SyncUseCase → InMemory repo
                                          ↕
                                   Redis Pub/Sub ↔ другие узлы
```

ENV-переменные: `HTTP_ADDR`, `REDIS_ADDR`, `NODE_ID`, `DOC_ID`.

Клиент шлёт `insert_intent`/`delete_intent` с позицией. Сервер резолвит позицию в `FugueInsertOp`/`FugueDeleteOp` и рассылает всем.

---

## Как запустить

```bash
# Один узел, без Redis (два браузера)
$env:REDIS_ADDR=""; go run ./cmd/demo-app

# Два узла через Redis (Docker)
docker compose up --build
```

Подробнее — [TESTING.md](./TESTING.md).

---

## Что запланировано

- **Epoch-based GC** — физическое удаление томбстоунов через Vector Clock (нет в коде, только VectorClock готов)
- **RLE-оптимизация** в Fugue-дереве (группировка последовательных символов)
- **Protobuf** вместо JSON для сериализации дельт
- **PostgreSQL** реализация репозитория (сейчас только in-memory)

---

## ADR-документация

| ADR | Решение |
|-----|---------|
| 0001 | Go как язык реализации |
| 0002 | Standard Go Project Layout |
| 0003 | Generic интерфейс `CRDTNode[State]` |
| 0004 | Выбор CRDT-примитивов (GSet, VectorClock, LWW, 2P-Set) |
| 0005 | Fugue вместо YATA (доказуемое non-interleaving) |
| 0006 | WebSocket + Redis Pub/Sub транспорт |
| 0007 | Clean Architecture: repository / usecase / delivery |
| 0008 | Docker + GitHub Actions CI |
