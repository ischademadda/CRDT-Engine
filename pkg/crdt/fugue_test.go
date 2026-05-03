package crdt

import (
	"testing"
)

// =====================================================================
// Базовые операции
// =====================================================================

func TestFugue_InsertSingleChar(t *testing.T) {
	tree := NewFugueTree("node-1")

	_, err := tree.InsertAt(0, 'A')
	if err != nil {
		t.Fatalf("InsertAt failed: %v", err)
	}

	if got := tree.ToString(); got != "A" {
		t.Errorf("ToString() = %q, want %q", got, "A")
	}
}

func TestFugue_InsertSequential(t *testing.T) {
	tree := NewFugueTree("node-1")

	for i, ch := range "Hello" {
		if _, err := tree.InsertAt(i, ch); err != nil {
			t.Fatalf("InsertAt(%d, %c) failed: %v", i, ch, err)
		}
	}

	if got := tree.ToString(); got != "Hello" {
		t.Errorf("ToString() = %q, want %q", got, "Hello")
	}
}

func TestFugue_InsertAtBeginning(t *testing.T) {
	tree := NewFugueTree("node-1")

	// Вставляем по одному символу в начало.
	// Fugue parent selection определяет структуру дерева:
	tree.InsertAt(0, 'A') // A = right child of root
	if got := tree.ToString(); got != "A" {
		t.Errorf("after 'A': got %q, want %q", got, "A")
	}

	tree.InsertAt(0, 'B') // B = left child of A (parent(A)=root=L)
	if got := tree.ToString(); got != "BA" {
		t.Errorf("after 'B' at 0: got %q, want %q", got, "BA")
	}

	// Вставка в конец — предсказуемо
	tree.InsertAt(2, 'C')
	if got := tree.ToString(); got != "BAC" {
		t.Errorf("after 'C' at end: got %q, want %q", got, "BAC")
	}
}

func TestFugue_InsertInMiddle(t *testing.T) {
	tree := NewFugueTree("node-1")

	// "AC" → вставляем 'B' в позицию 1 → "ABC"
	tree.InsertAt(0, 'A')
	tree.InsertAt(1, 'C')
	tree.InsertAt(1, 'B')

	if got := tree.ToString(); got != "ABC" {
		t.Errorf("ToString() = %q, want %q", got, "ABC")
	}
}

func TestFugue_Delete(t *testing.T) {
	tree := NewFugueTree("node-1")

	tree.InsertAt(0, 'A')
	tree.InsertAt(1, 'B')
	tree.InsertAt(2, 'C')

	if _, err := tree.DeleteAt(1); err != nil { // Удаляем 'B'
		t.Fatalf("DeleteAt failed: %v", err)
	}

	if got := tree.ToString(); got != "AC" {
		t.Errorf("ToString() = %q, want %q", got, "AC")
	}
}

func TestFugue_DeleteAll(t *testing.T) {
	tree := NewFugueTree("node-1")

	tree.InsertAt(0, 'A')
	tree.InsertAt(1, 'B')

	tree.DeleteAt(0)
	tree.DeleteAt(0) // Теперь 'B' на позиции 0

	if got := tree.ToString(); got != "" {
		t.Errorf("ToString() = %q, want empty", got)
	}
}

func TestFugue_Len(t *testing.T) {
	tree := NewFugueTree("node-1")

	if tree.Len() != 0 {
		t.Errorf("empty tree Len() = %d, want 0", tree.Len())
	}

	tree.InsertAt(0, 'A')
	tree.InsertAt(1, 'B')
	tree.InsertAt(2, 'C')
	tree.DeleteAt(1) // 'B'

	if tree.Len() != 2 {
		t.Errorf("Len() = %d, want 2", tree.Len())
	}
}

func TestFugue_InsertOutOfRange(t *testing.T) {
	tree := NewFugueTree("node-1")

	if _, err := tree.InsertAt(1, 'A'); err == nil {
		t.Error("expected error for out-of-range insert, got nil")
	}
	if _, err := tree.InsertAt(-1, 'A'); err == nil {
		t.Error("expected error for negative position, got nil")
	}
}

// =====================================================================
// Конкурентные вставки — ГЛАВНЫЙ ТЕСТ: отсутствие переплетения
// =====================================================================

