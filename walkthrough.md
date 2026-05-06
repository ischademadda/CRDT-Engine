# Walkthrough: CRDT-Engine — Фазы 0–1 + Fugue

## Git Graph

```
*   7d5606e  Merge feat/fugue-algorithm
|\
| * 023db64  docs: ADR-0005 — Fugue algorithm selection
| * c003143  test(fugue): 17 tests — no-interleaving proof
| * 342f14a  feat(fugue): Fugue tree core
| * 5ea8e29  feat(engine): OpID.Compare, IsZero, String
|/
*   d850650  Merge feat/phase-1-crdt-primitives
|\
| * 422fdf0  docs: ADR-0004 — CRDT primitives selection
| * 77c8943  feat: VectorClock, LWW-Register, 2P-Set
|/
*   f9828cc  Merge feat/phase-0-cleanup
|\
| * bee82d9  feat: CRDTNode interface, GSet rewrite, cleanup
|/
```

---

## Реализованные CRDT-типы

| Тип | Файл | Назначение | Тестов |
|-----|------|-----------|--------|
| **GSet** | [gset.go](file:///c:/Users/ischa/PROJECTS/self/CRDT-Engine/pkg/crdt/gset.go) | Grow-only Set | 10 |
| **VectorClock** | [vclock.go](file:///c:/Users/ischa/PROJECTS/self/CRDT-Engine/pkg/crdt/vclock.go) | Каузальный порядок | 11 |
| **LWW-Register** | [lww_register.go](file:///c:/Users/ischa/PROJECTS/self/CRDT-Engine/pkg/crdt/lww_register.go) | Last-Writer-Wins | 11 |
| **2P-Set** | [twopset.go](file:///c:/Users/ischa/PROJECTS/self/CRDT-Engine/pkg/crdt/twopset.go) | Two-Phase Set (remove-wins) | 12 |
| **Fugue** | [fugue.go](file:///c:/Users/ischa/PROJECTS/self/CRDT-Engine/pkg/crdt/fugue.go) | Sequence CRDT (текст) — **без переплетения** | 17 |

**Итого: 60 тестов, все PASS** ✅

---

## Ключевой результат: Fugue — нет переплетения

Тест `TestFugue_ConcurrentInsert_NoInterleaving`:
- Пользователь 1 набирает "Hello", пользователь 2 набирает "World" в одну позицию
- Результат: **"HelloWorld"** (не "HWeolrllod")
- 3 пользователя (AA, BB, CC) → **"AABBCC"** — каждая группа непрерывна

---

## ADR документация

| ADR | Решение |
|-----|---------|
| [ADR-0003](file:///c:/Users/ischa/PROJECTS/self/CRDT-Engine/docs/adr/0003-crdtnode-interface.md) | CRDTNode generic interface |
| [ADR-0004](file:///c:/Users/ischa/PROJECTS/self/CRDT-Engine/docs/adr/0004-crdt-primitives.md) | Выбор CRDT-примитивов |
| [ADR-0005](file:///c:/Users/ischa/PROJECTS/self/CRDT-Engine/docs/adr/0005-fugue-algorithm.md) | Fugue vs YATA |

---

## Следующие шаги
- **Фаза 2**: WebSocket Hub + Worker Pool (`feat/websocket-hub`)
- **Фаза 3**: Redis Pub/Sub (`feat/redis-pub-sub`)
- **Фаза 4**: Epoch-based GC, RLE оптимизации
