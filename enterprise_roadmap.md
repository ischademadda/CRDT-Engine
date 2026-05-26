# 🏢 CRDT-Engine → Enterprise On-Premise: Полный план разработки

> **Продукт:** Корпоративная платформа совместного редактирования документов в реальном времени.
> **Целевая аудитория:** Банки, госструктуры, крупный бизнес (B2B Enterprise).
> **Ключевое УТП:** Self-Hosted, развертывание внутри закрытого контура заказчика, без утечки данных в облако.

---

## 📍 Текущее состояние проекта — `v0.5.0-dev`

> [!IMPORTANT]
> Аудит кодовой базы показал, что проект значительно продвинулся и уже закрывает несколько ранних этапов. Ниже — обоснование текущей оценки версии.

### Что уже реализовано:

| Критерий | Статус | Детали |
|----------|--------|--------|
| CRDT-ядро Fugue | ✅ Готово | `pkg/crdt`: Insert, Delete, Merge, ApplyRemote, Idempotency, Non-interleaving |
| Unit-тесты ≥ 80% | ✅ **85.6%** | `pkg/crdt` — 85.6%, `internal/redis` — 83.6%, `repository` — 82.9%, `usecase` — 81.1%, `websocket` — 88.3%, `worker` — 86.8% |
| Интеграционные тесты | ✅ Готово | `two_nodes_test.go`: propagation, echo-loop guard, concurrent convergence (miniredis) |
| CI/CD pipeline | ✅ Готово | GitHub Actions: `go vet`, `golangci-lint` (8 линтеров), `go test -cover`, `docker build` |
| Документация | ✅ Готово | 8 ADR, C4-диаграммы (Context/Container/Component), flow-диаграммы, `TESTING.md` (5 сценариев), `PROJECT_SUMMARY.md` |
| Мультинодовый кластер | ✅ Готово | Docker Compose: 2 узла `demo-app` через Redis Pub/Sub |
| Микросервис аналитики | ✅ Готово | `analytics-service`: Redis consumer → PostgreSQL (UPSERT), REST API дашборд |
| Микросервис истории | ✅ Готово | `history-service`: Event Store (append-only log), Event Sourcing (state reconstruction), Time Travel slider |
| Фронтенд 3-Way Merge | ✅ Готово | Инкрементальное применение удалённых правок, сохранение курсора |
| Multi-stage Dockerfiles | ✅ Готово | distroless runtime, nonroot user |

### Что ещё НЕ реализовано (осталось сделать):
- ❌ Fuzz-тестирование CRDT
- ❌ Tombstone GC / Snapshot Checkpointing
- ❌ Бенчмарки и нагрузочные тесты
- ❌ Персистентность документов между перезапусками
- ❌ Аутентификация / RBAC
- ❌ Rich Text редактор

---

## Фаза 0 — Финализация ядра `v0.5.0 → v0.7.0`

> **Цель:** Закрыть технический долг ядра: fuzz, GC, бенчмарки.

---

### Этап 0.1 — Fuzz-тестирование и Conventional Commits `v0.5.0`

| # | Шаг | Описание |
|---|------|----------|
| 1 | Fuzz-тестирование CRDT | Go-native fuzzing для `ApplyRemoteInsert` / `ApplyRemoteDelete` — поиск паник |
| 2 | Conventional Commits + SemVer | `commitlint`, автогенерация `CHANGELOG.md` |

**AC:**
- [ ] Fuzz-тесты проходят 60 секунд без паник
- [ ] `CHANGELOG.md` генерируется автоматически из коммитов

---

### Этап 0.2 — Garbage Collection для CRDT `v0.6.0`

| # | Шаг | Описание |
|---|------|----------|
| 1 | Tombstone Compaction | Удаление tombstone-узлов после подтверждения получения всеми репликами (Causal Stability) |
| 2 | Snapshot Checkpointing | Периодическая сериализация `FugueTree` в бинарный снимок (protobuf/msgpack) → Redis/PostgreSQL |
| 3 | Восстановление из снимка | Новый узел загружает последний снимок + дельты после него |

