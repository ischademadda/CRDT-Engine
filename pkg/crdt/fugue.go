package crdt

import (
	"errors"
	"sort"
	"strings"
	"sync"
)

// FugueTree — sequence CRDT для совместного редактирования текста.
//
// Алгоритм Fugue гарантирует максимальное отсутствие переплетения (maximal non-interleaving):
// при конкурентной вставке двумя пользователями в одну позицию их тексты не перемешиваются.
//
// Структура: дерево, порядок документа определяется in-order обходом:
//
//	left children → node → right children
//
// Правило выбора родителя (Fugue Parent Selection):
//
//	При вставке между L и R:
//	- Если R != nil И parent(R) == L → новый узел = left child of R
//	- Иначе → новый узел = right child of L

// FugueSide — сторона дочернего узла (left или right child).
type FugueSide int

const (
	FugueLeft  FugueSide = iota // появляется ДО родителя в обходе
	FugueRight                  // появляется ПОСЛЕ родителя в обходе
)

// FugueNode — один символ документа в дереве Fugue.
type FugueNode struct {
	ID            OpID
	Value         rune
	ParentID      OpID
	Side          FugueSide
	IsDeleted     bool         // томбстоун: символ удалён, но узел хранится для CRDT-логики
	LeftChildren  []*FugueNode // отсортированы по ID для детерминированного порядка
	RightChildren []*FugueNode
}

// FugueInsertOp — операция вставки символа.
type FugueInsertOp struct {
	NodeID   OpID
	Value    rune
	ParentID OpID
	Side     FugueSide
}

func (o FugueInsertOp) OpType() string { return "fugue_insert" }
func (o FugueInsertOp) ID() OpID       { return o.NodeID }

// FugueDeleteOp — операция удаления (пометка томбстоуном).
type FugueDeleteOp struct {
	TargetID OpID
	SourceID OpID
}

func (o FugueDeleteOp) OpType() string { return "fugue_delete" }
func (o FugueDeleteOp) ID() OpID       { return o.SourceID }

var rootSentinelID = OpID{ReplicaID: "__root__", Counter: 0}

// FugueTree хранит дерево символов и предоставляет методы для вставки, удаления и слияния.
type FugueTree struct {
	root      *FugueNode
	nodes     map[string]*FugueNode
	replicaID string
	counter   uint64
	mu        sync.RWMutex
}

// NewFugueTree создаёт пустое дерево для реплики replicaID.
// replicaID должен быть уникальным среди всех реплик документа.
func NewFugueTree(replicaID string) *FugueTree {
	root := &FugueNode{ID: rootSentinelID}
	ft := &FugueTree{
		root:      root,
		nodes:     make(map[string]*FugueNode),
		replicaID: replicaID,
	}
	ft.nodes[rootSentinelID.String()] = root
	return ft
}

func (ft *FugueTree) nextID() OpID {
	ft.counter++
	return OpID{ReplicaID: ft.replicaID, Counter: ft.counter}
}

func (ft *FugueTree) getNode(id OpID) *FugueNode {
	return ft.nodes[id.String()]
}

// insertChild добавляет дочерний узел в отсортированный по ID slice.
func insertChild(children []*FugueNode, child *FugueNode) []*FugueNode {
	i := sort.Search(len(children), func(j int) bool {
		return children[j].ID.Compare(child.ID) >= 0
	})
	if i < len(children) && children[i].ID.Compare(child.ID) == 0 {
		return children // дубликат
	}
	children = append(children, nil)
	copy(children[i+1:], children[i:])
	children[i] = child
	return children
}

// traverse — in-order обход: left → node → right. Root sentinel исключается.
func (ft *FugueTree) traverse() []*FugueNode {
	result := make([]*FugueNode, 0, len(ft.nodes))
	ft.traverseNode(ft.root, &result)
	return result
}

func (ft *FugueTree) traverseNode(node *FugueNode, result *[]*FugueNode) {
	for _, child := range node.LeftChildren {
		ft.traverseNode(child, result)
	}
	if node.ID.Compare(rootSentinelID) != 0 {
		*result = append(*result, node)
	}
	for _, child := range node.RightChildren {
		ft.traverseNode(child, result)
	}
}

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

// InsertAt вставляет символ на видимую позицию pos (0-indexed).
// Возвращает операцию для рассылки другим репликам.
func (ft *FugueTree) InsertAt(pos int, char rune) (FugueInsertOp, error) {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	visible := ft.visibleNodes()
	if pos < 0 || pos > len(visible) {
		return FugueInsertOp{}, errors.New("insert position out of range")
	}

	var leftOrigin, rightOrigin *FugueNode
	if pos == 0 {
		leftOrigin = ft.root
	} else {
		leftOrigin = visible[pos-1]
	}
	if pos < len(visible) {
		rightOrigin = visible[pos]
	}

	parentID, side := ft.selectParent(leftOrigin, rightOrigin)
	newID := ft.nextID()
	op := FugueInsertOp{NodeID: newID, Value: char, ParentID: parentID, Side: side}
	ft.applyInsert(op)
	return op, nil
}

