# Подробный план работы: CRDT-Engine

## 1. Аудит текущего состояния проекта

### 1.1. Структура репозитория (ветка `main`)

```
CRDT-Engine/
├── .gitignore
├── go.mod                          # module github.com/ischademadda/CRDT-Engine, go 1.26.1
├── README.md                       # Описание фичей (Fugue, Epoch GC, RLE, Fan-In, Generics)
├── sessions.go                     # ⚠️ Файл в корне, package main — не относится к движку
├── cmd/
│   └── demo-app/
│       └── .gitkeep                # Пусто
├── internal/
│   ├── README.md
│   ├── redis/                      # Пусто
│   └── websocket/                  # Пусто
├── pkg/
│   └── crdt/
│       ├── README.md
│       ├── gset.go                 # GSet[T comparable] — базовый Grow-only Set с sync.RWMutex
│       ├── gset_test.go            # Тест: Add + WriteTo (без assertions!)
│       ├── parse_delta.go          # ParseDelta(payload any) — type switch демо
│       ├── parse_test.go           # Тест ParseDelta (без assertions)
│       ├── worker_pool_test.go     # Демо Worker Pool (без assertions)
│       └── output.txt              # Артефакт тестов
├── docs/
│   ├── README.md
│   ├── adr/
│   │   ├── 0001-language-choice.md
│   │   └── 0002-repository-structure.md
│   ├── c4/
│   │   ├── context-diagram.dsl     # Structurizr DSL
│   │   ├── 01. Context-Diagram.svg
│   │   ├── 02. Container-Diagram.svg
│   │   └── 03. Component-Diagram.svg
│   └── flow/
│       └── single-node-ingestion-flow.mmd  # Sequence diagram
└── Два .md документа (анализ + карьера)
```

### 1.2. Ветка `test/crdt-basic-logic`

Один коммит: `197c186 feat/CRDTNode-basic`. Ключевые изменения:

| Файл | Изменение |
|------|-----------|
| `pkg/crdt/engine.go` | **[NEW]** — Интерфейсы `Operation` и `CRDTNode[State any]` (Merge, ApplyOperation, State) |
| `pkg/crdt/gset.go` | Расширен: добавлены `AddOp[T]`, реализация `CRDTNode` (Merge, State, ApplyOperation) |
| `pkg/crdt/gset_test.go` | Переписан: `TestGSet_Merge` — тест слияния двух сетов |
| `README.md` | Упрощён до одной строки |
| Удалены | Все документы SA (ADR, C4, flow, .md-аналитика) |

> [!WARNING]
> Ветка `test/crdt-basic-logic` удалила всю документацию SA. Это значит, что при мёрже нужно будет очень аккуратно разрешать конфликты — **документация должна остаться**. Скорее всего, мёрж стоит делать cherry-pick только Go-кода.

### 1.3. Другие ветки (для справки)

| Ветка | Содержание |
|-------|-----------|
| `docs/adr` | ADR документы |
| `docs/c4-architecture` | C4 диаграммы |
| `docs/erd` | ER-диаграммы БД |
| `docs/sequence-diagram` | Sequence-диаграммы |
| `feat/crdt-engine/gset` | Первоначальная GSet логика |

---

## 2. Gap-анализ: Текущее состояние vs. Целевая архитектура

На основе двух документов («Анализ CRDT-движка» и «CRDT-Проект для карьеры»), вот что **заявлено в README/документах**, но **НЕ реализовано в коде**:

| Компонент | README заявляет | Реальное состояние | Приоритет |
|-----------|----------------|-------------------|-----------|
| **Алгоритм Fugue** | ✅ Заявлен | ❌ Нет реализации | 🔴 Критический |
| **CRDTNode интерфейс** | ✅ Заявлен (Generics) | ⚠️ Есть в ветке `test/`, не в `main` | 🟡 Нужен мёрж |
| **GSet с Merge** | ✅ Заявлен | ⚠️ Есть в ветке `test/`, базовый в `main` | 🟡 Нужен мёрж |
| **Epoch-based GC** | ✅ Заявлен | ❌ Нет реализации | 🟡 Средний |
| **RLE / B-tree** | ✅ Заявлен | ❌ Нет реализации | 🟡 Средний |
| **WebSocket слой** | ✅ Заявлен | ❌ Пустая директория `internal/websocket/` | 🔴 Критический |
| **Fan-In / Worker Pool** | ✅ Заявлен | ⚠️ Есть демо-тест, не в production коде | 🟡 Средний |
| **Redis Pub/Sub** | ✅ Заявлен | ❌ Пустая директория `internal/redis/` | 🟡 Средний |
| **Demo App** | ✅ Заявлен | ❌ Только `.gitkeep` | 🟡 Средний |
| **LWW-Register** | Упомянут в документах | ❌ Нет реализации | 🟡 Средний |
| **Vector Clocks** | Упомянуты для GC | ❌ Нет реализации | 🟡 Средний |
| **Clean Architecture слои** | Описаны в документах | ⚠️ Структура директорий есть, кода нет | 🟡 Средний |
| **Нормальные тесты** | Подразумеваются | ❌ Тесты без assertions, fmt.Println вместо t.Error | 🟡 Средний |

> [!IMPORTANT]
> `sessions.go` в корне проекта с `package main` — это учебный файл, не относящийся к движку. Нужно решить: удалить его или перенести.

---

## 3. Предлагаемый план работы (по фазам)

### Фаза 0: Подготовка и наведение порядка
> Приоритет: 🔴 Сделать первым

- [ ] **Мёрж кода из `test/crdt-basic-logic` в `main`**
  - Cherry-pick Go-файлов (`engine.go`, обновлённый `gset.go`, `gset_test.go`)
  - **НЕ** мёржить удалённую документацию
- [ ] **Удалить/переместить `sessions.go`** из корня (учебный файл, нарушает структуру)
- [ ] **Удалить `parse_delta.go` и `parse_test.go`** — это демо type-switch, не CRDT логика
- [ ] **Удалить `output.txt`** из `pkg/crdt/` (тестовый артефакт)
- [ ] **Переписать тесты** — добавить реальные assertions вместо `fmt.Println`
- [ ] **Исправить ADR-0002**: заголовок `# ADR-0001` — должен быть `# ADR-0002`

---

### Фаза 1: Ядро CRDT-примитивов (`pkg/crdt/`)
> Приоритет: 🔴 Фундамент всего движка

#### 1.1. Доработка интерфейсов (`engine.go`)
- [ ] Уточнить интерфейс `Operation` — добавить метод `OpType() string` 
- [ ] Добавить интерфейс `Mergeable` (как описано в документах)
- [ ] Реализовать `OpID` — глобальный идентификатор операции `{ReplicaID, Counter}`

#### 1.2. Vector Clocks (`pkg/crdt/vclock.go`)
- [ ] Реализовать `VectorClock` — map[string]uint64 с `Merge()`, `Increment()`, `Compare()`
- [ ] Тесты на коммутативность и ассоциативность

#### 1.3. LWW-Register (`pkg/crdt/lww_register.go`)
- [ ] Реализовать Last-Writer-Wins Register с generic типом
- [ ] `Set(value T, timestamp)`, `Get() T`, `Merge(other)` 
- [ ] Тесты на разрешение конфликтов по timestamp

#### 1.4. 2P-Set (Two-Phase Set) (`pkg/crdt/twopset.go`)
- [ ] Реализовать на базе двух GSet (add-set + remove-set)
- [ ] `Add()`, `Remove()`, `Contains()`, `Merge()`
- [ ] Тесты на идемпотентность удаления

#### 1.5. Алгоритм Fugue (`pkg/crdt/fugue.go`)
- [ ] Реализовать `FugueNode` — узел дерева Fugue
- [ ] Реализовать `FugueTree` — основное B-дерево с RLE-блоками
- [ ] `InsertAt(index, char, replicaID)` — вставка с относительной адресацией
- [ ] `Delete(index)` — пометка томбстоуном
- [ ] `Merge(remoteDelta)` — слияние удалённых операций
- [ ] `ToString()` — материализация текста из дерева
- [ ] Тесты:
  - Конкурентная вставка в один индекс → **НЕТ переплетения**
  - Коммутативность: merge(A,B) == merge(B,A)
  - Идемпотентность: merge(A,A) == A

---

### Фаза 2: Транспортный слой (`internal/websocket/`, `internal/redis/`)
> Приоритет: 🟡 Следующий после ядра