**AC:**
- [ ] 100K операций (50% удалений) после compaction — ≤ 40% исходной памяти
- [ ] Новый узел синхронизируется за ≤ 2 секунды

---

### Этап 0.3 — Бенчмарки и стресс-тесты `v0.7.0`

| # | Шаг | Описание |
|---|------|----------|
| 1 | Бенчмарк-сьют | `go test -bench`: одиночная вставка, 10K конкурентных операций, сериализация |
| 2 | Нагрузочное тестирование | Go-клиент: 500 WS-подключений × 5 символов/сек. Замер p50/p95/p99, RAM, CPU |
| 3 | Профилирование | `pprof`: устранение аллокаций и блокировок в горячих путях |

**AC:**
- [ ] p99 ≤ 50ms при 500 подключениях
- [ ] RAM ≤ 500MB при 500 подключениях и документе в 50K символов

---

## Фаза 1 — Персистентность и безопасность `v0.8.0 → v1.0.0`

> **Цель:** Документы переживают перезапуск, появляется управление пользователями.

---

### Этап 1.1 — Персистентное хранилище документов `v0.8.0`

| # | Шаг | Описание |
|---|------|----------|
| 1 | Таблица `documents` | `id`, `title`, `owner_id`, `created_at`, `updated_at`, `is_archived` |
| 2 | Таблица `document_snapshots` | `document_id`, `version`, `tree_data` (BYTEA), `created_at` |
| 3 | Автосохранение | Фоновая горутина: каждые 30 сек или N операций → INSERT snapshot |
| 4 | Восстановление при старте | Загрузка последних снимков активных документов из PostgreSQL |

**AC:**
- [ ] `docker compose down && docker compose up` — текст документа сохраняется
- [ ] Одновременная работа с ≥ 50 документами
- [ ] Снимок 10K символов ≤ 100KB

---

### Этап 1.2 — Аутентификация и авторизация `v0.9.0`

| # | Шаг | Описание |
|---|------|----------|
| 1 | Микросервис `auth-service` | Регистрация, логин, JWT access + refresh. PostgreSQL + `bcrypt` |
| 2 | JWT на WebSocket | Валидация подписи, извлечение `user_id` при `ws.connect` |
| 3 | RBAC | Роли: `owner`, `editor`, `viewer`. Таблица `document_permissions` |
| 4 | LDAP/AD интеграция | Опциональный адаптер LDAP bind. Маппинг групп → роли |

**AC:**
- [ ] Неаутентифицированный WS-запрос → `4001 Unauthorized`
- [ ] `viewer` не может отправлять `insert_intent`
- [ ] LDAP проверена с тестовым OpenLDAP в `docker-compose.test.yml`

---

### Этап 1.3 — Admin Panel `v1.0.0`

| # | Шаг | Описание |
|---|------|----------|
| 1 | Логин / Регистрация | Формы, валидация, JWT в `httpOnly` cookies |
| 2 | Список документов | Карточки, «Создать», «Архивировать», «Поделиться» |
| 3 | Управление доступом | Модальное окно: добавление пользователей, выбор роли |
| 4 | Админ-панель | Список пользователей, отключение сессий, логи аудита |

**AC:**
- [ ] Создать документ + пригласить коллегу ≤ 3 клика
- [ ] Администратор видит все сессии и может отключить пользователя

---

## Фаза 2 — Rich Text и продвинутый редактор `v1.1.0 → v1.3.0`

> **Цель:** От plain-text `<textarea>` к WYSIWYG уровня Google Docs.

---

### Этап 2.1 — Интеграция с ProseMirror / Tiptap `v1.1.0`

| # | Шаг | Описание |
|---|------|----------|
| 1 | Замена `<textarea>` на Tiptap | Тулбар форматирования |
| 2 | Адаптер Intent → CRDT | Перехват транзакций ProseMirror → `insert_intent` / `delete_intent` |
| 3 | Атрибуты | `FugueInsertOp.attributes: map[string]string` (bold, italic, heading) |
| 4 | Рендеринг атрибутов | Конвертация CRDT-узлов → ProseMirror JSON Schema |

