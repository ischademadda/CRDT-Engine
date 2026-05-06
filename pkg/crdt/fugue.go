package crdt

import (
	"errors"
	"sort"
	"strings"
	"sync"
)

// ========================================================================
// Fugue — последовательностный CRDT для совместного редактирования текста.
//
// Алгоритм Fugue гарантирует «максимальное отсутствие переплетения»
// (maximal non-interleaving): при конкурентной вставке двумя пользователями
// в одну позицию их текст никогда не перемешивается посимвольно.
//
// Структура: дерево, где порядок документа определяется in-order обходом:
//   left children → node → right children
//
// Правило вставки (Fugue Parent Selection):
//   При вставке между L (левый сосед) и R (правый сосед):
//   - Если R != nil И parent(R) == L → новый узел = left child of R
//   - Иначе → новый узел = right child of L
// ========================================================================

// FugueSide — сторона дочернего узла (левый или правый ребёнок).
type FugueSide int

const (
	FugueLeft  FugueSide = iota // Левый ребёнок (появляется ДО родителя в обходе)
	FugueRight                  // Правый ребёнок (появляется ПОСЛЕ родителя в обходе)
)

// FugueNode — узел в дереве Fugue. Представляет один символ документа.
type FugueNode struct {
	ID            OpID
	Value         rune
	ParentID      OpID
	Side          FugueSide
	IsDeleted     bool         // Томбстоун: символ удалён, но узел остаётся для CRDT-логики
	LeftChildren  []*FugueNode // Отсортированы по ID (детерминированный порядок)
	RightChildren []*FugueNode // Отсортированы по ID
}

// --- Операции Fugue (CmRDT) ---

// FugueInsertOp — операция вставки символа в дерево.
type FugueInsertOp struct {
	NodeID   OpID      // ID нового узла
	Value    rune      // Символ
	ParentID OpID      // Родитель в дереве
	Side     FugueSide // Сторона (left/right child)
}

func (o FugueInsertOp) OpType() string { return "fugue_insert" }
func (o FugueInsertOp) ID() OpID       { return o.NodeID }

// FugueDeleteOp — операция удаления (пометка томбстоуном).
type FugueDeleteOp struct {
	TargetID OpID // ID удаляемого узла
	SourceID OpID // ID самой операции
}

func (o FugueDeleteOp) OpType() string { return "fugue_delete" }
func (o FugueDeleteOp) ID() OpID       { return o.SourceID }

// rootSentinelID — ID корневого sentinel-узла.
var rootSentinelID = OpID{ReplicaID: "__root__", Counter: 0}

// FugueTree — основная структура Fugue CRDT.
// Хранит дерево символов и предоставляет методы для вставки, удаления и слияния.
type FugueTree struct {
	root      *FugueNode            // Корневой sentinel (не отображается в тексте)
	nodes     map[string]*FugueNode // Все узлы по ключу "replicaID:counter"
	replicaID string                // ID текущей реплики
	counter   uint64                // Локальный монотонный счётчик
	mu        sync.RWMutex
}

// NewFugueTree returns an empty FugueTree owned by replicaID.
//
// replicaID must be unique across all replicas of the same document — it
// becomes part of every [OpID] this replica generates and is what makes
// concurrent operations totally orderable. A common choice is a stable
// node identifier (hostname, container ID, or a UUID generated once per
// process).
func NewFugueTree(replicaID string) *FugueTree {
	root := &FugueNode{
		ID:    rootSentinelID,
		Value: 0, // sentinel не имеет значения
	}
	ft := &FugueTree{
		root:      root,
		nodes:     make(map[string]*FugueNode),
		replicaID: replicaID,
	}
	ft.nodes[rootSentinelID.String()] = root
	return ft
}

// nextID генерирует следующий уникальный OpID для этой реплики.
func (ft *FugueTree) nextID() OpID {
	ft.counter++
	return OpID{ReplicaID: ft.replicaID, Counter: ft.counter}
}

// getNode возвращает узел по ID. Nil, если не найден.
func (ft *FugueTree) getNode(id OpID) *FugueNode {
	return ft.nodes[id.String()]
}