#### 2.1. WebSocket-менеджер (`internal/websocket/`)
- [ ] `Hub` — управление активными подключениями (map + sync.RWMutex)
- [ ] `Client` — обёртка над gorilla/websocket conn
- [ ] `HandleUpgrade()` — HTTP → WebSocket upgrade handler
- [ ] Fan-In: все входящие сообщения → единый `chan Operation`
- [ ] Fan-Out: broadcast resolved delta → все клиенты документа

#### 2.2. Worker Pool (`internal/worker/`)
- [ ] Конфигурируемый пул горутин для обработки операций из канала
- [ ] Graceful shutdown через context.Context

#### 2.3. Redis Pub/Sub adapter (`internal/redis/`)
- [ ] `Publisher` — публикация дельт в канал документа
- [ ] `Subscriber` — подписка на каналы документов
- [ ] Сериализация дельт (JSON → позже Protobuf)

---

### Фаза 3: Clean Architecture слои
> Приоритет: 🟡

#### 3.1. Repository layer (`internal/repository/`)
- [ ] Интерфейс `DocumentRepository`
- [ ] In-memory реализация (для тестов и MVP)
- [ ] PostgreSQL реализация (для production)

#### 3.2. UseCase layer (`internal/usecase/`)
- [ ] `SyncUseCase` — оркестрация: получить дельту → применить к движку → рассылка → сохранение
- [ ] `DocumentUseCase` — загрузка/создание документов

#### 3.3. Delivery layer (`cmd/demo-app/`)
- [ ] HTTP-сервер с WebSocket endpoint
- [ ] Простой HTML/JS клиент для демонстрации

---

### Фаза 4: Оптимизации
> Приоритет: 🟢

- [ ] Epoch-based GC для томбстоунов
- [ ] RLE-оптимизация в Fugue-дереве
- [ ] Protobuf сериализация дельт
- [ ] Docker + docker-compose (Go + Redis + PostgreSQL)
- [ ] CI/CD (GitHub Actions: тесты + golangci-lint)

---

### Фаза 5: SA-документация
> Приоритет: 🟢 (параллельно с разработкой)

- [ ] ADR-0003: Выбор алгоритма Fugue вместо YATA
- [ ] ADR-0004: Стратегия управления томбстоунами (Epoch GC)
- [ ] ADR-0005: Транспортный слой (WebSocket + Redis Pub/Sub)
- [ ] Обновить C4 диаграммы (добавить Component Level 3 с кодовыми модулями)
- [ ] Обновить Sequence diagram под реальный flow

---

## 4. Открытые вопросы

> [!IMPORTANT]
> **Вопрос 1:** С чего начинаем? Я предлагаю сначала **Фазу 0** (наведение порядка + мёрж) → затем **Фазу 1** (CRDT-примитивы). Но если у тебя есть приоритеты (например, нужно сначала показать демо преподавателю), скажи — подстроим план.

> [!IMPORTANT]
> **Вопрос 2:** Файл `sessions.go` в корне — это учебный пример? Его можно удалить? Он в `package main` и не связан с движком.

> [!IMPORTANT]
> **Вопрос 3:** Ветка `test/crdt-basic-logic` удалила всю документацию. Правильно ли я понимаю, что нужно забрать **только Go-код** (engine.go, обновлённый gset.go) оттуда в main, а документацию сохранить?

> [!IMPORTANT]
> **Вопрос 4:** На каком этапе сейчас учебный процесс? На какую неделю из плана (документ «12 недель») ты ориентируешься? Это поможет расставить приоритеты.

> [!IMPORTANT]
> **Вопрос 5:** Хочешь ли ты, чтобы я параллельно писал ADR-документы по мере реализации, или сначала фокусируемся чисто на коде?

---

## 5. План верификации

### Автоматические тесты
```bash
# Запуск всех тестов
go test ./pkg/crdt/... -v -race

# Запуск с проверкой покрытия
go test ./pkg/crdt/... -v -race -cover

# Линтер
golangci-lint run ./...
```

### Ручная проверка
- Запуск demo-app, открытие двух вкладок браузера, одновременное редактирование
- Хаос-тестирование: обрыв WebSocket → проверка восстановления состояния
- Бенчмарки: `go test -bench=. ./pkg/crdt/...`