**AC:**
- [ ] Ctrl+B → жирный текст у всех участников
- [ ] Конкурентное форматирование корректно объединяется

---

### Этап 2.2 — Блочная структура `v1.2.0`

| # | Шаг | Описание |
|---|------|----------|
| 1 | Блоки | Параграфы, H1–H3, списки, цитаты, разделители |
| 2 | Вложенные списки | Drag-and-drop с CRDT-синхронизацией |
| 3 | Таблицы | Вставка строк/столбцов, редактирование ячеек |

**AC:**
- [ ] 5+ типов блоков синхронизируются между двумя узлами
- [ ] Drag-and-drop списка отражается мгновенно

---

### Этап 2.3 — Медиа и вложения `v1.3.0`

| # | Шаг | Описание |
|---|------|----------|
| 1 | `file-service` | Multipart upload → S3/MinIO |
| 2 | Inline-изображения | Drag-and-drop и Ctrl+V |
| 3 | Превью вложений | Генерация thumbnail'ов |

**AC:**
- [ ] Изображение отображается у всех ≤ 1 секунда
- [ ] Лимит вложений конфигурируем (по умолчанию 100MB)

---

## Фаза 3 — Enterprise-функции `v1.4.0 → v1.7.0`

> **Цель:** Довести продукт до уровня, за который корпорации готовы платить.

---

### Этап 3.1 — Observability `v1.4.0`

| # | Шаг | Описание |
|---|------|----------|
| 1 | Prometheus metrics | `ws_connections_active`, `crdt_ops_total`, `http_request_duration_seconds` |
| 2 | Grafana dashboards | Предустановленные JSON: «Обзор», «Здоровье кластера», «Нагрузка» |
| 3 | Structured logging | `slog` + JSON. Интеграция с ELK/Loki |
| 4 | Health endpoints | `/healthz`, `/readyz` для Kubernetes probes |

**AC:**
- [ ] Grafana показывает WS-подключения в реальном времени
- [ ] Все логи содержат `trace_id`

---

### Этап 3.2 — Kubernetes и Helm `v1.5.0`

| # | Шаг | Описание |
|---|------|----------|
| 1 | K8s Deployments | `Deployment`, `Service`, `ConfigMap`, `Secret` для каждого сервиса |
| 2 | Helm Chart | Параметризованный через `values.yaml` |
| 3 | HPA | Автоскейлинг `demo-app` по метрике `ws_connections_active` |
| 4 | PVC | Persistent Volumes для PostgreSQL и MinIO |

**AC:**
- [ ] `helm install` разворачивает кластер за ≤ 5 минут
- [ ] HPA масштабирует при > 200 WS на pod

---

### Этап 3.3 — Безопасность и комплаенс `v1.6.0`

| # | Шаг | Описание |
|---|------|----------|
| 1 | Аудит-лог | Все действия → `audit_log` с IP, User-Agent, timestamp |
| 2 | Шифрование at-rest | AES-256 для снимков в PostgreSQL |
| 3 | TLS everywhere | mTLS между сервисами, TLS на Ingress |
| 4 | Политики хранения | TTL для `document_history` (90 дней → cold storage) |

**AC:**
- [ ] Выгрузка аудит-лога в CSV/JSON
- [ ] Прохождение OWASP Top 10

---

### Этап 3.4 — Offline-first `v1.7.0`

| # | Шаг | Описание |
|---|------|----------|
| 1 | CRDT в браузере | `FugueTree` → WebAssembly (Go→Wasm) или TypeScript port. IndexedDB |
| 2 | Offline queue | Локальная очередь → replay при реконнекте |
| 3 | Sync indicator | UI: «Сохранено ✓» / «Синхронизация...» / «Оффлайн» |

**AC:**
- [ ] Отключить Wi-Fi → напечатать 100 символов → включить → бесшовная синхронизация
- [ ] Два offline-пользователя → корректное объединение

---

## Фаза 4 — Коммерциализация `v2.0.0 → v2.1.0`

> **Цель:** Упаковать продукт для продажи.

---