// insertChild добавляет дочерний узел в отсортированный slice children.
func insertChild(children []*FugueNode, child *FugueNode) []*FugueNode {
	i := sort.Search(len(children), func(j int) bool {
		return children[j].ID.Compare(child.ID) >= 0
	})
	// Дубликат — не вставляем
	if i < len(children) && children[i].ID.Compare(child.ID) == 0 {
		return children
	}
	children = append(children, nil)
	copy(children[i+1:], children[i:])
	children[i] = child
	return children
}

// --- Обход дерева ---

// traverse выполняет in-order обход дерева и возвращает все узлы (включая deleted).
// Порядок: left children → node → right children (рекурсивно).
// Root sentinel исключается из результата.
func (ft *FugueTree) traverse() []*FugueNode {
	result := make([]*FugueNode, 0, len(ft.nodes))
	ft.traverseNode(ft.root, &result)
	return result
}

func (ft *FugueTree) traverseNode(node *FugueNode, result *[]*FugueNode) {
	// Left children
	for _, child := range node.LeftChildren {
		ft.traverseNode(child, result)
	}
	// Сам узел (кроме root sentinel)
	if node.ID.Compare(rootSentinelID) != 0 {
		*result = append(*result, node)
	}
	// Right children
	for _, child := range node.RightChildren {
		ft.traverseNode(child, result)
	}
}

// visibleNodes возвращает только неудалённые узлы в порядке документа.
func (ft *FugueTree) visibleNodes() []*FugueNode {
	all := ft.traverse()
	visible := make([]*FugueNode, 0, len(all))
	for _, n := range all {
		if !n.IsDeleted {
			visible = append(visible, n)
		}
	}
	return visible
}

// --- Локальные операции ---

// InsertAt inserts char at the visible position pos (0-indexed, where 0 is
// the start and Len() is the end) and returns the resulting [FugueInsertOp]
// for broadcast to other replicas.
//
// pos refers to the visible character offset, ignoring tombstones. Returns
// an error if pos is outside [0, Len()].
func (ft *FugueTree) InsertAt(pos int, char rune) (FugueInsertOp, error) {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	visible := ft.visibleNodes()

	if pos < 0 || pos > len(visible) {
		return FugueInsertOp{}, errors.New("insert position out of range")
	}

	// Определяем L (left origin) и R (right origin)
	var leftOrigin, rightOrigin *FugueNode

	if pos == 0 {
		leftOrigin = ft.root
	} else {
		leftOrigin = visible[pos-1]
	}

	if pos < len(visible) {
		rightOrigin = visible[pos]
	}

	// Fugue Parent Selection Rule
	parentID, side := ft.selectParent(leftOrigin, rightOrigin)

	newID := ft.nextID()
	op := FugueInsertOp{
		NodeID:   newID,
		Value:    char,
		ParentID: parentID,
		Side:     side,
	}

	ft.applyInsert(op)
	return op, nil
}

// DeleteAt marks the character at visible position pos as a tombstone and
// returns the resulting [FugueDeleteOp] for broadcast.
//
// The node is kept in the tree (the CRDT contract requires it for
// idempotent merges) but is excluded from [FugueTree.State] and from future
// position math. Physical removal happens later via epoch-based GC.
func (ft *FugueTree) DeleteAt(pos int) (FugueDeleteOp, error) {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	visible := ft.visibleNodes()

	if pos < 0 || pos >= len(visible) {
		return FugueDeleteOp{}, errors.New("delete position out of range")
	}

	target := visible[pos]
	op := FugueDeleteOp{
		TargetID: target.ID,
		SourceID: ft.nextID(),
	}

	ft.applyDelete(op)
	return op, nil
}

// --- Применение удалённых операций ---

// ApplyRemoteInsert applies an insert operation that arrived from another
// replica. Idempotent: applying the same op twice is a no-op.
func (ft *FugueTree) ApplyRemoteInsert(op FugueInsertOp) {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	ft.applyInsert(op)
}

// ApplyRemoteDelete applies a delete operation that arrived from another
// replica. Idempotent.
func (ft *FugueTree) ApplyRemoteDelete(op FugueDeleteOp) {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	ft.applyDelete(op)
}

// --- Внутренняя логика ---