// DeleteAt помечает символ на позиции pos томбстоуном.
// Узел остаётся в дереве — физическое удаление через Epoch GC (запланировано).
func (ft *FugueTree) DeleteAt(pos int) (FugueDeleteOp, error) {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	visible := ft.visibleNodes()
	if pos < 0 || pos >= len(visible) {
		return FugueDeleteOp{}, errors.New("delete position out of range")
	}

	target := visible[pos]
	op := FugueDeleteOp{TargetID: target.ID, SourceID: ft.nextID()}
	ft.applyDelete(op)
	return op, nil
}

// InsertAtWithReplica вставляет символ на видимую позицию pos (0-indexed) от имени конкретной реплики.
func (ft *FugueTree) InsertAtWithReplica(pos int, char rune, replicaID string) (FugueInsertOp, error) {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	visible := ft.visibleNodes()
	if pos < 0 || pos > len(visible) {
		return FugueInsertOp{}, errors.New("insert position out of range")
	}

	var leftOrigin, rightOrigin *FugueNode
	if pos == 0 {
		leftOrigin = ft.root
	} else {
		leftOrigin = visible[pos-1]
	}
	if pos < len(visible) {
		rightOrigin = visible[pos]
	}

	parentID, side := ft.selectParent(leftOrigin, rightOrigin)
	ft.counter++
	newID := OpID{ReplicaID: replicaID, Counter: ft.counter}
	op := FugueInsertOp{NodeID: newID, Value: char, ParentID: parentID, Side: side}
	ft.applyInsert(op)
	return op, nil
}

// DeleteAtWithReplica помечает символ на позиции pos томбстоуном от имени конкретной реплики.
func (ft *FugueTree) DeleteAtWithReplica(pos int, replicaID string) (FugueDeleteOp, error) {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	visible := ft.visibleNodes()
	if pos < 0 || pos >= len(visible) {
		return FugueDeleteOp{}, errors.New("delete position out of range")
	}

	target := visible[pos]
	ft.counter++
	deleteID := OpID{ReplicaID: replicaID, Counter: ft.counter}
	op := FugueDeleteOp{TargetID: target.ID, SourceID: deleteID}
	ft.applyDelete(op)
	return op, nil
}

// ApplyRemoteInsert применяет операцию вставки от другой реплики. Идемпотентен.
func (ft *FugueTree) ApplyRemoteInsert(op FugueInsertOp) {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	ft.applyInsert(op)
}

// ApplyRemoteDelete применяет операцию удаления от другой реплики. Идемпотентен.
func (ft *FugueTree) ApplyRemoteDelete(op FugueDeleteOp) {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	ft.applyDelete(op)
}

// selectParent реализует Fugue Parent Selection Rule.
func (ft *FugueTree) selectParent(left, right *FugueNode) (OpID, FugueSide) {
	if right != nil && right.ParentID.Compare(left.ID) == 0 {
		return right.ID, FugueLeft
	}
	return left.ID, FugueRight
}

func (ft *FugueTree) applyInsert(op FugueInsertOp) {
	key := op.NodeID.String()
	if _, exists := ft.nodes[key]; exists {
		return // идемпотентность
	}
	parent := ft.getNode(op.ParentID)
	if parent == nil {
		return
	}
	node := &FugueNode{
		ID:       op.NodeID,
		Value:    op.Value,
		ParentID: op.ParentID,
		Side:     op.Side,
	}
	if op.Side == FugueLeft {
		parent.LeftChildren = insertChild(parent.LeftChildren, node)
	} else {
		parent.RightChildren = insertChild(parent.RightChildren, node)
	}
	ft.nodes[key] = node
}

func (ft *FugueTree) applyDelete(op FugueDeleteOp) {
	if node := ft.getNode(op.TargetID); node != nil {
		node.IsDeleted = true
	}
}

// ToString возвращает текст документа — видимые символы в порядке документа.
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

// Len возвращает число видимых (не-томбстоун) символов.
func (ft *FugueTree) Len() int {
	ft.mu.RLock()
	defer ft.mu.RUnlock()
	return len(ft.visibleNodes())
}

func (ft *FugueTree) State() string { return ft.ToString() }

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

// Merge сливает все узлы другого дерева в текущее. Обе реплики сходятся к одному состоянию.
func (ft *FugueTree) Merge(other CRDTNode[string]) error {
	otherTree, ok := other.(*FugueTree)
	if !ok {
		return errors.New("cannot merge: incompatible CRDT type, expected *FugueTree")
	}

	otherTree.mu.RLock()
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

	// Сортируем по Counter: родитель всегда имеет меньший Counter внутри реплики.
	sort.Slice(ops, func(i, j int) bool {
		return ops[i].NodeID.Counter < ops[j].NodeID.Counter
	})

	ft.mu.Lock()
	defer ft.mu.Unlock()
	for _, op := range ops {
		ft.applyInsert(op)
	}
	for _, op := range deletes {
		ft.applyDelete(op)
	}
	return nil
}