### Этап 4.1 — Лицензирование `v2.0.0`

| # | Шаг | Описание |
|---|------|----------|
| 1 | License Server | Ключ ограничивает: кол-во пользователей, срок, набор фич |
| 2 | Feature gates | Аудит (Pro), LDAP (Enterprise), шифрование (Enterprise) |
| 3 | Telemetry (opt-in) | Анонимная телеметрия для планирования |

**AC:**
- [ ] Истечение лицензии → read-only с предупреждением

---

### Этап 4.2 — Документация и SDK `v2.1.0`

| # | Шаг | Описание |
|---|------|----------|
| 1 | Deployment Guide | Docker Compose, Kubernetes, bare-metal |
| 2 | Admin Guide | Управление, бэкапы, мониторинг, обновления |
| 3 | API Reference | OpenAPI 3.0 + встроенный Swagger UI |
| 4 | SDK | Go и TypeScript SDK для интеграции |

**AC:**
- [ ] DevOps-инженер разворачивает систему только по документации

---

## Сводная таблица версий

| Версия | Название | Ключевой результат | Статус |
|--------|----------|-------------------|--------|
| `v0.1.0` | CRDT Core | Алгоритм Fugue, unit-тесты | ✅ Готово |
| `v0.2.0` | Transport Layer | Redis Pub/Sub, WebSocket Hub | ✅ Готово |
| `v0.3.0` | Multi-Node Cluster | Docker Compose, 2 узла, интеграционные тесты | ✅ Готово |
| `v0.4.0` | Analytics & History | 2 доп. микросервиса, Event Sourcing, Time Travel | ✅ Готово |
| `v0.5.0` | Fuzz & SemVer | Fuzz-тесты, Conventional Commits | 🔜 Следующий |
| `v0.6.0` | GC & Snapshots | Tombstone compaction, checkpoint restore | ⬜ |
| `v0.7.0` | Benchmarks | Нагрузочные тесты, p99 ≤ 50ms | ⬜ |
| `v0.8.0` | Persistence | Документы переживают перезапуск | ⬜ |
| `v0.9.0` | Auth & RBAC | JWT, роли, LDAP | ⬜ |
| `v1.0.0` | Admin Panel | Веб-интерфейс управления | ⬜ |
| `v1.1.0` | Rich Text | WYSIWYG через Tiptap/ProseMirror | ⬜ |
| `v1.2.0` | Block Editor | Заголовки, списки, таблицы | ⬜ |
| `v1.3.0` | Media & Files | S3/MinIO, inline-картинки | ⬜ |
| `v1.4.0` | Observability | Prometheus, Grafana, structured logs | ⬜ |
| `v1.5.0` | Kubernetes | Helm Chart, HPA, PVC | ⬜ |
| `v1.6.0` | Security | Аудит, шифрование, mTLS | ⬜ |
| `v1.7.0` | Offline-First | Wasm CRDT, IndexedDB, auto-sync | ⬜ |
| `v2.0.0` | Commercial Release | Лицензирование, feature gates | ⬜ |
| `v2.1.0` | Documentation | Guides, OpenAPI, SDK | ⬜ |

---

## Оценка трудозатрат (грубая, от текущего состояния)

| Фаза | Один разработчик | Команда из 3 |
|-------|-----------------|-------------|
| Фаза 0 (Fuzz + GC + Bench) | 3–4 недели | 1–2 недели |
| Фаза 1 (Persistence + Auth) | 8–10 недель | 3–4 недели |
| Фаза 2 (Rich Text) | 10–14 недель | 4–6 недель |
| Фаза 3 (Enterprise) | 12–16 недель | 5–7 недель |
| Фаза 4 (Коммерциализация) | 4–6 недель | 2–3 недели |
| **Итого (оставшееся)** | **~37–50 недель** | **~15–22 недели** |

> [!TIP]
> Версии `v0.1.0`–`v0.4.0` уже закрыты. Проект находится на уровне `v0.4.0` и готов к переходу на `v0.5.0`. Для пилотных продаж как MVP достаточно дойти до `v1.0.0` (Persistence + Auth + Admin Panel).