// selectParent реализует Fugue Parent Selection Rule.
//
// Правило:
//   При вставке между L и R:
//   - Если R != nil И parent(R).ID == L.ID → новый узел = LEFT child of R
//   - Иначе → новый узел = RIGHT child of L
func (ft *FugueTree) selectParent(left, right *FugueNode) (OpID, FugueSide) {
	if right != nil && right.ParentID.Compare(left.ID) == 0 {
		return right.ID, FugueLeft
	}
	return left.ID, FugueRight
}

// applyInsert добавляет узел в дерево (без блокировки — вызывается из locked-методов).
func (ft *FugueTree) applyInsert(op FugueInsertOp) {
	key := op.NodeID.String()

	// Идемпотентность: если узел уже есть, пропускаем
	if _, exists := ft.nodes[key]; exists {
		return
	}

	parent := ft.getNode(op.ParentID)
	if parent == nil {
		return // Родитель не найден (нештатная ситуация)
	}

	node := &FugueNode{
		ID:       op.NodeID,
		Value:    op.Value,
		ParentID: op.ParentID,
		Side:     op.Side,
	}

	// Вставляем в отсортированный список children
	if op.Side == FugueLeft {
		parent.LeftChildren = insertChild(parent.LeftChildren, node)
	} else {
		parent.RightChildren = insertChild(parent.RightChildren, node)
	}

	ft.nodes[key] = node
}

// applyDelete помечает узел как удалённый (без блокировки).
func (ft *FugueTree) applyDelete(op FugueDeleteOp) {
	node := ft.getNode(op.TargetID)
	if node == nil {
		return
	}
	node.IsDeleted = true
}

// --- Публичные query-методы ---

// ToString returns the document text — visible characters in document order.
func (ft *FugueTree) ToString() string {
	ft.mu.RLock()
	defer ft.mu.RUnlock()

	var buf strings.Builder
	visible := ft.visibleNodes()
	buf.Grow(len(visible))
	for _, n := range visible {
		buf.WriteRune(n.Value)
	}
	return buf.String()
}

// Len returns the number of visible (non-tombstone) characters.
func (ft *FugueTree) Len() int {
	ft.mu.RLock()
	defer ft.mu.RUnlock()
	return len(ft.visibleNodes())
}

// --- Реализация CRDTNode[string] ---

// State возвращает текущий текст документа.
func (ft *FugueTree) State() string {
	return ft.ToString()
}

// ApplyOperation применяет FugueInsertOp или FugueDeleteOp.
func (ft *FugueTree) ApplyOperation(op Operation) error {
	switch typedOp := op.(type) {
	case FugueInsertOp:
		ft.ApplyRemoteInsert(typedOp)
		return nil
	case FugueDeleteOp:
		ft.ApplyRemoteDelete(typedOp)
		return nil
	default:
		return errors.New("unsupported operation type for FugueTree")
	}
}

// Merge pulls every node and tombstone from other into this tree, leaving
// both replicas convergent. other must be a [*FugueTree]; passing any other
// implementation of [CRDTNode] returns an error.
func (ft *FugueTree) Merge(other CRDTNode[string]) error {
	otherTree, ok := other.(*FugueTree)
	if !ok {
		return errors.New("cannot merge: incompatible CRDT type, expected *FugueTree")
	}

	otherTree.mu.RLock()
	// Собираем все операции из другого дерева
	var ops []FugueInsertOp
	var deletes []FugueDeleteOp
	for _, node := range otherTree.nodes {
		if node.ID.Compare(rootSentinelID) == 0 {
			continue
		}
		ops = append(ops, FugueInsertOp{
			NodeID:   node.ID,
			Value:    node.Value,
			ParentID: node.ParentID,
			Side:     node.Side,
		})
		if node.IsDeleted {
			deletes = append(deletes, FugueDeleteOp{TargetID: node.ID})
		}
	}
	otherTree.mu.RUnlock()

	ft.mu.Lock()
	defer ft.mu.Unlock()

	// Сортируем по Counter для правильного порядка применения
	// (родитель всегда имеет меньший Counter внутри одной реплики)
	sort.Slice(ops, func(i, j int) bool {
		return ops[i].NodeID.Counter < ops[j].NodeID.Counter
	})

	for _, op := range ops {
		ft.applyInsert(op)
	}
	for _, op := range deletes {
		ft.applyDelete(op)
	}

	return nil
}
