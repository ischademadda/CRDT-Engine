package crdt

import(
	"fmt"
)

func ParseDelta(payload any){
	switch v := payload.(type){
	case string:
		fmt.Printf("Добавлен один элемент: %s\n", v)
	case []string:
		fmt.Printf("Добавлен пакет из %d элементов\n", len(v))
	case int:
		fmt.Printf("Ошибка: дельта не может быть числом\n")
	default:
		fmt.Println("Неизвестный формат данных")
	}
}
