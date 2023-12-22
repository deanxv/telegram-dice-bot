package bot

import "sync"

const (
	ChatGroupUserLockKey = "%v_%v"
)

// 在包级别定义一个映射，存储每个userID对应的互斥锁
var userLocks = make(map[string]*sync.Mutex)
var userLocksMutex sync.Mutex

var chatLocks = make(map[string]*sync.Mutex)
var chatLocksMutex sync.Mutex

// getUserLock 根据userID获取对应的互斥锁，如果不存在则创建一个新的锁
func getUserLock(userID string) *sync.Mutex {
	userLocksMutex.Lock()
	defer userLocksMutex.Unlock()

	if _, ok := userLocks[userID]; !ok {
		userLocks[userID] = &sync.Mutex{}
	}

	return userLocks[userID]
}

// getUserLock 根据userID获取对应的互斥锁，如果不存在则创建一个新的锁
func getChatLock(chatId string) *sync.Mutex {
	chatLocksMutex.Lock()
	defer chatLocksMutex.Unlock()

	if _, ok := userLocks[chatId]; !ok {
		chatLocks[chatId] = &sync.Mutex{}
	}

	return chatLocks[chatId]
}
