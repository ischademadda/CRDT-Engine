package main

import "fmt"

type SessionManager struct {
	active map[string]string
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		active: make(map[string]string),
	}
}

func (sm *SessionManager) Add(userID, ip string) {
	sm.active[userID] = ip

}

func (sm *SessionManager) Remove(userID string) {
	delete(sm.active, userID)
}

func (sm *SessionManager) GetIP(userID string) (string, error) {
	ip, exists := sm.active[userID]
	if !exists {
		return "", fmt.Errorf("user not found")
	}
	return ip, nil

}

func main() {
	sm := NewSessionManager()
	sm.Add("user-1","192.123.12.1")
	sm.Add("user-3","223.45.12.2")

	ip, err := sm.GetIP("user-2")
	if err != nil {
		fmt.Println("Ошибка:", err)
	} else {
		fmt.Println("IP:", ip)
	}
}