func TestFugue_ConcurrentInsert_NoInterleaving(t *testing.T) {
	// Два пользователя одновременно вставляют текст в одну позицию.
	// YATA мог бы дать "HWeolrllod". Fugue гарантирует "HelloWorld" или "WorldHello".

	// Общее начальное состояние: пустой документ
	tree1 := NewFugueTree("node-1")
	tree2 := NewFugueTree("node-2")

	// Пользователь 1 печатает "Hello"
	var ops1 []FugueInsertOp
	for i, ch := range "Hello" {
		op, _ := tree1.InsertAt(i, ch)
		ops1 = append(ops1, op)
	}

	// Пользователь 2 печатает "World"
	var ops2 []FugueInsertOp
	for i, ch := range "World" {
		op, _ := tree2.InsertAt(i, ch)
		ops2 = append(ops2, op)
	}

	// Синхронизация: каждый применяет операции другого
	for _, op := range ops2 {
		tree1.ApplyRemoteInsert(op)
	}
	for _, op := range ops1 {
		tree2.ApplyRemoteInsert(op)
	}

	result1 := tree1.ToString()
	result2 := tree2.ToString()

	// 1. Оба дерева должны дать одинаковый результат (конвергенция)
	if result1 != result2 {
		t.Fatalf("convergence failed: tree1=%q, tree2=%q", result1, result2)
	}

	// 2. Результат должен быть "HelloWorld" ИЛИ "WorldHello" — НЕ переплетённый
	if result1 != "HelloWorld" && result1 != "WorldHello" {
		t.Errorf("INTERLEAVING DETECTED: got %q, expected 'HelloWorld' or 'WorldHello'", result1)
	}

	t.Logf("Concurrent insert result: %q (no interleaving ✓)", result1)
}

func TestFugue_ConcurrentInsert_SamePosition_ThreeUsers(t *testing.T) {
	// Три пользователя вставляют в одну позицию
	tree1 := NewFugueTree("alice")
	tree2 := NewFugueTree("bob")
	tree3 := NewFugueTree("carol")

	var ops1, ops2, ops3 []FugueInsertOp
	for i, ch := range "AA" {
		op, _ := tree1.InsertAt(i, ch)
		ops1 = append(ops1, op)
	}
	for i, ch := range "BB" {
		op, _ := tree2.InsertAt(i, ch)
		ops2 = append(ops2, op)
	}
	for i, ch := range "CC" {
		op, _ := tree3.InsertAt(i, ch)
		ops3 = append(ops3, op)
	}

	// Все применяют операции всех
	allOps := append(append(ops1, ops2...), ops3...)
	trees := []*FugueTree{tree1, tree2, tree3}

	for _, tree := range trees {
		for _, op := range allOps {
			tree.ApplyRemoteInsert(op) // Идемпотентно
		}
	}

	results := make([]string, 3)
	for i, tree := range trees {
		results[i] = tree.ToString()
	}

	// Все реплики должны сойтись
	if results[0] != results[1] || results[1] != results[2] {
		t.Fatalf("convergence failed: %q, %q, %q", results[0], results[1], results[2])
	}

	// Проверяем: нет переплетения — каждая подстрока "AA", "BB", "CC" непрерывна
	result := results[0]
	for _, sub := range []string{"AA", "BB", "CC"} {
		if !containsSubstring(result, sub) {
			t.Errorf("INTERLEAVING: %q does not contain contiguous %q", result, sub)
		}
	}

	t.Logf("Three-user concurrent result: %q (no interleaving ✓)", result)
}

// =====================================================================
// CRDT-свойства
// =====================================================================

func TestFugue_Merge_Convergence(t *testing.T) {
	tree1 := NewFugueTree("node-1")
	tree2 := NewFugueTree("node-2")

	// Каждый вставляет свой текст
	for i, ch := range "AB" {
		tree1.InsertAt(i, ch)
	}
	for i, ch := range "CD" {
		tree2.InsertAt(i, ch)
	}

	// State-based merge
	tree1.Merge(tree2)
	tree2.Merge(tree1)

	if tree1.ToString() != tree2.ToString() {
		t.Errorf("convergence failed after merge: %q != %q", tree1.ToString(), tree2.ToString())
	}
}

func TestFugue_Merge_Idempotency(t *testing.T) {
	tree1 := NewFugueTree("node-1")
	tree2 := NewFugueTree("node-2")

	for i, ch := range "Hello" {
		tree1.InsertAt(i, ch)
	}
	for i, ch := range "World" {
		tree2.InsertAt(i, ch)
	}

	tree1.Merge(tree2)
	before := tree1.ToString()

	tree1.Merge(tree2) // Повторный merge
	tree1.Merge(tree2) // И ещё

	if got := tree1.ToString(); got != before {
		t.Errorf("idempotency violated: before=%q, after=%q", before, got)
	}
}

func TestFugue_Merge_Commutativity(t *testing.T) {
	makeTree := func(id string, text string) *FugueTree {
		tree := NewFugueTree(id)
		for i, ch := range text {
			tree.InsertAt(i, ch)
		}
		return tree
	}

	// merge(A,B) == merge(B,A)
	a1 := makeTree("node-1", "AB")
	b1 := makeTree("node-2", "CD")
	a1.Merge(b1)

	a2 := makeTree("node-1", "AB")
	b2 := makeTree("node-2", "CD")
	b2.Merge(a2)

	if a1.ToString() != b2.ToString() {
		t.Errorf("commutativity violated: merge(A,B)=%q, merge(B,A)=%q",
			a1.ToString(), b2.ToString())
	}
}

// =====================================================================
// Remote Operations (CmRDT)
// =====================================================================

func TestFugue_ApplyOperation_Insert(t *testing.T) {
	tree := NewFugueTree("node-1")

	op := FugueInsertOp{
		NodeID:   OpID{ReplicaID: "node-2", Counter: 1},
		Value:    'X',
		ParentID: rootSentinelID,
		Side:     FugueRight,
	}

	if err := tree.ApplyOperation(op); err != nil {
		t.Fatalf("ApplyOperation failed: %v", err)
	}

	if got := tree.ToString(); got != "X" {
		t.Errorf("ToString() = %q, want %q", got, "X")
	}
}

func TestFugue_ApplyOperation_Delete(t *testing.T) {
	tree := NewFugueTree("node-1")
	insertOp, _ := tree.InsertAt(0, 'A')

	deleteOp := FugueDeleteOp{
		TargetID: insertOp.NodeID,
		SourceID: OpID{ReplicaID: "node-2", Counter: 1},
	}

	if err := tree.ApplyOperation(deleteOp); err != nil {
		t.Fatalf("ApplyOperation failed: %v", err)
	}

	if got := tree.ToString(); got != "" {
		t.Errorf("ToString() = %q, want empty after delete", got)
	}
}

func TestFugue_ApplyOperation_Idempotent(t *testing.T) {
	tree := NewFugueTree("node-1")

	op := FugueInsertOp{
		NodeID:   OpID{ReplicaID: "node-2", Counter: 1},
		Value:    'X',
		ParentID: rootSentinelID,
		Side:     FugueRight,
	}

	tree.ApplyOperation(op)
	tree.ApplyOperation(op) // Повторное применение
	tree.ApplyOperation(op) // И ещё

	if got := tree.ToString(); got != "X" {
		t.Errorf("idempotent insert failed: got %q, want %q", got, "X")
	}
}

// =====================================================================
// Удаление + вставка в конкурентном сценарии
// =====================================================================

func TestFugue_ConcurrentDeleteAndInsert(t *testing.T) {
	// Начальное состояние: "ABC"
	tree1 := NewFugueTree("node-1")
	tree2 := NewFugueTree("node-2")

	var insertOps []FugueInsertOp
	for i, ch := range "ABC" {
		op, _ := tree1.InsertAt(i, ch)
		insertOps = append(insertOps, op)
	}
	// Синхронизируем tree2
	for _, op := range insertOps {
		tree2.ApplyRemoteInsert(op)
	}

	// tree1 удаляет 'B' (позиция 1)
	delOp, _ := tree1.DeleteAt(1)

	// tree2 вставляет 'X' после 'B' (позиция 2)
	insOp, _ := tree2.InsertAt(2, 'X')

	// Синхронизация
	tree1.ApplyRemoteInsert(insOp)
	tree2.ApplyRemoteDelete(delOp)

	// Оба должны сойтись
	if tree1.ToString() != tree2.ToString() {
		t.Errorf("convergence failed: tree1=%q, tree2=%q", tree1.ToString(), tree2.ToString())
	}

	// 'B' удалён, 'X' вставлен → "AXC"
	if got := tree1.ToString(); got != "AXC" {
		t.Errorf("expected 'AXC', got %q", got)
	}
}

// =====================================================================
// Helpers
// =====================================================================

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
